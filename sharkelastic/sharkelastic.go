package sharkelastic

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9"
)

type Config struct {
	Host     string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地址
	User     string `json:"user" yaml:"user" mapstructure:"user"`             // 连接用户名
	Password string `json:"password" yaml:"password" mapstructure:"password"` // 连接密码
}

func New(ctx context.Context, config *Config) (*SharkElastic, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	client, err := elasticsearch.New(
		elasticsearch.WithAddresses(config.Host),
		elasticsearch.WithBasicAuth(config.User, config.Password),
	)
	if err != nil {
		return nil, err
	}
	_, err = client.Ping()
	if err != nil {
		return nil, err
	}
	return &SharkElastic{
		Client: client,
	}, nil
}
