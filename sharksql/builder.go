package sharksql

import (
	"fmt"
	"reflect"
	"strings"
)

type group struct {
	conditions []string
	args       []any
}

// Builder 动态 SQL 条件构建器。
//
// 设计规则：
//   - 同一 group 内的条件以 AND 连接。
//   - 不同 group 之间以 OR 连接。
//   - 空值（nil / 空切片 / 空 map）自动跳过，无需手动判空。
//
// 使用示例：
//
// 1. 简单等值查询
//
//	b.Eq("name", "张三")
//	// → name = ?
//
// 2. 多条件 AND 查询（同一 group）
//
//	b.Eq("status", 1).Gte("age", 18).Lt("age", 60)
//	// → (status = ? AND age >= ? AND age < ?)
//
// 3. OR 查询（不同 group）
//
//	b.Eq("created_by", uid).Or(NewBuilder().Eq("assignee", uid))
//	// → (created_by = ?) OR (assignee = ?)
//
// 4. (a = ?) AND (b = ? OR c = ?)
//
//	b := NewBuilder().Eq("a", 1)
//	sub := NewBuilder().Eq("b", 2).Or(NewBuilder().Eq("c", 3))
//	b.And(sub)
//	// → (a = ? AND (b = ? OR c = ?))
//
// 5. 嵌套 AND/OR：查询"待处理或处理中，且为本人相关"的任务
//
//	b.Eq("deleted", 0)
//	statusB := NewBuilder().Eq("status", "pending").Or(NewBuilder().Eq("status", "in_progress"))
//	b.And(statusB)
//	peopleB := NewBuilder().Eq("created_by", uid).Or(NewBuilder().Eq("assignee", uid))
//	b.And(peopleB)
//	// → (deleted = ? AND (status = ? OR status = ?) AND (created_by = ? OR assignee = ?))
//
// 6. 区间查询 Between（左闭右开）
//
//	b.Between("created_at", startTime, endTime)
//	// → created_at >= ? AND created_at < ?
//
// 7. 模糊搜索 + 空值自动跳过
//
//	b.Eq("type", "").Like("title", "订单").LikeRight("code", "ORD")
//	// type 为空跳过 → (title LIKE ? AND code LIKE ?) 参数: [%订单%, ORD%]
//
// 8. IN 查询 + IS NULL
//
//	b.In("category", []int{1, 2, 3}).IsNull("deleted_at")
//	// → (category IN ? AND deleted_at IS NULL)
//
// 9. 混合查询：多条件 + OR + Between + And
//
//	b.Eq("org_id", orgID).Between("created_at", start, end).Eq("deleted", 0)
//	tagB := NewBuilder().Like("tags", "紧急").Or(NewBuilder().Like("tags", "重要"))
//	b.And(tagB)
//	// → (org_id = ? AND created_at >= ? AND created_at < ? AND deleted = ? AND (tags LIKE ? OR tags LIKE ?))
type Builder struct {
	groups []group
}

// NewBuilder 创建一个空的 Builder。
func NewBuilder() *Builder {
	return &Builder{
		groups: []group{},
	}
}

// isEmpty 判断值是否为空（nil / 空指针 / 空切片 / 空 map）。
func (t *Builder) isEmpty(v any) bool {
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
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0
	}
	return false
}

// isSlice 判断值是否为切片或数组。
func (t *Builder) isSlice(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return true
	default:
		return false
	}
}

// current 返回当前（最后一个）group，若不存在则创建。
func (b *Builder) current() *group {
	if len(b.groups) == 0 {
		b.groups = append(b.groups, group{})
	}
	return &b.groups[len(b.groups)-1]

}

// Eq 添加等值条件：column = ?。
// 为空值时跳过。
//
// 示例：b.Eq("name", "张三")  →  name = ?
func (b *Builder) Eq(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" = ?")
	g.args = append(g.args, value)
	return b
}

// Neq 添加不等条件：column <> ?。
// 为空值时跳过。
//
// 示例：b.Neq("status", 0)  →  status <> ?
func (b *Builder) Neq(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" <> ?")
	g.args = append(g.args, value)
	return b
}

// Gt 添加大于条件：column > ?。
// 为空值时跳过。
//
// 示例：b.Gt("age", 18)  →  age > ?
func (b *Builder) Gt(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" > ?")
	g.args = append(g.args, value)
	return b
}

// Gte 添加大于等于条件：column >= ?。
// 为空值时跳过。
//
// 示例：b.Gte("score", 60)  →  score >= ?
func (b *Builder) Gte(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" >= ?")
	g.args = append(g.args, value)
	return b
}

// Lt 添加小于条件：column < ?。
// 为空值时跳过。
//
// 示例：b.Lt("price", 100)  →  price < ?
func (b *Builder) Lt(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" < ?")
	g.args = append(g.args, value)
	return b
}

// Lte 添加小于等于条件：column <= ?。
// 为空值时跳过。
//
// 示例：b.Lte("stock", 50)  →  stock <= ?
func (b *Builder) Lte(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" <= ?")
	g.args = append(g.args, value)
	return b
}

// Between 添加左闭右开区间条件：[lo, hi)，即 column >= ? AND column < ?。
// lo 或 hi 为空值时跳过。
//
// 示例：b.Between("age", 18, 60)  →  age >= ? AND age < ?
func (b *Builder) Between(column string, lo, hi any) *Builder {
	if b.isEmpty(lo) || b.isEmpty(hi) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" >= ?", column+" < ?")
	g.args = append(g.args, lo, hi)
	return b
}

