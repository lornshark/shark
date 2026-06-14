package sharketcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Config struct {
	Host     string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地址
	Port     int    `json:"port" yaml:"port" mapstructure:"port"`             // 连接端口，默认 2379
	User     string `json:"user" yaml:"user" mapstructure:"user"`             // 用户名，可选
	Password string `json:"password" yaml:"password" mapstructure:"password"` // 密码，可选
}

func New(ctx context.Context, config *Config) (*clientv3.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	if config.Host == "" {
		return nil, fmt.Errorf("host required")
	}
	port := config.Port
	if port <= 0 {
		port = 2379
	}

	cfg := clientv3.Config{
		Endpoints:   []string{fmt.Sprintf("%s:%d", config.Host, port)},
		DialTimeout: 5 * time.Second,
	}
	if config.User != "" {
		cfg.Username = config.User
		cfg.Password = config.Password
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 etcd 客户端失败: %w", err)
	}

	// 健康检查
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = client.Get(ctx, "health_check")
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("etcd 健康检查失败: %w", err)
	}

	return client, nil
}
