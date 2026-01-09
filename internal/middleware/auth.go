package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type contextKey string

const userIDKey contextKey = "userID"

// AuthMiddleware проверяет/создаёт cookie с ID пользователя
func AuthMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пытаемся получить cookie
			cookie, err := r.Cookie("user_id")

			var userID string
			if err != nil || cookie.Value == "" {
				// Cookie нет, создаём новый ID пользователя
				userID = generateUserID()

				// Устанавливаем cookie
				http.SetCookie(w, &http.Cookie{
					Name:  "user_id",
					Value: userID,
					Path:  "/",
				})
			} else {
				userID = cookie.Value
			}

			// Добавляем userID в контекст
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateUserID генерирует случайный ID пользователя
func generateUserID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetUserID извлекает userID из контекста
func GetUserID(ctx context.Context, logger *zap.Logger) string {
	value := ctx.Value(userIDKey)
	if value == nil {
		// Это нормальная ситуация, когда middleware не был вызван
		return ""
	}

	userID, ok := value.(string)
	if !ok {
		// Это ненормальная ситуация - в контексте что-то неожиданное
		logger.Warn("unexpected value type in context for userID",
			zap.String("expected", "string"),
			zap.String("actual", fmt.Sprintf("%T", value)),
		)
		return ""
	}
	return userID
}
