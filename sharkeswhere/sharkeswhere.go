// Package sharkeswhere 提供将 SQL WHERE 风格字符串解析为 Elasticsearch Query DSL 的能力。
//
// 支持的操作符映射：
//
//	=      -> term query
//	!=     -> bool.must_not + term
//	>      -> range gt
//	>=     -> range gte
//	<      -> range lt
//	<=     -> range lte
//	in     -> terms
//	like   -> match
//	and    -> bool.must (自动扁平化)
//	or     -> bool.should (自动扁平化)
//	()     -> 分组
//
// 完整语法：
//
//	select field1, field2 from indexname where conditions limit N offset N
//
// select   → ES _source 字段过滤（可选）
// from     → 指定索引名（可选，在 where 之前）
// where    → 查询条件
// limit    → ES size（可选）
// offset   → ES from（分页偏移，可选，必须在 limit 之后）
//
// 使用示例：
//
//	pq, err := sharkeswhere.Build("status = 1 and age >= 18")
//	pq, err := sharkeswhere.Build("select name,age from users where status = 1 limit 10")
package sharkeswhere

import (
	"fmt"
	"strings"
)

// ParsedQuery 是 Build 的解析结果。
type ParsedQuery struct {
	// Index 为 from 子句指定的索引名，空字符串表示未指定。
	Index string
	// Body 为构建好的 ES 查询 DSL，可直接传给 ES Search API。
	// 包含 "query"、"size"、"from"、"_source" 等键。
	Body map[string]any
}

// Build 将 SQL 风格查询字符串解析为 ES 查询 DSL。
//
// 完整语法：
//
//	[select field,...] [from indexname] where conditions [limit N] [offset N]
//
// select 和 from 是可选的，向后兼容旧用法 "status = 1"。
func Build(where string) (*ParsedQuery, error) {
	input := strings.TrimSpace(where)
	if input == "" {
		return nil, fmt.Errorf("WHERE 子句不能为空")
	}

	var selectFields []string
	var fromIndex string
	var whereClause string

	// 从左到右解析：select / from / where / limit / offset
	remaining := input

	// 1. 提取最前面的 "select field1, field2, ..."
	remaining, selectFields = extractSelect(remaining)

	// 2. 提取 "from indexname"
	remaining, fromIndex = extractFrom(remaining)

	// 3. 提取 "where" 关键字，剩余部分为 where 条件
	var hasWhere bool
	remaining, hasWhere = extractWhere(remaining)

	if hasWhere {
		whereClause = remaining
	} else {
		// 如果没有显式 where 关键字，且之前有 select/from，剩余部分就是 where
		if len(selectFields) > 0 || fromIndex != "" {
			whereClause = remaining
		} else {
			// 纯 where 子句（向后兼容）
			whereClause = remaining
		}
	}

	// 4. 从尾部提取 order by / limit / offset
	var sortBody []any
	whereClause, sortBody = extractOrderBy(whereClause)
	var size, esFrom int
	whereClause, size, esFrom = extractLimitOffset(whereClause)

	// 5. Tokenize 并解析 where 条件
	whereClause = strings.TrimSpace(whereClause)
	var esQuery map[string]any
	if whereClause == "" {
		// 无条件查询，使用 match_all
		esQuery = map[string]any{"match_all": map[string]any{}}
	} else {
		tokens, err := tokenize(whereClause)
		if err != nil {
			return nil, fmt.Errorf("词法分析失败: %w", err)
		}

		p := &parser{tokens: tokens, pos: 0}
		esQuery, err = p.parseWhere()
		if err != nil {
			return nil, fmt.Errorf("语法解析失败: %w", err)
		}
		if p.pos < len(p.tokens)-1 {
			return nil, fmt.Errorf("语法解析失败: 第 %d 个 token 附近有未预期的内容 '%s'", p.pos, p.tokens[p.pos].value)
		}
	}

	// 6. 构建最终 body
	body := map[string]any{"query": esQuery}
	if size > 0 {
		body["size"] = size
	}
	if esFrom > 0 {
		body["from"] = esFrom
	}
	if len(selectFields) > 0 {
		body["_source"] = selectFields
	}
	if len(sortBody) > 0 {
		body["sort"] = sortBody
	}

	return &ParsedQuery{
		Index: fromIndex,
		Body:  body,
	}, nil
}

