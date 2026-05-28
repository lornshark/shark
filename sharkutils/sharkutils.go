package sharkutils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	crand "math/rand/v2"
	"runtime"

	"go.uber.org/zap"
)

// RandNum 生成[min,max)之间的随机数
func RandNum(min int, max int) int {
	if min >= max {
		return min
	}
	return crand.IntN(max-min) + min
}

// RandString 生成指定长度的随机字符串
func Md5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Recover 从 panic 中恢复，并记录日志
func Recover(logger *zap.Logger, name string) {
	if r := recover(); r != nil {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		logger.Error(fmt.Sprintf("panic: %s %v\n%s", name, r, string(buf[:n])))
	}
}
