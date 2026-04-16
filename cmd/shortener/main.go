package main

import (
	"fmt"
	"log"

	"github.com/mmeow0/meow-shortener/internal/app"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func na(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func main() {
	fmt.Printf("Build version: %s\n", na(buildVersion))
	fmt.Printf("Build date: %s\n", na(buildDate))
	fmt.Printf("Build commit: %s\n", na(buildCommit))

	app, err := app.InitializeApp()
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
