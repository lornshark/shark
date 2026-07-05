package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lornshark/shark/sharkapp"
	"github.com/redis/go-redis/v9"
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
	options, err := sharkapp.NewOption("kgame", "game-test")
	if err != nil {
		panic(err)
	}
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
	cli := MyClient{t.svc.RedisCluster}

	x := &Testx{
		Name:     "test",
		Password: "123456",
	}

	err := cli.SetObject(context.Background(), "test", &x).Err()
	if err != nil {
		fmt.Println(err)
	}

	var y Testx
	err = cli.GetObject(context.Background(), "test", &y).Err()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(y)

}

type Testx struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type MyClient struct {
	*redis.ClusterClient
}

func (t *MyClient) SetObject(ctx context.Context, key string, value any) *redis.StatusCmd {
	bytes, err := json.Marshal(value)
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	return t.Set(ctx, key, string(bytes), 0)
}

func (t *MyClient) GetObject(ctx context.Context, key string, value any) *redis.StatusCmd {
	result, err := t.Get(ctx, key).Result()
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	err = json.Unmarshal([]byte(result), value)
	if err != nil {
		return redis.NewStatusResult("", err)
	}
	return redis.NewStatusResult("OK", nil)
}
