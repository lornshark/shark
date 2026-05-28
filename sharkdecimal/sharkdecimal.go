package sharkdecimal

import (
	"math/big"

	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
)

const defaultNormalizeScale int32 = 8

// 转换为decimal.Decimal,并截断保留2位小数
func Normalize2(v any) decimal.Decimal {
	return Normalize(v, 2)
}

// 转换为decimal.Decimal,并截断保留4位小数
func Normalize6(v any) decimal.Decimal {
	return Normalize(v, 6)
}

// 转换为decimal.Decimal,并截断保留指定小数位数
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
	d = d.Round(defaultNormalizeScale)
	return d.Truncate(places)
}
