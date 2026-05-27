package main

import (
	"context"
	"shark/sharkapp"
	"shark/sharkkafka"
	"shark/sharkredis"
)

func main() {
	options := sharkapp.NewOption().
		WithProject("kgame").
		WithName("game-test").
		WithId("1").
		WithEnv("dev").
		WithKafka(&sharkkafka.Config{
			Host:     "192.168.191.100",
			Port:     9092,
			User:     "",
			Password: "",
		}).
		WithRedis(&sharkredis.Config{
			Host:        "192.168.191.100",
			Port:        6379,
			Password:    "CEki57pxTJyYaLD",
			ClusterHost: "",
		})

	app, err := sharkapp.NewSharkApp(options)
	if err != nil {
		panic(err)
	}
	app.Redis.Set(context.Background(), "test", "v", 0)
	app.Logger.Info("SharkApp started")
}
