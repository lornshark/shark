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
	Name string
	Type MappingType
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
			props[m.Name] = map[string]any{"type": string(m.Type)}
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
		props[m.Name] = map[string]any{"type": string(m.Type)}
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
