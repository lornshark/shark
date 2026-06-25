package sharkelastic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
	"github.com/lornshark/shark/sharkeswhere"
	"github.com/tidwall/gjson"
)

type SharkElastic struct {
	Client *elasticsearch.Client
}

// MappingType 定义 Elasticsearch 字段类型常量
type MappingType string

const (
	MappingTypeText    MappingType = "text"
	MappingTypeKeyword MappingType = "keyword"
	MappingTypeInteger MappingType = "integer"
	MappingTypeLong    MappingType = "long"
	MappingTypeFloat   MappingType = "float"
	MappingTypeDouble  MappingType = "double"
	MappingTypeBoolean MappingType = "boolean"
	MappingTypeDate    MappingType = "date"
	MappingTypeBinary  MappingType = "binary"
	MappingTypeObject  MappingType = "object"
	MappingTypeNested  MappingType = "nested"
)

// FieldMapping 定义索引字段映射
type FieldMapping struct {
	Name   string
	Type   MappingType
	Format string // 可选，字段格式，例如 date 类型的 "yyyy-MM-dd" 或 "yyyy-MM-dd HH:mm:ss"
}

// CreateIndex 创建索引，可指定分片数和字段映射
// index: 索引名称
// shards: 主分片数量（<=0 时使用 ES 默认值）
// mappings: 可变参数，字段映射列表；不传则只创建索引不带 mapping
//
// 使用示例:
//
//	// 创建索引并指定分片数和映射
//	err := client.CreateIndex(ctx, "users", 2,
//	    sharkelastic.FieldMapping{Name: "name", Type: sharkelastic.MappingTypeText},
//	    sharkelastic.FieldMapping{Name: "age", Type: sharkelastic.MappingTypeInteger},
//	    sharkelastic.FieldMapping{Name: "email", Type: sharkelastic.MappingTypeKeyword},
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 创建索引不指定映射（分片数 1）
//	err := client.CreateIndex(ctx, "orders", 1)
func (s *SharkElastic) CreateIndex(ctx context.Context, index string, shards int, mappings ...FieldMapping) error {
	if index == "" {
		return fmt.Errorf("索引名称不能为空")
	}

	settings := map[string]any{}
	if shards > 0 {
		settings["number_of_shards"] = shards
	}

	body := map[string]any{}
	if len(settings) > 0 {
		body["settings"] = settings
	}
	if len(mappings) > 0 {
		props := map[string]any{}
		for _, m := range mappings {
			field := map[string]any{"type": string(m.Type)}
			if m.Format != "" {
				field["format"] = m.Format
			}
			props[m.Name] = field
		}
		body["mappings"] = map[string]any{"properties": props}
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyBytes, err := sonic.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化创建索引请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  bodyReader,
	}
	resp, err := req.Do(ctx, s.Client)
	if err != nil {
		return fmt.Errorf("创建索引请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		errBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("创建索引失败(状态:%s): %s", resp.Status(), string(errBytes))
	}

	return nil
}

// CreateIndexStrict 创建严格模式的索引，不可以动态添加未定义的字段
// index: 索引名称
// shards: 主分片数量（<=0 时使用 ES 默认值）
// mappings: 可变参数，字段映射列表
//
// 使用示例:
//
//	err := client.CreateIndexStrict(ctx, "users", 2,
//	    sharkelastic.FieldMapping{Name: "name", Type: sharkelastic.MappingTypeText},
//	    sharkelastic.FieldMapping{Name: "age", Type: sharkelastic.MappingTypeInteger},
//	)
func (s *SharkElastic) CreateIndexStrict(ctx context.Context, index string, shards int, mappings ...FieldMapping) error {
	if index == "" {
		return fmt.Errorf("索引名称不能为空")
	}

	settings := map[string]any{}
	if shards > 0 {
		settings["number_of_shards"] = shards
	}

	body := map[string]any{}
	if len(settings) > 0 {
		body["settings"] = settings
	}

	mappingsBody := map[string]any{
		"dynamic": "strict",
	}
	if len(mappings) > 0 {
		props := map[string]any{}
		for _, m := range mappings {
			field := map[string]any{"type": string(m.Type)}
			if m.Format != "" {
				field["format"] = m.Format
			}
			props[m.Name] = field
		}
		mappingsBody["properties"] = props
	}
	body["mappings"] = mappingsBody

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyBytes, err := sonic.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化创建索引请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  bodyReader,
	}
	resp, err := req.Do(ctx, s.Client)
	if err != nil {
		return fmt.Errorf("创建索引请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		errBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("创建索引失败(状态:%s): %s", resp.Status(), string(errBytes))
	}

	return nil
}

// SetIndexMapping 为已存在的索引添加或更新字段映射
// index: 索引名称
// mappings: 可变参数，要添加/更新的字段映射列表
//
// 使用示例:
//
//	// 为已有索引添加新字段映射
//	err := client.SetIndexMapping(ctx, "users",
//	    sharkelastic.FieldMapping{Name: "phone", Type: sharkelastic.MappingTypeKeyword},
//	    sharkelastic.FieldMapping{Name: "birthday", Type: sharkelastic.MappingTypeDate},
//	)
func (s *SharkElastic) SetIndexMapping(ctx context.Context, index string, mappings ...FieldMapping) error {
	if index == "" {
		return fmt.Errorf("索引名称不能为空")
	}
	if len(mappings) == 0 {
		return fmt.Errorf("至少需要提供一个字段映射")
	}

	props := map[string]any{}
	for _, m := range mappings {
		field := map[string]any{"type": string(m.Type)}
		if m.Format != "" {
			field["format"] = m.Format
		}
		props[m.Name] = field
	}
	body := map[string]any{
		"properties": props,
	}
	bodyBytes, err := sonic.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化映射请求体失败: %w", err)
	}

	req := esapi.IndicesPutMappingRequest{
		Index: []string{index},
		Body:  bytes.NewReader(bodyBytes),
	}
	resp, err := req.Do(ctx, s.Client)
	if err != nil {
		return fmt.Errorf("更新映射请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		errBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("更新映射失败(状态:%s): %s", resp.Status(), string(errBytes))
	}

	return nil
}

// Search 在指定索引中执行搜索查询
// index: 索引名称
// query: 搜索查询参数，必须是一个非空的 map[string]any 类型，表示 Elasticsearch 查询 DSL
// 返回原始响应字节和错误
//
// 用法示例:
//
//	// 构建搜索查询参数
//	searchParams := map[string]any{
//	    "query": map[string]any{
//	        "match": map[string]any{
//	            "name": "张三",
//	        },
//	    },
//	}
//	// 执行搜索查询
//	respBytes, err := elasticClient.Search(context.Background(), "users", searchParams)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("搜索结果:", string(respBytes))
func (s *SharkElastic) Search(ctx context.Context, index string, query map[string]any) ([]byte, error) {
	if index == "" {
		return nil, fmt.Errorf("索引名称不能为空")
	}
	if len(query) == 0 {
		return nil, fmt.Errorf("搜索参数不能为空")
	}

	queryBytes, err := sonic.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("搜索参数序列化失败: %w", err)
	}

	searchReq := esapi.SearchRequest{
		Index: []string{index},
		Body:  bytes.NewReader(queryBytes),
	}

	resp, err := searchReq.Do(ctx, s.Client)
	if err != nil {
		return nil, fmt.Errorf("搜索请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		errBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("搜索失败(状态:%s), 读取错误响应失败: %w", resp.Status(), readErr)
		}
		return nil, fmt.Errorf("搜索失败: %s", string(errBytes))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取搜索响应失败: %w", err)
	}

	return respBytes, nil
}

