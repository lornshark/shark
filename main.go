package main

import (
	"github.com/lornshark/shark/sharkapp"
)

func main() {
	options := sharkapp.NewOption("kgame", "game-test")
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt()
}
