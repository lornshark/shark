package sharksql

import (
	"fmt"
	"strings"
)

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

// ========== COALESCE / IFNULL 表达式 ==========

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
	return "DISTINCT " + strings.Join(columns, ", ")
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
