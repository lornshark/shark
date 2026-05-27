package sharkutils

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	crand "math/rand/v2"
	"runtime"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/pquerna/otp/totp"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

// ZipCompress 压缩
func ZipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ZipDecompress 解压
func ZipDecompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RandNum 生成[min,max)之间的随机数
func RandNum(min int, max int) int {
	if min >= max {
		return min
	}
	return crand.IntN(max-min) + min
}

func Md5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func ParseJsonBytes[T any](value []byte) *T {
	var result T
	err := sonic.Unmarshal(value, &result)
	if err != nil {
		return nil
	}
	return &result
}

func ParseJsonString[T any](value string) *T {
	if value == "" {
		return nil
	}
	return ParseJsonBytes[T]([]byte(value))
}

func MarshalJsonToBytes(v any) []byte {
	if v == nil {
		return nil
	}
	data, err := sonic.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func MarshalJsonToString(v any) string {
	return string(MarshalJsonToBytes(v))
}

// 验证谷歌验证码
func VerifyGoogleCode(secret string, code string) bool {
	return totp.Validate(code, secret)
}

// 生成谷歌验证码秘钥和二维码URL
func NewGoogleSecret(issuer string, accountName string) (string, string) {
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	return key.Secret(), key.URL()
}

// 获取谷歌验证码二维码URL
func GetGoogleQrCodeUrl(secret string, issuer string, accountName string) string {
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Secret:      []byte(secret),
	})
	return key.URL()
}

// 指定时间内完成调用,否则返回超时错误
func CallWithTimeout(timeout time.Duration, fn func(context.Context)) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
			done <- struct{}{}
		}()

		fn(ctx)
	}()

	select {
	case <-ctx.Done():
		return errors.New("timeout")
	case <-done:
		return err
	}
}

// 并行执行,funcs都执行完了才返回
func ParallelRun(funcs ...func()) (err error) {
	var wg sync.WaitGroup
	for _, fn := range funcs {
		wg.Add(1)
		go func(f func()) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
				wg.Done()
			}()
			f()
		}(fn)
	}
	wg.Wait()
	return err
}

// 判断any是否为基本类型
func IsBasicType(v any) bool {
	switch v.(type) {
	case uint, int, int32, int64, float32, float64, bool, string, int16, int8, uint32, uint64, uint16, uint8, decimal.Decimal:
		return true
	default:
		return false
	}
}

// 转换为decimal.Decimal,并截断保留指定小数位数
func NormalizeDecimal(v any, places int32) decimal.Decimal {
	if places < 0 {
		places = 0
	}
	var d decimal.Decimal
	switch val := v.(type) {
	case decimal.Decimal:
		d = val
	case string:
		dd, err := decimal.NewFromString(val)
		if err != nil {
			return decimal.Zero
		}
		d = dd
	case float32:
		d = decimal.NewFromFloat32(val)
	case float64:
		d = decimal.NewFromFloat(val)
	case int:
		d = decimal.NewFromInt(int64(val))
	case int8:
		d = decimal.NewFromInt(int64(val))
	case int16:
		d = decimal.NewFromInt(int64(val))
	case int32:
		d = decimal.NewFromInt(int64(val))
	case int64:
		d = decimal.NewFromInt(val)
	case uint:
		d = decimal.NewFromInt(int64(val))
	case uint8:
		d = decimal.NewFromInt(int64(val))
	case uint16:
		d = decimal.NewFromInt(int64(val))
	case uint32:
		d = decimal.NewFromInt(int64(val))
	case uint64:
		d = decimal.NewFromBigInt(new(big.Int).SetUint64(val), 0)
	default:
		s := cast.ToString(v)
		dd, err := decimal.NewFromString(s)
		if err != nil {
			return decimal.Zero
		}
		d = dd
	}
	d = d.Round(places + 2)
	return d.Truncate(places)
}

func Recover(logger *zap.Logger, name string) {
	if r := recover(); r != nil {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		logger.Error(fmt.Sprintf("panic: %s %v\n%s", name, r, string(buf[:n])))
	}
}
