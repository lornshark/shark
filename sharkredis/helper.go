package sharkredis

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Helper 是对 go-redis 单节点客户端（*redis.Client）和集群客户端（*redis.ClusterClient）的统一封装。
//
// 通过 NewHelperWithClient 或 NewHelperWithCluster 构造，所有方法内部会自动判断使用哪种客户端。
// 两者只会有一个非 nil，若都为 nil 则方法返回 redis.Nil 或零值。
type Helper struct {
	client  *redis.Client
	cluster *redis.ClusterClient
}

// NewHelperWithClient 使用单节点 Redis 客户端构造 Helper。
func NewHelperWithClient(client *redis.Client) *Helper {
	return &Helper{
		client: client,
	}
}

// NewHelperWithCluster 使用 Redis 集群客户端构造 Helper。
func NewHelperWithCluster(cluster *redis.ClusterClient) *Helper {
	return &Helper{
		cluster: cluster,
	}
}

// Keys 是 Scan 的别名，返回匹配 pattern 的所有 key 的流式 channel。
// 参见 Scan 方法说明。
func (h *Helper) Keys(ctx context.Context, pattern string) <-chan string {
	return h.Scan(ctx, pattern)
}

// Scan 以游标迭代方式扫描匹配 pattern 的所有 key，结果通过 channel 流式返回。
//
// 特性：
//   - 单节点：在 h.client 上循环执行 SCAN，每次取 100 条。
//   - 集群：通过 ForEachMaster 遍历所有主节点，每个节点启动独立 goroutine 并发扫描，
//     结果汇入同一 channel；不同节点间不保证去重。
//   - channel 缓冲为 1024，避免扫描速度远快于消费时阻塞 goroutine。
//   - 支持 ctx 取消：cancel 后所有 goroutine 会在下一次 select 处退出。
//   - 遇到错误时静默退出（不向外暴露错误），若需要错误信息请使用包级函数 ScanKeys。
//   - client/cluster 均为 nil 时返回一个已关闭的空 channel。
func (h *Helper) Scan(ctx context.Context, pattern string) <-chan string {
	if h.client != nil {
		out := make(chan string, 1024)
		go func() {
			defer close(out)
			var cursor uint64
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				keys, cur, err := h.client.Scan(ctx, cursor, pattern, 100).Result()
				if err != nil {
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
		}()
		return out
	}
	if h.cluster != nil {
		out := make(chan string, 1024)
		var wg sync.WaitGroup
		go func() {
			defer close(out)
			_ = h.cluster.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
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
			wg.Wait()
		}()
		return out
	}
	out := make(chan string)
	close(out)
	return out
}

// Unlink 扫描匹配 pattern 的所有 key，并使用 UNLINK 命令异步删除。
//
// UNLINK 为非阻塞删除，大数据量场景性能远优于 DEL。
//
// 特性：
//   - 单节点：在 h.client 上循环 SCAN，每批最多 50 个 key 执行 UNLINK，遇错立即返回。
//   - 集群：通过 ForEachMaster 遍历所有主节点，每个节点串行扫描并批量 UNLINK，
//     任意节点出错则 ForEachMaster 立即返回该错误。
//   - 支持 ctx 取消：cancel 后返回 ctx.Err()。
//   - pattern 为空字符串时不做额外校验，请调用方自行保证。
func (h *Helper) Unlink(ctx context.Context, pattern string) error {
	if h.client != nil {
		var cursor uint64
		const batchSize = 50
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			keys, cur, err := h.client.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return err
			}
			for i := 0; i < len(keys); i += batchSize {
				end := i + batchSize
				if end > len(keys) {
					end = len(keys)
				}
				if err := h.client.Unlink(ctx, keys[i:end]...).Err(); err != nil {
					return err
				}
			}
			if cur == 0 {
				return nil
			}
			cursor = cur
		}
	}
	if h.cluster != nil {
		return h.cluster.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
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
					if err := node.Unlink(ctx, keys[i:end]...).Err(); err != nil {
						return err
					}
				}
				if cur == 0 {
					return nil
				}
				cursor = cur
			}
		})
	}
	return nil
}

// SetObject 将任意 Go 值序列化为 JSON 后以 SET 命令写入 Redis。
//
// 参数：
//   - key：Redis key。
//   - value：任意可 JSON 序列化的 Go 值（结构体、map、slice 等）。
//   - expiration：key 的过期时间；传 0 表示永不过期。
//
// 返回 *redis.StatusCmd，调用 .Err() 判断是否成功。
func (h *Helper) SetObject(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	bytes, err := json.Marshal(value)
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	if h.client != nil {
		return h.client.Set(ctx, key, string(bytes), expiration)
	}
	if h.cluster != nil {
		return h.cluster.Set(ctx, key, string(bytes), expiration)
	}
	return nil
}

