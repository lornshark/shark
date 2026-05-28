package sharkjson

import "github.com/bytedance/sonic"

func ParseJsonBytes[T any](value []byte) *T {
	if len(value) == 0 {
		return nil
	}
	var result T
	if err := sonic.Unmarshal(value, &result); err != nil {
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

func ToJsonBytes(v any) []byte {
	if v == nil {
		return nil
	}
	data, err := sonic.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func ToJsonString(v any) string {
	return string(ToJsonBytes(v))
}
