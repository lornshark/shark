// Package sharkredis 提供了 Redis 客户端（集群/单机模式）的创建和连接管理。
//
// 封装了 go-redis v9 的集群客户端（ClusterClient）和单机客户端（Client）的创建，
// 支持:
//   - 自定义 Dialer（用于容器化环境中替换地址，如将 Kubernetes Service IP 替换为 Pod IP）
//   - 统一的连接池配置（空闲连接、最大连接、超时时间、重试策略等）
//   - 连接健康检查（Ping 验证）
//
// 类型说明:
//   - NewCluster: 创建 Redis 集群模式客户端
//   - NewClient: 创建 Redis 单机/主从模式客户端
package sharkredis

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config 是 Redis 的连接配置。
//
// 支持集群模式和单机模式，通过 Host 列表区分：
//   - 集群模式: Host 包含所有集群节点地址
//   - 单机模式: Host[0] 作为连接地址
//
// ReplaceFrom / ReplaceTo 用于在容器化环境中替换 redis cluster 返回的节点地址，
// 例如将 Kubernetes Service IP 替换为具体的 Pod IP。
//
// 支持从 JSON、YAML、viper（mapstructure）等多种配置源加载。
type Config struct {
	// Host Redis 节点地址列表，格式为 "host:port"
	Host []string `json:"host" yaml:"host" mapstructure:"host"`
	// Password 连接密码，为空时不使用密码认证
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	// ReplaceFrom 需要被替换的地址前缀（如 k8s service host）
	ReplaceFrom string `json:"replace_from" yaml:"replace_from" mapstructure:"replace_from"`
	// ReplaceTo 替换后的地址（如 Pod IP）
	ReplaceTo string `json:"replace_to" yaml:"replace_to" mapstructure:"replace_to"`
}

// NewCluster 创建 Redis 集群模式客户端并验证连接。
//
// 连接池配置:
//   - PoolSize: 200（最大连接数）
//   - MinIdleConns: 20（最小空闲连接数）
//   - ConnMaxIdleTime: 10 分钟
//   - ConnMaxLifetime: 30 分钟
//   - 读写/连接超时: 2 秒
//   - 重试: 最多 2 次，退避间隔 100ms~1s
//
// 使用示例:
//
//	cluster, err := sharkredis.NewCluster(ctx, &sharkredis.Config{
//	    Host:     []string{"127.0.0.1:6379", "127.0.0.1:6380"},
//	    Password: "myPassword",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cluster.Close()
//
//	// 使用集群客户端
//	cluster.Set(ctx, "mykey", "myvalue", 0)
//	val, _ := cluster.Get(ctx, "mykey").Result()
func NewCluster(ctx context.Context, config *Config) (*redis.ClusterClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           config.Host,
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

// NewClient 创建 Redis 单机/主从模式客户端并验证连接。
//
// 使用 config.Host[0] 作为连接地址，连接池参数与 NewCluster 一致。
//
// 使用示例:
//
//	client, err := sharkredis.NewClient(ctx, &sharkredis.Config{
//	    Host:     []string{"127.0.0.1:6379"},
//	    Password: "myPassword",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// 使用单机客户端
//	client.Set(ctx, "mykey", "myvalue", 10*time.Second)
func NewClient(ctx context.Context, config *Config) (*redis.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	client := redis.NewClient(&redis.Options{
		Addr:            config.Host[0], // 只使用第一个地址
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
