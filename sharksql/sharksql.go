// Package sharksql 提供 SQL 条件构建器和常用数据库操作辅助函数。
//
// 核心功能：
//  1. 条件构建器：一组链式函数，将 Go 表达式转换为参数化的 SQL WHERE 条件片段
//  2. 聚合函数构建器：Sum/Count/Avg/Max/Min 及其带别名的变体
//  3. 排序构建器：Asc/Desc 生成 ORDER BY 子句
//  4. JSON 操作：MySQL JSON 函数封装（JSON_SEARCH、JSON_CONTAINS、JSON_ARRAY_APPEND、JSON_SET）
//  5. 分页查询：泛型分页函数 PageQuery，自动计算 total + offset + limit
//  6. 工具函数：重复键检测、JSON 路径构建、表名.列名拼接
//
// 设计理念：
//   - 全部函数返回参数化条件（条件字符串 + 参数值），天然防止 SQL 注入
//   - 条件函数与 GORM 的 Where()/Having() 等方法无缝配合
//   - 泛型分页函数 PageQuery 支持任意 GORM 模型类型
//
// 使用示例：
//
//	import "github.com/lornshark/shark/sharksql"
//
//	// 构建查询条件
//	db.Where(
//	    sharksql.Eq("status", 1),
//	    sharksql.Gte("age", 18),
//	    sharksql.Like("name", "张三"),
//	).Find(&users)
//
//	// 分页查询
//	users, total, err := sharksql.PageQuery[User](db, 1, 20)
//
//	// JSON 字段操作
//	db.Update("tags", gorm.Expr(
//	    sharksql.JsonArrayAppend("tags", "new_tag"),
//	))
package sharksql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/go-sql-driver/mysql"
	"github.com/lornshark/shark/sharkjson"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// Pagination 定义分页请求参数。
//
// 字段说明：
//   - Page:     页码，从 1 开始（Page < 1 时自动修正为 1）
//   - PageSize: 每页条数（PageSize < 10 时自动修正为 10）
//
// 使用示例：
//
//	type ListUsersReq struct {
//	    sharksql.Pagination          // 嵌入分页参数
//	    Name     string `json:"name"`  // 业务筛选字段
//	    Status   int    `json:"status"`
//	}
//
//	req := ListUsersReq{
//	    Pagination: sharksql.Pagination{Page: 1, PageSize: 20},
//	    Status:     1,
//	}
type Pagination struct {
	Page     int `json:"page"`      // 页码，从 1 开始
	PageSize int `json:"page_size"` // 每页条数
}

// ========== 比较运算符（WHERE 条件构建）==========

// Eq 构建等于条件（=）。
//
// 示例：
//
//	// SELECT * FROM users WHERE status = 1
//	db.Where(sharksql.Eq("status", 1)).Find(&users)
//
//	// 多条件配合
//	db.Where(
//	    sharksql.Eq("status", 1),
//	    sharksql.Eq("deleted", 0),
//	).Find(&users)
func Eq(column string, value any) (string, any) {
	return column + " = ?", value
}

// Neq 构建不等于条件（<>）。
//
// 示例：
//
//	// SELECT * FROM users WHERE status <> 0
//	db.Where(sharksql.Neq("status", 0)).Find(&users)
func Neq(column string, value any) (string, any) {
	return column + " <> ?", value
}

// Gt 构建大于条件（>）。
//
// 示例：
//
//	// SELECT * FROM users WHERE age > 18
//	db.Where(sharksql.Gt("age", 18)).Find(&users)
//
//	// 大于指定时间
//	db.Where(sharksql.Gt("created_at", time.Now().Add(-24*time.Hour))).Find(&users)
func Gt(column string, value any) (string, any) {
	return column + " > ?", value
}

// Gte 构建大于等于条件（>=）。
//
// 示例：
//
//	// SELECT * FROM orders WHERE amount >= 100
//	db.Where(sharksql.Gte("amount", 100)).Find(&orders)
func Gte(column string, value any) (string, any) {
	return column + " >= ?", value
}

// Lt 构建小于条件（<）。
//
// 示例：
//
//	// SELECT * FROM users WHERE age < 60
//	db.Where(sharksql.Lt("age", 60)).Find(&users)
func Lt(column string, value any) (string, any) {
	return column + " < ?", value
}

// Lte 构建小于等于条件（<=）。
//
// 示例：
//
//	// SELECT * FROM products WHERE price <= 5000
//	db.Where(sharksql.Lte("price", 5000)).Find(&products)
func Lte(column string, value any) (string, any) {
	return column + " <= ?", value
}

// ========== 模糊匹配运算符 ==========

// Like 构建模糊匹配条件（LIKE %value%）。
//
// 注意：value 会自动被 % 包裹，无需手动添加通配符。
//
// 示例：
//
//	// SELECT * FROM users WHERE name LIKE '%张三%'
//	db.Where(sharksql.Like("name", "张三")).Find(&users)
//
//	// 多字段模糊搜索
//	db.Where(
//	    sharksql.Or(
//	        sharksql.Like("name", keyword),
//	        sharksql.Like("email", keyword),
//	    ),
//	).Find(&users)
func Like(column string, value any) (string, any) {
	return column + " LIKE ?", "%" + fmt.Sprint(value) + "%"
}

// NotLike 构建反向模糊匹配条件（NOT LIKE %value%）。
//
// 示例：
//
//	// SELECT * FROM users WHERE name NOT LIKE '%test%'
//	db.Where(sharksql.NotLike("name", "test")).Find(&users)
func NotLike(column string, value any) (string, any) {
	return column + " NOT LIKE ?", "%" + fmt.Sprint(value) + "%"
}

// ========== 集合运算符 ==========

// In 构建 IN 条件。
//
// 注意：GORM 会自动展开切片参数，value 应传入切片类型。
//
// 示例：
//
//	// SELECT * FROM users WHERE status IN (1, 2, 3)
//	db.Where(sharksql.In("status", []int{1, 2, 3})).Find(&users)
//
//	// 结合字符串切片
//	db.Where(sharksql.In("city", []string{"北京", "上海", "深圳"})).Find(&users)
func In(column string, value any) (string, any) {
	return column + " IN (?)", value
}

// NotIn 构建 NOT IN 条件。
//
// 示例：
//
//	// SELECT * FROM users WHERE status NOT IN (4, 5)
//	db.Where(sharksql.NotIn("status", []int{4, 5})).Find(&users)
func NotIn(column string, value any) (string, any) {
	return column + " NOT IN (?)", value
}

// ========== NULL 运算符 ==========

// IsNull 构建 IS NULL 条件。
//
// 示例：
//
//	// SELECT * FROM users WHERE deleted_at IS NULL
//	db.Where(sharksql.IsNull("deleted_at")).Find(&users)
func IsNull(column string) string {
	return column + " IS NULL"
}

