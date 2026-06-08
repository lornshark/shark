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

type Builder struct {
	groups []group
}

func NewBuilder() *Builder {
	return &Builder{
		groups: []group{},
	}
}

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
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return rv.Len() == 0
	}
	return false
}

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

func (b *Builder) current() *group {
	if len(b.groups) == 0 {
		b.groups = append(b.groups, group{})
	}
	return &b.groups[len(b.groups)-1]

}

func (b *Builder) Eq(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" = ?")
	g.args = append(g.args, value)
	return b
}

func (b *Builder) Neq(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" <> ?")
	g.args = append(g.args, value)
	return b
}

func (b *Builder) Gt(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" > ?")
	g.args = append(g.args, value)
	return b
}

func (b *Builder) Gte(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" >= ?")
	g.args = append(g.args, value)
	return b
}

func (b *Builder) Lt(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" < ?")
	g.args = append(g.args, value)
	return b
}

func (b *Builder) Lte(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" <= ?")
	g.args = append(g.args, value)
	return b
}

func (b *Builder) Like(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" LIKE ?")
	g.args = append(g.args, "%"+fmt.Sprint(value)+"%")
	return b
}

func (b *Builder) NotLike(column string, value any) *Builder {
	if b.isEmpty(value) {
		return b
	}
	g := b.current()
	g.conditions = append(g.conditions, column+" NOT LIKE ?")
	g.args = append(g.args, "%"+fmt.Sprint(value)+"%")
	return b
}

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

func (b *Builder) IsNull(column string) *Builder {
	g := b.current()
	g.conditions = append(g.conditions, column+" IS NULL")
	return b
}

func (b *Builder) IsNotNull(column string) *Builder {
	g := b.current()
	g.conditions = append(g.conditions, column+" IS NOT NULL")
	return b
}

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
		sb.WriteString("(")
		sb.WriteString(strings.Join(group.conditions, " AND "))
		sb.WriteString(")")
		args = append(args, group.args...)
		first = false
	}
	return sb.String(), args
}
