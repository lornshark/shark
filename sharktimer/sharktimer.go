package sharktimer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lornshark/shark/sharksnowflake"

	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
)

// 轻量级,单机定时器,不重试,定时器误差1s

type TimerRedis interface {
	ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
}

type Timer struct {
	ctx             context.Context
	timerCallback   sync.Map
	timerKey        string
	snowFlake       *sharksnowflake.Snowflake
	pool            *ants.Pool
	redis           TimerRedis
	defaultCallback func(timerId string)
}

// NewTimer 创建一个新的 Timer 实例，并根据提供的配置进行初始化。
func NewTimer(ctx context.Context, project string, name string, id string, redis TimerRedis) *Timer {
	pool, _ := ants.NewPool(10)
	t := &Timer{
		ctx:           ctx,
		redis:         redis,
		timerCallback: sync.Map{},
		timerKey:      fmt.Sprintf("%v:timer:%v-%v", project, project, id),
		snowFlake:     sharksnowflake.NewSnowflake(),
		pool:          pool,
	}
	go t.processTimer()
	return t
}

// processTimer 定时器处理函数，定时检查 Redis 中的定时器列表，触发到期的定时器并执行对应的回调函数。
func (t *Timer) processTimer() {
	if t.redis == nil {
		return
	}
	safeCallback := func(cb func()) {
		defer func() {
			if r := recover(); r != nil {
				// 回调函数应该保证不抛出异常,这里吞掉异常,避免定时器线程崩溃
			}
		}()
		cb()
	}
	for {
		select {
		case <-t.ctx.Done():
			t.pool.Release()
			return
		default:
			result := t.redis.ZRangeByScoreWithScores(t.ctx, t.timerKey, &redis.ZRangeBy{
				Min:    "0",
				Max:    fmt.Sprintf("%v", time.Now().UnixMilli()),
				Offset: 0,
				Count:  100,
			}).Val()
			if len(result) == 0 {
				time.Sleep(time.Second)
				continue
			}
			members := make([]interface{}, 0, len(result))
			for _, v := range result {
				members = append(members, v.Member)
			}
			t.redis.ZRem(t.ctx, t.timerKey, members...)
			for _, v := range result {
				cb, ok := t.timerCallback.LoadAndDelete(v.Member)
				if ok {
					if callback, ok := cb.(func()); ok {
						t.pool.Submit(func() {
							safeCallback(callback)
						})
					}
				} else {
					if t.defaultCallback != nil {
						t.pool.Submit(func() {
							safeCallback(func() {
								t.defaultCallback(cast.ToString(v.Member))
							})
						})
					}
				}
			}
		}
	}
}

// AddTimer 添加定时器,返回定时器ID,如果需要删除定时器,可以通过定时器ID删除
func (t *Timer) AddTimer(durnation time.Duration, callback func()) string {
	id := cast.ToString(t.snowFlake.Generate())
	timestamp := time.Now().Add(durnation).UnixMilli()
	t.redis.ZAdd(t.ctx, t.timerKey, redis.Z{Score: float64(timestamp), Member: id})
	if callback != nil {
		t.timerCallback.Store(id, callback)
	}
	return id
}

// RemoveTimer 删除定时器,如果定时器不存在,则不做任何操作
func (t *Timer) RemoveTimer(timerId string) {
	t.timerCallback.Delete(timerId)
	t.redis.ZRem(t.ctx, t.timerKey, timerId)
}

// AddTimeWithId 添加定时器,指定定时器ID,如果定时器ID已存在,则覆盖原有定时器
func (t *Timer) AddTimeWithId(timerId string, durnation time.Duration, callback func()) {
	timestamp := time.Now().Add(durnation).UnixMilli()
	t.redis.ZAdd(t.ctx, t.timerKey, redis.Z{Score: float64(timestamp), Member: timerId})
	if callback != nil {
		t.timerCallback.Store(timerId, callback)
	}
}

// DefaultCallback 定时器回调函数,当定时器触发时,如果没有找到对应的回调函数,则调用默认回调函数,默认回调函数参数为定时器ID
func (t *Timer) DefaultCallback(cb func(timerId string)) {
	t.defaultCallback = cb
}
