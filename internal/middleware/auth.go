package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey string

const UserIDKey contextKey = "userID"

// AuthMiddleware проверяет/создаёт cookie с ID пользователя
func AuthMiddleware(next http.Handler) http.Handler {
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
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateUserID генерирует случайный ID пользователя
func generateUserID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetUserID извлекает userID из контекста
func GetUserID(ctx context.Context) string {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}