// GetObject 通过 GET 命令读取 Redis 中的 JSON 字符串，并反序列化到 value 指向的对象。
//
// 参数：
//   - key：Redis key。
//   - value：必须是指针，反序列化结果将写入该指针指向的对象。
//
// 返回 *redis.StatusCmd：
//   - 成功时 Val() 为 "OK"，Err() 为 nil。
//   - key 不存在时 Err() 为 redis.Nil。
//   - JSON 解析失败时 Err() 为对应错误。
func (h *Helper) GetObject(ctx context.Context, key string, value any) *redis.StatusCmd {
	var result string
	var err error
	if h.client != nil {
		result, err = h.client.Get(ctx, key).Result()
	} else if h.cluster != nil {
		result, err = h.cluster.Get(ctx, key).Result()
	} else {
		return redis.NewStatusResult("", redis.Nil)
	}
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	err = json.Unmarshal([]byte(result), value)
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	return redis.NewStatusResult("OK", nil)
}

// HSetObject 将任意 Go 值序列化为 JSON，再转为 map 后以 HSET 命令写入 Redis Hash。
//
// 实现方式：value → JSON → map[string]any → HSET。
// JSON tag 会被自动识别，即结构体字段的 json tag 名称即为 Hash field 名。
//
// 参数：
//   - key：Redis Hash key。
//   - value：任意可 JSON 序列化的 Go 值，顶层必须可展开为 object（结构体或 map）。
//
// 返回 *redis.IntCmd，Val() 为写入的字段数，Err() 为错误。
func (h *Helper) HSetObject(ctx context.Context, key string, value any) *redis.IntCmd {
	var mvalue map[string]any
	bytes, err := json.Marshal(value)
	if err != nil {
		return redis.NewIntResult(0, err)
	}
	err = json.Unmarshal(bytes, &mvalue)
	if err != nil {
		return redis.NewIntResult(0, err)
	}
	if h.client != nil {
		return h.client.HSet(ctx, key, mvalue)
	}
	if h.cluster != nil {
		return h.cluster.HSet(ctx, key, mvalue)
	}
	return nil
}

// HGetAllObject 通过 HGETALL 获取 Redis Hash 的全部字段，并反序列化到 value 指向的对象。
//
// 实现方式：HGETALL → map[string]string → map[string]any → JSON → value。
//
// 参数：
//   - key：Redis Hash key。
//   - value：必须是指针，反序列化结果将写入该指针指向的对象。
//
// 返回 *redis.StatusCmd：
//   - 成功时 Val() 为 "OK"，Err() 为 nil。
//   - key 不存在或 client 未设置时 Err() 为 redis.Nil。
func (h *Helper) HGetAllObject(ctx context.Context, key string, value any) *redis.StatusCmd {
	var mvalue map[string]any
	var result map[string]string
	var err error
	if h.client != nil {
		result, err = h.client.HGetAll(ctx, key).Result()
	} else if h.cluster != nil {
		result, err = h.cluster.HGetAll(ctx, key).Result()
	} else {
		return redis.NewStatusResult("", redis.Nil)
	}
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	mvalue = make(map[string]any)
	for k, v := range result {
		mvalue[k] = v
	}
	bytes, err := json.Marshal(mvalue)
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	err = json.Unmarshal(bytes, &value)
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	return redis.NewStatusResult("OK", nil)
}

// HGetObject 从 Redis Hash 中获取指定字段并反序列化到 value 指向的对象。
//
// 参数：
//   - key：Redis Hash key。
//   - value：必须是指针，反序列化结果将写入该指针指向的对象。
//   - fields：需要获取的字段名列表。
//   - 若传入 fields，使用 HMGET 只拉取指定字段，避免拉取整个 Hash。
//   - 若不传 fields，退化为 HGETALL 获取全部字段（等价于 HGetAllObject）。
//
// 实现方式：
//   - 有 fields：HMGET → map[string]any（nil 值保留）→ JSON → value。
//   - 无 fields：HGETALL → map[string]any → JSON → value。
//
// 返回 *redis.IntCmd：
//   - Val() 为实际填充到 map 的字段数（含值为 nil 的字段）。
//   - Err() 为错误；client 未设置时返回 redis.Nil。
func (h *Helper) HGetObject(ctx context.Context, key string, value any, fields ...string) *redis.IntCmd {
	var cmdable redis.Cmdable
	if h.client != nil {
		cmdable = h.client
	} else if h.cluster != nil {
		cmdable = h.cluster
	} else {
		return redis.NewIntResult(0, redis.Nil)
	}

	var mvalue map[string]any

	if len(fields) > 0 {
		vals, err := cmdable.HMGet(ctx, key, fields...).Result()
		if err != nil {
			return redis.NewIntResult(0, err)
		}
		mvalue = make(map[string]any, len(fields))
		for i, f := range fields {
			mvalue[f] = vals[i]
		}
	} else {
		result, err := cmdable.HGetAll(ctx, key).Result()
		if err != nil {
			return redis.NewIntResult(0, err)
		}
		mvalue = make(map[string]any, len(result))
		for k, v := range result {
			mvalue[k] = v
		}
	}

	b, err := json.Marshal(mvalue)
	if err != nil {
		return redis.NewIntResult(0, err)
	}
	if err := json.Unmarshal(b, value); err != nil {
		return redis.NewIntResult(0, err)
	}
	return redis.NewIntResult(int64(len(mvalue)), nil)
}
