package main

import (
	"github.com/lornshark/shark/sharkapp"
)

type MyApp struct {
	*sharkapp.App
}

func main() {
	project := "kgame"
	name := "game-test"
	id := "1"
	options := sharkapp.NewOptionWithRedis("dev", project, name, id, "192.168.191.100", "6379", "CEki57pxTJyYaLD")
	options.WithDb(nil).WithRedis(nil).WithKafka(nil).WithElastic(nil).
		WithMongodb(nil).WithRabbitmq(nil).WithRisingwave(nil)
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt()
}
