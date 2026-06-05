package sharkredis

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Host        string `json:"host" yaml:"host" mapstructure:"host"`                         // 连接地
	Port        int    `json:"port" yaml:"port" mapstructure:"port"`                         // 连接端口
	Password    string `json:"password" yaml:"password" mapstructure:"password"`             // 连接密码，默认值为 "" 表示不使用密码连接
	ReplaceFrom string `json:"replace_from" yaml:"replace_from" mapstructure:"replace_from"` // 替换前缀，默认值为 "" 表示不替换
	ReplaceTo   string `json:"replace_to" yaml:"replace_to" mapstructure:"replace_to"`       // 替换后缀，默认值为 "" 表示不替换
}

func NewCluster(ctx context.Context, config *Config) (*redis.ClusterClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           []string{fmt.Sprintf("%v:%v", config.Host, config.Port)},
		Username:        "default",
		Password:        config.Password,
		MaxRetries:      2,                      // 最大重试次数
		MinIdleConns:    20,                     // 连接池中的最小空闲连接数
		PoolSize:        200,                    // 连接池中的最大连接数
		ConnMaxIdleTime: 10 * time.Minute,       // 空闲连接最大存活时间
		ConnMaxLifetime: 30 * time.Minute,       // 最大连接存活时间
		ReadTimeout:     2 * time.Second,        // 读取超时时间
		WriteTimeout:    2 * time.Second,        // 写入超时时间
		DialTimeout:     2 * time.Second,        // 连接超时时间
		MinRetryBackoff: 100 * time.Millisecond, // 最小重试间隔
		MaxRetryBackoff: time.Second,            // 最大重试间隔
		NewClient: func(opt *redis.Options) *redis.Client {
			return redis.NewClient(opt)
		},
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if config.ReplaceFrom != "" && config.ReplaceTo != "" {
				addr = strings.ReplaceAll(addr, config.ReplaceFrom, config.ReplaceTo)
			}
			return net.Dial(network, addr)
		},
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func NewClient(ctx context.Context, config *Config) (*redis.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	client := redis.NewClient(&redis.Options{
		Addr:            fmt.Sprintf("%v:%v", config.Host, config.Port),
		Username:        "default",
		Password:        config.Password,
		MaxRetries:      2,                      // 最大重试次数
		MinIdleConns:    20,                     // 连接池中的最小空闲连接数
		PoolSize:        200,                    // 连接池中的最大连接数
		ConnMaxIdleTime: 10 * time.Minute,       // 空闲连接最大存活时间
		ConnMaxLifetime: 30 * time.Minute,       // 最大连接存活时间
		ReadTimeout:     2 * time.Second,        // 读取超时时间
		WriteTimeout:    2 * time.Second,        // 写入超时时间
		DialTimeout:     2 * time.Second,        // 连接超时时间
		MinRetryBackoff: 100 * time.Millisecond, // 最小重试间隔
		MaxRetryBackoff: time.Second,            // 最大重试间隔
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}
