package main

import (
	"fmt"

	"github.com/lornshark/shark/sharkapp"
	"github.com/lornshark/shark/sharkcsv"
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

type TestUser struct {
	A string `json:"a"`
	B int    `json:"b"`
	C string `json:"c"`
}

func (t *Test) Start() {
	csv := sharkcsv.OpenFile("x.csv", nil)
	x := []TestUser{}
	csv.Scan(&x)
	fmt.Println(x)
}
