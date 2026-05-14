package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/mmeow0/meow-shortener/internal/app"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func main() {
	fmt.Printf("Build version: %s\n", valueOrNA(buildVersion))
	fmt.Printf("Build date: %s\n", valueOrNA(buildDate))
	fmt.Printf("Build commit: %s\n", valueOrNA(buildCommit))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	application, err := app.InitializeApp()
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
