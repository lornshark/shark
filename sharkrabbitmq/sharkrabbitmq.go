package sharkrabbitmq

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/howeyc/crc16"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type Config struct {
	Host     []string        `json:"host"`     // 连接地址
	User     string          `json:"user"`     // 连接用户名
	Password string          `json:"password"` // 连接密码
	Name     string          `json:"name"`     // 工程名称
	Id       string          `json:"id"`       // 实例Id
	Logger   *zap.Logger     `json:"-"`        // 日志记录器
	Wg       *sync.WaitGroup `json:"-"`        // 等待组
	Running  *atomic.Bool    `json:"-"`        // 运行状态
}

type Client struct {
	config     *Config
	conn       *amqp.Connection
	connLock   sync.Mutex
	done       context.Context
	doneCancel context.CancelFunc
	channel    []*amqp.Channel
	publish    chan publishMsg
}

type publishMsg struct {
	exchange string
	key      string
	value    *amqp.Publishing
}

func (c *Client) Exit() {
	c.doneCancel()
}

func New(config *Config) (*Client, error) {
	index := crc16.Checksum([]byte(config.Name), crc16.IBMTable)
	index = index % uint16(len(config.Host))
	config.Host = []string{config.Host[index]}
	client := &Client{
		config: config,
	}
	client.done, client.doneCancel = context.WithCancel(context.Background())
	client.publish = make(chan publishMsg, 10000)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	connected := false
	go func() {
		for {
			client.connLock.Lock()
			amqpurl := "amqp://" + config.User + ":" + config.Password + "@" + config.Host[index]
			conn, err := amqp.Dial(amqpurl)
			if err != nil {
				config.Logger.Error("连接Rabbitmq失败", zap.String("host", config.Host[index]), zap.Error(err))
				client.connLock.Unlock()
				time.Sleep(1 * time.Second)
				continue
			}
			for i := 0; i < 5; i++ {
				ch, _ := conn.Channel()
				if ch != nil {
					client.channel = append(client.channel, ch)
				}
			}
			client.conn = conn
			config.Logger.Info("成功连接Rabbitmq", zap.String("host", config.Host[index]))
			connErr := make(chan *amqp.Error, 1)
			conn.NotifyClose(connErr)
			client.connLock.Unlock()
			if !connected {
				connected = true
				wg.Done()
			}
			e := <-connErr
			client.connLock.Lock()
			config.Logger.Warn("Rabbitmq连接已关闭", zap.String("host", config.Host[index]), zap.Error(e))
			client.conn.Close()
			client.conn = nil
			client.channel = nil
			client.connLock.Unlock()
		}
	}()
	config.Wg.Add(1)
	go func() {
		defer config.Wg.Done()
		index := 0
		for {
			select {
			case msg := <-client.publish:
				for {
					client.connLock.Lock()
					if client.conn == nil || len(client.channel) == 0 {
						client.connLock.Unlock()
						time.Sleep(1 * time.Second)
						continue
					}
					index++
					index = index % len(client.channel)
					channel := client.channel[index]
					err := channel.Publish(msg.exchange, msg.key, false, false, *msg.value)
					if err != nil {
						client.connLock.Unlock()
						time.Sleep(1 * time.Second)
						continue
					}
					client.connLock.Unlock()
					break
				}
			case <-client.done.Done():
				return
			}
		}
	}()
	wg.Wait()
	return client, nil
}

func (c *Client) Consume(exchange string, queue string, key string, handler func(*amqp.Delivery)) {
	safeHandler := func(msg *amqp.Delivery) {
		c.config.Wg.Add(1)
		defer func() {
			if r := recover(); r != nil {
				c.config.Logger.Error("Rabbitmq消费者处理消息panic", zap.Any("r", r), zap.String("stack", string(debug.Stack())))
			}
			c.config.Wg.Done()
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
			for msg := range ch {
				if !c.config.Running.Load() {
					break
				}
				safeHandler(&msg)
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