// Insert 使用 Bulk API 一次请求批量插入文档到指定索引
// 内部通过 gjson 从每个文档的序列化 JSON 中提取 idField 对应的值作为 _id，
// 然后拼接 NDJSON 请求体发送 esapi.BulkRequest。
// 任意一条文档失败则整体返回汇总错误信息（包含失败条目序号、状态码和原因），全部成功返回 nil。
//
// 参数:
//   - ctx:     上下文
//   - index:   Elasticsearch 索引名称
//   - idField: 文档中作为 _id 的字段名，该字段必须存在且非空
//   - docs:    可变参数，一个或多个待插入文档（any 类型）
//
// 使用示例:
//
//	// 批量插入
//	docs := []any{
//	    map[string]any{"user_id": "1", "name": "张三", "age": 25},
//	    map[string]any{"user_id": "2", "name": "李四", "age": 30},
//	}
//	if err := client.Insert(ctx, "users", "user_id", docs...); err != nil {
//	    log.Fatal(err)
//	}
func (s *SharkElastic) Insert(ctx context.Context, index string, idField string, docs ...any) error {
	if index == "" {
		return fmt.Errorf("索引名称不能为空")
	}
	if idField == "" {
		return fmt.Errorf("文档Id字段名称不能为空")
	}
	if len(docs) == 0 {
		return fmt.Errorf("至少需要提供一个文档")
	}

	// 构建 Bulk 请求体的 NDJSON
	var buf bytes.Buffer
	for i, doc := range docs {
		// 序列化文档为 JSON
		docBytes, err := sonic.Marshal(doc)
		if err != nil {
			return fmt.Errorf("第 %d 个文档序列化失败: %w", i+1, err)
		}

		// 使用 gjson 从 JSON 字节中直接提取 ID 字段
		idResult := gjson.GetBytes(docBytes, idField)
		if !idResult.Exists() {
			return fmt.Errorf("第 %d 个文档中未找到ID字段 '%s'", i+1, idField)
		}
		docID := idResult.String()
		if docID == "" {
			return fmt.Errorf("第 %d 个文档的ID字段 '%s' 值为空", i+1, idField)
		}

		// 写入 action 行
		action := map[string]any{
			"index": map[string]any{
				"_index": index,
				"_id":    docID,
			},
		}
		actionBytes, err := sonic.Marshal(action)
		if err != nil {
			return fmt.Errorf("第 %d 个文档 action 序列化失败: %w", i+1, err)
		}
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		// 写入文档行
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	// 发送 Bulk 请求
	bulkReq := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}
	resp, err := bulkReq.Do(ctx, s.Client)
	if err != nil {
		return fmt.Errorf("Bulk 请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取 Bulk 响应失败: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("Bulk 请求失败(状态:%s): %s", resp.Status(), string(respBytes))
	}

	// 解析响应，检查是否有失败的条目
	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
			Error  struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"items"`
	}
	if err := sonic.Unmarshal(respBytes, &bulkResp); err != nil {
		return fmt.Errorf("解析 Bulk 响应失败: %w", err)
	}

	if bulkResp.Errors {
		var errMsgs []string
		for i, item := range bulkResp.Items {
			for _, result := range item {
				if result.Error.Type != "" || result.Error.Reason != "" {
					errMsgs = append(errMsgs, fmt.Sprintf(
						"第 %d 个文档: [%d] %s: %s", i+1, result.Status, result.Error.Type, result.Error.Reason,
					))
				}
			}
		}
		return fmt.Errorf("Bulk 操作部分失败:\n%s", strings.Join(errMsgs, "\n"))
	}

	return nil
}

// Upsert 使用 Bulk API 批量 upsert 文档（不存在则插入，存在则局部更新）
// 函数签名与 Insert 完全一致，内部使用 update action + doc_as_upsert: true，
// 实现文档不存在时插入、存在时只更新传入字段（局部更新，不覆盖未传入字段）。
//
// 参数:
//   - ctx:     上下文
//   - index:   Elasticsearch 索引名称
//   - idField: 文档中作为 _id 的字段名，该字段必须存在且非空
//   - docs:    可变参数，一个或多个待 upsert 文档（any 类型）
//
// 使用示例:
//
//	// 批量 upsert - 文档不存在则插入，存在则只更新传入字段
//	docs := []any{
//	    map[string]any{"user_id": "1", "name": "张三"},
//	    map[string]any{"user_id": "2", "name": "李四"},
//	}
//	if err := client.Upsert(ctx, "users", "user_id", docs...); err != nil {
//	    log.Fatal(err)
//	}
func (s *SharkElastic) Upsert(ctx context.Context, index string, idField string, docs ...any) error {
	if index == "" {
		return fmt.Errorf("索引名称不能为空")
	}
	if idField == "" {
		return fmt.Errorf("文档Id字段名称不能为空")
	}
	if len(docs) == 0 {
		return fmt.Errorf("至少需要提供一个文档")
	}

	// 构建 Bulk 请求体的 NDJSON
	var buf bytes.Buffer
	for i, doc := range docs {
		// 序列化文档为 JSON
		docBytes, err := sonic.Marshal(doc)
		if err != nil {
			return fmt.Errorf("第 %d 个文档序列化失败: %w", i+1, err)
		}

		// 使用 gjson 从 JSON 字节中直接提取 ID 字段
		idResult := gjson.GetBytes(docBytes, idField)
		if !idResult.Exists() {
			return fmt.Errorf("第 %d 个文档中未找到ID字段 '%s'", i+1, idField)
		}
		docID := idResult.String()
		if docID == "" {
			return fmt.Errorf("第 %d 个文档的ID字段 '%s' 值为空", i+1, idField)
		}

		// 写入 update action 行，doc_as_upsert: true 实现 upsert
		action := map[string]any{
			"update": map[string]any{
				"_index": index,
				"_id":    docID,
			},
		}
		actionBytes, err := sonic.Marshal(action)
		if err != nil {
			return fmt.Errorf("第 %d 个文档 action 序列化失败: %w", i+1, err)
		}
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		// 写入 doc 包装行（局部更新，只更新传入的字段）
		updateBody := map[string]any{"doc": doc, "doc_as_upsert": true}
		updateBytes, err := sonic.Marshal(updateBody)
		if err != nil {
			return fmt.Errorf("第 %d 个文档 update body 序列化失败: %w", i+1, err)
		}
		buf.Write(updateBytes)
		buf.WriteByte('\n')
	}

	// 发送 Bulk 请求
	bulkReq := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}
	resp, err := bulkReq.Do(ctx, s.Client)
	if err != nil {
		return fmt.Errorf("Bulk 请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取 Bulk 响应失败: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("Bulk 请求失败(状态:%s): %s", resp.Status(), string(respBytes))
	}

	// 解析响应，检查是否有失败的条目
	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
			Error  struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"items"`
	}
	if err := sonic.Unmarshal(respBytes, &bulkResp); err != nil {
		return fmt.Errorf("解析 Bulk 响应失败: %w", err)
	}

	if bulkResp.Errors {
		var errMsgs []string
		for i, item := range bulkResp.Items {
			for _, result := range item {
				if result.Error.Type != "" || result.Error.Reason != "" {
					errMsgs = append(errMsgs, fmt.Sprintf(
						"第 %d 个文档: [%d] %s: %s", i+1, result.Status, result.Error.Type, result.Error.Reason,
					))
				}
			}
		}
		return fmt.Errorf("Bulk 操作部分失败:\n%s", strings.Join(errMsgs, "\n"))
	}

	return nil
}

