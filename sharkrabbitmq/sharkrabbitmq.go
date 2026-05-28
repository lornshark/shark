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
	Host     []string `json:"host"`     // 连接地址
	User     string   `json:"user"`     // 连接用户名
	Password string   `json:"password"` // 连接密码
	Name     string   `json:"name"`     // 工程名称
	Id       string   `json:"id"`       // 实例Id
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
}

type publishMsg struct {
	exchange string
	key      string
	value    *amqp.Publishing
}

func New(ctx context.Context, logger *zap.Logger, wg *sync.WaitGroup, config *Config) (*Client, error) {
	index := crc16.Checksum([]byte(config.Name), crc16.IBMTable)
	index = index % uint16(len(config.Host))
	config.Host = []string{config.Host[index]}
	client := &Client{
		config: config,
		ctx:    ctx,
		wg:     wg,
		logger: logger,
	}
	client.publish = make(chan publishMsg, 10000)
	innerwg := &sync.WaitGroup{}
	innerwg.Add(1)
	go client.connect(int(index), innerwg)
	wg.Add(1)
	go client.publis_msg()
	innerwg.Wait()
	return client, nil
}

func (c *Client) connect(index int, wg *sync.WaitGroup) {
	connected := false
	for {
		c.connLock.Lock()
		amqpurl := "amqp://" + c.config.User + ":" + c.config.Password + "@" + c.config.Host[index]
		conn, err := amqp.Dial(amqpurl)
		if err != nil {
			c.logger.Error("连接Rabbitmq失败", zap.String("host", c.config.Host[index]), zap.Error(err))
			c.connLock.Unlock()
			time.Sleep(1 * time.Second)
			continue
		}
		for i := 0; i < 5; i++ {
			ch, _ := conn.Channel()
			if ch != nil {
				c.channel = append(c.channel, ch)
			}
		}
		c.conn = conn
		c.logger.Info("成功连接Rabbitmq", zap.String("host", c.config.Host[index]))
		connErr := make(chan *amqp.Error, 1)
		conn.NotifyClose(connErr)
		c.connLock.Unlock()
		if !connected {
			connected = true
			wg.Done()
		}
		e := <-connErr
		c.connLock.Lock()
		c.logger.Warn("Rabbitmq连接已关闭", zap.String("host", c.config.Host[index]), zap.Error(e))
		c.conn.Close()
		c.conn = nil
		c.channel = nil
		c.connLock.Unlock()
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
			for {
				c.connLock.Lock()
				if c.conn == nil || len(c.channel) == 0 {
					c.connLock.Unlock()
					time.Sleep(1 * time.Second)
					continue
				}
				index++
				index = index % len(c.channel)
				channel := c.channel[index]
				err := channel.Publish(msg.exchange, msg.key, false, false, *msg.value)
				if err != nil {
					c.connLock.Unlock()
					time.Sleep(1 * time.Second)
					continue
				}
				c.connLock.Unlock()
				break
			}
		case <-c.ctx.Done():
			return
		}
	}
}

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
			if conn == nil {
				c.connLock.Unlock()
				time.Sleep(1 * time.Second)
				continue
			}
			channel, _ := conn.Channel()
			if channel == nil {
				c.connLock.Unlock()
				time.Sleep(1 * time.Second)
				continue
			}
			channel.Qos(10000, 0, false)
			channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
			channel.QueueDeclare(queue, true, false, false, false, nil)
			channel.QueueBind(queue, key, exchange, false, nil)
			ch, err := channel.Consume(queue, fmt.Sprintf("%v.%v", c.config.Name, c.config.Id), false, false, false, false, nil)
			if err != nil {
				c.connLock.Unlock()
				time.Sleep(1 * time.Second)
				continue
			}
			c.connLock.Unlock()
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
