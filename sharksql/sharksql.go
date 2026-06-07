package sharksql

import (
	"fmt"
	"strings"

	"github.com/spf13/cast"
	"gorm.io/gorm"
)

type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// ==
// 例如：Eq("status", 1) -> "status = ?", 1
func Eq(column string, value any) (string, any) {
	return column + " = ?", value
}

// <>
// 例如：Neq("status", 1) -> "status <> ?", 1
func Neq(column string, value any) (string, any) {
	return column + " <> ?", value
}

// >
// 例如：Gt("age", 18) -> "age > ?", 18
func Gt(column string, value any) (string, any) {
	return column + " > ?", value
}

// >=
// 例如：Gte("age", 18) -> "age >= ?", 18
func Gte(column string, value any) (string, any) {
	return column + " >= ?", value
}

// <
// 例如：Lt("age", 18) -> "age < ?", 18
func Lt(column string, value any) (string, any) {
	return column + " < ?", value
}

// <=
// 例如：Lte("age", 18) -> "age <= ?", 18
func Lte(column string, value any) (string, any) {
	return column + " <= ?", value
}

// LIKE
// 例如：Like("name", "John") -> "name LIKE ?", "%John%"
func Like(column string, value any) (string, any) {
	return column + " LIKE ?", "%" + cast.ToString(value) + "%"
}

// NOT LIKE
// 例如：NotLike("name", "John") -> "name NOT LIKE ?", "%John%"
func NotLike(column string, value any) (string, any) {
	return column + " NOT LIKE ?", "%" + cast.ToString(value) + "%"
}

// IN
// 例如：In("status", []int{1, 2, 3}) -> "status IN (?)", []int{1, 2, 3}
func In(column string, value any) (string, any) {
	return column + " IN (?)", value
}

// NOT IN
// 例如：NotIn("status", []int{1, 2, 3}) -> "status NOT IN (?)", []int{1, 2, 3}
func NotIn(column string, value any) (string, any) {
	return column + " NOT IN (?)", value
}

// IS NULL
// 例如：IsNull("deleted_at") -> "deleted_at IS NULL"
func IsNull(column string) string {
	return column + " IS NULL"
}

// IS NOT NULL
// 例如：IsNotNull("deleted_at") -> "deleted_at IS NOT NULL"
func IsNotNull(column string) string {
	return column + " IS NOT NULL"
}

// 字段加
// 例如：Add("bet_amount", 100) -> "bet_amount + ?", 100
func Add(column string, value any) (string, any) {
	return column + " + ?", value
}

// 字段减
// 例如：Sub("bet_amount", 100) -> "bet_amount - ?", 100
func Sub(column string, value any) (string, any) {
	return column + " - ?", value
}

// 字段乘
// 例如：Mul("bet_amount", 2) -> "bet_amount * ?", 2
func Mul(column string, value any) (string, any) {
	return column + " * ?", value
}

// 字段除
// 例如：Div("bet_amount", 2) -> "bet_amount / ?", 2
func Div(column string, value any) (string, any) {
	return column + " / ?", value
}

// asc
// 例如：Asc("created_at") -> "created_at ASC"
func Asc(column string) string {
	return column + " ASC"
}

// desc
// 例如：Desc("created_at") -> "created_at DESC"
func Desc(column string) string {
	return column + " DESC"
}

// FromTo [a,b) 区间查询，包含 from 不包含 to
// 例如：FromTo("created_at", "2023-01-01", "2023-12-31") -> "created_at >= ? AND created_at < ?", "2023-01-01", "2023-12-31"
func FromTo(column string, from any, to any) (string, any, any) {
	return column + " >= ? AND " + column + " < ?", from, to
}

// sum 语句带 as 别名, 别名和字段相同
// 例如：Sum("bet_amount", "win_amount") -> sum(bet_amount) as bet_amount, sum(win_amount) as win_amount
func Sum(column ...string) string {
	sql := ""
	for i := 0; i < len(column); i++ {
		sql += fmt.Sprintf("sum(%v) as %v, ", column[i], column[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// sum 语句带 as 别名
// column 参数需要成对出现，前面是字段名，后面是别名
// 例如：SumAs("bet_amount", "total_bet", "win_amount", "total_win") -> sum(bet_amount) as total_bet, sum(win_amount) as total_win
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

// count 语句带 as 别名
// column 参数需要成对出现，前面是字段名，后面是别名
// 例如： CountAs("bet_amount", "total_bet", "win_amount", "total_win") -> count(bet_amount) as total_bet, count(win_amount) as total_win
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

// avg 语句带 as 别名, 别名和字段相同
// 例如：Avg("bet_amount", "win_amount") -> avg(bet_amount) as bet_amount, avg(win_amount) as win_amount
func Avg(columns ...string) string {
	sql := ""
	for i := 0; i < len(columns); i++ {
		sql += fmt.Sprintf("avg(%v) as %v, ", columns[i], columns[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// avg 语句带 as 别名
// column 参数需要成对出现，前面是字段名，后面是别名
// 例如：AvgAs("bet_amount", "avg_bet", "win_amount", "avg_win") -> avg(bet_amount) as avg_bet, avg(win_amount) as avg_win
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

// max 语句带 as 别名, 别名和字段相同
// 例如：Max("bet_amount") -> max(bet_amount) as bet_amount
// 例如：Max("bet_amount", "win_amount") -> max(bet_amount) as bet_amount, max(win_amount) as win_amount
func Max(columns ...string) string {
	sql := ""
	for i := 0; i < len(columns); i++ {
		sql += fmt.Sprintf("max(%v) as %v, ", columns[i], columns[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// max 语句带 as 别名
// column 参数需要成对出现，前面是字段名，后面是别名
// 例如：MaxAs("bet_amount", "max_bet", "win_amount", "max_win") -> max(bet_amount) as max_bet, max(win_amount) as max_win
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

// min 语句带 as 别名, 别名和字段相同
// 例如：Min("bet_amount", "win_amount") -> min(bet_amount) as bet_amount, min(win_amount) as win_amount
func Min(columns ...string) string {
	sql := ""
	for i := 0; i < len(columns); i++ {
		sql += fmt.Sprintf("min(%v) as %v, ", columns[i], columns[i])
	}
	sql = strings.TrimSuffix(sql, ", ")
	return sql
}

// min 语句带 as 别名
// column 参数需要成对出现，前面是字段名，后面是别名
// 例如：MinAs("bet_amount", "min_bet", "win_amount", "min_win") -> min(bet_amount) as min_bet, min(win_amount) as min_win
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

// 分页查询 PageQuery 根据页码和每页条数查询数据,并返回总记录数
func PageQuery[T any](db *gorm.DB, page int, pageSize int) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 10 {
		pageSize = 10
	}
	if page*pageSize > 10000 {
		return nil, 0, fmt.Errorf("offset cannot exceed 10000")
	}
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
