package sharkcache

import (
	"crypto/md5"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"golang.org/x/sync/singleflight"
)

var ErrNotFound = errors.New("sharkcache: not found")
var ErrTypeAssertion = errors.New("sharkcache: type assertion failed")

type Cache[T any] struct {
	seekers []func(args ...any) (*T, error)
	sg      singleflight.Group
}

func NewCache[T any](seekers ...func(args ...any) (*T, error)) *Cache[T] {
	return &Cache[T]{
		seekers: seekers,
	}
}

func (c *Cache[T]) Get(args ...any) (*T, error) {
	bytes, _ := sonic.Marshal(args)
	sum := md5.Sum(bytes)
	key := fmt.Sprintf("%x", sum)
	v, err, _ := c.sg.Do(key, func() (any, error) {
		for _, seeker := range c.seekers {
			v, err := seeker(args...)
			if err != nil {
				// 如果当前 seeker 出现错误，继续尝试下一个 seeker
				continue
			}
			if v != nil {
				// 如果当前 seeker 成功获取到值，直接返回
				return v, nil
			}
		}
		// 如果所有 seeker 都无法获取到值，返回 nil 和 nil 错误
		return nil, ErrNotFound
	})
	if err != nil {
		return nil, err
	}
	if v, ok := v.(*T); ok {
		return v, nil
	}
	return nil, ErrTypeAssertion
}
