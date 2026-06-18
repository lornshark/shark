package test

import (
	"strings"
	"testing"

	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharksql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newSharkTable 创建一个连接内存 SQLite 的 SharkTable，用于 DryRun 验证 SQL。
func newSharkTable(t *testing.T, tableName string) *sharkdb.SharkTable {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("创建 DryRun DB 失败: %v", err)
	}
	return sharkdb.NewTable(db.Table(tableName))
}

// findAndSQL 执行 Find 并返回生成的 SQL。
func findAndSQL(table *sharkdb.SharkTable) string {
	var result []map[string]any
	table.Gorm().Find(&result)
	return strings.TrimSpace(table.Gorm().Statement.SQL.String())
}

// ========== 基础方法 ==========

func TestSharkTableGorm(t *testing.T) {
	table := newSharkTable(t, "users")
	db := table.Gorm()
	if db == nil {
		t.Error("Gorm() 不应返回 nil")
	}
}

// ========== SELECT / DISTINCT ==========

func TestSharkTableSelect(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Select("id, name")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "id, name") {
		t.Errorf("应包含 'id, name': %s", sql)
	}
}

func TestSharkTableDistinctSingle(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Distinct("status")
	sql := findAndSQL(table)
	if !strings.Contains(strings.ToUpper(sql), "DISTINCT") {
		t.Errorf("应包含 DISTINCT: %s", sql)
	}
}

func TestSharkTableDistinctMultiple(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Distinct("user_id", "org_id")
	sql := findAndSQL(table)
	if !strings.Contains(strings.ToUpper(sql), "DISTINCT") {
		t.Errorf("应包含 DISTINCT: %s", sql)
	}
}

func TestSharkTableDistinctEmpty(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Distinct()
	sql := findAndSQL(table)
	if strings.Contains(strings.ToUpper(sql), "DISTINCT") {
		t.Errorf("空参数不应生成 DISTINCT: %s", sql)
	}
}

// ========== 比较运算符 ==========

func TestSharkTableEq(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Eq("status", 1)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "status = ?") {
		t.Errorf("应包含 'status = ?': %s", sql)
	}
}

func TestSharkTableEqEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Eq("name", nil).Eq("status", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "name = ?") {
		t.Errorf("nil 值应跳过: %s", sql)
	}
	if !strings.Contains(sql, "status = ?") {
		t.Errorf("应包含 status = ?: %s", sql)
	}
}

func TestSharkTableNe(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Ne("status", 0)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "status <> ?") {
		t.Errorf("应包含 'status <> ?': %s", sql)
	}
}

func TestSharkTableGt(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Gt("age", 18)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "age > ?") {
		t.Errorf("应包含 'age > ?': %s", sql)
	}
}

func TestSharkTableGte(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Gte("score", 60)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "score >= ?") {
		t.Errorf("应包含 'score >= ?': %s", sql)
	}
}

func TestSharkTableLt(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Lt("price", 100)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "price < ?") {
		t.Errorf("应包含 'price < ?': %s", sql)
	}
}

func TestSharkTableLe(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Le("stock", 50)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "stock <= ?") {
		t.Errorf("应包含 'stock <= ?': %s", sql)
	}
}

// ========== 范围查询 ==========

func TestSharkTableFromTo(t *testing.T) {
	table := newSharkTable(t, "users")
	table.FromTo("age", 18, 60)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "age >= ?") {
		t.Errorf("应包含 'age >= ?': %s", sql)
	}
	if !strings.Contains(sql, "age < ?") {
		t.Errorf("应包含 'age < ?': %s", sql)
	}
}

func TestSharkTableFromToEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.FromTo("age", nil, 60).Eq("id", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "age") {
		t.Errorf("nil 参数应跳过 FromTo: %s", sql)
	}
	if !strings.Contains(sql, "id = ?") {
		t.Errorf("应包含 id = ?: %s", sql)
	}
}

// ========== 模糊匹配 ==========

func TestSharkTableLike(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Like("name", "张")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "name LIKE ?") {
		t.Errorf("应包含 'name LIKE ?': %s", sql)
	}
}

func TestSharkTableLikeEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Like("name", nil).Eq("id", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "LIKE") {
		t.Errorf("nil 值应跳过 LIKE: %s", sql)
	}
	if !strings.Contains(sql, "id = ?") {
		t.Errorf("应包含 id = ?: %s", sql)
	}
}

func TestSharkTableNotLike(t *testing.T) {
	table := newSharkTable(t, "users")
	table.NotLike("name", "test")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "name NOT LIKE ?") {
		t.Errorf("应包含 'name NOT LIKE ?': %s", sql)
	}
}

