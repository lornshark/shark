package test

import (
	"context"
	"testing"
	"time"

	"github.com/lornshark/shark/sharkelastic"
)

func TestElasticNew(t *testing.T) {
	cfg := loadElasticConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	es, err := sharkelastic.New(ctx, cfg)
	if err != nil {
		t.Fatalf("连接 Elasticsearch 失败: %v", err)
	}
	t.Log("Elasticsearch 连接成功")

	// 创建测试索引
	indexName := "test_shark_index"
	err = es.CreateIndex(ctx, indexName, 1,
		sharkelastic.FieldMapping{Name: "name", Type: sharkelastic.MappingTypeText},
		sharkelastic.FieldMapping{Name: "age", Type: sharkelastic.MappingTypeInteger},
	)
	if err != nil {
		t.Fatalf("CreateIndex 失败: %v", err)
	}
	t.Logf("创建索引: %s", indexName)

	// 批量插入
	err = es.Insert(ctx, indexName, "user_id",
		map[string]any{"user_id": "1", "name": "张三", "age": 25},
		map[string]any{"user_id": "2", "name": "李四", "age": 30},
	)
	if err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	t.Log("Bulk Insert 成功")

	// 刷新索引使文档可搜索
	es.Client.Indices.Refresh(es.Client.Indices.Refresh.WithIndex(indexName))

	// 搜索
	resp, err := es.Search(ctx, indexName, map[string]any{
		"query": map[string]any{
			"match": map[string]any{"name": "张三"},
		},
	})
	if err != nil {
		t.Fatalf("Search 失败: %v", err)
	}
	t.Logf("搜索结果: %s", string(resp))

	// 清理
	es.Client.Indices.Delete([]string{indexName})
	t.Log("Elasticsearch CRUD 验证通过")
}

func TestElasticSetIndexMapping(t *testing.T) {
	cfg := loadElasticConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	es, err := sharkelastic.New(ctx, cfg)
	if err != nil {
		t.Fatalf("连接 Elasticsearch 失败: %v", err)
	}

	indexName := "test_shark_mapping"
	es.CreateIndex(ctx, indexName, 1, sharkelastic.FieldMapping{Name: "title", Type: sharkelastic.MappingTypeText})

	// 添加新字段映射
	err = es.SetIndexMapping(ctx, indexName,
		sharkelastic.FieldMapping{Name: "tags", Type: sharkelastic.MappingTypeKeyword},
		sharkelastic.FieldMapping{Name: "score", Type: sharkelastic.MappingTypeFloat},
	)
	if err != nil {
		t.Fatalf("SetIndexMapping 失败: %v", err)
	}
	t.Log("SetIndexMapping 成功")

	es.Client.Indices.Delete([]string{indexName})
}
