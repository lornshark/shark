package sharkredis

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Host        string `json:"host"`         // 连接地
	Port        int    `json:"port"`         // 连接端口
	Password    string `json:"password"`     // 连接密码，默认值为 "" 表示不使用密码连接
	ClusterHost string `json:"cluster_host"` // 集群连接地址，默认值为 "" 表示不替换集群连接地址
}

func New(ctx context.Context, config *Config) (*redis.ClusterClient, error) {
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
		MaxRetryBackoff: 1 * time.Second,        // 最大重试间隔
		NewClient: func(opt *redis.Options) *redis.Client {
			return redis.NewClient(opt)
		},
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			clusterAddr := addr
			// 如果 test-host 配置了值,且不是Prod环境，则使用 cluster-host 连接 Redis 集群
			if config.ClusterHost != "" {
				clusterAddr = fmt.Sprintf("%v:%v", config.ClusterHost, config.Port)
			}
			return net.Dial(network, clusterAddr)
		},
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// ScanKeys 扫描 Redis 集群中所有主节点的键，并返回匹配指定模式的键列表。
func ScanKeys(ctx context.Context, client *redis.ClusterClient, pattern string) ([]string, error) {
	var result []string
	err := client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		var cursor uint64
		for {
			keys, cur, err := node.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return err
			}
			result = append(result, keys...)
			if cur == 0 {
				break
			}
			cursor = cur
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// 全局 delete 函数,删除指定模式的键
func DeleteKeys(ctx context.Context, client *redis.ClusterClient, pattern string) error {
	err := client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		var cursor uint64
		for {
			keys, cur, err := node.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return err
			}
			if len(keys) > 0 {
				if err := node.Del(ctx, keys...).Err(); err != nil {
					return err
				}
			}
			if cur == 0 {
				break
			}
			cursor = cur
		}
		return nil
	})
	return err
}