// IsNotNull 构建 IS NOT NULL 条件。
//
// 示例：
//
//	// SELECT * FROM users WHERE email IS NOT NULL
//	db.Where(sharksql.IsNotNull("email")).Find(&users)
func IsNotNull(column string) string {
	return column + " IS NOT NULL"
}

// ========== 算术运算符（用于 UPDATE SET 子句）==========

// Add 构建字段加法表达式（column + value）。
// 通常配合 Update 的 SET 子句使用。
//
// 示例：
//
//	// UPDATE accounts SET balance = balance + 100 WHERE id = 1
//	db.Model(&Account{}).Where("id = ?", 1).
//	    Update("balance", gorm.Expr(sharksql.Add("balance", 100)))
func Add(column string, value any) (string, any) {
	return column + " + ?", value
}

// Sub 构建字段减法表达式（column - value）。
//
// 示例：
//
//	// UPDATE accounts SET balance = balance - 50 WHERE id = 1
//	db.Model(&Account{}).Where("id = ?", 1).
//	    Update("balance", gorm.Expr(sharksql.Sub("balance", 50)))
func Sub(column string, value any) (string, any) {
	return column + " - ?", value
}

// Mul 构建字段乘法表达式（column * value）。
//
// 示例：
//
//	// UPDATE products SET price = price * 1.1 WHERE category = 'electronics'
//	db.Model(&Product{}).Where("category = ?", "electronics").
//	    Update("price", gorm.Expr(sharksql.Mul("price", 1.1)))
func Mul(column string, value any) (string, any) {
	return column + " * ?", value
}

// Div 构建字段除法表达式（column / value）。
//
// 示例：
//
//	// UPDATE scores SET avg_score = total_score / count WHERE count > 0
//	db.Model(&Score{}).Where("count > ?", 0).
//	    Update("avg_score", gorm.Expr(sharksql.Div("total_score", "count")))
func Div(column string, value any) (string, any) {
	return column + " / ?", value
}

// ========== 字段对字段算术运算符（SELECT/UPDATE 子句）==========

// AddCol 构建字段对字段加法表达式：column + otherColumn。
// 用于 SELECT 子句中将两个字段值相加。
//
// 示例：
//
//	// UPDATE accounts SET total = principal + interest
//	db.Update("total", gorm.Expr(sharksql.AddCol("principal", "interest")))
func AddCol(column string, otherColumn string) string {
	return column + " + " + otherColumn
}

// AddColAs 构建字段对字段加法表达式并指定别名：(column + otherColumn) as alias。
// 用于 SELECT 子句中将两个字段值相加并指定别名。
//
// 示例：
//
//	// SELECT (base_salary + bonus) as total FROM employees
//	db.Select(sharksql.AddColAs("base_salary", "bonus", "total")).Find(&results)
func AddColAs(column string, otherColumn string, as string) string {
	return fmt.Sprintf("(%v + %v) as %v", column, otherColumn, as)
}

// SubCol 构建字段对字段减法表达式：column - otherColumn。
// 用于 SELECT 子句中将两个字段值相减。
//
// 示例：
//
//	// UPDATE accounts SET profit = revenue - cost
//	db.Update("profit", gorm.Expr(sharksql.SubCol("revenue", "cost")))
func SubCol(column string, otherColumn string) string {
	return column + " - " + otherColumn
}

// SubColAs 构建字段对字段减法表达式并指定别名：(column - otherColumn) as alias。
// 用于 SELECT 子句中将两个字段值相减并指定别名。
//
// 示例：
//
//	// SELECT (revenue - cost) as profit FROM accounts
//	db.Select(sharksql.SubColAs("revenue", "cost", "profit")).Find(&results)
func SubColAs(column string, otherColumn string, as string) string {
	return fmt.Sprintf("(%v - %v) as %v", column, otherColumn, as)
}

// MulCol 构建字段对字段乘法表达式：column * otherColumn。
// 用于 SELECT 子句中将两个字段值相乘。
//
// 示例：
//
//	// UPDATE products SET total = unit_price * amount
//	db.Update("total", gorm.Expr(sharksql.MulCol("unit_price", "amount")))
func MulCol(column string, otherColumn string) string {
	return column + " * " + otherColumn
}

// MulColAs 构建字段对字段乘法表达式并指定别名：(column * otherColumn) as alias。
// 用于 SELECT 子句中将两个字段值相乘并指定别名。
//
// 示例：
//
//	// SELECT (price * quantity) as total_amount FROM orders
//	db.Select(sharksql.MulColAs("price", "quantity", "total_amount")).Find(&results)
func MulColAs(column string, otherColumn string, as string) string {
	return fmt.Sprintf("(%v * %v) as %v", column, otherColumn, as)
}

// DivCol 构建字段对字段除法表达式：column / otherColumn。
// 用于 SELECT 子句中将两个字段值相除。
//
// 示例：
//
//	// UPDATE scores SET avg = total_score / count WHERE count > 0
//	db.Update("avg", gorm.Expr(sharksql.DivCol("total_score", "count")))
func DivCol(column string, otherColumn string) string {
	return column + " / " + otherColumn
}

// DivColAs 构建字段对字段除法表达式并指定别名：(column / otherColumn) as alias。
// 用于 SELECT 子句中将两个字段值相除并指定别名。
//
// 示例：
//
//	// SELECT (total_score / count) as avg_score FROM scores
//	db.Select(sharksql.DivColAs("total_score", "count", "avg_score")).Find(&results)
func DivColAs(column string, otherColumn string, as string) string {
	return fmt.Sprintf("(%v / %v) as %v", column, otherColumn, as)
}

// ========== 别名/表达式辅助 ==========

// As 为表达式添加别名：(expression) as alias。
// 可与任意字段表达式配合使用（AddCol/SubCol/MulCol/DivCol/聚合函数等）。
//
// 示例：
//
//	// SELECT (price * quantity) as total_amount FROM orders
//	db.Select(sharksql.As(sharksql.MulCol("price", "quantity"), "total_amount")).Find(&results)
//
//	// SELECT (revenue - cost) as profit FROM accounts
//	db.Select(sharksql.As(sharksql.SubCol("revenue", "cost"), "profit")).Find(&results)
//
//	// SELECT (principal + interest) as total FROM accounts
//	db.Select(sharksql.As(sharksql.AddCol("principal", "interest"), "total")).Find(&results)
func As(expression string, alias string) string {
	return fmt.Sprintf("(%v) as %v", expression, alias)
}

