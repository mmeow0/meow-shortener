package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

func main() {
	urlRepo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService)

	r := chi.NewRouter()
	r.Post("/", urlHandler.CreateShortURL)
	r.Get("/{id}", urlHandler.GetOriginalURL)

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		panic(err)
	}
}
