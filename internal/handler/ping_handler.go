package handler

import (
	"net/http"

	"github.com/mmeow0/meow-shortener/internal/database"
	"go.uber.org/zap"
)

// PingHandler отвечает за проверку доступности PostgreSQL (эндпоинт GET /ping).
type PingHandler struct {
	db     *database.DB
	logger *zap.Logger
}

// NewPingHandler создаёт обработчик ping. Если db == nil, Ping вернёт 500 Internal Server Error.
func NewPingHandler(db *database.DB, logger *zap.Logger) *PingHandler {
	return &PingHandler{
		db:     db,
		logger: logger,
	}
}

// Ping вызывает database.DB.Ping. Успех — 200 OK; ошибка или отсутствие БД — 500.
func (h *PingHandler) Ping(res http.ResponseWriter, req *http.Request) {
	// Если база данных не инициализирована, возвращаем 500
	if h.db == nil {
		h.logger.Error("Database is not initialized")
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Проверяем соединение
	if err := h.db.Ping(); err != nil {
		h.logger.Error("Failed to ping database", zap.Error(err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}
