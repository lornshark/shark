package main

import (
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
	ctx := t.svc.Context
	etcd := t.svc.Etcd

	// Put 一个键值对
	_, err := etcd.Put(ctx, "greeting", "Hello from shark Etcd module!")
	if err != nil {
		panic(err)
	}
	fmt.Println("Put 成功: greeting = Hello from shark Etcd module!")

	// Get 读取
	resp, err := etcd.Get(ctx, "greeting")
	if err != nil {
		panic(err)
	}
	for _, kv := range resp.Kvs {
		fmt.Printf("Get 成功: %s = %s\n", kv.Key, kv.Value)
	}
}