// extractSelect 从字符串开头提取 "select field1, field2, ..." 子句。
// 返回剩余字符串和字段列表。
func extractSelect(input string) (remaining string, fields []string) {
	lower := strings.ToLower(input)
	if !strings.HasPrefix(lower, "select ") && !strings.HasPrefix(lower, "select\t") {
		return input, nil
	}

	// 跳过 "select" 关键字
	rest := input[6:] // len("select")
	rest = strings.TrimLeft(rest, " \t")

	// 找到下一个关键字 "from" 或 "where"，字段列表在这之前
	lowerRest := strings.ToLower(rest)
	endIdx := findKeywordBoundary(lowerRest, " from ")
	if endIdx < 0 {
		endIdx = findKeywordEndBoundary(lowerRest, " from")
	}
	if endIdx < 0 {
		endIdx = findKeywordBoundary(lowerRest, " where ")
	}
	if endIdx < 0 {
		endIdx = findKeywordEndBoundary(lowerRest, " where")
	}

	var fieldsStr string
	if endIdx >= 0 {
		fieldsStr = strings.TrimSpace(rest[:endIdx])
		remaining = rest[endIdx:]
	} else {
		// select 后面没有 from/where，整个剩余为字段
		fieldsStr = strings.TrimSpace(rest)
		remaining = ""
	}

	if fieldsStr == "" || fieldsStr == "*" {
		return remaining, nil
	}

	for _, f := range strings.Split(fieldsStr, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			fields = append(fields, f)
		}
	}
	return remaining, fields
}

// findKeywordBoundary 在 s 中查找 " keyword " 并返回 keyword 的起始索引。
func findKeywordBoundary(s, keyword string) int {
	idx := strings.Index(s, keyword)
	if idx < 0 {
		return -1
	}
	return idx
}

// findKeywordEndBoundary 在 s 中查找 " keyword" 且 keyword 在末尾的情况。
func findKeywordEndBoundary(s, keyword string) int {
	trimmed := strings.TrimRight(s, " \t")
	if strings.HasSuffix(strings.ToLower(trimmed), keyword) {
		return len(trimmed) - len(keyword)
	}
	return -1
}

// extractFrom 从字符串开头提取 "from indexname" 子句。
// 返回剩余字符串和索引名。
func extractFrom(input string) (remaining string, indexName string) {
	trimmed := strings.TrimLeft(input, " \t")
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "from ") && !strings.HasPrefix(lower, "from\t") {
		return input, ""
	}

	rest := trimmed[4:] // len("from")
	rest = strings.TrimLeft(rest, " \t")

	// 索引名到下一个空格或关键字为止
	idx := strings.IndexAny(rest, " \t")
	if idx < 0 {
		return "", rest
	}
	indexName = rest[:idx]
	remaining = strings.TrimSpace(rest[idx:])
	return remaining, indexName
}

// extractWhere 从字符串开头提取 "where" 关键字。
// 返回剩余字符串和是否找到 where。
func extractWhere(input string) (remaining string, found bool) {
	trimmed := strings.TrimLeft(input, " \t")
	lowerTrimmed := strings.ToLower(trimmed)

	// 正好是 "where"，后面没有条件（可用 match_all）
	if lowerTrimmed == "where" {
		return "", true
	}
	if strings.HasPrefix(lowerTrimmed, "where ") || strings.HasPrefix(lowerTrimmed, "where\t") {
		// 找到 where 关键字，跳过它
		skipped := trimmed[5:] // len("where")
		return strings.TrimLeft(skipped, " \t"), true
	}
	return input, false
}

// extractLimitOffset 从 WHERE 字符串末尾提取 "limit N" 和 "offset N"。
// 返回去除这两个子句后的剩余字符串、size、from。
// offset 必须在 limit 之后，例如 "x = 1 limit 10 offset 5"。
func extractLimitOffset(input string) (remaining string, size int, from int) {
	lower := strings.ToLower(input)

	// 1. 先尝试匹配末尾的 "offset N"
	offsetIdx := lastKeywordIndex(lower, "offset")
	if offsetIdx >= 0 {
		after := strings.TrimSpace(input[offsetIdx+6:]) // 6 = len("offset")
		n, ok := parseInt(after)
		if ok {
			if n > 0 {
				from = n
			}
			input = strings.TrimSpace(input[:offsetIdx])
			lower = strings.ToLower(input)
		}
	}

	// 2. 再匹配末尾的 "limit N"
	limitIdx := lastKeywordIndex(lower, "limit")
	if limitIdx >= 0 {
		after := strings.TrimSpace(input[limitIdx+5:]) // 5 = len("limit")
		n, ok := parseInt(after)
		if ok {
			if n > 0 {
				size = n
			}
			input = strings.TrimSpace(input[:limitIdx])
		}
	}

	return input, size, from
}

