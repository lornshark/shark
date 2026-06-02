package sharkrabbitmq

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/howeyc/crc16"
	"github.com/lornshark/shark/sharkfunc"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// 投递语义说明：
// - 尽力而为（Best effort）投递
// - 当发布缓冲区已满时，消息可能被丢弃
// - 在优雅关闭过程中，消息可能被丢弃
// - 不对“发送过程中（in-flight）消息”提供持久化保障
// - 消费端为至少一次投递（at-least-once），业务必须保证幂等性

type Config struct {
	Host     []string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地址
	User     string   `json:"user" yaml:"user" mapstructure:"user"`             // 连接用户名
	Password string   `json:"password" yaml:"password" mapstructure:"password"` // 连接密码
}

type Client struct {
	config    *Config
	wg        *sync.WaitGroup
	ctx       context.Context
	logger    *zap.Logger
	conn      *amqp.Connection
	connLock  sync.Mutex
	channel   *amqp.Channel
	publish   chan publishMsg
	name      string
	id        string
	closeLock sync.Mutex
}

type publishMsg struct {
	exchange string
	key      string
	value    *amqp.Publishing
}

func New(ctx context.Context, logger *zap.Logger, wg *sync.WaitGroup, config *Config, name string, id string) (*Client, error) {
	index := crc16.Checksum([]byte(name), crc16.IBMTable)
	index = index % uint16(len(config.Host))
	client := &Client{
		config: config,
		ctx:    ctx,
		wg:     wg,
		logger: logger,
		name:   name,
		id:     id,
	}
	client.publish = make(chan publishMsg, 100000)
	innerwg := &sync.WaitGroup{}
	innerwg.Add(1)
	go client.connect(int(index), innerwg)
	innerwg.Wait()
	wg.Add(1)
	go client.publish_msg()
	go client.exit_waiting()
	return client, nil
}

func (c *Client) exit_waiting() {
	<-c.ctx.Done()
	c.closeLock.Lock()
	defer c.closeLock.Unlock()
	close(c.publish)
}

func (c *Client) get_conn() *amqp.Connection {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	conn := c.conn
	return conn
}

func (c *Client) get_channel() *amqp.Channel {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	channel := c.channel
	return channel
}

func (c *Client) set_conn(conn *amqp.Connection) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	if c.conn != nil && conn == nil {
		c.conn.Close()
	}
	c.conn = conn
}

func (c *Client) set_channel(channel *amqp.Channel) {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	if c.channel != nil && channel == nil {
		c.channel.Close()
	}
	c.channel = channel
}

// connect 连接 Rabbitmq，连接断开时会自动重连
func (c *Client) connect(index int, wg *sync.WaitGroup) {
	count := 0
	for {
		if c.ctx.Err() != nil {
			break
		}
		amqpurl := "amqp://" + c.config.User + ":" + c.config.Password + "@" + c.config.Host[index]
		conn, err := amqp.Dial(amqpurl)
		if err != nil {
			c.logger.Error("连接Rabbitmq失败", zap.String("host", c.config.Host[index]), zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		channel, err := conn.Channel()
		if err != nil {
			c.logger.Error("创建Rabbitmq Channel失败", zap.String("host", c.config.Host[index]), zap.Error(err))
			conn.Close()
			time.Sleep(time.Second)
			continue
		}
		c.set_conn(conn)
		c.set_channel(channel)
		if count > 0 {
			c.logger.Info("重连Rabbitmq成功", zap.String("host", c.config.Host[index]))
		} else {
			wg.Done()
		}
		connErr := make(chan *amqp.Error, 1)
		conn.NotifyClose(connErr)
		select {
		case <-c.ctx.Done():
			conn.Close()
			return
		case e := <-connErr:
			c.logger.Warn("Rabbitmq连接已关闭", zap.String("host", c.config.Host[index]), zap.Error(e))
			c.set_channel(nil)
			c.set_conn(nil)
			count++
		}
	}
}

// 发布消息
func (c *Client) publish_msg() {
	defer c.wg.Done()
	for msg := range c.publish {
		for {
			ch := c.get_channel()
			if c.ctx.Err() != nil && ch == nil {
				// 如果上下文已关闭且没有可用的连接,丢掉消息退出
				c.logger.Error("消息丢失: Rabbitmq连接已关闭且上下文已结束", zap.String("exchange", msg.exchange), zap.String("key", msg.key), zap.ByteString("value", msg.value.Body))
				break
			}
			if ch == nil {
				time.Sleep(time.Second)
				continue
			}
			err := ch.Publish(msg.exchange, msg.key, false, false, *msg.value)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			break
		}
	}
}

// Consume 消息消费，exchange、queue、key 由调用方指定，handler 是处理消息的回调函数。handler 需要负责消息 ack
// 消息处理失败时，handler 可以选择不 ack 消息，RabbitMQ 会重新投递该消息给其他消费者（或同一消费者的下一次消费），直到消息被成功处理并 ack。
// 注意：handler 内部应该捕获异常，避免 panic 导致消费者崩溃。
func (c *Client) Consume(exchange string, queue string, key string, handler func(amqp.Delivery)) {
	go func() {
		for {
			if c.ctx.Err() != nil {
				return
			}
			conn := c.get_conn()
			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			channel, err := conn.Channel()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			channel.Qos(10000, 0, false)
			channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
			channel.QueueDeclare(queue, true, false, false, false, nil)
			channel.QueueBind(queue, key, exchange, false, nil)
			data, err := channel.Consume(queue, fmt.Sprintf("%v.%v", c.name, c.id), false, false, false, false, nil)
			if err != nil {
				time.Sleep(time.Second)
			} else {
				c.handle_channel(data, handler)
			}
			channel.Close()
		}
	}()
}

func (c *Client) self_handle(msg amqp.Delivery, handler func(amqp.Delivery)) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("Rabbitmq消费者处理消息panic", zap.Any("r", r), zap.String("stack", string(debug.Stack())))
		}
	}()
	handler(msg)
}

