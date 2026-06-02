package main

import (
	"fmt"

	"github.com/lornshark/shark/sharkapp"
	"github.com/segmentio/kafka-go"
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

type TestComponent struct {
	app *sharkapp.App
}

func (t *TestComponent) Init() {
}

func NewTestComponent(app *sharkapp.App) *TestComponent {
	return &TestComponent{app: app}
}

func main() {
	options := sharkapp.NewOption("kgame", "game-test")
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt(NewTestComponent(app))
}

func (t *TestComponent) Start() {
	topic := "kgame_game_log"
	group := "kgame.game-log"
	go t.app.Kafka.BatchConsumer(topic, group, t.handleMessage)
}

func (t *TestComponent) handleMessage(msg []kafka.Message) bool {
	for _, m := range msg {
		fmt.Printf("message at topic:%v partition:%v offset:%v key:%v value:%v\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
	}
	return true
}