// SqlRaw 执行 SQL 风格查询并返回原始 ES 响应 JSON 字节。
//
// 设计目的
//
// SqlRaw 面向需要直接处理 ES 原始响应的场景（如手动解析 hits 元数据、调试 SQL 翻译结果等）。
// 它返回 ES Search API 的完整 JSON 响应，调用方可自行提取 hits.hits、hits.total 等字段。
//
// 与 Find 的边界
//
//   - SqlRaw → 返回原始 ES JSON 字节，灵活但需手动解析。适合需要 total、_score 等元信息、
//     或自定义反序列化逻辑的场景。
//   - Find   → 自动提取 _source 并反序列化到切片。仅需文档列表时更便捷（类似 GORM Find）。
//
// SQL 完整格式：
//
//	[select field,...] from indexname where conditions [order by field [asc|desc], ...] [limit N] [offset N]
//
// 各子句映射：
//
//	select name,age   → ES _source 字段过滤（仅返回指定字段）
//	from indexname    → 目标索引（必填，无默认值）
//	where conditions  → 过滤条件，仅支持以下运算符和逻辑组合
//	order by field... → ES sort
//	limit N           → ES size，返回文档数上限
//	offset N          → ES from，分页偏移量（必须在 limit 之后）
//
// where 条件支持的运算符：
//
//	=      → term         精确匹配
//	!=     → bool.must_not
//	>      → range gt      大于
//	>=     → range gte     大于等于
//	<      → range lt      小于
//	<=     → range lte     小于等于
//	in     → terms         多值匹配
//	like   → match         全文搜索（⚠️ 非 SQL LIKE 通配符，是 ES match 查询）
//	and    → bool.must     逻辑与（AND 优先级高于 OR）
//	or     → bool.should   逻辑或
//	()     → 分组，改变优先级
//
// order by 排序：
//
//	order by field1 [asc|desc], field2 [asc|desc], ...
//	默认排序方向为 asc，多字段用逗号分隔
//	位于 limit/offset 之前：where ... order by age desc limit 10
//
// 不支持的功能及其原因：
//
//   - 聚合（aggregations）、分组（group by）
//     → 聚合返回聚合桶而非文档列表，与当前文档查询模型完全不同
//   - select 别名（如 select name as n）
//     → ES _source 不支持字段重命名，需 script_fields 替代，但返回结构从 _source 变为 fields，
//     且 Find() 的"反序列化到结构体"流程会完全失效
//   - select DISTINCT
//     → ES 无原生 distinct 查询，需 collapse（折叠）或 terms aggregation，
//     本质是聚合去重，不再是文档查询语义
//   - select 表达式（如 select age+1, price*0.9）
//     → ES 需 script_fields + Painless 脚本，返回 fields 数组而非 _source，
//     且引入脚本注入风险和性能开销
//   - SQL 通配符 LIKE（%）、BETWEEN、IS NULL
//     → 当前 like 映射为 ES match（全文搜索），非通配符匹配
//   - 算术表达式、函数调用、子查询、JOIN
//     → ES 不支持 SQL 级别的关系运算和嵌套查询
//
// 注意：
//   - 字符串值需用引号包裹：name = '张三' 或 name = "张三"
//   - IN 值列表用括号：status in (1, 2, 3)
//   - like 映射为 ES Match Query（全文搜索语义），不是 SQL 的通配符匹配
//
// 使用示例：
//
//	// 简单条件查询
//	resp, err := es.SqlRaw(ctx, "from users where status = 1")
//
//	// 多条件 + 范围 + 分页
//	resp, err := es.SqlRaw(ctx, "from users where status = 1 and age >= 18 limit 10")
//
//	// 字段过滤 + OR 分组
//	resp, err := es.SqlRaw(ctx, "select name,age from users where (city = '北京' or city = '上海') and status = 1")
//
//	// 排序 + 分页
//	resp, err := es.SqlRaw(ctx, "from users where status = 1 order by age desc limit 10 offset 5")
//
// 参数：
//   - ctx: 上下文
//   - sql: SQL 风格查询字符串，必须包含 from 子句
//
// 返回：
//   - []byte: ES 搜索原始响应 JSON
//   - error:  解析失败、缺少 from 子句或 ES 请求失败时返回错误
func (s *SharkElastic) SqlRaw(ctx context.Context, sql string) ([]byte, error) {
	pq, err := sharkeswhere.Build(sql)
	if err != nil {
		return nil, err
	}
	if pq.Index == "" {
		return nil, fmt.Errorf("SQL 缺少 from 子句指定索引")
	}
	return s.Search(ctx, pq.Index, pq.Body)
}

