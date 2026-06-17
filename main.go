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
	// tb := "x_user"
	// db := t.svc.Db.Table(tb).Select(
	// 	Column(tb, "id"),
	// 	Column(tb, "name"),
	// 	ColumnAs(tb, "age", "user_age"),
	// ).Joins("")
	// stmt := db.Session(&gorm.Session{DryRun: true}).Find(nil).Statement
	// fmt.Println("====:", stmt.SQL.String())

}

func Column(table string, column string) string {
	return fmt.Sprintf("%v.%v", table, column)
}

func ColumnAs(table string, column string, as string) string {
	return fmt.Sprintf("%v.%v as %v", table, column, as)
}

type MyJoin struct {
	table string
}

func LeftJoin(table string) *MyJoin {
	return &MyJoin{table: table}
}

func (j *MyJoin) OnEq(l string, r string) string {
	return fmt.Sprintf("%v = %v", l, r)
}
