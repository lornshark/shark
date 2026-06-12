package sharkdb

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// TableScan 提供基于 Keyset Pagination（游标分页）的高性能表扫描功能。
// 相比传统 LIMIT/OFFSET 分页：
//
//	SELECT * FROM user LIMIT 100000, 20
//
// TableScan 使用“上一条记录”作为游标进行分页：
//
//	SELECT * FROM user
//	WHERE id > ?
//	ORDER BY id ASC
//	LIMIT 20
//
// 优点：
//   - 深分页性能极高
//   - 不会随着页数增加变慢
//   - 可利用索引范围扫描
//   - 适合大表扫描
//   - 适合增量同步
//   - 适合数据导出
//   - 适合消息消费
//
// 注意事项：
//  1. 排序字段必须建立联合索引
//  2. 排序字段必须稳定
//  3. 最后一个排序字段最好唯一（如 id）
//  4. 不允许排序字段存在 NULL
//
// 推荐：
//
//	OrderAsc("create_time", "id")
//
// 对应索引：
//
//	CREATE INDEX idx_ctime_id ON table(create_time,id);
//
// 支持：
//   - 多字段排序
//   - ASC / DESC
//   - 下一页
//   - 上一页
//   - 继承已有 where 条件
//   - 双向 seek pagination
//
// 示例：
//
//	scaner := sharkdb.NewTableScan[XAdminLog]().
//		PageSize(100).
//		OrderAsc("create_time").
//		OrderAsc("auto_id")
//
// 下一页：
//
//	var last *XAdminLog
//	for {
//		db := app.Db.Table("x_admin_log")
//		results, err := scaner.Next(db, last)
//		if err != nil {
//			panic(err)
//		}
//		if len(results) == 0 {
//			break
//		}
//		// 下一页游标：当前页最后一条
//		last = &results[len(results)-1]
//
//		for _, result := range results {
//			fmt.Println(result.Id)
//		}
//	}
//
// 上一页：
//
//	// 当前页第一条
//	first := &results[0]
//
//	prevResults, err := scaner.Prev(db, first)
//
// SQL 示例：
//
//	OrderAsc("create_time","id")
//
// 下一页：
//
//	WHERE
//	(
//	    create_time > ?
//	)
//	OR
//	(
//	    create_time = ? AND id > ?
//	)
//
//	ORDER BY create_time ASC,id ASC
//
// 上一页：
//
//	WHERE
//	(
//	    create_time < ?
//	)
//	OR
//	(
//	    create_time = ? AND id < ?
//	)
//
//	ORDER BY create_time DESC,id DESC
//
// 然后内部自动 reverse 结果，保证返回顺序始终一致。
//
// 注意：
// Prev() 查询时会自动反转 ORDER BY，
// 查询完成后再 reverse slice，
// 最终返回结果顺序与 Next() 保持一致。

type tableScanOrder struct {
	columns string
	order   string
}
type TableScan[T any] struct {
	pagesize int
	orders   []tableScanOrder
}

func NewTableScan[T any]() *TableScan[T] {
	return &TableScan[T]{}
}

func (p *TableScan[T]) PageSize(size int) *TableScan[T] {
	p.pagesize = size
	return p
}

func (p *TableScan[T]) OrderAsc(columns ...string) *TableScan[T] {
	for _, column := range columns {
		p.orders = append(p.orders, tableScanOrder{columns: column, order: "asc"})
	}
	return p
}

func (p *TableScan[T]) OrderDesc(columns ...string) *TableScan[T] {
	for _, column := range columns {
		p.orders = append(p.orders, tableScanOrder{columns: column, order: "desc"})
	}
	return p
}

func getFieldValue[T any](obj *T, name string) any {
	if obj == nil {
		return nil
	}
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	field := v.FieldByName(name)
	if field.IsValid() && field.CanInterface() {
		return field.Interface()
	}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		gormTag := sf.Tag.Get("gorm")
		if gormTag != "" {
			tags := strings.Split(gormTag, ";")
			for _, tag := range tags {
				if strings.HasPrefix(tag, "column:") {
					column := strings.TrimPrefix(tag, "column:")
					if column == name {
						f := v.Field(i)
						if f.IsValid() && f.CanInterface() {
							return f.Interface()
						}
					}
				}
			}
		}
	}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		jsonTag := sf.Tag.Get("json")
		if jsonTag == "" {
			continue
		}
		jsonName := strings.Split(jsonTag, ",")[0]
		if jsonName == name {
			f := v.Field(i)
			if f.IsValid() && f.CanInterface() {
				return f.Interface()
			}
		}
	}
	return nil
}