func (c *Client) handle_channel(channel <-chan amqp.Delivery, handler func(amqp.Delivery)) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-channel:
			if !ok {
				return
			}
			c.self_handle(msg, handler)
		}
	}
}

// BatchConsume 批量处理消息,handler 返回false或panic停止处理,且不会 ack 消息
// handler 不能 ack 消息，BatchConsume 内部会在 handler 返回 true 时统一 ack 消息
func (c *Client) BatchConsume(exchange string, queue string, key string, handler func([]amqp.Delivery) bool) {
	go func() {
		for {
			if c.ctx.Err() != nil {
				return
			}
			conn := c.get_conn()
			if conn == nil {
				time.Sleep(time.Second)
				continue
			}
			channel, err := conn.Channel()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			channel.Qos(10000, 0, false)
			channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
			channel.QueueDeclare(queue, true, false, false, false, nil)
			channel.QueueBind(queue, key, exchange, false, nil)
			dataChannel, err := channel.Consume(queue, fmt.Sprintf("%v.%v", c.name, c.id), false, false, false, false, nil)
			if err != nil {
				channel.Close()
				time.Sleep(time.Second)
				continue
			}
			ctx, cancel := context.WithCancel(c.ctx)
			safeHandler := func(msgs []amqp.Delivery) (result bool) {
				defer func() {
					if r := recover(); r != nil {
						c.logger.Error("Rabbitmq消费者处理消息panic", zap.Any("r", r), zap.String("stack", string(debug.Stack())))
						result = false
					}
				}()
				return handler(msgs)
			}
			for {
				messages := sharkfunc.DrainChannelN(ctx, dataChannel, 5000)
				if len(messages) == 0 && ctx.Err() != nil {
					cancel()
					break
				}
				result := safeHandler(messages)
				if !result {
					cancel()
					break
				}
				for _, msg := range messages {
					msg.Ack(false)
				}
			}
			cancel()
			channel.Close()
		}
	}()
}

// Publish 发布消息，exchange 和 key 由调用方指定，value 可以是 string、[]byte 或任意结构体
// string,[]byte 必须是 json 结构,消息体最大支持10KB
func (c *Client) Publish(exchange string, key string, value any) error {
	var body []byte
	switch v := value.(type) {
	case string:
		body = []byte(v)
	case []byte:
		body = v
	default:
		body, _ = sonic.Marshal(value)
	}
	if len(body) > 1024*10 {
		return fmt.Errorf("消息体过大，最大支持10KB")
	}
	msg := publishMsg{
		exchange: exchange,
		key:      key,
		value: &amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		},
	}
	if c.ctx.Err() != nil {
		return fmt.Errorf("client is closed")
	}
	c.closeLock.Lock()
	defer c.closeLock.Unlock()
	select {
	case c.publish <- msg:
		return nil
	default:
		return fmt.Errorf("publish channel is full")
	}
}

// DeleteQueue 删除队列,未消费的消息会被删除
func (c *Client) DeleteQueue(queue string) {
	for {
		if c.ctx.Err() != nil {
			return
		}
		conn := c.get_conn()
		if conn == nil {
			time.Sleep(time.Second)
			continue
		}
		channel, err := conn.Channel()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		channel.QueueDelete(queue, false, false, false)
		channel.Close()
		break
	}
}
