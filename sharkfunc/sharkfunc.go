package sharkfunc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrTimeout = errors.New("sharkfunc: timeout")

// 指定时间内完成调用,否则返回超时错误
func WithTimeout(timeout time.Duration, fn func(context.Context)) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case ch <- fmt.Errorf("sharkfunc: panic: %v", r):
				default:
				}
				return
			}
			select {
			case ch <- nil:
			default:
			}
		}()
		fn(ctx)
	}()
	select {
	case <-ctx.Done():
		return ErrTimeout
	case err := <-ch:
		return err
	}
}

// 并行执行,funcs都执行完了才返回
func ParallelRun(funcs ...func()) error {
	var wg sync.WaitGroup
	ch := make(chan error, len(funcs))
	for _, fn := range funcs {
		wg.Add(1)
		go func(f func()) {
			defer func() {
				if r := recover(); r != nil {
					ch <- fmt.Errorf("panic: %v", r)
				}
				wg.Done()
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