func (p *TableScan[T]) Next(db *gorm.DB, last *T) ([]T, error) {
	tx := db.Session(&gorm.Session{})
	for _, order := range p.orders {
		tx = tx.Order(order.columns + " " + order.order)
	}
	if last != nil && len(p.orders) > 0 {
		var orSQL []string
		var args []any
		for i := 0; i < len(p.orders); i++ {
			var andSQL []string
			for j := 0; j < i; j++ {
				col := p.orders[j]
				andSQL = append(andSQL,
					fmt.Sprintf("%s = ?", col.columns),
				)
				args = append(args,
					getFieldValue(last, col.columns),
				)
			}
			col := p.orders[i]
			op := ">"
			if strings.ToLower(col.order) == "desc" {
				op = "<"
			}
			andSQL = append(andSQL,
				fmt.Sprintf("%s %s ?", col.columns, op),
			)
			args = append(args,
				getFieldValue(last, col.columns),
			)
			orSQL = append(orSQL,
				"("+strings.Join(andSQL, " AND ")+")",
			)
		}
		tx = tx.Where(
			strings.Join(orSQL, " OR "),
			args...,
		)
	}
	var list []T
	err := tx.Limit(p.pagesize).Find(&list).Error
	return list, err
}

func (p *TableScan[T]) Prev(db *gorm.DB, first *T) ([]T, error) {
	tx := db.Session(&gorm.Session{})
	for _, order := range p.orders {
		orderType := strings.ToLower(order.order)
		if orderType == "asc" {
			orderType = "desc"
		} else {
			orderType = "asc"
		}
		tx = tx.Order(order.columns + " " + orderType)
	}
	if first != nil && len(p.orders) > 0 {
		var orSQL []string
		var args []any
		for i := 0; i < len(p.orders); i++ {
			var andSQL []string
			for j := 0; j < i; j++ {
				col := p.orders[j]
				andSQL = append(andSQL,
					fmt.Sprintf("%s = ?", col.columns),
				)
				args = append(args,
					getFieldValue(first, col.columns),
				)
			}
			col := p.orders[i]
			op := "<"
			if strings.ToLower(col.order) == "desc" {
				op = ">"
			}
			andSQL = append(andSQL,
				fmt.Sprintf("%s %s ?", col.columns, op),
			)
			args = append(args,
				getFieldValue(first, col.columns),
			)
			orSQL = append(orSQL,
				"("+strings.Join(andSQL, " AND ")+")",
			)
		}
		tx = tx.Where(
			strings.Join(orSQL, " OR "),
			args...,
		)
	}
	var list []T
	err := tx.Limit(p.pagesize).Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

func (p *TableScan[T]) Export(db *gorm.DB, name string, header []any, cb func(T) []any) (string, error) {
	excelFile := excelize.NewFile()
	defer excelFile.Close()
	streamWriter, err := excelFile.NewStreamWriter("Sheet1")
	if err != nil {
		return "", err
	}
	if err := streamWriter.SetRow("A1", header); err != nil {
		return "", err
	}
	index := 0
	var last *T
	for {
		values, err := p.Next(db, last)
		if err != nil {
			return "", err
		}
		if len(values) == 0 {
			break
		}
		for i := 0; i < len(values); i++ {
			row := cb(values[i])
			d := make([]any, 0, len(row))
			for _, v := range row {
				d = append(d, excelize.Cell{StyleID: 49, Value: fmt.Sprint(v)})
			}
			cell, _ := excelize.CoordinatesToCellName(1, index+2)
			if err := streamWriter.SetRow(cell, d); err != nil {
				return "", err
			}
			index++
		}
		last = &values[len(values)-1]
	}
	if err := streamWriter.Flush(); err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("%v_%v.xlsx", name, time.Now().Format("20060102150405"))
	if err := excelFile.SaveAs(path.Join(os.TempDir(), fileName)); err != nil {
		return "", err
	}
	return fileName, nil
}
