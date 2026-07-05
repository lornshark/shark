package main

import (
	"context"
	"fmt"

	"github.com/lornshark/shark/sharkapp"
	"github.com/shopspring/decimal"
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

	x := &Testx{
		Name:     "test",
		Password: "123456",
		Id:       123,
		Abc: struct {
			Def string `json:"def"`
		}{
			Def: "abc",
		},
		X: []int{1, 2, 3},
		F: decimal.NewFromFloat(3.14),
	}

	err := t.svc.RedisHelper.HSetObject(context.Background(), "test", &x).Err()
	if err != nil {
		fmt.Println(err)
	}

	var y Testx
	err = t.svc.RedisHelper.HGetObject(context.Background(), "test", &y)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(y)

}

type Testx struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Abc      struct {
		Def string `json:"def"`
	} `json:"abc"`
	X []int           `json:"x"`
	F decimal.Decimal `json:"f"`
}