func TestSharkTableLikeLeft(t *testing.T) {
	table := newSharkTable(t, "users")
	table.LikeLeft("email", "@qq.com")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "email LIKE ?") {
		t.Errorf("应包含 'email LIKE ?': %s", sql)
	}
}

func TestSharkTableLikeRight(t *testing.T) {
	table := newSharkTable(t, "users")
	table.LikeRight("phone", "138")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "phone LIKE ?") {
		t.Errorf("应包含 'phone LIKE ?': %s", sql)
	}
}

// ========== 集合运算符 ==========

func TestSharkTableIn(t *testing.T) {
	table := newSharkTable(t, "users")
	table.In("status", []int{1, 2, 3})
	sql := findAndSQL(table)
	if !strings.Contains(sql, "status IN ") {
		t.Errorf("应包含 'status IN': %s", sql)
	}
}

func TestSharkTableInEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.In("status", []int{}).Eq("id", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "IN") {
		t.Errorf("空切片应跳过 IN: %s", sql)
	}
	if !strings.Contains(sql, "id = ?") {
		t.Errorf("应包含 id = ?: %s", sql)
	}
}

func TestSharkTableNotIn(t *testing.T) {
	table := newSharkTable(t, "users")
	table.NotIn("id", []int64{1, 2})
	sql := findAndSQL(table)
	if !strings.Contains(sql, "id NOT IN") {
		t.Errorf("应包含 'id NOT IN': %s", sql)
	}
}

// ========== NULL 运算符 ==========

func TestSharkTableIsNull(t *testing.T) {
	table := newSharkTable(t, "users")
	table.IsNull("deleted_at")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "deleted_at IS NULL") {
		t.Errorf("应包含 'deleted_at IS NULL': %s", sql)
	}
}

func TestSharkTableIsNotNull(t *testing.T) {
	table := newSharkTable(t, "users")
	table.IsNotNull("email")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "email IS NOT NULL") {
		t.Errorf("应包含 'email IS NOT NULL': %s", sql)
	}
}

// ========== 排序 ==========

func TestSharkTableAsc(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Asc("id")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "id ASC") {
		t.Errorf("应包含 'id ASC': %s", sql)
	}
}

func TestSharkTableDesc(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Desc("score")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "score DESC") {
		t.Errorf("应包含 'score DESC': %s", sql)
	}
}

// ========== Group ==========

func TestSharkTableGroup(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Group("category", "status")
	sql := findAndSQL(table)
	if !strings.Contains(sql, "GROUP BY category, status") {
		t.Errorf("应包含 GROUP BY: %s", sql)
	}
}

func TestSharkTableGroupEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Group()
	sql := findAndSQL(table)
	if strings.Contains(sql, "GROUP BY") {
		t.Errorf("空参数应跳过 Group: %s", sql)
	}
}

// ========== JSON SELECT 辅助方法 ==========

func TestSharkTableJsonExtract(t *testing.T) {
	table := newSharkTable(t, "users")
	s := table.JsonExtract("metadata", "$.name")
	if s != "JSON_EXTRACT(metadata, '$.name')" {
		t.Errorf("JsonExtract = %s", s)
	}
}

func TestSharkTableJsonExtractMultiple(t *testing.T) {
	table := newSharkTable(t, "users")
	s := table.JsonExtract("metadata", "$.name", "$.age")
	if s != "JSON_EXTRACT(metadata, '$.name', '$.age')" {
		t.Errorf("JsonExtract = %s", s)
	}
}

func TestSharkTableJsonUnquote(t *testing.T) {
	table := newSharkTable(t, "users")
	s := table.JsonUnquote("metadata", "$.city")
	if s != "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.city'))" {
		t.Errorf("JsonUnquote = %s", s)
	}
}

func TestSharkTableJsonLength(t *testing.T) {
	table := newSharkTable(t, "users")
	s := table.JsonLength("tags")
	if s != "JSON_LENGTH(tags)" {
		t.Errorf("JsonLength = %s", s)
	}
}

func TestSharkTableJsonKeys(t *testing.T) {
	table := newSharkTable(t, "users")
	s := table.JsonKeys("metadata")
	if s != "JSON_KEYS(metadata)" {
		t.Errorf("JsonKeys = %s", s)
	}
}

func TestSharkTableJsonType(t *testing.T) {
	table := newSharkTable(t, "users")
	s := table.JsonType("metadata")
	if s != "JSON_TYPE(metadata)" {
		t.Errorf("JsonType = %s", s)
	}
}

