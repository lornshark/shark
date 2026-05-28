package sharkkafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type Config struct {
	Host     string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地
	Port     int    `json:"port" yaml:"port" mapstructure:"port"`             // 连接端口
	User     string `json:"user" yaml:"user" mapstructure:"user"`             // 连接用户名，默认值为 "" 不起用 SASL 验证
	Password string `json:"password" yaml:"password" mapstructure:"password"` // 连接密码，默认值为 "" 不起用 SASL 验证
}
type SharkKafka struct {
	ctx     context.Context
	config  *Config
	writers map[string]*kafka.Writer
	lock    sync.Mutex
	dialer  *kafka.Dialer
}

// New 创建一个新的 SharkKafka 实例，并根据提供的配置进行初始化。
func New(ctx context.Context, config *Config) (*SharkKafka, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	var dialer *kafka.Dialer
	if config.User != "" && config.Password != "" {
		mechanism, err := scram.Mechanism(scram.SHA512, config.User, config.Password)
		if err != nil {
			return nil, err
		}
		dialer = &kafka.Dialer{
			Timeout:       10 * time.Second,
			SASLMechanism: mechanism,
			TLS:           nil,
		}
	}

	return &SharkKafka{
		config:  config,
		writers: make(map[string]*kafka.Writer),
		dialer:  dialer,
	}, nil
}

// Writer 获取指定 topic 的 Kafka Writer，如果不存在则创建一个新的 Writer 并返回。
func (s *SharkKafka) Writer(topic string) (*kafka.Writer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if writer, ok := s.writers[topic]; ok {
		return writer, nil
	}
	writerConfig := kafka.WriterConfig{
		Brokers:      []string{fmt.Sprintf("%v:%v", s.config.Host, s.config.Port)},
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		Dialer:       s.dialer,
		BatchSize:    1000,
		BatchBytes:   1024 * 1024, // 一次最多发送1MB的数据
		BatchTimeout: 100 * time.Millisecond,
		RequiredAcks: int(kafka.RequireOne),
		Async:        false,
	}
	writer := kafka.NewWriter(writerConfig)
	s.writers[topic] = writer
	return writer, nil
}

// CloseWriter 关闭指定的 Kafka Writer，并从 SharkKafka 的 writers 中移除它。
func (s *SharkKafka) CloseWriter(topic string) error {
	s.lock.Lock()
	writer, ok := s.writers[topic]
	if ok {
		delete(s.writers, topic)
	}
	s.lock.Unlock()
	if ok {
		return writer.Close()
	}
	return nil
}

// Close 关闭 SharkKafka 实例中的所有 Kafka Writer，并清空 writers 映射。
func (s *SharkKafka) Close() error {
	s.lock.Lock()
	writers := s.writers
	s.writers = make(map[string]*kafka.Writer)
	s.lock.Unlock()
	var firstErr error
	for _, writer := range writers {
		if err := writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Reader 创建并返回一个 Kafka Reader
func (s *SharkKafka) Reader(topic string, group string) *kafka.Reader {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{fmt.Sprintf("%v:%v", s.config.Host, s.config.Port)},
		Topic:       topic,
		GroupID:     group,
		MinBytes:    1,                // 有数据就返回
		MaxBytes:    10 * 1024 * 1024, // 一次最多返回10MB的数据
		Dialer:      s.dialer,
		StartOffset: kafka.FirstOffset, // 从最新的消息开始消费
	})
	return reader
}