// Paren 为列名或表达式添加括号：(column)。
// 用于需要显式括号包裹的 SQL 场景，如子查询中的表达式、复杂 WHERE 条件等。
//
// 示例：
//
//	// SELECT (score) FROM exams
//	db.Select(sharksql.Paren("score")).Find(&results)
//
//	// SELECT (base_salary + bonus) as total FROM employees
//	db.Select(sharksql.As(sharksql.Paren(sharksql.AddCol("base_salary", "bonus")), "total")).Find(&results)
//
//	// WHERE (age >= 18 AND age <= 60)
//	db.Where(sharksql.Paren("age >= 18 AND age <= 60")).Find(&users)
func Paren(column string) string {
	return fmt.Sprintf("(%v)", column)
}

// ========== 排序构建器（ORDER BY）==========

// Asc 构建升序排序表达式。
//
// 示例：
//
//	// SELECT * FROM users ORDER BY created_at ASC
//	db.Order(sharksql.Asc("created_at")).Find(&users)
//
//	// 多字段排序
//	db.Order(
//	    sharksql.Asc("status"),
//	    sharksql.Desc("created_at"),
//	).Find(&users)
func Asc(column string) string {
	return column + " ASC"
}

// Desc 构建降序排序表达式。
//
// 示例：
//
//	// SELECT * FROM orders ORDER BY amount DESC
//	db.Order(sharksql.Desc("amount")).Find(&orders)
func Desc(column string) string {
	return column + " DESC"
}

// ========== 范围查询 ==========

// Between 构建左闭右开区间条件 [lo, hi)：column >= ? AND column < ?。
// 与 FromTo 等价，语义更清晰。
//
// 示例：
//
//	// SELECT * FROM orders WHERE amount >= ? AND amount < ?
//	db.Where(sharksql.Between("amount", 100, 500)).Find(&orders)
func Between(column string, lo any, hi any) (string, any, any) {
	return column + " >= ? AND " + column + " < ?", lo, hi
}

// FromTo 构建左闭右开区间条件 [from, to)。
// 等价于: column >= from AND column < to
//
// 示例：
//
//	// SELECT * FROM orders WHERE created_at >= '2025-01-01' AND created_at < '2025-02-01'
//	db.Where(sharksql.FromTo("created_at", "2025-01-01", "2025-02-01")).Find(&orders)
//
//	// 数值范围查询
//	db.Where(sharksql.FromTo("age", 18, 60)).Find(&users)
func FromTo(column string, from any, to any) (string, any, any) {
	return column + " >= ? AND " + column + " < ?", from, to
}

// ========== 聚合函数构建器（SELECT 子句）==========

// Count 构建 COUNT 聚合表达式。
// 用于 SELECT 子句中的行计数。
//
// 示例：
//
//	// SELECT count(id) FROM users
//	db.Select(sharksql.Count("id")).Find(&result)
//
//	// SELECT count(*) FROM users
//	db.Select(sharksql.Count("*")).Find(&result)
//
//	// SELECT count(DISTINCT user_id) FROM orders
//	db.Select(sharksql.Count("DISTINCT user_id")).Find(&result)
func Count(column string) string {
	return fmt.Sprintf("count(%v)", column)
}

