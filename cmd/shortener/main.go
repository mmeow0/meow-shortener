package main

import (
	"log"

	"github.com/mmeow0/meow-shortener/internal/app"
)

func main() {
	app, err := app.InitializeApp()
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