// ========== JSON WHERE 条件 ==========

func TestSharkTableJsonSearchOne(t *testing.T) {
	table := newSharkTable(t, "users")
	table.JsonSearchOne("tags", "vip")
	sql := findAndSQL(table)
	if !strings.Contains(strings.ReplaceAll(sql, " ", ""), "JSON_SEARCH(tags,'one',?)ISNOTNULL") {
		t.Errorf("应包含 JSON_SEARCH: %s", sql)
	}
}

func TestSharkTableJsonSearchOneEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.JsonSearchOne("tags", nil).Eq("status", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "JSON_SEARCH") {
		t.Errorf("空值不应生成 JSON_SEARCH: %s", sql)
	}
	if !strings.Contains(sql, "status = ?") {
		t.Errorf("应包含 status = ?: %s", sql)
	}
}

func TestSharkTableJsonContains(t *testing.T) {
	table := newSharkTable(t, "users")
	table.JsonContains("roles", `"admin"`)
	sql := findAndSQL(table)
	if !strings.Contains(strings.ReplaceAll(sql, " ", ""), "JSON_CONTAINS(roles,?)") {
		t.Errorf("应包含 JSON_CONTAINS: %s", sql)
	}
}

func TestSharkTableJsonContainsEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.JsonContains("roles", nil).Eq("id", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "JSON_CONTAINS") {
		t.Errorf("空值不应生成 JSON_CONTAINS: %s", sql)
	}
	if !strings.Contains(sql, "id = ?") {
		t.Errorf("应包含 id = ?: %s", sql)
	}
}

// ========== JSON SELECT 组合使用 ==========

func TestSharkTableJsonSelect(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Select(table.JsonExtract("metadata", "$.name"))
	sql := findAndSQL(table)
	if !strings.Contains(sql, "JSON_EXTRACT(metadata, '$.name')") {
		t.Errorf("应包含 JSON_EXTRACT: %s", sql)
	}
}

// ========== OR 运算符 ==========

func TestSharkTableOr(t *testing.T) {
	table := newSharkTable(t, "users")
	b := sharksql.NewSql().Eq("status", "pending").Or(sharksql.NewSql().Eq("status", "done"))
	table.Eq("deleted", 0).Or(b)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "deleted = ?") {
		t.Errorf("应包含 deleted = ?: %s", sql)
	}
	if !strings.Contains(sql, "OR") {
		t.Errorf("应包含 OR: %s", sql)
	}
}

func TestSharkTableOrMultipleBuilders(t *testing.T) {
	table := newSharkTable(t, "users")
	b1 := sharksql.NewSql().Like("name", "张")
	b2 := sharksql.NewSql().Like("phone", "138")
	table.Or(b1, b2)
	sql := findAndSQL(table)
	if !strings.Contains(sql, "OR") {
		t.Errorf("应包含 OR: %s", sql)
	}
	if !strings.Contains(sql, "name LIKE ?") {
		t.Errorf("应包含 name LIKE ?: %s", sql)
	}
}

func TestSharkTableOrNilSkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Or(nil).Eq("id", 1)
	sql := findAndSQL(table)
	if strings.Contains(sql, "OR") {
		t.Errorf("nil builder 应跳过 OR: %s", sql)
	}
	if !strings.Contains(sql, "id = ?") {
		t.Errorf("应包含 id = ?: %s", sql)
	}
}

func TestSharkTableOrEmptySkip(t *testing.T) {
	table := newSharkTable(t, "users")
	table.Or(sharksql.NewSql()) // 空 Builder
	sql := findAndSQL(table)
	if strings.Contains(sql, "OR") {
		t.Errorf("空 Builder 应跳过 OR: %s", sql)
	}
}

// ========== 链式调用综合测试 ==========

func TestSharkTableChainedQuery(t *testing.T) {
	table := newSharkTable(t, "users")
	table.
		Select("id", "name", "status").
		Eq("status", 1).
		Eq("deleted", 0).
		Gte("age", 18).
		Lt("age", 60).
		Like("name", "张").
		Asc("id").
		Desc("created_at")
	sql := findAndSQL(table)
	checks := []string{
		"id", "name", "status",
		"status = ?",
		"deleted = ?",
		"age >= ?",
		"age < ?",
		"name LIKE ?",
		"id ASC",
		"created_at DESC",
	}
	for _, check := range checks {
		if !strings.Contains(sql, check) {
			t.Errorf("应包含 '%s': %s", check, sql)
		}
	}
}
