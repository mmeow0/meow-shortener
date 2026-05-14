// Package router собирает HTTP-маршруты сервиса сокращения ссылок (практический трек)
// и подключает gzip, логирование и аутентификацию по подписанной cookie user_id.
//
// Зарегистрированные эндпоинты:
//   - POST / — тело text/plain, оригинальный URL; ответ text/plain с короткой ссылкой (201 или 409).
//   - POST /api/shorten — JSON {"url": "..."}; ответ {"result": "..."} (201 или 409).
//   - POST /api/shorten/batch — пакетное сокращение JSON-массивом.
//   - GET /api/user/urls — список ссылок пользователя (требуется валидная cookie; 204 если пусто).
//   - DELETE /api/user/urls — мягкое удаление по JSON-массиву коротких id или полных URL (202).
//   - GET /{id} — редирект 307 на оригинальный URL (404, 410 если удалено).
//   - GET /api/internal/stats — внутренняя статистика (200 только для X-Real-IP из trusted_subnet, иначе 403).
//   - GET /ping — проверка БД (200 при успешном Ping, 500 при ошибке или отсутствии БД).
package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/logger"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"go.uber.org/zap"
)

// NewRouter возвращает chi.Mux с маршрутами хендлеров и общим middleware (gzip, лог, AuthMiddleware).
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
	r.Get("/api/internal/stats", urlHandler.GetInternalStats)
	r.Get("/ping", pingHandler.Ping)
	r.Get("/{id}", urlHandler.GetOriginalURL)

	return r
}
