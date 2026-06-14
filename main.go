package main

import (
	"context"
	"fmt"

	"github.com/lornshark/shark/sharkapp"
)

// @title game-demo API
// @version 1.0

// @BasePath  /api

// @securityDefinitions.apiKey ApiKeyAuth
// @in header
// @name x-token

// go get -u github.com/swaggo/swag/cmd/swag
// go install github.com/swaggo/swag/cmd/swag@v1.16.4
// swag v1.16.4

// swag init --parseDependency -g  main.go

func main() {
	options := sharkapp.NewOption("kgame", "game-test")
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt(&Test{svc: app})
}

type Test struct {
	svc *sharkapp.App
}

func (t *Test) Start() {
	index := "ktestabc1"

	// // 创建索引，指定 2 个分片 + 字段映射
	// if err := t.svc.Elastic.CreateIndex(context.Background(), index, 2,
	// 	sharkelastic.FieldMapping{Name: "a", Type: sharkelastic.MappingTypeText},
	// 	sharkelastic.FieldMapping{Name: "b", Type: sharkelastic.MappingTypeInteger},
	// 	sharkelastic.FieldMapping{Name: "c", Type: sharkelastic.MappingTypeKeyword},
	// ); err != nil {
	// 	panic(err)
	// }
	// fmt.Println("创建索引成功")

	// // 为已有索引追加新字段映射
	// if err := t.svc.Elastic.SetIndexMapping(context.Background(), index,
	// 	sharkelastic.FieldMapping{Name: "d", Type: sharkelastic.MappingTypeBoolean},
	// ); err != nil {
	// 	panic(err)
	// }
	// fmt.Println("追加映射成功")

	// // 测试 Bulk Insert
	docs := []any{
		map[string]any{"a": "I am kent abc", "b": 22, "c": "abu", "d": true},
		map[string]any{"a": "hello world", "b": 33, "c": "xyz", "d": false},
	}
	if err := t.svc.Elastic.Insert(context.Background(), index, "c", docs...); err != nil {
		panic(err)
	}
	fmt.Println("Insert 成功")

	// 搜索验证
	result, err := t.svc.Elastic.Search(context.Background(), index, map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(string(result))
}
