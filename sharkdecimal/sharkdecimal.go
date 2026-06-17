package sharkdecimal

import (
	"math/big"

	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
)

// minRoundScale Round 的最小精度，用于消除浮点累积误差。
const minRoundScale int32 = 8

// Normalize2 将任意数值转换为 decimal.Decimal，并截断保留 2 位小数。
//
// 适用于金额、价格等需要两位小数的场景。
//
// 示例：
//
//	d := Normalize2(19.999)    // → 19.99
//	d := Normalize2("19.999")  // → 19.99
//	d := Normalize2(20)        // → 20.00
func Normalize2(v any) decimal.Decimal {
	return Normalize(v, 2)
}

// Normalize6 将任意数值转换为 decimal.Decimal，并截断保留 6 位小数。
//
// 适用于汇率、费率等需要高精度小数的场景。
//
// 示例：
//
//	d := Normalize6(3.1415926535) // → 3.141592
//	d := Normalize6("1.23456789") // → 1.234567
func Normalize6(v any) decimal.Decimal {
	return Normalize(v, 6)
}

// Normalize 将任意数值转换为 decimal.Decimal，并截断保留指定小数位数。
//
// 支持的输入类型：
//   - decimal.Decimal — 直接使用
//   - float32 / float64 — 转换为 Decimal（会先高精度 Round 消除浮点误差）
//   - int / int8 / int16 / int32 / int64 — 整数转换
//   - uint / uint8 / uint16 / uint32 / uint64 — 无符号整数转换
//   - string — 解析为 Decimal
//   - 其他类型 — 通过 cast.ToString 转为字符串再解析
//
// 处理流程：
//  1. 类型转换：将 v 转为 decimal.Decimal
//  2. Round(8)：以 8 位精度四舍五入，消除浮点累积误差
//  3. Truncate(places)：截断到目标位数（不做四舍五入）
//
// places < 0 时自动修正为 0。
// 无法转换的值返回 decimal.Zero。
//
// 示例：
//
//	d := Normalize(19.999, 2)     // → 19.99（截断，不是四舍五入）
//	d := Normalize(19.999, 0)     // → 19
//	d := Normalize("abc", 2)      // → 0.00（非法字符串返回 Zero）
func Normalize(v any, places int32) decimal.Decimal {
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
	// 先用不低于 8 位的高精度 Round 消除浮点误差，
	// 精度取 max(places+2, 8)，确保后续 Truncate 不会因 Round 精度不足而丢失低位。
	roundScale := places + 2
	if roundScale < minRoundScale {
		roundScale = minRoundScale
	}
	d = d.Round(roundScale)
	return d.Truncate(places)
}