// Find 执行 SQL 风格查询，自动提取 _source 并反序列化到目标切片。
//
// 设计目的
//
// Find 面向简单的文档列表查询场景，提供类似 GORM Find 的开发体验。
// 内部自动完成：SQL 解析 → ES 搜索 → 提取 hits.hits._source 数组 → JSON 反序列化到 result。
// 不返回 total、_score 等元数据，仅返回文档内容列表。
//
// 与 SqlRaw 的边界
//
//   - Find   → 仅返回文档 _source 切片，适合"查列表"场景。result 必须是非 nil 切片指针。
//   - SqlRaw → 返回完整 ES 响应（含 hits.total、_score、shards 等），适合需要元数据的场景。
//
// SQL 格式：
//
//	[select field,...] from indexname where conditions [order by field [asc|desc], ...] [limit N] [offset N]
//
// 支持的运算符：= != > >= < <= in like and or ()、order by、limit、offset
//
// 不支持的功能及其原因：
//
//   - 聚合（aggregations）、分组（group by）
//     → 聚合返回聚合桶而非文档列表，与当前文档查询模型完全不同
//   - select 别名（如 select name as n）
//     → ES _source 不支持字段重命名，需 script_fields 替代，但返回结构从 _source 变为 fields，
//     且 Find() 的"反序列化到结构体"流程会完全失效
//   - select DISTINCT
//     → ES 无原生 distinct 查询，需 collapse（折叠）或 terms aggregation，
//     本质是聚合去重，不再是文档查询语义
//   - select 表达式（如 select age+1, price*0.9）
//     → ES 需 script_fields + Painless 脚本，返回 fields 数组而非 _source，
//     且引入脚本注入风险和性能开销
//   - SQL 通配符 LIKE（%）、BETWEEN、IS NULL
//     → 当前 like 映射为 ES match（全文搜索），非通配符匹配
//   - 算术表达式、函数调用、子查询、JOIN
//     → ES 不支持 SQL 级别的关系运算和嵌套查询
//
// 注意事项
//
//   - result 必须是指向切片的指针（如 &[]User{}），否则反序列化失败
//   - result 对应的结构体字段需要使用 json tag 匹配 ES 文档字段名
//   - 查询无结果时，result 指向的空切片长度为 0，不返回错误
//   - from 子句为必填，缺少时返回错误
//   - like 映射为 ES Match Query（全文搜索），不是 SQL 通配符
//
// 使用示例：
//
//	type User struct {
//	    UserID string `json:"user_id"`
//	    Name   string `json:"name"`
//	    Age    int    `json:"age"`
//	}
//
//	// 条件查询
//	var users []User
//	err := es.Find(ctx, "from users where status = 1", &users)
//
//	// 字段过滤 + 分页
//	err = es.Find(ctx, "select name,age from users where age >= 18 limit 10 offset 5", &users)
//
//	// 无结果（users 为空切片，err 为 nil）
//	err = es.Find(ctx, "from users where age > 200", &users)
//
// 参数：
//   - ctx:    上下文
//   - sql:    SQL 风格查询字符串，必须包含 from 子句
//   - result: 指向切片的指针（如 &[]User{}），不能为 nil
//
// 返回：
//   - error: 解析失败、缺少 from 子句、ES 请求失败或反序列化失败时返回错误
func (s *SharkElastic) Find(ctx context.Context, sql string, result any) error {
	if strings.TrimSpace(sql) == "" {
		return fmt.Errorf("SQL 不能为空")
	}
	if result == nil {
		return fmt.Errorf("result 不能为 nil")
	}

	// 1. 构建查询 DSL
	pq, err := sharkeswhere.Build(sql)
	if err != nil {
		return fmt.Errorf("构建查询失败: %w", err)
	}
	if pq.Index == "" {
		return fmt.Errorf("SQL 缺少 from 子句指定索引")
	}

	// 2. 执行搜索
	respBytes, err := s.Search(ctx, pq.Index, pq.Body)
	if err != nil {
		return err
	}

	// 3. 解析 ES 响应：提取 hits.hits[]._source 数组
	sourcesJSON := gjson.GetBytes(respBytes, "hits.hits.#._source")
	if !sourcesJSON.Exists() {
		return fmt.Errorf("解析 ES 响应失败: 未找到 hits.hits")
	}

	// 将 JSON 数组反序列化到 result
	if err := sonic.Unmarshal([]byte(sourcesJSON.Raw), result); err != nil {
		return fmt.Errorf("反序列化结果失败: %w", err)
	}

	return nil
}