// extractOrderBy 从字符串末尾提取 "order by field1 [asc|desc], field2 [asc|desc], ..."。
// 返回去除排序子句后的剩余字符串（保留 limit/offset）和 ES sort 数组。
//
// 语法:
//
//	order by field1 [asc|desc], field2 [asc|desc], ...
//
// 默认排序方向为 asc。order by 后的 limit/offset 会被保留在原位置供后续处理。
//
// 示例:
//
//	"name = '张' order by age desc"              → remaining="name = '张'", sort=[{"age":"desc"}]
//	"name = '张' order by age desc limit 10"     → remaining="name = '张'  limit 10", sort=[{"age":"desc"}]
//	"name = '张' order by age, name desc"        → remaining="name = '张'", sort=[{"age":"asc"},{"name":"desc"}]
func extractOrderBy(input string) (remaining string, sortBody []any) {
	lower := strings.ToLower(input)
	idx := lastKeywordIndex(lower, "order by")
	if idx < 0 {
		return input, nil
	}

	// 提取 "order by" 之后的所有内容
	after := strings.TrimSpace(input[idx+8:]) // 8 = len("order by")

	// 找到排序字段的结束位置（limit 或 offset 关键字之前）
	sortEnd := len(after)
	lowerAfter := strings.ToLower(after)
	if limitIdx := lastKeywordIndex(lowerAfter, "limit"); limitIdx >= 0 {
		sortEnd = limitIdx
	} else if offsetIdx := lastKeywordIndex(lowerAfter, "offset"); offsetIdx >= 0 {
		sortEnd = offsetIdx
	}

	// 排序字段部分
	fieldsStr := strings.TrimSpace(after[:sortEnd])
	// 剩余的 limit/offset 部分（保留原样拼回）
	tail := strings.TrimSpace(after[sortEnd:])

	// 解析字段列表：field1 [asc|desc], field2 [asc|desc], ...
	if fieldsStr != "" {
		parts := strings.Split(fieldsStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			words := strings.Fields(part)
			if len(words) == 0 {
				continue
			}
			field := words[0]
			dir := "asc"
			if len(words) >= 2 {
				lowerDir := strings.ToLower(words[1])
				if lowerDir == "desc" {
					dir = "desc"
				}
			}
			sortBody = append(sortBody, map[string]any{field: dir})
		}
	}

	// 拼接：where 条件 + 保留的 limit/offset
	remaining = strings.TrimSpace(input[:idx])
	if tail != "" {
		remaining = remaining + " " + tail
	}
	return remaining, sortBody
}

// lastKeywordIndex 返回 keyword 在 s 中最后一次出现的索引，要求 keyword 前后是单词边界。
func lastKeywordIndex(s, keyword string) int {
	idx := strings.LastIndex(s, keyword)
	if idx < 0 {
		return -1
	}
	// 检查前面是边界（开头或空白）
	if idx > 0 && s[idx-1] != ' ' && s[idx-1] != '\t' && s[idx-1] != '\n' && s[idx-1] != '\r' {
		return -1
	}
	// 检查后面是边界（结尾或空白）
	end := idx + len(keyword)
	if end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '\n' && s[end] != '\r' {
		return -1
	}
	return idx
}

// parseInt 解析正整数，返回值和是否成功。
func parseInt(s string) (int, bool) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// ---------------------------------------------------------------------------
// 公开类型（供测试使用）
// ---------------------------------------------------------------------------

// Token 表示一个词法单元。
type Token struct {
	Typ string // FIELD / OP / VALUE / LPAREN / RPAREN / COMMA / AND / OR
	Val string // 原始文本
}

// Tokenize 对 WHERE 字符串执行词法分析，返回 Token 列表。
func Tokenize(input string) ([]Token, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	var result []Token
	for _, tok := range tokens {
		if tok.typ == tokEOF {
			break
		}
		result = append(result, Token{
			Typ: tokTypeName(tok.typ),
			Val: tok.value,
		})
	}
	return result, nil
}

// ParseValue 将字符串值转为合适的 Go 类型（int64 / float64 / string）。
func ParseValue(s string) any {
	return parseValue(s)
}

// ---------------------------------------------------------------------------
// 词法分析 (Lexer)
// ---------------------------------------------------------------------------

type tokType int

const (
	tokEOF tokType = iota
	tokField
	tokOp
	tokValue
	tokLParen
	tokRParen
	tokComma
	tokAnd
	tokOr
)

