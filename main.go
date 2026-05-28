package main

import (
	"shark/sharkapp"
	"shark/sharkdb"
	"shark/sharkelastic"
	"shark/sharkkafka"
	"shark/sharkminio"
	"shark/sharkmongodb"
	"shark/sharkrabbitmq"
	"shark/sharkredis"
	"shark/sharkrisingwave"
)

type MyApp struct {
	*sharkapp.App
}

func main() {
	options := sharkapp.NewOption().
		WithId("1").
		WithEnv("dev").
		WithProject("kgame").
		WithName("game-test").
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
		}).
		WithDb(&sharkdb.Config{
			Host:     "192.168.191.100",
			Port:     4000,
			User:     "root",
			Password: "CEki57pxTJyYaLD",
			Database: "kgame",
		}).
		WithTimer(true).
		WithElastic(&sharkelastic.Config{
			Host:     "http://192.168.191.100:9200",
			User:     "elastic",
			Password: "CEki57pxTJyYaLD",
		}).
		WithRisingwave(&sharkrisingwave.Config{
			Host:     "192.168.191.100",
			Port:     4566,
			User:     "root",
			Password: "CEki57pxTJyYaLD",
			Database: "kgame",
		}).
		WithRabbitmq(&sharkrabbitmq.Config{
			Host:     []string{"192.168.191.100:5672", "192.168.191.100:5673"},
			User:     "root",
			Password: "CEki57pxTJyYaLD",
		}).
		WithPprof(3922).
		WithHealthCheck(8311).
		WithMongodb(&sharkmongodb.Config{
			Host:     "192.168.191.100",
			Port:     27017,
			User:     "root",
			Password: "CEki57pxTJyYaLD",
		}).
		WithMinio(&sharkminio.Config{
			Host:     "192.168.191.100",
			Port:     9000,
			User:     "root",
			Password: "CEki57pxTJyYaLD",
		}).WithGrpc(2212)

	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt()
}
