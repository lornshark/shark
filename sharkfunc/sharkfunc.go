package sharkfunc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"
)

var ErrTimeout = errors.New("sharkfunc: timeout")

// 指定时间内完成调用,否则返回超时错误
func WithTimeout(parent context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ch := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				select {
				case ch <- fmt.Errorf("panic: %v\n%s", r, debug.Stack()):
				default:
				}
				return
			}
			select {
			case ch <- err:
			default:
			}
		}()
		err = fn(ctx)
	}()
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		select {
		case err := <-ch:
			return err
		default:
			return ErrTimeout
		}
	}
}

// 并行执行,funcs都执行完了才返回
func ParallelCall(funcs ...func()) error {
	var wg sync.WaitGroup
	ch := make(chan error, len(funcs))
	for _, fn := range funcs {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					ch <- fmt.Errorf("panic: %v", r)
				}
			}()
			f()
		}(fn)
	}
	wg.Wait()
	close(ch)
	for err := range ch {
		if err != nil {
			return err
		}
	}
	return nil
}

// 从channel中读取数据,读取到指定条数的数据,channel关闭了,context超时了,channel 没有数据 返回
// timer 用于控制没有数据时的超时,避免一直阻塞
func DrainChannelN[T any](ctx context.Context, ch <-chan T, size int) []T {
	var result []T = make([]T, 0, size)
	if ch == nil || size <= 0 {
		return result
	}
	// 先阻塞拿第一个
	select {
	case <-ctx.Done():
		return result
	case v, ok := <-ch:
		if !ok {
			return result
		}
		result = append(result, v)
	}
	// 再尽量多拿一些,不阻塞了
	for len(result) < size {
		select {
		case <-ctx.Done():
			return result
		case v, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, v)
		default:
			// 没有更多数据了
			return result
		}
	}
	return result
}

// Recover 从 panic 中恢复，并记录日志
func Recover(logger *zap.Logger, name string) {
	if r := recover(); r != nil {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		logger.Error(fmt.Sprintf("panic: %s %v\n%s", name, r, string(buf[:n])))
	}
}

// Pointer 返回值的指针
func Ptr[T any](v T) *T {
	return &v
}
