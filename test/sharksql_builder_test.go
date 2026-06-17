package test

import (
	"testing"

	"github.com/lornshark/shark/sharksql"
)

func TestBuilderBasic(t *testing.T) {
	b := sharksql.NewBuilder().Eq("status", 1)
	sql, args := b.Build()
	if sql != "status = ?" {
		t.Errorf("sql = %s, want status = ?", sql)
	}
	if len(args) != 1 || args[0] != 1 {
		t.Errorf("args = %v, want [1]", args)
	}
}

func TestBuilderMultipleAND(t *testing.T) {
	b := sharksql.NewBuilder().
		Eq("status", 1).
		Gte("age", 18).
		Lt("age", 60)
	sql, args := b.Build()
	expected := "(status = ? AND age >= ? AND age < ?)"
	if sql != expected {
		t.Errorf("sql = %s, want %s", sql, expected)
	}
	if len(args) != 3 {
		t.Errorf("args count = %d, want 3", len(args))
	}
}

func TestBuilderOr(t *testing.T) {
	b := sharksql.NewBuilder().
		Eq("created_by", 1).
		Or(sharksql.NewBuilder().Eq("assignee", 1))
	sql, args := b.Build()
	if sql != "created_by = ? OR assignee = ?" {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("args count = %d, want 2", len(args))
	}
}

func TestBuilderAndSingleGroup(t *testing.T) {
	b := sharksql.NewBuilder().Eq("a", 1)
	other := sharksql.NewBuilder().Eq("b", 2).Eq("c", 3)
	b.And(other)
	sql, _ := b.Build()
	if sql != "(a = ? AND b = ? AND c = ?)" {
		t.Errorf("sql = %s", sql)
	}
}

func TestBuilderAndMultiGroup(t *testing.T) {
	b := sharksql.NewBuilder().Eq("deleted", 0)
	sub := sharksql.NewBuilder().Eq("status", "pending").Or(sharksql.NewBuilder().Eq("status", "in_progress"))
	b.And(sub)
	sql, _ := b.Build()
	expected := "(deleted = ? AND (status = ? OR status = ?))"
	if sql != expected {
		t.Errorf("sql = %s, want %s", sql, expected)
	}
}

func TestBuilderEmptyValueSkip(t *testing.T) {
	b := sharksql.NewBuilder().
		Eq("name", nil).
		Eq("status", 1).
		In("ids", []int{})
	sql, args := b.Build()
	if sql != "status = ?" {
		t.Errorf("空值应跳过, sql = %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want [1]", args)
	}
}

func TestBuilderBetween(t *testing.T) {
	b := sharksql.NewBuilder().Between("age", 18, 60)
	sql, args := b.Build()
	expected := "(age >= ? AND age < ?)"
	if sql != expected {
		t.Errorf("sql = %s, want %s", sql, expected)
	}
	if len(args) != 2 || args[0] != 18 || args[1] != 60 {
		t.Errorf("args = %v, want [18, 60]", args)
	}
}

func TestBuilderBetweenNilSkip(t *testing.T) {
	b := sharksql.NewBuilder().Between("age", 18, nil).Eq("status", 1)
	sql, _ := b.Build()
	if sql != "status = ?" {
		t.Errorf("Between with nil should skip, sql = %s", sql)
	}
}

