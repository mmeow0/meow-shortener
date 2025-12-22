package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/logger"
)

func NewRouter(urlHandler *handler.URLHandler) *chi.Mux {
	r := chi.NewRouter()

	// Подключаем middleware для логирования
	r.Use(logger.RequestLogger)

	r.Post("/", urlHandler.CreateShortURLPlain)
	r.Post("/api/shorten", urlHandler.CreateShortURL)
	r.Get("/{id}", urlHandler.GetOriginalURL)

	return r
}
