package logger

import (
    "net/http"
    "time"

    "go.uber.org/zap"
)

// Log будет доступен всему коду как синглтон.
var Log *zap.Logger = zap.NewNop()

// Initialize инициализирует синглтон логера с необходимым уровнем логирования.
func Initialize(level string) error {
    // преобразуем текстовый уровень логирования в zap.AtomicLevel
    lvl, err := zap.ParseAtomicLevel(level)
    if err != nil {
        return err
    }
    // создаём новую конфигурацию логера
    cfg := zap.NewProductionConfig()
    // устанавливаем уровень
    cfg.Level = lvl
    // создаём логер на основе конфигурации
    zl, err := cfg.Build()
    if err != nil {
        return err
    }
    // устанавливаем синглтон
    Log = zl
    return nil
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

// RequestLogger — middleware-логер для входящих HTTP-запросов.
func RequestLogger(next http.Handler) http.Handler {
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
        Log.Info("HTTP request completed",
            zap.String("uri", r.RequestURI),
            zap.String("method", r.Method),
            zap.Duration("duration", duration),
            zap.Int("status", rw.statusCode),
            zap.Int("size", rw.size),
        )
    })
}
