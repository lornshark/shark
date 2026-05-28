package sharkredis

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

// ScanKeys 扫描 Redis Cluster 所有主节点 keys（无去重版本）
// 特点：
// 1. 并发扫描各 master node
// 2. 流式输出（channel）避免内存爆炸
// 3. 支持 context cancel
func ScanKeys(ctx context.Context, client *redis.ClusterClient, pattern string) (<-chan string, <-chan error) {
	out := make(chan string, 1024)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	err := client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		wg.Add(1)
		go func(node *redis.Client) {
			defer wg.Done()
			var cursor uint64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				keys, cur, err := node.Scan(ctx, cursor, pattern, 100).Result()
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				for _, k := range keys {
					select {
					case out <- k:
					case <-ctx.Done():
						return
					}
				}
				if cur == 0 {
					return
				}
				cursor = cur
			}
		}(node)
		return nil
	})
	if err != nil {
		close(out)
		close(errCh)
		return out, errCh
	}
	go func() {
		wg.Wait()
		close(out)
		close(errCh)
	}()
	return out, errCh

}

// DeleteKeys 删除 Redis Cluster 中匹配 pattern 的 keys
// 特点：
// 1. scan + delete 同步进行
// 2. batch delete 防止 Redis 阻塞
// 3. cluster master node 全覆盖
func DeleteKeys(ctx context.Context, client *redis.ClusterClient, pattern string) error {
	return client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		var cursor uint64
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			keys, cur, err := node.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return err
			}
			// batch delete
			const batchSize = 50
			for i := 0; i < len(keys); i += batchSize {
				end := i + batchSize
				if end > len(keys) {
					end = len(keys)
				}
				if err := node.Del(ctx, keys[i:end]...).Err(); err != nil {
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
}
