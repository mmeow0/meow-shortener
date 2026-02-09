package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/logger"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"go.uber.org/zap"
)

func NewRouter(urlHandler *handler.URLHandler, pingHandler *handler.PingHandler, secretKey string, log *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Подключаем middleware для gzip сжатия
	r.Use(middleware.GzipMiddleware)
	// Подключаем middleware для логирования
	r.Use(logger.RequestLogger(log))
	// Подключаем middleware для аутентификации
	r.Use(middleware.AuthMiddleware(secretKey, log))

	r.Post("/", urlHandler.CreateShortURLPlain)
	r.Post("/api/shorten", urlHandler.CreateShortURL)
	r.Post("/api/shorten/batch", urlHandler.CreateShortURLBatch)
	r.Get("/api/user/urls", urlHandler.GetUserURLs)
	r.Delete("/api/user/urls", urlHandler.DeleteUserURLs)
	r.Get("/ping", pingHandler.Ping)
	r.Get("/{id}", urlHandler.GetOriginalURL)

	return r
}