type token struct {
	typ   tokType
	value string
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	runes := []rune(input)
	pos := 0
	size := len(runes)

	skipWhitespace := func() {
		for pos < size && (runes[pos] == ' ' || runes[pos] == '\t' || runes[pos] == '\n' || runes[pos] == '\r') {
			pos++
		}
	}

	readField := func() string {
		start := pos
		for pos < size && (isLetter(runes[pos]) || isDigit(runes[pos]) || runes[pos] == '_' || runes[pos] == '.') {
			pos++
		}
		return string(runes[start:pos])
	}

	readNumber := func() string {
		start := pos
		if pos < size && runes[pos] == '-' {
			pos++
		}
		for pos < size && isDigit(runes[pos]) {
			pos++
		}
		if pos < size && runes[pos] == '.' {
			pos++
			for pos < size && isDigit(runes[pos]) {
				pos++
			}
		}
		return string(runes[start:pos])
	}

	readQuoted := func(quote rune) (string, error) {
		pos++ // 跳过开引号
		var buf []rune
		for pos < size {
			if runes[pos] == '\\' && pos+1 < size && runes[pos+1] == quote {
				buf = append(buf, quote)
				pos += 2
				continue
			}
			if runes[pos] == quote {
				pos++ // 跳过闭引号
				return string(buf), nil
			}
			buf = append(buf, runes[pos])
			pos++
		}
		return "", fmt.Errorf("未闭合的字符串字面量")
	}

	for pos < size {
		skipWhitespace()
		if pos >= size {
			break
		}

		c := runes[pos]

		if c == '(' {
			tokens = append(tokens, token{typ: tokLParen, value: "("})
			pos++
			continue
		}
		if c == ')' {
			tokens = append(tokens, token{typ: tokRParen, value: ")"})
			pos++
			continue
		}
		if c == ',' {
			tokens = append(tokens, token{typ: tokComma, value: ","})
			pos++
			continue
		}
		if c == '\'' || c == '"' {
			s, err := readQuoted(c)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{typ: tokValue, value: s})
			continue
		}
		if isDigit(c) || (c == '-' && pos+1 < size && isDigit(runes[pos+1])) {
			num := readNumber()
			tokens = append(tokens, token{typ: tokValue, value: num})
			continue
		}

		if c == '!' && pos+1 < size && runes[pos+1] == '=' {
			tokens = append(tokens, token{typ: tokOp, value: "!="})
			pos += 2
			continue
		}
		if c == '>' && pos+1 < size && runes[pos+1] == '=' {
			tokens = append(tokens, token{typ: tokOp, value: ">="})
			pos += 2
			continue
		}
		if c == '<' && pos+1 < size && runes[pos+1] == '=' {
			tokens = append(tokens, token{typ: tokOp, value: "<="})
			pos += 2
			continue
		}
		if c == '=' || c == '>' || c == '<' {
			tokens = append(tokens, token{typ: tokOp, value: string(c)})
			pos++
			continue
		}

		if isLetter(c) || c == '_' {
			word := readField()
			lower := strings.ToLower(word)
			switch lower {
			case "and":
				tokens = append(tokens, token{typ: tokAnd, value: word})
			case "or":
				tokens = append(tokens, token{typ: tokOr, value: word})
			case "in":
				tokens = append(tokens, token{typ: tokOp, value: "in"})
			case "like":
				tokens = append(tokens, token{typ: tokOp, value: "like"})
			default:
				tokens = append(tokens, token{typ: tokField, value: word})
			}
			continue
		}

		return nil, fmt.Errorf("位置 %d: 未预期的字符 '%c'", pos, c)
	}

	tokens = append(tokens, token{typ: tokEOF, value: ""})
	return tokens, nil
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func tokTypeName(t tokType) string {
	switch t {
	case tokEOF:
		return "EOF"
	case tokField:
		return "FIELD"
	case tokOp:
		return "OP"
	case tokValue:
		return "VALUE"
	case tokLParen:
		return "LPAREN"
	case tokRParen:
		return "RPAREN"
	case tokComma:
		return "COMMA"
	case tokAnd:
		return "AND"
	case tokOr:
		return "OR"
	default:
		return "UNKNOWN"
	}
}

// ---------------------------------------------------------------------------
// 语法解析 (Recursive Descent Parser)
// ---------------------------------------------------------------------------

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) cur() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{typ: tokEOF}
}

func (p *parser) advance() { p.pos++ }

// parseWhere 解析完整 WHERE 子句。
//
// 语法（优先级从低到高）:
//
//	or_expr  := and_expr ('or' and_expr)*
//	and_expr := unary ('and' unary)*
//	unary    := '(' or_expr ')' | comparison
//	comparison := field op value | field 'in' '(' value (',' value)* ')'
func (p *parser) parseWhere() (map[string]any, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (map[string]any, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.cur().typ == tokOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = mergeShould(left, right)
	}
	return left, nil
}