func TestBuilderLike(t *testing.T) {
	b := sharksql.NewBuilder().Like("name", "张")
	sql, args := b.Build()
	if sql != "name LIKE ?" {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 1 || args[0] != "%张%" {
		t.Errorf("args = %v, want [%%张%%]", args)
	}
}

func TestBuilderLikeLeft(t *testing.T) {
	b := sharksql.NewBuilder().LikeLeft("email", "@qq.com")
	_, args := b.Build()
	if args[0] != "%@qq.com" {
		t.Errorf("args[0] = %v, want %%@qq.com", args[0])
	}
}

func TestBuilderLikeRight(t *testing.T) {
	b := sharksql.NewBuilder().LikeRight("phone", "138")
	_, args := b.Build()
	if args[0] != "138%" {
		t.Errorf("args[0] = %v, want 138%%", args[0])
	}
}

func TestBuilderIn(t *testing.T) {
	b := sharksql.NewBuilder().In("status", []int{1, 2, 3})
	sql, _ := b.Build()
	if sql != "status IN ?" {
		t.Errorf("sql = %s", sql)
	}
}

func TestBuilderNotIn(t *testing.T) {
	b := sharksql.NewBuilder().NotIn("id", []int64{100, 200})
	sql, _ := b.Build()
	if sql != "id NOT IN ?" {
		t.Errorf("sql = %s", sql)
	}
}

func TestBuilderInNonSliceSkip(t *testing.T) {
	b := sharksql.NewBuilder().In("status", "not-a-slice").Eq("id", 1)
	sql, _ := b.Build()
	if sql != "id = ?" {
		t.Errorf("非切片 In 应跳过, sql = %s", sql)
	}
}

func TestBuilderIsNull(t *testing.T) {
	b := sharksql.NewBuilder().IsNull("deleted_at").Eq("status", 1)
	sql, _ := b.Build()
	if sql != "(deleted_at IS NULL AND status = ?)" {
		t.Errorf("sql = %s", sql)
	}
}

func TestBuilderIsNotNull(t *testing.T) {
	b := sharksql.NewBuilder().IsNotNull("email")
	sql, _ := b.Build()
	if sql != "email IS NOT NULL" {
		t.Errorf("sql = %s", sql)
	}
}

func TestBuilderNotLike(t *testing.T) {
	b := sharksql.NewBuilder().NotLike("name", "test")
	sql, args := b.Build()
	if sql != "name NOT LIKE ?" {
		t.Errorf("sql = %s", sql)
	}
	if args[0] != "%test%" {
		t.Errorf("args[0] = %v", args[0])
	}
}

func TestBuilderMultiplyOR(t *testing.T) {
	b := sharksql.NewBuilder().
		Eq("status", "pending").
		Or(sharksql.NewBuilder().Eq("status", "in_progress")).
		Or(sharksql.NewBuilder().Eq("status", "done"))
	sql, _ := b.Build()
	expected := "status = ? OR status = ? OR status = ?"
	if sql != expected {
		t.Errorf("sql = %s, want %s", sql, expected)
	}
}

func TestBuilderAllEmpty(t *testing.T) {
	b := sharksql.NewBuilder().
		Eq("name", nil).
		In("ids", []int{}).
		Between("age", nil, 60)
	sql, args := b.Build()
	if sql != "" {
		t.Errorf("空 Builder 应返回空字符串, got %s", sql)
	}
	if len(args) != 0 {
		t.Errorf("空 Builder args 应为空, got %v", args)
	}
}

func TestBuilderComplexNested(t *testing.T) {
	// (deleted = ? AND (status = ? OR status = ?) AND (created_by = ? OR assignee = ?))
	b := sharksql.NewBuilder().Eq("deleted", 0).
		And(sharksql.NewBuilder().Eq("status", "pending").Or(sharksql.NewBuilder().Eq("status", "done"))).
		And(sharksql.NewBuilder().Eq("created_by", 1).Or(sharksql.NewBuilder().Eq("assignee", 2)))
	sql, args := b.Build()
	expected := "(deleted = ? AND (status = ? OR status = ?) AND (created_by = ? OR assignee = ?))"
	if sql != expected {
		t.Errorf("sql = %s, want %s", sql, expected)
	}
	if len(args) != 5 {
		t.Errorf("args count = %d, want 5", len(args))
	}
	t.Logf("复杂嵌套 SQL: %s", sql)
	t.Logf("参数: %v", args)
}

func TestBuilderOrNil(t *testing.T) {
	b := sharksql.NewBuilder().Eq("a", 1).Or(nil)
	sql, _ := b.Build()
	if sql != "a = ?" {
		t.Errorf("Or(nil) should be noop, got %s", sql)
	}
}

func TestBuilderAndNil(t *testing.T) {
	b := sharksql.NewBuilder().Eq("a", 1).And(nil)
	sql, _ := b.Build()
	if sql != "a = ?" {
		t.Errorf("And(nil) should be noop, got %s", sql)
	}
}

func TestBuilderAndEmpty(t *testing.T) {
	b := sharksql.NewBuilder().Eq("a", 1).And(sharksql.NewBuilder())
	sql, _ := b.Build()
	if sql != "a = ?" {
		t.Errorf("And(empty) should be noop, got %s", sql)
	}
}

func TestBuilderNeq(t *testing.T) {
	b := sharksql.NewBuilder().Neq("status", 0)
	sql, _ := b.Build()
	if sql != "status <> ?" {
		t.Errorf("sql = %s, want status <> ?", sql)
	}
}

func TestBuilderGteLte(t *testing.T) {
	b := sharksql.NewBuilder().Gte("score", 60).Lte("score", 100)
	sql, _ := b.Build()
	expected := "(score >= ? AND score <= ?)"
	if sql != expected {
		t.Errorf("sql = %s, want %s", sql, expected)
	}
}
