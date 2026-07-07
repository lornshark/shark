package sharkexcel

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/lornshark/shark/sharkfunc"
	"github.com/spf13/cast"
	"github.com/xuri/excelize/v2"
)

type SharkExcel struct {
	// Excel文件路径
	FilePath string
	// Excel文件是否包含表头，默认为true
	HasHeader bool
	// 工作表名称，默认为Sheet1
	SheetName string
}

type SharkExcelOption struct {
	// Excel文件是否包含表头，默认为true
	HasHeader *bool
	// 工作表名称，默认为Sheet1
	SheetName *string
}

func OpenFile(filePath string, cfg *SharkExcelOption) *SharkExcel {
	if cfg == nil {
		cfg = &SharkExcelOption{
			HasHeader: sharkfunc.Pointer(true),
			SheetName: sharkfunc.Pointer("Sheet1"),
		}
	}
	if cfg.HasHeader == nil {
		cfg.HasHeader = sharkfunc.Pointer(true)
	}
	if cfg.SheetName == nil {
		cfg.SheetName = sharkfunc.Pointer("Sheet1")
	}
	return &SharkExcel{
		FilePath:  filePath,
		HasHeader: *cfg.HasHeader,
		SheetName: *cfg.SheetName,
	}
}

// setFieldValue 设置反射字段值
func setFieldValue(f reflect.Value, val string) error {
	if !f.CanSet() {
		return nil
	}
	trimmed := strings.TrimSpace(val)
	switch f.Kind() {
	case reflect.String:
		f.SetString(val) // 原样保留
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if trimmed == "" {
			f.SetInt(0)
			return nil
		}
		v, err := cast.ToInt64E(trimmed)
		if err != nil {
			return err
		}
		f.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if trimmed == "" {
			f.SetUint(0)
			return nil
		}
		v, err := cast.ToUint64E(trimmed)
		if err != nil {
			return err
		}
		f.SetUint(v)
	case reflect.Float32, reflect.Float64:
		if trimmed == "" {
			f.SetFloat(0)
			return nil
		}
		v, err := cast.ToFloat64E(trimmed)
		if err != nil {
			return err
		}
		f.SetFloat(v)
	case reflect.Bool:
		if trimmed == "" {
			f.SetBool(false)
			return nil
		}
		v, err := cast.ToBoolE(trimmed)
		if err != nil {
			return err
		}
		f.SetBool(v)
	case reflect.Struct:
		if f.Type().String() == "time.Time" {
			if trimmed == "" {
				f.Set(reflect.ValueOf(time.Time{}))
				return nil
			}
			v, err := cast.ToTimeE(trimmed)
			if err != nil {
				return err
			}
			f.Set(reflect.ValueOf(v))
		}
	case reflect.Ptr:
		if trimmed == "" {
			f.Set(reflect.Zero(f.Type()))
			return nil
		}
		elem := reflect.New(f.Type().Elem())
		err := setFieldValue(elem.Elem(), val)
		if err != nil {
			return err
		}
		f.Set(elem)
	}
	return nil
}