// Like 添加模糊匹配条件：column LIKE '%value%'（前后通配）。
// 为空值时跳过。
//
// 示例：b.Like("name", "张")  →  name LIKE ?  参数: %张%
func (b *Builder) Like(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" LIKE ?")
	g.args = append(g.args, "%"+fmt.Sprint(value)+"%")
	return b
}

// NotLike 添加反向模糊匹配条件：column NOT LIKE '%value%'。
// 为空值时跳过。
//
// 示例：b.NotLike("name", "test")  →  name NOT LIKE ?  参数: %test%
func (b *Builder) NotLike(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" NOT LIKE ?")
	g.args = append(g.args, "%"+fmt.Sprint(value)+"%")
	return b
}

// LikeLeft 添加后缀匹配条件：column LIKE '%value'，匹配以 value 结尾的字符串。
// 为空值时跳过。
//
// 示例：b.LikeLeft("email", "@qq.com")  →  email LIKE ?  参数: %@qq.com
func (b *Builder) LikeLeft(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" LIKE ?")
	g.args = append(g.args, "%"+fmt.Sprint(value))
	return b
}

// LikeRight 添加前缀匹配条件：column LIKE 'value%'，匹配以 value 开头的字符串。
// 为空值时跳过。
//
// 示例：b.LikeRight("phone", "138")  →  phone LIKE ?  参数: 138%
func (b *Builder) LikeRight(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" LIKE ?")
	g.args = append(g.args, fmt.Sprint(value)+"%")
	return b
}

// In 添加 IN 条件：column IN (?, ?, ...)。
// value 必须是切片/数组，为空或非切片时跳过。
//
// 示例：b.In("status", []int{1, 2, 3})  →  status IN ?
func (b *Builder) In(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	if !b.isSlice(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" IN ?")
	g.args = append(g.args, value)
	return b
}

// NotIn 添加 NOT IN 条件：column NOT IN (?, ?, ...)。
// value 必须是切片/数组，为空或非切片时跳过。
//
// 示例：b.NotIn("id", []int64{100, 200})  →  id NOT IN ?
func (b *Builder) NotIn(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	if !b.isSlice(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" NOT IN ?")
	g.args = append(g.args, value)
	return b
}

// IsNull 添加 IS NULL 条件。
//
// 示例：b.IsNull("deleted_at")  →  deleted_at IS NULL
func (b *Builder) IsNull(column string) *Builder {
	g := b.current()
	g.conditions = append(g.conditions, column+" IS NULL")
	return b
}

// IsNotNull 添加 IS NOT NULL 条件。
//
// 示例：b.IsNotNull("email")  →  email IS NOT NULL
func (b *Builder) IsNotNull(column string) *Builder {
	g := b.current()
	g.conditions = append(g.conditions, column+" IS NOT NULL")
	return b
}

// Or 以 OR 方式合并另一个 Builder 的全部条件。
// other 为 nil 或无有效条件时跳过。
//
// 示例：
//
//	b1 := NewBuilder().Eq("a", 1)                           // a = 1
//	b2 := NewBuilder().Eq("b", 2).Eq("c", 3)                // b = 2 AND c = 3
//	b1.Or(b2)                                                // (a = 1) OR (b = 2 AND c = 3)
func (b *Builder) Or(other *Builder) *Builder {
	if other == nil {
		return b
	}
	if len(other.groups) == 0 {
		return b
	}
	b.groups = append(b.groups, other.groups...)
	return b
}

// And 以 AND 方式合并另一个 Builder 的全部条件。
// other 为 nil 或无有效条件时跳过。
//
// 如果 other 只有单个 group（纯 AND 条件），直接合并到当前 group。
// 如果 other 有多个 group（含 OR 子句），先构建子表达式 (...) 再嵌入。
//
// 示例：
//
//	// (a = 2) AND (b = 1 OR b = 2)
//	b := NewBuilder().Eq("a", 2)
//	sub := NewBuilder().Eq("b", 1).Or(NewBuilder().Eq("b", 2))
//	b.And(sub)
//	// 结果: (a = ? AND (b = ? OR b = ?))
func (b *Builder) And(other *Builder) *Builder {
	if other == nil {
		return b
	}
	if len(other.groups) == 0 {
		return b
	}
	g := b.current()
	if len(other.groups) == 1 {
		g.conditions = append(g.conditions, other.groups[0].conditions...)
		g.args = append(g.args, other.groups[0].args...)
		return b
	}
	subSQL, subArgs := other.Build()
	if subSQL == "" {
		return b
	}
	g.conditions = append(g.conditions, "("+subSQL+")")
	g.args = append(g.args, subArgs...)
	return b
}

// Build 生成最终的 WHERE 子句和参数列表。
// 返回的 SQL 字符串可直接用于 WHERE 后，参数列表用于参数化查询。
//
// 示例：
//
//	b := NewBuilder().Eq("a", 1).Between("b", 10, 20)
//	sql, args := b.Build()
//	// sql:  (a = ? AND b >= ? AND b < ?)
//	// args: [1, 10, 20]
func (b *Builder) Build() (string, []any) {
	var sb strings.Builder
	var args []any
	first := true
	for _, group := range b.groups {
		if len(group.conditions) == 0 {
			continue
		}
		if !first {
			sb.WriteString(" OR ")
		}
		if len(group.conditions) > 1 {
			sb.WriteString("(")
		}
		sb.WriteString(strings.Join(group.conditions, " AND "))
		if len(group.conditions) > 1 {
			sb.WriteString(")")
		}
		args = append(args, group.args...)
		first = false
	}
	return sb.String(), args
}
