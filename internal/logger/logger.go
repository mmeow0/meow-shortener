// Package logger настраивает zap-логер и middleware логирования HTTP-запросов.
package logger

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// NewLogger создаёт новый логер с указанным уровнем логирования.
func NewLogger(level string) (*zap.Logger, error) {
	// преобразуем текстовый уровень логирования в zap.AtomicLevel
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return nil, fmt.Errorf("failed to set log level: %w", err)
	}
	// создаём новую конфигурацию логера
	cfg := zap.NewProductionConfig()
	// устанавливаем уровень
	cfg.Level = lvl
	// создаём логер на основе конфигурации
	zl, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return zl, nil
}

// responseWriter — обертка над http.ResponseWriter для перехвата кода статуса и размера ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader перехватывает код статуса
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write перехватывает размер записанных данных
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// RequestLogger возвращает middleware-логер для входящих HTTP-запросов.
func RequestLogger(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Создаем обертку для ResponseWriter
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // по умолчанию 200
				size:           0,
			}

			// Вызываем следующий обработчик
			next.ServeHTTP(rw, r)

			// Замеряем время выполнения
			duration := time.Since(start)

			// Логируем информацию о запросе и ответе на уровне Info
			logger.Info("HTTP request completed",
				zap.String("uri", r.RequestURI),
				zap.String("method", r.Method),
				zap.Duration("duration", duration),
				zap.Int("status", rw.statusCode),
				zap.Int("size", rw.size),
			)
		})
	}
}