// Scan (values any) - 一次读取 1000 条,从头读到尾
// 支持 values 为回调函数 func(batch []T) error / func(batch []T) bool
// 也支持 values 为指针类型 *[]T （其中 T 可以为 struct，或者 []string）
func (s *SharkExcel) Scan(values any) error {
	if values == nil {
		return fmt.Errorf("values cannot be nil")
	}

	valOf := reflect.ValueOf(values)
	var callbackVal reflect.Value
	var sliceVal reflect.Value
	var sliceType reflect.Type
	var elemType reflect.Type

	if valOf.Kind() == reflect.Func {
		// 回调函数模式
		callbackVal = valOf
		funcType := valOf.Type()
		if funcType.NumIn() != 1 {
			return fmt.Errorf("values callback must have exactly one parameter")
		}
		sliceType = funcType.In(0)
		if sliceType.Kind() != reflect.Slice {
			return fmt.Errorf("callback parameter must be a slice")
		}
		elemType = sliceType.Elem()

		if funcType.NumOut() > 1 {
			return fmt.Errorf("callback must have at most one output parameter")
		}
		if funcType.NumOut() == 1 {
			outType := funcType.Out(0)
			if outType.Name() != "error" && outType.String() != "error" && outType.Kind() != reflect.Bool {
				return fmt.Errorf("callback output must be error or bool")
			}
		}
	} else if valOf.Kind() == reflect.Ptr {
		// 指针模式，必须是切片指针（*[]T）
		sliceVal = valOf.Elem()
		if sliceVal.Kind() != reflect.Slice {
			return fmt.Errorf("values pointer must point to a slice")
		}
		sliceType = sliceVal.Type()
		elemType = sliceType.Elem()
	} else {
		return fmt.Errorf("values must be a function callback or a pointer to slice")
	}

	// 打开文件
	file, err := excelize.OpenFile(s.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 开流式迭代器
	rows, err := file.Rows(s.SheetName)
	if err != nil {
		return err
	}
	defer rows.Close()

	var headers []string
	if s.HasHeader {
		if !rows.Next() {
			return nil // 空白工作表
		}
		headers, err = rows.Columns()
		if err != nil {
			return err
		}
		for i, h := range headers {
			headers[i] = strings.ToLower(strings.TrimSpace(h))
		}
	}

	// 映射 headers 到 struct 字段的索引
	var fieldIndices []int
	if elemType.Kind() == reflect.Struct && len(headers) > 0 {
		fieldIndices = make([]int, len(headers))
		for i := range fieldIndices {
			fieldIndices[i] = -1
		}
		numFields := elemType.NumField()
		for colIdx, headerName := range headers {
			headerNameLower := strings.ToLower(headerName)
			found := false
			for fIdx := 0; fIdx < numFields; fIdx++ {
				sf := elemType.Field(fIdx)
				if strings.ToLower(sf.Name) == headerNameLower {
					fieldIndices[colIdx] = fIdx
					found = true
					break
				}
				csvTag := sf.Tag.Get("csv")
				if csvTag != "" {
					csvName := strings.ToLower(strings.Split(csvTag, ",")[0])
					if csvName == headerNameLower {
						fieldIndices[colIdx] = fIdx
						found = true
						break
					}
				}
				excelTag := sf.Tag.Get("excel") // 支持 excel 独有 tag
				if excelTag != "" {
					excelName := strings.ToLower(strings.Split(excelTag, ",")[0])
					if excelName == headerNameLower {
						fieldIndices[colIdx] = fIdx
						found = true
						break
					}
				}
				gormTag := sf.Tag.Get("gorm")
				if gormTag != "" {
					tags := strings.Split(gormTag, ";")
					for _, tag := range tags {
						if strings.HasPrefix(tag, "column:") {
							column := strings.ToLower(strings.TrimPrefix(tag, "column:"))
							if column == headerNameLower {
								fieldIndices[colIdx] = fIdx
								found = true
								break
							}
						}
					}
					if found {
						break
					}
				}
				jsonTag := sf.Tag.Get("json")
				if jsonTag != "" {
					jsonName := strings.ToLower(strings.Split(jsonTag, ",")[0])
					if jsonName == headerNameLower {
						fieldIndices[colIdx] = fIdx
						found = true
						break
					}
				}
			}
		}
	}

	batchSize := 1000
	var finalSlice reflect.Value
	if sliceVal.IsValid() {
		// 切片指针模式：将其先清空，并为其赋初始 Slice
		sliceVal.Set(reflect.MakeSlice(sliceType, 0, batchSize))
		finalSlice = sliceVal
	}

	for {
		batchSlice := reflect.MakeSlice(sliceType, 0, batchSize)
		eofReached := false

		for i := 0; i < batchSize; i++ {
			if !rows.Next() {
				eofReached = true
				break
			}
			record, err := rows.Columns()
			if err != nil {
				return err
			}

			// 构造单个 T
			elemVal := reflect.New(elemType).Elem()

			if elemType.Kind() == reflect.Slice && elemType.Elem().Kind() == reflect.String {
				elemVal.Set(reflect.ValueOf(record))
			} else if elemType.Kind() == reflect.Struct {
				if len(fieldIndices) == 0 {
					numFields := elemVal.NumField()
					for colIdx := 0; colIdx < len(record) && colIdx < numFields; colIdx++ {
						f := elemVal.Field(colIdx)
						if f.CanSet() {
							_ = setFieldValue(f, record[colIdx])
						}
					}
				} else {
					for colIdx, fIdx := range fieldIndices {
						if fIdx == -1 || colIdx >= len(record) {
							continue
						}
						f := elemVal.Field(fIdx)
						if f.CanSet() {
							_ = setFieldValue(f, record[colIdx])
						}
					}
				}
			} else {
				return fmt.Errorf("unsupported slice element type: %v", elemType)
			}

			batchSlice = reflect.Append(batchSlice, elemVal)
		}

		if batchSlice.Len() == 0 {
			break
		}

		if callbackVal.IsValid() {
			// 回调模式：反射调用
			out := callbackVal.Call([]reflect.Value{batchSlice})
			if len(out) == 1 {
				res := out[0]
				if res.Type().Name() == "error" || res.Type().String() == "error" {
					if !res.IsNil() {
						return res.Interface().(error)
					}
				} else if res.Kind() == reflect.Bool {
					if !res.Bool() {
						break // bool 返回 false 代表提前中断
					}
				}
			}
		} else if sliceVal.IsValid() {
			// 切片指针模式：直接将分批追加到 finalSlice
			finalSlice = reflect.AppendSlice(finalSlice, batchSlice)
		}

		if eofReached {
			break
		}
	}

	if sliceVal.IsValid() {
		sliceVal.Set(finalSlice)
	}

	return nil
}
