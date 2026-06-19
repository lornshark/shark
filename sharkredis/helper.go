package sharkredis

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// ScanKeys 扫描 Redis Cluster 所有主节点中匹配 pattern 的 keys。
//
// 实现要点：
//  1. 先通过 ForEachMaster 收集所有 master 节点，再并发生成 goroutine——避免 ForEachMaster
//     中途失败导致已启动 goroutine 泄漏。
//  2. 流式输出（channel）防止内存膨胀。
//  3. 错误 channel 缓冲区 16 避免多节点并发错误被静默丢弃。
//  4. 支持 context cancel 提前终止。
//
// 不同 master 之间不保证去重（如 key 迁移/reshard 期间可能重复）。
func ScanKeys(ctx context.Context, client *redis.ClusterClient, pattern string) (<-chan string, <-chan error) {
	if client == nil {
		out := make(chan string)
		close(out)
		ch := make(chan error, 1)
		ch <- fmt.Errorf("client required")
		close(ch)
		return out, ch
	}

	// 步骤 1：收集所有 master 节点（ForEachMaster 同步回调，不会泄漏）
	var nodes []*redis.Client
	err := client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		nodes = append(nodes, node)
		return nil
	})
	if err != nil || len(nodes) == 0 {
		out := make(chan string)
		close(out)
		ch := make(chan error, 1)
		if err != nil {
			ch <- err
		}
		close(ch)
		return out, ch
	}

	out := make(chan string, 1024)
	ch := make(chan error, 16) // 足够容纳各 node 的首个错误
	var wg sync.WaitGroup

	for _, node := range nodes {
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
// 实现要点：
//  1. scan + unlink 流式处理，防止大规模删除阻塞 Redis。
//  2. 批量 UNLINK（异步删除），不阻塞主线程。
//  3. 覆盖所有 cluster master node。
//
// Deprecated：建议使用 UnlinkKeys()，语义更明确。
// DeleteKeys 内部委托给 UnlinkKeys 实现。
func DeleteKeys(ctx context.Context, client *redis.ClusterClient, pattern string) error {
	return deleteKeys(ctx, client, pattern, unlinkBatch)
}

// UnlinkKeys 异步删除 Redis Cluster 中所有主节点上匹配 pattern 的 keys。
//
// 使用 UNLINK 命令（非阻塞），大数据量场景性能远优于 DEL。
func UnlinkKeys(ctx context.Context, client *redis.ClusterClient, pattern string) error {
	return deleteKeys(ctx, client, pattern, unlinkBatch)
}

// deleteAction 定义批量删除的操作类型。
type deleteAction func(ctx context.Context, node *redis.Client, keys ...string) error

func unlinkBatch(ctx context.Context, node *redis.Client, keys ...string) error {
	return node.Unlink(ctx, keys...).Err()
}

// deleteKeys 是在 cluster 所有 master 上扫描并批量删除 keys 的通用实现。
func deleteKeys(ctx context.Context, client *redis.ClusterClient, pattern string, del deleteAction) error {
	if client == nil {
		return fmt.Errorf("client required")
	}
	if pattern == "" {
		return fmt.Errorf("pattern required")
	}

	return client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		var cursor uint64
		const batchSize = 50
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
			for i := 0; i < len(keys); i += batchSize {
				end := i + batchSize
				if end > len(keys) {
					end = len(keys)
				}
				if err := del(ctx, node, keys[i:end]...); err != nil {
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
