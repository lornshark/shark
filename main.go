package main

import (
	"github.com/lornshark/shark/sharkapp"
	"github.com/lornshark/shark/sharkboot"
)

type MyApp struct {
	*sharkapp.App
}

func main() {
	boot := sharkboot.New("kgame", "game-test", "1", "dev", "192.168.191.100", "6379", "CEki57pxTJyYaLD")
	boot.WithRedis().WithDb().WithTimer().WithElastic().WithKafka().WithMinio().WithMongodb().WithRabbitmq().WithRisingwave()
	app, err := sharkapp.New(boot.Options())
	if err != nil {
		panic(err)
	}
	app.Hunt()
}