func (p *parser) parseAnd() (map[string]any, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.cur().typ == tokAnd {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = mergeMust(left, right)
	}
	return left, nil
}

func (p *parser) parseUnary() (map[string]any, error) {
	if p.cur().typ == tokLParen {
		p.advance()
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.cur().typ != tokRParen {
			return nil, fmt.Errorf("期望 ')'，但得到 '%s'", p.cur().value)
		}
		p.advance()
		return expr, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (map[string]any, error) {
	if p.cur().typ != tokField {
		return nil, fmt.Errorf("期望字段名，但得到 '%s'", p.cur().value)
	}
	field := p.cur().value
	p.advance()

	if p.cur().typ != tokOp {
		return nil, fmt.Errorf("期望运算符，但得到 '%s'（字段: %s）", p.cur().value, field)
	}
	op := p.cur().value
	p.advance()

	switch op {
	case "=":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return termQuery(field, val), nil
	case "!=":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return mustNotQuery(termQuery(field, val)), nil
	case ">":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return rangeQuery(field, "gt", val), nil
	case ">=":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return rangeQuery(field, "gte", val), nil
	case "<":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return rangeQuery(field, "lt", val), nil
	case "<=":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return rangeQuery(field, "lte", val), nil
	case "like":
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return matchQuery(field, val), nil
	case "in":
		return p.parseIn(field)
	default:
		return nil, fmt.Errorf("不支持的运算符: '%s'", op)
	}
}

func (p *parser) parseValue() (any, error) {
	if p.cur().typ != tokValue {
		return nil, fmt.Errorf("期望值，但得到 '%s'", p.cur().value)
	}
	val := parseValue(p.cur().value)
	p.advance()
	return val, nil
}

func (p *parser) parseIn(field string) (map[string]any, error) {
	if p.cur().typ != tokLParen {
		return nil, fmt.Errorf("IN 后面期望 '('，但得到 '%s'", p.cur().value)
	}
	p.advance()

	var values []any
	for {
		if p.cur().typ == tokRParen {
			p.advance()
			if len(values) == 0 {
				return nil, fmt.Errorf("IN 列表不能为空（字段: %s）", field)
			}
			return termsQuery(field, values), nil
		}
		if len(values) > 0 {
			if p.cur().typ != tokComma {
				return nil, fmt.Errorf("IN 列表中期望 ','，但得到 '%s'", p.cur().value)
			}
			p.advance()
		}
		if p.cur().typ != tokValue {
			return nil, fmt.Errorf("IN 列表中期望值，但得到 '%s'", p.cur().value)
		}
		values = append(values, parseValue(p.cur().value))
		p.advance()
	}
}

// ---------------------------------------------------------------------------
// 值解析
// ---------------------------------------------------------------------------

func parseValue(s string) any {
	var iv int64
	if n, err := fmt.Sscanf(s, "%d", &iv); err == nil && n == 1 && fmt.Sprintf("%d", iv) == s {
		return iv
	}
	var fv float64
	if n, err := fmt.Sscanf(s, "%f", &fv); err == nil && n == 1 && fmt.Sprintf("%g", fv) == s {
		return fv
	}
	return s
}

// ---------------------------------------------------------------------------
// ES Query DSL 构建
// ---------------------------------------------------------------------------

func termQuery(field string, value any) map[string]any {
	return map[string]any{"term": map[string]any{field: value}}
}

func termsQuery(field string, values []any) map[string]any {
	return map[string]any{"terms": map[string]any{field: values}}
}

func matchQuery(field string, value any) map[string]any {
	return map[string]any{"match": map[string]any{field: value}}
}

func rangeQuery(field, op string, value any) map[string]any {
	return map[string]any{"range": map[string]any{field: map[string]any{op: value}}}
}

func mustNotQuery(inner map[string]any) map[string]any {
	return map[string]any{"bool": map[string]any{"must_not": inner}}
}

func mergeMust(left, right map[string]any) map[string]any {
	lc := extractClauses(left, "must")
	rc := extractClauses(right, "must")
	return map[string]any{"bool": map[string]any{"must": append(lc, rc...)}}
}

func mergeShould(left, right map[string]any) map[string]any {
	lc := extractClauses(left, "should")
	rc := extractClauses(right, "should")
	return map[string]any{"bool": map[string]any{"should": append(lc, rc...)}}
}

func extractClauses(q map[string]any, clause string) []any {
	if b, ok := q["bool"]; ok {
		if bm, ok := b.(map[string]any); ok {
			if e, ok := bm[clause]; ok {
				if arr, ok := e.([]any); ok {
					return arr
				}
			}
		}
	}
	return []any{q}
}
