package sharkrabbitmq

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/howeyc/crc16"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type Config struct {
	Host     []string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地址
	User     string   `json:"user" yaml:"user" mapstructure:"user"`             // 连接用户名
	Password string   `json:"password" yaml:"password" mapstructure:"password"` // 连接密码
}

type Client struct {
	config   *Config
	wg       *sync.WaitGroup
	ctx      context.Context
	logger   *zap.Logger
	conn     *amqp.Connection
	connLock sync.Mutex
	channel  []*amqp.Channel
	publish  chan publishMsg
	name     string
	id       string
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
	client.publish = make(chan publishMsg, 10000)
	innerwg := &sync.WaitGroup{}
	innerwg.Add(1)
	go client.connect(int(index), innerwg)
	innerwg.Wait()
	wg.Add(1)
	go client.publis_msg()
	return client, nil
}

func (c *Client) connect(index int, wg *sync.WaitGroup) {
	count := 0
	for {
		amqpurl := "amqp://" + c.config.User + ":" + c.config.Password + "@" + c.config.Host[index]
		conn, err := amqp.Dial(amqpurl)
		if err != nil {
			c.logger.Error("连接Rabbitmq失败", zap.String("host", c.config.Host[index]), zap.Error(err))
			time.Sleep(1 * time.Second)
			continue
		}
		c.connLock.Lock()
		c.conn = conn
		for i := 0; i < 5; i++ {
			ch, _ := conn.Channel()
			if ch != nil {
				c.channel = append(c.channel, ch)
			}
		}
		c.connLock.Unlock()
		if count > 0 {
			c.logger.Info("重连Rabbitmq成功", zap.String("host", c.config.Host[index]))
		} else {
			wg.Done()
		}
		connErr := make(chan *amqp.Error, 1)
		conn.NotifyClose(connErr)
		e := <-connErr
		c.logger.Warn("Rabbitmq连接已关闭", zap.String("host", c.config.Host[index]), zap.Error(e))
		c.connLock.Lock()
		c.conn.Close()
		c.conn = nil
		c.channel = nil
		c.connLock.Unlock()
		count++
	}
}

func (c *Client) publis_msg() {
	defer c.wg.Done()
	index := 0
	for {
		select {
		case msg, ok := <-c.publish:
			if !ok {
				return
			}
		loop:
			for {
				select {
				case <-c.ctx.Done():
					return
				default:
					index++
					index = index % len(c.channel)
					var ch *amqp.Channel
					c.connLock.Lock()
					if len(c.channel) > 0 {
						ch = c.channel[index]
					}
					c.connLock.Unlock()
					if ch == nil {
						time.Sleep(1 * time.Second)
						continue
					}
					err := ch.Publish(msg.exchange, msg.key, false, false, *msg.value)
					if err != nil {
						time.Sleep(1 * time.Second)
						continue
					}
					break loop
				}
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// Consume 消息消费，exchange、queue、key 由调用方指定，handler 是处理消息的回调函数。
// 消息处理失败时，handler 可以选择不 ack 消息，RabbitMQ 会重新投递该消息给其他消费者（或同一消费者的下一次消费），直到消息被成功处理并 ack。
// 注意：handler 内部应该捕获异常，避免 panic 导致消费者崩溃。
func (c *Client) Consume(exchange string, queue string, key string, handler func(*amqp.Delivery)) {
	safeHandler := func(msg *amqp.Delivery) {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Error("Rabbitmq消费者处理消息panic", zap.Any("r", r), zap.String("stack", string(debug.Stack())))
			}
		}()
		handler(msg)
	}
	go func() {
		for {
			c.connLock.Lock()
			conn := c.conn
			c.connLock.Unlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			channel, err := conn.Channel()
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			channel.Qos(10000, 0, false)
			channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
			channel.QueueDeclare(queue, true, false, false, false, nil)
			channel.QueueBind(queue, key, exchange, false, nil)
			ch, err := channel.Consume(queue, fmt.Sprintf("%v.%v", c.name, c.id), false, false, false, false, nil)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
		loop:
			for {
				select {
				case <-c.ctx.Done():
					break loop
				case msg, ok := <-ch:
					if !ok {
						break loop
					}
					safeHandler(&msg)
				}
			}
			channel.Close()
		}
	}()
}

// Publish 发布消息，exchange 和 key 由调用方指定，value 可以是 string、[]byte 或任意结构体
// string,[]byte 必须是 json 结构
func (c *Client) Publish(exchange string, key string, value any) {
	var body []byte
	switch v := value.(type) {
	case string:
		body = []byte(v)
	case []byte:
		body = v
	default:
		body, _ = sonic.Marshal(value)
	}
	c.publish <- publishMsg{
		exchange: exchange,
		key:      key,
		value: &amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		},
	}
}

// DeleteQueue 删除队列,未消费的消息会被删除
func (c *Client) DeleteQueue(queue string) {
	for {
		c.connLock.Lock()
		conn := c.conn
		c.connLock.Unlock()
		if conn == nil {
			time.Sleep(1 * time.Second)
			continue
		}
		channel, err := c.conn.Channel()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		channel.QueueDelete(queue, false, false, false)
	}
}
