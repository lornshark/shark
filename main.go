package main

import (
	"github.com/lornshark/shark/sharkapp"
)

type MyApp struct {
	*sharkapp.App
}

func main() {
	options := sharkapp.NewOptionWithRedis("kgame", "game-test", "1", "dev", "192.168.191.100", "6379", "CEki57pxTJyYaLD")
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt()
}
