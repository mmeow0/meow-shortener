package main

import (
	"net/http"

	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

func main() {
	urlRepo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService)

	mux := http.NewServeMux()
	mux.HandleFunc("/", urlHandler.HandleRoot)

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