// Sum 构建 SUM 聚合表达式。
// 多个字段以逗号分隔，不追加别名。
//
// 示例：
//
//	// SELECT sum(bet_amount), sum(win_amount) FROM orders
//	db.Select(sharksql.Sum("bet_amount", "win_amount")).Find(&result)
func Sum(column ...string) string {
	sql := ""
	for i := 0; i < len(column); i++ {
		sql += fmt.Sprintf("sum(%v), ", column[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// SumAs 构建 SUM 聚合表达式，支持自定义别名。
// 参数需成对出现：字段名, 别名, 字段名, 别名 ...
//
// 示例：
//
//	// SELECT sum(bet_amount) AS total_bet, sum(win_amount) AS total_win FROM orders
//	db.Select(sharksql.SumAs("bet_amount", "total_bet", "win_amount", "total_win")).Find(&result)
func SumAs(column ...string) string {
	if len(column)%2 != 0 {
		return ""
	}
	sql := ""
	for i := 0; i < len(column); i += 2 {
		if i+1 < len(column) {
			sql += fmt.Sprintf("sum(%v) as %v, ", column[i], column[i+1])
		}
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// CountAs 构建 COUNT 聚合表达式，支持自定义别名。
// 参数需成对出现：字段名, 别名, 字段名, 别名 ...
//
// 示例：
//
//	// SELECT count(id) AS total_count, count(DISTINCT user_id) AS unique_users FROM orders
//	db.Select(sharksql.CountAs("id", "total_count", "user_id", "unique_users")).Find(&result)
func CountAs(columns ...string) string {
	if len(columns)%2 != 0 {
		return ""
	}
	sql := ""
	for i := 0; i < len(columns); i += 2 {
		if i+1 < len(columns) {
			sql += fmt.Sprintf("count(%v) as %v, ", columns[i], columns[i+1])
		}
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// Avg 构建 AVG 聚合表达式。
// 多个字段以逗号分隔，不追加别名。
//
// 示例：
//
//	// SELECT avg(score) FROM exams
//	db.Select(sharksql.Avg("score")).Find(&result)
//
//	// 多字段平均
//	db.Select(sharksql.Avg("math_score", "english_score")).Find(&result)
func Avg(columns ...string) string {
	sql := ""
	for i := 0; i < len(columns); i++ {
		sql += fmt.Sprintf("avg(%v), ", columns[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// AvgAs 构建 AVG 聚合表达式，支持自定义别名。
// 参数需成对出现：字段名, 别名, 字段名, 别名 ...
//
// 示例：
//
//	// SELECT avg(math_score) AS avg_math, avg(english_score) AS avg_english FROM exams
//	db.Select(sharksql.AvgAs("math_score", "avg_math", "english_score", "avg_english")).Find(&result)
func AvgAs(columns ...string) string {
	if len(columns)%2 != 0 {
		return ""
	}
	sql := ""
	for i := 0; i < len(columns); i += 2 {
		if i+1 < len(columns) {
			sql += fmt.Sprintf("avg(%v) as %v, ", columns[i], columns[i+1])
		}
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// Max 构建 MAX 聚合表达式。
// 多个字段以逗号分隔，不追加别名。
//
// 示例：
//
//	// SELECT max(score) FROM exams
//	db.Select(sharksql.Max("score")).Find(&result)
//
//	// 多字段最大值
//	db.Select(sharksql.Max("high_temp", "low_temp")).Find(&result)
func Max(columns ...string) string {
	sql := ""
	for i := 0; i < len(columns); i++ {
		sql += fmt.Sprintf("max(%v), ", columns[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// MaxAs 构建 MAX 聚合表达式，支持自定义别名。
// 参数需成对出现：字段名, 别名, 字段名, 别名 ...
//
// 示例：
//
//	// SELECT max(high_temp) AS max_high, max(low_temp) AS max_low FROM weather
//	db.Select(sharksql.MaxAs("high_temp", "max_high", "low_temp", "max_low")).Find(&result)
func MaxAs(columns ...string) string {
	if len(columns)%2 != 0 {
		return ""
	}
	sql := ""
	for i := 0; i < len(columns); i += 2 {
		if i+1 < len(columns) {
			sql += fmt.Sprintf("max(%v) as %v, ", columns[i], columns[i+1])
		}
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// Min 构建 MIN 聚合表达式。
// 多个字段以逗号分隔，不追加别名。
//
// 示例：
//
//	// SELECT min(price) FROM products
//	db.Select(sharksql.Min("price")).Find(&result)
func Min(columns ...string) string {
	sql := ""
	for i := 0; i < len(columns); i++ {
		sql += fmt.Sprintf("min(%v), ", columns[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// MinAs 构建 MIN 聚合表达式，支持自定义别名。
// 参数需成对出现：字段名, 别名, 字段名, 别名 ...
//
// 示例：
//
//	// SELECT min(price) AS min_price, min(discount) AS min_discount FROM products
//	db.Select(sharksql.MinAs("price", "min_price", "discount", "min_discount")).Find(&result)
func MinAs(columns ...string) string {
	if len(columns)%2 != 0 {
		return ""
	}
	sql := ""
	for i := 0; i < len(columns); i += 2 {
		if i+1 < len(columns) {
			sql += fmt.Sprintf("min(%v) as %v, ", columns[i], columns[i+1])
		}
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// ========== COALESCE / IFNULL 表达式 ==========

// Coalesce 构建 COALESCE 表达式：COALESCE(column, defaultValue)。
// 用于 SELECT 子句中为 NULL 列提供默认值。
//
// column 为列名或表达式（如 sum(amount)），defaultValue 为回退默认值。
//
// 示例：
//
//	// SELECT COALESCE(nickname, '匿名用户') FROM users
//	db.Select(sharksql.Coalesce("nickname", "'匿名用户'")).Find(&results)
//
//	// SELECT COALESCE(sum(amount), 0) as total_amount FROM orders
//	db.Select(sharksql.Coalesce(sharksql.Sum("amount"), "0")).Find(&results)
//
// Coalesce 构建 COALESCE 表达式：COALESCE(v1, v2, ...)。
// 参数个数可变，通过 Sprintf 拼接为 COALESCE(a, b, c, ...)。
//
// 示例：
//
//	// SELECT COALESCE(nickname, '匿名用户') FROM users
//	db.Select(sharksql.Coalesce("nickname", "'匿名用户'")).Find(&results)
//
//	// SELECT COALESCE(a, b, c) FROM t
//	db.Select(sharksql.Coalesce("a", "b", "c")).Find(&results)
func Coalesce(args ...any) string {
	ss := make([]string, len(args))
	for i, v := range args {
		ss[i] = fmt.Sprint(v)
	}
	return "COALESCE(" + strings.Join(ss, ", ") + ")"
}

// CoalesceAs 构建 COALESCE 表达式并指定别名：COALESCE(column, args...) as alias。
// 最后一个参数为 alias。
//
// 示例：
//
//	// SELECT COALESCE(nickname, '匿名用户') as display_name FROM users
//	db.Select(sharksql.CoalesceAs("nickname", "'匿名用户'", "display_name")).Find(&results)
func CoalesceAs(column string, args ...any) string {
	if len(args) == 0 {
		return column
	}
	alias := fmt.Sprint(args[len(args)-1])
	values := make([]string, 1, len(args))
	values[0] = column
	for i := 0; i < len(args)-1; i++ {
		values = append(values, fmt.Sprint(args[i]))
	}
	return "COALESCE(" + strings.Join(values, ", ") + ") as " + alias
}

// IfNull 构建 IFNULL 表达式：IFNULL(column, defaultValue)。
// MySQL 特有函数，功能与 COALESCE 类似但只接受两个参数。
//
// 示例：
//
//	// SELECT IFNULL(remark, '无备注') FROM tasks
//	db.Select(sharksql.IfNull("remark", "'无备注'")).Find(&results)
//
//	// SELECT IFNULL(sum(amount), 0) as total FROM orders
//	db.Select(sharksql.IfNull(sharksql.Sum("amount"), "0")).Find(&results)
func IfNull(column string, value any) string {
	return fmt.Sprintf("IFNULL(%v, %v)", column, value)
}

// IfNullAs 构建 IFNULL 表达式并指定别名：IFNULL(column, defaultValue) as alias。
//
// 示例：
//
//	// SELECT IFNULL(remark, '无备注') as remark_text FROM tasks
//	db.Select(sharksql.IfNullAs("remark", "'无备注'", "remark_text")).Find(&results)
func IfNullAs(column string, value any, alias string) string {
	return fmt.Sprintf("IFNULL(%v, %v) as %v", column, value, alias)
}

// ========== CASE 表达式 ==========

// Case 构建参数化 CASE 表达式：CASE column WHEN ? THEN ? ... END。
//
// 第一个参数为列名，后续参数为值/结果对。
// 参数个数规则：
//   - 除列名外为偶数个参数：无 ELSE 分支，全部为 WHEN/THEN 对
//   - 除列名外为奇数个参数：最后一个参数为 ELSE 值，其余为 WHEN/THEN 对
//
// 所有值通过 ? 占位符参数化，返回 (sql, args) 元组。
//
// 示例：
//
//	// 有 ELSE
//	sql, args := sharksql.Case("status", 0, "待支付", 1, "已支付", 2, "已取消", "未知")
//	// sql:  CASE status WHEN ? THEN ? WHEN ? THEN ? WHEN ? THEN ? ELSE ? END
//	// args: [0, 待支付, 1, 已支付, 2, 已取消, 未知]
//	db.Select(sql, args...).Find(&results)
//
//	// 无 ELSE
//	sql, args := sharksql.Case("score", 90, "优秀", 80, "良好", 60, "及格")
//	// sql:  CASE score WHEN ? THEN ? WHEN ? THEN ? WHEN ? THEN ? END
//	// args: [90, 优秀, 80, 良好, 60, 及格]
func Case(column string, pairs ...any) string {
	var sb strings.Builder
	sb.WriteString("CASE ")
	sb.WriteString(column)

	total := len(pairs)
	hasElse := total%2 != 0
	pairCount := total
	if hasElse {
		pairCount = total - 1
	}

	for i := 0; i < pairCount; i += 2 {
		sb.WriteString(" WHEN ")
		sb.WriteString(fmt.Sprint(pairs[i]))
		sb.WriteString(" THEN ")
		sb.WriteString(fmt.Sprint(pairs[i+1]))
	}

	if hasElse {
		sb.WriteString(" ELSE ")
		sb.WriteString(fmt.Sprint(pairs[total-1]))
	}

	sb.WriteString(" END")
	return sb.String()
}

// CaseAs 构建 CASE 表达式并指定别名：CASE column WHEN ... THEN ... END as alias。
// 最后一个参数为 alias。
//
// 示例：
//
//	s := sharksql.CaseAs("status", "status_name", 0, "'待支付'", 1, "'已支付'", "'未知'")
//	// CASE status WHEN 0 THEN '待支付' WHEN 1 THEN '已支付' ELSE '未知' END as status_name
func CaseAs(column string, alias string, pairs ...any) string {
	return Case(column, pairs...) + " as " + alias
}

// ========== WHEN 表达式 ==========

// When 构建 WHEN ... THEN ... 表达式片段。
// 参数双数（偶数个）：全部为 WHEN/THEN 对，无 ELSE。
// 参数单数（奇数个）：最后一个参数为 ELSE 值，其余为 WHEN/THEN 对。
//
// 示例：
//
//	s := sharksql.When("a=1", 1, "b=2", 2, "c=3", 3, "d=4", 0)
//	// WHEN a=1 THEN 1 WHEN b=2 THEN 2 WHEN c=3 THEN 3 WHEN d=4 THEN 0 (偶数，无 ELSE)
//
//	s := sharksql.When("a=1", 1, "b=2", 2, "c=3")
//	// WHEN a=1 THEN 1 WHEN b=2 THEN 2 ELSE c=3 (奇数，最后是 ELSE)
func When(args ...any) string {
	total := len(args)
	hasElse := total%2 != 0
	pairCount := total
	if hasElse {
		pairCount = total - 1
	}

	var sb strings.Builder
	for i := 0; i < pairCount; i += 2 {
		sb.WriteString("WHEN ")
		sb.WriteString(fmt.Sprint(args[i]))
		sb.WriteString(" THEN ")
		sb.WriteString(fmt.Sprint(args[i+1]))
		sb.WriteString(" ")
	}

	if hasElse {
		sb.WriteString("ELSE ")
		sb.WriteString(fmt.Sprint(args[total-1]))
	}

	return strings.TrimSpace(sb.String())
}

// WhenAs 构建 WHEN ... THEN ... 表达式片段并指定别名。
// 第二个参数为 alias，其余规则同 When。
//
// 示例：
//
//	s := sharksql.WhenAs("alias", "a=1", 1, "b=2", 2)
//	// WHEN a=1 THEN 1 WHEN b=2 THEN 2 as alias (偶数，无 ELSE)
func WhenAs(alias string, args ...any) string {
	return When(args...) + " as " + alias
}

// ========== 分页查询 ==========

// PageQuery 执行泛型分页查询，返回指定页的数据和总记录数。
//
// 参数：
//   - db:       GORM 查询链（已包含 WHERE 条件等）
//   - page:     页码（从 1 开始，< 1 时自动修正为 1，> 500 时受 offset 限制）
//   - pageSize: 每页条数（< 10 时自动修正为 10）
//
// 返回值：
//   - []T:  当前页数据切片
//   - int64: 总记录数
//   - error: 查询失败时返回错误
//
// 限制：
//   - offset（page * pageSize）不能超过 10000，防止深分页性能问题
//   - 使用 Session(&gorm.Session{}) 创建独立会话执行 COUNT，避免被原查询链的 Select 覆盖
//
// 使用示例：
//
//	type User struct {
//	    ID     int64  `gorm:"column:id"`
//	    Name   string `gorm:"column:name"`
//	    Status int    `gorm:"column:status"`
//	}
//
//	// 分页查询状态正常的用户
//	db := gormDB.Where(sharksql.Eq("status", 1))
//	users, total, err := sharksql.PageQuery[User](db, 1, 20)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("第 1 页，共 %d 条记录\n", total)
//	for _, u := range users {
//	    fmt.Println(u.Name)
//	}
//
//	// 结合复杂条件分页
//	db = gormDB.Where(
//	    sharksql.Eq("status", 1),
//	    sharksql.Like("name", "张"),
//	    sharksql.Gte("age", 18),
//	).Order(sharksql.Desc("created_at"))
//	users, total, err = sharksql.PageQuery[User](db, 2, 10)
func PageQuery[T any](db *gorm.DB, page int, pageSize int) ([]T, int64, error) {
	// 参数修正
	if page < 1 {
		page = 1
	}
	if pageSize < 10 {
		pageSize = 10
	}
	// offset 限制：防止深分页拖垮数据库
	if page*pageSize > 10000 {
		return nil, 0, fmt.Errorf("offset cannot exceed 10000")
	}
	// 使用独立 Session 计数，避免被 Select 子句影响
	var total int64
	err := db.Session(&gorm.Session{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	var results []T
	err = db.Offset(offset).Limit(pageSize).Find(&results).Error
	return results, total, err
}

// ========== 数据库工具函数 ==========

// IsDuplicateKey 判断错误是否为 MySQL 的重复键错误（错误码 1062）。
//
// 常用于 INSERT 操作的幂等性处理：检测到重复键时返回已有记录或忽略。
//
// 示例：
//
//	err := db.Create(&user).Error
//	if err != nil {
//	    if sharksql.IsDuplicateKey(err) {
//	        // 重复键：返回已存在的记录
//	        db.Where("email = ?", user.Email).First(&user)
//	        return user, nil
//	    }
//	    return nil, err
//	}
func IsDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		if mysqlErr.Number == 1062 {
			return true
		}
	}
	return false
}

// ========== MySQL JSON 函数封装 ==========

// JsonSearchOne 构建 JSON_SEARCH 条件（搜索 JSON 数组/对象中是否包含某值）。
// 使用 'one' 模式，找到第一个匹配即返回。
//
// 注意：value 会被自动包裹 % 通配符。
//
// 示例：
//
//	// SELECT * FROM users WHERE JSON_SEARCH(tags, 'one', '%vip%') IS NOT NULL
//	db.Where(sharksql.JsonSearchOne("tags", "vip")).Find(&users)
//
//	// 搜索 JSON 对象中嵌套的值
//	db.Where(sharksql.JsonSearchOne("metadata", "active")).Find(&users)
func JsonSearchOne(column string, value any) (string, string) {
	sql := fmt.Sprintf("JSON_SEARCH(%v, 'one', ?) IS NOT NULL", column)
	data := fmt.Sprintf("%%%v%%", cast.ToString(value))
	return sql, data
}

// JsonContains 构建 JSON_CONTAINS 条件（检查 JSON 文档是否包含指定值）。
//
// 示例：
//
//	// SELECT * FROM users WHERE JSON_CONTAINS(roles, '"admin"')
//	db.Where(sharksql.JsonContains("roles", `"admin"`)).Find(&users)
//
//	// 检查是否包含数组元素
//	db.Where(sharksql.JsonContains("tags", `["vip","premium"]`)).Find(&users)
func JsonContains(column string, value any) (string, string) {
	sql := fmt.Sprintf("JSON_CONTAINS(%v, ?)", column)
	data := fmt.Sprintf("%v", value)
	return sql, data
}

// JsonArrayAppendObject 构建 JSON_ARRAY_APPEND 表达式，将对象转 JSON 后追加到数组末尾。
// 如果列值为 NULL，自动初始化为空数组 JSON_ARRAY()。
//
// 参数：
//   - column: JSON 列名
//   - value:  要追加的值（可变参数，支持多个）。字符串直接追加，其他类型通过 sharkjson.ToJsonString 序列化
//
// 示例：
//
//	// UPDATE users SET tags = JSON_ARRAY_APPEND(COALESCE(tags, JSON_ARRAY()), '$', CAST('{"name":"vip"}' AS JSON))
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("tags", gorm.Expr(sharksql.JsonArrayAppendObject("tags", map[string]any{"name": "vip"})))
//
//	// 追加多个对象
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("tags", gorm.Expr(sharksql.JsonArrayAppendObject("tags", obj1, obj2)))
func JsonArrayAppendObject(column string, value ...any) (string, []any) {
	sql := fmt.Sprintf("JSON_ARRAY_APPEND(COALESCE(%v, JSON_ARRAY())", column)
	args := []any{}
	for _, v := range value {
		switch v.(type) {
		case string:
			sql += ",'$', CAST(? AS JSON)"
			args = append(args, v)
		default:
			sql += ",'$', CAST(? AS JSON)"
			args = append(args, sharkjson.ToJsonString(v))
		}
	}
	sql += ")"
	return sql, args
}

// JsonArrayAppend 构建 JSON_ARRAY_APPEND 表达式，将原始值追加到数组末尾。
// 与 JsonArrayAppendObject 的区别：不进行 JSON 序列化，直接使用原始值。
//
// 示例：
//
//	// UPDATE logs SET event_ids = JSON_ARRAY_APPEND(COALESCE(event_ids, JSON_ARRAY()), '$', 12345)
//	db.Model(&Log{}).Where("id = ?", 1).
//	    Update("event_ids", gorm.Expr(sharksql.JsonArrayAppend("event_ids", 12345)))
//
//	// 追加多个值
//	db.Model(&Log{}).Update("event_ids", gorm.Expr(
//	    sharksql.JsonArrayAppend("event_ids", 100, 200, 300),
//	))
func JsonArrayAppend(column string, value ...any) (string, []any) {
	sql := fmt.Sprintf("JSON_ARRAY_APPEND(COALESCE(%v, JSON_ARRAY())", column)
	args := []any{}
	for _, v := range value {
		sql += ",'$', ?"
		args = append(args, v)
	}
	sql += ")"
	return sql, args
}

// JsonSetObject 构建 JSON_SET 表达式，将对象转 JSON 后设置到指定路径。
// 如果列值为 NULL，自动初始化为空对象 JSON_OBJECT()。
//
// 参数：
//   - column: JSON 列名
//   - path:   JSON 路径（如 "$.name" 或使用 JsonPath() 构建）
//   - value:  要设置的值（非字符串类型会通过 sharkjson.ToJsonString 序列化）
//
// 示例：
//
//	// UPDATE users SET metadata = JSON_SET(COALESCE(metadata, JSON_OBJECT()), '$.vip_level', CONVERT('{"level":3}',JSON))
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("metadata", gorm.Expr(sharksql.JsonSetObject("metadata", "$.vip_level",
//	        map[string]any{"level": 3})))
func JsonSetObject(column string, path string, value any) (string, []any) {
	sql := fmt.Sprintf("JSON_SET(COALESCE(%v, JSON_OBJECT()), '%v', CONVERT(?,JSON))", column, path)
	return sql, []any{sharkjson.ToJsonString(value)}
}

// JsonSet 构建 JSON_SET 表达式，将原始值设置到指定路径。
// 与 JsonSetObject 的区别：不进行 JSON 序列化，直接使用原始值。
//
// 示例：
//
//	// UPDATE users SET metadata = JSON_SET(COALESCE(metadata, JSON_OBJECT()), '$.age', 25)
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("metadata", gorm.Expr(sharksql.JsonSet("metadata", "$.age", 25)))
//
//	// 设置字符串值
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("metadata", gorm.Expr(sharksql.JsonSet("metadata", "$.city", "北京")))
func JsonSet(column string, path string, value any) (string, []any) {
	sql := fmt.Sprintf("JSON_SET(COALESCE(%v, JSON_OBJECT()), '%v', ?)", column, path)
	return sql, []any{value}
}

// JsonPath 构建 MySQL JSON 路径表达式。
//
// 示例：
//
//	// 构建 $.user.address.city
//	path := sharksql.JsonPath("user", "address", "city")
//	// 结果: "$.user.address.city"
//
//	// 用于 JSON_EXTRACT
//	db.Select(fmt.Sprintf("JSON_EXTRACT(metadata, '%s')", sharksql.JsonPath("name"))).Find(&result)
func JsonPath(path ...string) string {
	jsonPath := "$"
	for _, p := range path {
		jsonPath += fmt.Sprintf(".%v", p)
	}
	return jsonPath
}

// JsonExtract 构建 JSON_EXTRACT 表达式，从 JSON 文档中提取路径对应的值。
// path 参数为 JSON 路径，可使用 JsonPath() 构建或直接传入 "$.xxx" 字符串。
//
// 示例：
//
//	// SELECT JSON_EXTRACT(metadata, '$.name') FROM users
//	db.Select(sharksql.JsonExtract("metadata", "$.name")).Find(&results)
//
//	// 配合 JsonPath 使用
//	db.Select(sharksql.JsonExtract("metadata", sharksql.JsonPath("user", "name"))).Find(&results)
//
//	// 提取多个路径
//	db.Select(sharksql.JsonExtract("metadata", "$.name", "$.age")).Find(&results)
func JsonExtract(column string, path ...string) string {
	if len(path) == 0 {
		return fmt.Sprintf("JSON_EXTRACT(%v, '$')", column)
	}
	if len(path) == 1 {
		return fmt.Sprintf("JSON_EXTRACT(%v, '%v')", column, path[0])
	}
	return fmt.Sprintf("JSON_EXTRACT(%v, '%v')", column, strings.Join(path, "', '"))
}

// JsonUnquote 构建 JSON_UNQUOTE(JSON_EXTRACT(...)) 表达式。
// 提取 JSON 值并去除引号，常用于 WHERE/Having 条件中与字符串比较。
//
// 示例：
//
//	// SELECT * FROM users WHERE JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.city')) = 'NYC'
//	db.Where(fmt.Sprintf("%s = ?", sharksql.JsonUnquote("metadata", "$.city")), "NYC").Find(&users)
func JsonUnquote(column string, path string) string {
	return fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(%v, '%v'))", column, path)
}

// JsonRemove 构建 JSON_REMOVE 表达式，从 JSON 文档中删除指定路径。
// 通常配合 Update 使用。
//
// 示例：
//
//	// UPDATE users SET tags = JSON_REMOVE(tags, '$[0]') WHERE id = 1
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("tags", gorm.Expr(sharksql.JsonRemove("tags", "$[0]")))
//
//	// 删除多个路径
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("metadata", gorm.Expr(sharksql.JsonRemove("metadata", "$.temp", "$.cache")))
func JsonRemove(column string, path ...string) string {
	if len(path) == 1 {
		return fmt.Sprintf("JSON_REMOVE(%v, '%v')", column, path[0])
	}
	return fmt.Sprintf("JSON_REMOVE(%v, '%v')", column, strings.Join(path, "', '"))
}

// JsonArrayInsert 构建 JSON_ARRAY_INSERT 表达式，在数组指定位置插入值。
// path 为插入位置（如 "$[0]"），value 为要插入的值。
// 如果列值为 NULL，自动初始化为空数组 JSON_ARRAY()。
//
// 示例：
//
//	// UPDATE users SET tags = JSON_ARRAY_INSERT(COALESCE(tags, JSON_ARRAY()), '$[0]', 'vip')
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("tags", gorm.Expr(sharksql.JsonArrayInsert("tags", "$[0]", "vip")))
//
//	// 支持非字符串类型（自动转 JSON）
//	db.Model(&User{}).Where("id = ?", 1).
//	    Update("tags", gorm.Expr(sharksql.JsonArrayInsert("tags", "$[1]", map[string]any{"name": "vip"})))
func JsonArrayInsert(column string, path string, value any) (string, []any) {
	sql := fmt.Sprintf("JSON_ARRAY_INSERT(COALESCE(%v, JSON_ARRAY()), '%v', CAST(? AS JSON))", column, path)
	v := value
	if _, ok := value.(string); !ok {
		v = sharkjson.ToJsonString(value)
	}
	return sql, []any{v}
}

// JsonLength 构建 JSON_LENGTH 表达式，返回 JSON 文档的长度。
// 数组返回元素个数，对象返回键的数量。
//
// 示例：
//
//	// SELECT JSON_LENGTH(tags) AS tag_count FROM users
//	db.Select(sharksql.JsonLength("tags")).Find(&results)
func JsonLength(column string) string {
	return fmt.Sprintf("JSON_LENGTH(%v)", column)
}

// JsonKeys 构建 JSON_KEYS 表达式，返回 JSON 对象的键名数组。
//
// 示例：
//
//	// SELECT JSON_KEYS(metadata) AS meta_keys FROM users
//	db.Select(sharksql.JsonKeys("metadata")).Find(&results)
func JsonKeys(column string) string {
	return fmt.Sprintf("JSON_KEYS(%v)", column)
}

// JsonType 构建 JSON_TYPE 表达式，返回 JSON 值的类型字符串。
// 返回值如 "OBJECT"、"ARRAY"、"STRING"、"INTEGER" 等。
//
// 示例：
//
//	// SELECT * FROM users WHERE JSON_TYPE(metadata) = 'OBJECT'
//	db.Where(fmt.Sprintf("%s = ?", sharksql.JsonType("metadata")), "OBJECT").Find(&users)
func JsonType(column string) string {
	return fmt.Sprintf("JSON_TYPE(%v)", column)
}

// ========== 去重 ==========

// Distinct 构建 DISTINCT column 表达式。
// 用于 SELECT 子句中去除重复列值。
//
// 示例：
//
//	// SELECT DISTINCT status FROM tasks
//	db.Select(sharksql.Distinct("status")).Find(&results)
//
//	// 多列去重
//	db.Select(sharksql.Distinct("user_id", "org_id")).Find(&results)
//	// → SELECT DISTINCT(user_id, org_id)
func Distinct(columns ...string) string {
	if len(columns) == 0 {
		return ""
	}
	if len(columns) == 1 {
		return "DISTINCT " + columns[0]
	}
	return "DISTINCT(" + strings.Join(columns, ", ") + ")"
}

// ========== 表名/列名辅助函数 ==========

// Column 构建 "table.column" 格式的限定列名。
// 用于多表 JOIN 查询时消除列名歧义。
//
// 示例：
//
//	// SELECT users.id, users.name FROM users JOIN orders ON users.id = orders.user_id
//	db.Select(
//	    sharksql.Column("users", "id"),
//	    sharksql.Column("users", "name"),
//	).Joins("JOIN orders ON users.id = orders.user_id").Find(&result)
func Column(table string, column string) string {
	return fmt.Sprintf("%v.%v", table, column)
}

// ColumnAs 构建 "table.column AS alias" 格式的带别名的限定列名。
//
// 示例：
//
//	// SELECT users.id AS user_id, users.name AS user_name, orders.amount AS order_amount
//	db.Select(
//	    sharksql.ColumnAs("users", "id", "user_id"),
//	    sharksql.ColumnAs("users", "name", "user_name"),
//	    sharksql.ColumnAs("orders", "amount", "order_amount"),
//	).Joins("JOIN orders ON users.id = orders.user_id").Find(&result)
func ColumnAs(table string, column string, as string) string {
	return fmt.Sprintf("%v.%v as %v", table, column, as)
}

// LeftJoin 构建 LEFT JOIN 子句字符串和参数列表。
//
// 参数：
//   - table: 要 JOIN 的表名及别名，如 "orders o" 或 "accounts a"
//   - on:    SqlBuilder 实例，用于构建 ON 条件（支持字段对字段和字段对值的混合）
//
// 返回值：
//   - string: 完整的 LEFT JOIN 子句（如 "LEFT JOIN orders o ON u.id = o.user_id AND o.deleted = ?"）
//   - []any:  ON 条件中的参数值切片
//
// 使用示例：
//
//	// 基本 JOIN，纯字段关联
//	onB := sharksql.NewSql().EqCol("u.id", "o.user_id")
//	joinSQL, args := sharksql.LeftJoin("orders o", onB)
//	// joinSQL: "LEFT JOIN orders o ON u.id = o.user_id"
//	// args:    nil
//	db.Joins(joinSQL, args...).Find(&results)
//
//	// 带额外筛选条件的 JOIN
//	onB := sharksql.NewSql().
//	    EqCol("u.id", "o.user_id").
//	    Eq("o.deleted", 0)
//	joinSQL, args := sharksql.LeftJoin("orders o", onB)
//	// joinSQL: "LEFT JOIN orders o ON u.id = o.user_id AND o.deleted = ?"
//	// args:    [0]
//	db.Joins(joinSQL, args...).Find(&results)
//
//	// 多表 JOIN
//	db.Joins(sharksql.LeftJoin("orders o", sharksql.NewSql().EqCol("u.id", "o.user_id"))).
//	    Joins(sharksql.LeftJoin("accounts a", sharksql.NewSql().EqCol("u.account_id", "a.id"))).
//	    Find(&results)
//
//	// on 为 nil 或 Build 结果为空时，返回空字符串
//	joinSQL, args := sharksql.LeftJoin("orders o", nil)
//	// joinSQL: ""
//	// args:    nil
func LeftJoin(table string, on *SqlBuilder) (string, []any) {
	if on == nil {
		return "", nil
	}
	sql, args := on.Build()
	if sql == "" {
		return "", nil
	}
	return "LEFT JOIN " + table + " ON " + sql, args
}

// InnerJoin 构建 INNER JOIN 子句字符串和参数列表。
//
// 参数和返回值说明同 LeftJoin，区别在于使用 INNER JOIN 而非 LEFT JOIN。
//
// 使用示例：
//
//	// 基本 INNER JOIN，纯字段关联
//	onB := sharksql.NewSql().EqCol("u.id", "o.user_id")
//	joinSQL, args := sharksql.InnerJoin("orders o", onB)
//	// joinSQL: "INNER JOIN orders o ON u.id = o.user_id"
//	// args:    nil
//	db.Joins(joinSQL, args...).Find(&results)
//
//	// 带额外筛选条件的 INNER JOIN
//	onB := sharksql.NewSql().
//	    EqCol("u.id", "o.user_id").
//	    Eq("o.deleted", 0)
//	joinSQL, args := sharksql.InnerJoin("orders o", onB)
//	// joinSQL: "INNER JOIN orders o ON u.id = o.user_id AND o.deleted = ?"
//	// args:    [0]
//
//	// on 为 nil 或 Build 结果为空时，返回空字符串
//	joinSQL, args := sharksql.InnerJoin("orders o", nil)
//	// joinSQL: ""
//	// args:    nil
func InnerJoin(table string, on *SqlBuilder) (string, []any) {
	if on == nil {
		return "", nil
	}
	sql, args := on.Build()
	if sql == "" {
		return "", nil
	}
	return "INNER JOIN " + table + " ON " + sql, args
}

// Where 根据结构体字段生成 WHERE 条件 (column = ? 形式)。
//
// 规则：
//   - 只处理指针字段（*T, *[]T, *map[K]V），其他类型忽略
//   - 列名取自 json tag
//   - 指针 nil 忽略
//   - *[]T / *[]T（非 nil）：column IN (?)
//   - *map[K]V（非 nil）：column = ?，值用 sonic 序列化为 JSON
//   - *struct/嵌套指针/slice/map/array（非 nil）：column = ?，值用 sonic 序列化为 JSON
//   - *decimal.Decimal（非 nil）：原值传递，不序列化
func Where(req any) (string, []any) {
	v := reflect.ValueOf(req)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return "", nil
	}

	t := v.Type()
	var conditions []string
	var args []any

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		column := field.Tag.Get("json")
		if column == "" {
			continue
		}

		fieldVal := v.Field(i)
		if fieldVal.Kind() != reflect.Ptr {
			continue
		}
		if fieldVal.IsNil() {
			continue
		}

		elem := fieldVal.Elem()

		// 判断指针指向的是否为切片/数组 → IN (?)
		if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array {
			conditions = append(conditions, column+" IN (?)")
			args = append(args, elem.Interface())
			continue
		}

		// 其他类型 → column = ?
		val := resolveValue(elem)
		if val != nil {
			conditions = append(conditions, column+" = ?")
			args = append(args, val)
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), args
}

//	type UpdateDemo struct {
//		Name   *string         `json:"name"`   // → "张三"
//		Age    *int            `json:"age"`    // → 25
//		Price  *decimal.Decimal `json:"price"` // → 19.99 (原值)
//		Tags   *[]string       `json:"tags"`   // → `["a","b"]` (JSON)
//		Meta   *map[string]any  `json:"meta"`  // → `{"k":"v"}` (JSON)
//		Ignore int             `json:"ignore"` // 非指针，忽略
//	}
//
//	req := UpdateDemo{
//		Name: strPtr("张三"),
//		Age:  intPtr(25),
//		Price: decPtr(decimal.NewFromFloat(19.99)),
//	}
//	data := sharksql.ToUpdate(req)
//	// data: map[string]any{"name":"张三", "age":25, "price":19.99}
//
// ToUpdate 根据结构体字段生成 UPDATE SET 列的 map[string]any。
//
// 规则：
//   - 只处理指针字段（*T, *[]T, *map[K]V），其他类型忽略
//   - 列名取自 json tag
//   - 指针 nil 忽略
//   - 值如果是复合类型，用 sonic 序列化为 JSON 字符串
//   - decimal.Decimal 不视为复合类型直接传值
func ToUpdate(req any) map[string]any {
	v := reflect.ValueOf(req)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	result := make(map[string]any)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		column := field.Tag.Get("json")
		if column == "" {
			continue
		}

		fieldVal := v.Field(i)
		if fieldVal.Kind() != reflect.Ptr {
			continue
		}
		if fieldVal.IsNil() {
			continue
		}

		elem := fieldVal.Elem()
		val := resolveValue(elem)
		if val != nil {
			result[column] = val
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// resolveValue 将 reflect.Value 转为实际可用值。
// 如果是复合类型（struct/slice/map/array），序列化为 JSON 字符串。
// decimal.Decimal 不算复合类型。
// 注意：调用前已经解完 *T 指针，此处 elem.Kind() 不应是 Ptr。
func resolveValue(rv reflect.Value) any {
	if !rv.IsValid() {
		return nil
	}
	iface := rv.Interface()
	// 先检查是否是 decimal.Decimal
	if _, ok := iface.(jsonDecimal); ok {
		return iface
	}
	switch rv.Kind() {
	case reflect.Struct:
		return toJSON(iface)
	case reflect.Slice, reflect.Array, reflect.Map:
		return toJSON(iface)
	default:
		return iface
	}
}

// jsonDecimal 是 decimal.Decimal 的一个简写别名，用于类型检测（无需直接 import decimal 包）。
type jsonDecimal interface {
	String() string
}

// toJSON 用 sonic 将值序列化为 JSON 字符串。失败返回 nil。
func toJSON(v any) any {
	b, err := sonic.Marshal(v)
	if err != nil {
		return nil
	}
	// 去掉可能的换行，sonic 默认不换行但保险
	return string(b)
}
