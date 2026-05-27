package sharkdb

import (
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

type SharkTable struct {
	db *gorm.DB
}

// NewTable 创建 SharkTable 实例
func NewTable(db *gorm.DB) *SharkTable {
	return &SharkTable{db: db}
}

// isEmpty 判断值是否为空，支持 nil、空字符串、空切片、空数组、空映射等
func (t *SharkTable) isEmpty(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return true
		}
		return t.isEmpty(rv.Elem().Interface())
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return rv.Len() == 0
	}
	return false
}

// Gorm 返回底层的 gorm.DB 实例，供进一步操作使用
func (t *SharkTable) Gorm() *gorm.DB {
	return t.db
}

// TiFlash 设置查询使用 TiFlash 引擎，返回 SharkTable 实例以支持链式调用
// 使用本方法,只能查询单表,不能查询关联表
func (t *SharkTable) SelectWithTiflash(columns ...any) *SharkTable {
	if len(columns) == 0 {
		return t
	}
	query := fmt.Sprintf("/*+ read_from_storage(tiflash[%v]) */ %v", t.db.Statement.Table, columns[0])
	t.db = t.db.Select(query, columns[1:]...)
	return t
}

// Select 设置查询字段，返回 gorm.DB 实例以支持链式调用
func (t *SharkTable) Select(query any, args ...any) *SharkTable {
	t.db = t.db.Select(query, args...)
	return t
}

// Eq 添加等于条件，value 为空时不添加条件
func (t *SharkTable) Eq(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" = ?", value)
	}
	return t
}

// Ne 添加不等于条件，value 为空时不添加条件
func (t *SharkTable) Ne(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" <> ?", value)
	}
	return t
}

// Gt 添加大于条件，value 为空时不添加条件
func (t *SharkTable) Gt(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" > ?", value)
	}
	return t
}

// Gte 添加大于等于条件，value 为空时不添加条件
func (t *SharkTable) Gte(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" >= ?", value)
	}
	return t
}

// Lt 添加小于条件，value 为空时不添加条件
func (t *SharkTable) Lt(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" < ?", value)
	}
	return t
}

// Lte 添加小于等于条件，value 为空时不添加条件
func (t *SharkTable) Le(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" <= ?", value)
	}
	return t
}

// Like 添加模糊匹配条件，value 为空时不添加条件
func (t *SharkTable) Like(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" LIKE ?", "%"+fmt.Sprint(value)+"%")
	}
	return t
}

// NotLike 添加模糊不匹配条件，value 为空时不添加条件
func (t *SharkTable) NotLike(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" NOT LIKE ?", "%"+fmt.Sprint(value)+"%")
	}
	return t
}

// In 添加 IN 条件，value 为空时不添加条件
func (t *SharkTable) In(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" IN ?", value)
	}
	return t
}

// NotIn 添加 NOT IN 条件，value 为空时不添加条件
func (t *SharkTable) NotIn(column string, value any) *SharkTable {
	if !t.isEmpty(value) {
		t.db = t.db.Where(column+" NOT IN ?", value)
	}
	return t
}

// IsNull 添加 IS NULL 条件，value 为空时不添加条件
func (t *SharkTable) IsNull(column string) *SharkTable {
	t.db = t.db.Where(column + " IS NULL")
	return t
}

// IsNotNull 添加 IS NOT NULL 条件，value 为空时不添加条件
func (t *SharkTable) IsNotNull(column string) *SharkTable {
	t.db = t.db.Where(column + " IS NOT NULL")
	return t
}

// Asc 添加升序排序，value 为空时不添加条件
func (t *SharkTable) Asc(column string) *SharkTable {
	if !t.isEmpty(column) {
		t.db = t.db.Order(column + " ASC")
	}
	return t
}

// Desc 添加降序排序，value 为空时不添加条件
func (t *SharkTable) Desc(column string) *SharkTable {
	if !t.isEmpty(column) {
		t.db = t.db.Order(column + " DESC")
	}
	return t
}

// FromTo 添加 [from, to) 区间查询条件，value 为空时不添加条件
func (t *SharkTable) FromTo(column string, from any, to any) *SharkTable {
	if !t.isEmpty(from) && !t.isEmpty(to) {
		t.db = t.db.Where(column+" >= ? AND "+column+" < ?", from, to)
	}
	return t
}

// Group 添加分组条件
func (t *SharkTable) Group(columns ...string) *SharkTable {
	if len(columns) == 0 {
		return t
	}
	t.db = t.db.Group(strings.Join(columns, ", "))
	return t
}
