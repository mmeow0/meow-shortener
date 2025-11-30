package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/config"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

func main() {
	cfg := config.NewConfig()

	urlRepo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService, cfg.BaseURL)

	r := chi.NewRouter()

	r.Post("/", urlHandler.CreateShortURLPlain)
	r.Post("/api/shorten", urlHandler.CreateShortURL)
	r.Get("/{id}", urlHandler.GetOriginalURL)

	// Используем адрес из конфигурации
	err := http.ListenAndServe(cfg.ServerAddress, r)
	if err != nil {
		panic(err)
	}
}
