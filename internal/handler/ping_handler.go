package handler

import (
	"net/http"

	"github.com/mmeow0/meow-shortener/internal/database"
	"go.uber.org/zap"
)

type PingHandler struct {
	db     *database.DB
	logger *zap.Logger
}

func NewPingHandler(db *database.DB, logger *zap.Logger) *PingHandler {
	return &PingHandler{
		db:     db,
		logger: logger,
	}
}

// Ping проверяет соединение с базой данных
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

