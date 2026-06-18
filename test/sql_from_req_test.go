package test

import (
	"encoding/json"
	"testing"

	"github.com/lornshark/shark/sharksql"
	"github.com/shopspring/decimal"
)

// ========== Where 测试 ==========

func TestWherePtrString(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
	}
	n := "张三"
	req := Req{Name: &n}
	sql, args := sharksql.Where(req)
	if sql != "name = ?" {
		t.Errorf("sql = %s, want 'name = ?'", sql)
	}
	if len(args) != 1 || args[0] != "张三" {
		t.Errorf("args = %v", args)
	}
}

func TestWherePtrNil(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
	}
	req := Req{Name: nil}
	sql, args := sharksql.Where(req)
	if sql != "" || len(args) != 0 {
		t.Errorf("sql = %s, want ''", sql)
	}
}

func TestWhereSlicePtr(t *testing.T) {
	type Req struct {
		Ids *[]int `json:"ids"`
	}
	ids := []int{1, 2, 3}
	req := Req{Ids: &ids}
	sql, args := sharksql.Where(req)
	if sql != "ids IN (?)" {
		t.Errorf("sql = %s, want 'ids IN (?)'", sql)
	}
	if len(args) != 1 {
		t.Fatalf("args count = %d", len(args))
	}
}

func TestWhereSlicePtrNil(t *testing.T) {
	type Req struct {
		Ids *[]int `json:"ids"`
	}
	req := Req{Ids: nil}
	sql, _ := sharksql.Where(req)
	if sql != "" {
		t.Errorf("sql = %s, want ''", sql)
	}
}

func TestWhereMapPtrJSON(t *testing.T) {
	type Req struct {
		Tags *map[string]any `json:"tags"`
	}
	m := map[string]any{"color": "red"}
	req := Req{Tags: &m}
	sql, args := sharksql.Where(req)
	if sql != "tags = ?" {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 1 {
		t.Fatalf("args count = %d", len(args))
	}
	s, ok := args[0].(string)
	if !ok {
		t.Fatalf("args[0] not string: %T", args[0])
	}
	var mm map[string]any
	json.Unmarshal([]byte(s), &mm)
	if mm["color"] != "red" {
		t.Errorf("color = %v", mm["color"])
	}
}

func TestWherePtrStructJSON(t *testing.T) {
	type Addr struct {
		City string `json:"city"`
	}
	type Req struct {
		Addr *Addr `json:"addr"`
	}
	req := Req{Addr: &Addr{City: "北京"}}
	sql, args := sharksql.Where(req)
	if sql != "addr = ?" {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 1 {
		t.Fatalf("args count = %d", len(args))
	}
	s, _ := args[0].(string)
	var a Addr
	json.Unmarshal([]byte(s), &a)
	if a.City != "北京" {
		t.Errorf("City = %s", a.City)
	}
}

func TestWhereDecimalNotJSON(t *testing.T) {
	type Req struct {
		Amount *decimal.Decimal `json:"amount"`
	}
	d := decimal.NewFromFloat(99.99)
	req := Req{Amount: &d}
	sql, args := sharksql.Where(req)
	if sql != "amount = ?" {
		t.Errorf("sql = %s", sql)
	}
	dd, ok := args[0].(decimal.Decimal)
	if !ok || !dd.Equals(d) {
		t.Errorf("args[0] = %v", args[0])
	}
}

func TestWhereNonPtrIgnored(t *testing.T) {
	type Req struct {
		Age int `json:"age"`
	}
	req := Req{Age: 18}
	sql, _ := sharksql.Where(req)
	if sql != "" {
		t.Errorf("sql = %s, want ''", sql)
	}
}

func TestWhereNoJsonTagIgnored(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
		Xxx  *string
	}
	n := "hello"
	req := Req{Name: &n, Xxx: &n}
	sql, args := sharksql.Where(req)
	if sql != "name = ?" || args[0] != "hello" {
		t.Errorf("sql = %s, args = %v", sql, args)
	}
}

func TestWhereMixed(t *testing.T) {
	type Req struct {
		UserId *int64  `json:"user_id"`
		Name   *string `json:"name"`
		Ids    *[]int  `json:"ids"`
	}
	uid := int64(1)
	n := "张三"
	ids := []int{1, 2}
	req := Req{UserId: &uid, Name: &n, Ids: &ids}
	sql, args := sharksql.Where(req)
	if sql != "user_id = ? AND name = ? AND ids IN (?)" {
		t.Errorf("sql = %s", sql)
	}
	if len(args) != 3 {
		t.Errorf("args count = %d, want 3", len(args))
	}
}

// ========== ToUpdate 测试 ==========

func TestToUpdatePtr(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
		Age  *int    `json:"age"`
	}
	n := "张三"
	a := 25
	req := Req{Name: &n, Age: &a}
	data := sharksql.ToUpdate(req)
	if len(data) != 2 {
		t.Fatalf("len = %d", len(data))
	}
	if data["name"] != "张三" || data["age"] != 25 {
		t.Errorf("data = %v", data)
	}
}

func TestToUpdateNil(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
		Age  *int    `json:"age"`
	}
	n := "张三"
	req := Req{Name: &n, Age: nil}
	data := sharksql.ToUpdate(req)
	if len(data) != 1 || data["name"] != "张三" {
		t.Errorf("data = %v", data)
	}
}

func TestToUpdateAllNil(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
	}
	req := Req{}
	data := sharksql.ToUpdate(req)
	if data != nil {
		t.Errorf("data = %v", data)
	}
}

func TestToUpdateSlicePtrJSON(t *testing.T) {
	type Req struct {
		Ids *[]int `json:"ids"`
	}
	ids := []int{1, 2, 3}
	req := Req{Ids: &ids}
	data := sharksql.ToUpdate(req)
	if len(data) != 1 {
		t.Fatalf("len = %d", len(data))
	}
	s, ok := data["ids"].(string)
	if !ok {
		t.Fatalf("ids not string: %T", data["ids"])
	}
	var arr []int
	json.Unmarshal([]byte(s), &arr)
	if len(arr) != 3 {
		t.Errorf("arr len = %d", len(arr))
	}
}

func TestToUpdateStructPtrJSON(t *testing.T) {
	type Meta struct {
		V int `json:"v"`
	}
	type Req struct {
		Meta *Meta `json:"meta"`
	}
	req := Req{Meta: &Meta{V: 1}}
	data := sharksql.ToUpdate(req)
	s, ok := data["meta"].(string)
	if !ok {
		t.Fatalf("meta not string: %T", data["meta"])
	}
	var m Meta
	json.Unmarshal([]byte(s), &m)
	if m.V != 1 {
		t.Errorf("V = %d", m.V)
	}
}

func TestToUpdateDecimal(t *testing.T) {
	type Req struct {
		Price *decimal.Decimal `json:"price"`
	}
	d := decimal.NewFromFloat(19.99)
	req := Req{Price: &d}
	data := sharksql.ToUpdate(req)
	dd, ok := data["price"].(decimal.Decimal)
	if !ok || !dd.Equals(d) {
		t.Errorf("price = %v", data["price"])
	}
}

func TestToUpdateNonPtrIgnored(t *testing.T) {
	type Req struct {
		Name *string `json:"name"`
		Age  int     `json:"age"`
	}
	n := "test"
	req := Req{Name: &n, Age: 18}
	data := sharksql.ToUpdate(req)
	if len(data) != 1 || data["name"] != "test" {
		t.Errorf("data = %v", data)
	}
}
