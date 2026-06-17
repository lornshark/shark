package sharkredis

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

// ScanKeys 扫描 Redis Cluster 所有主节点中匹配 pattern 的 keys。
//
// 特点：
//  1. 并发扫描各 master node
//  2. 流式输出（channel）避免内存爆炸
//  3. 支持 context cancel
//
// 注意：不同 master node 之间不保证去重（如 key 迁移期间可能重复）。
//
// 使用示例:
//
//	// 扫描所有用户相关的 key
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	keysCh, errCh := sharkredis.ScanKeys(ctx, clusterClient, "user:*")
//	go func() {
//	    for err := range errCh {
//	        log.Printf("扫描出错: %v", err)
//	    }
//	}()
//	for key := range keysCh {
//	    fmt.Println("找到 key:", key)
//	    if count > 10000 {
//	        cancel() // 达到预期数量后取消扫描
//	    }
//	}
func ScanKeys(ctx context.Context, client *redis.ClusterClient, pattern string) (<-chan string, <-chan error) {
	out := make(chan string, 1024)
	ch := make(chan error, 1)
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
					case ch <- err:
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
		close(ch)
		return out, ch
	}
	go func() {
		wg.Wait()
		close(out)
		close(ch)
	}()
	return out, ch

}

// DeleteKeys 删除 Redis Cluster 中所有主节点上匹配 pattern 的 keys。
//
// 特点：
//  1. scan + delete 同步进行，流式处理
//  2. batch delete（每批 50 个）防止 Redis 阻塞
//  3. cluster master node 全覆盖
//
// 使用示例:
//
//	// 删除所有缓存 key
//	err := sharkredis.DeleteKeys(ctx, clusterClient, "cache:*")
//	if err != nil {
//	    log.Printf("删除 key 失败: %v", err)
//	}
//
// 参数:
//   - ctx: 上下文，可用于取消操作
//   - client: Redis 集群客户端
//   - pattern: key 匹配模式（如 "cache:*"、"session:*"）
//
// 返回:
//   - error: 扫描或删除出错时返回
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
