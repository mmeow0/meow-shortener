package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type contextKey string

const (
	userIDKey  contextKey = "userID"
	validCookie contextKey = "validCookie"
)

// AuthMiddleware проверяет/создаёт cookie с ID пользователя
func AuthMiddleware(secretKey string, logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пытаемся получить cookie
			cookie, err := r.Cookie("user_id")

			var userID string
			var isValidCookie bool

			if err != nil || cookie.Value == "" {
				// Cookie нет, создаём новый ID пользователя
				userID = generateUserID()
				signedValue := signUserID(userID, secretKey)

				// Устанавливаем cookie
				http.SetCookie(w, &http.Cookie{
					Name:  "user_id",
					Value: signedValue,
					Path:  "/",
				})
				isValidCookie = true
			} else {
				// Проверяем подпись cookie
				userID, isValidCookie = verifySignedUserID(cookie.Value, secretKey)
				
				if !isValidCookie {
					// Cookie невалидна, создаём новую
					logger.Warn("invalid cookie signature, creating new user ID")
					userID = generateUserID()
					signedValue := signUserID(userID, secretKey)

					http.SetCookie(w, &http.Cookie{
						Name:  "user_id",
						Value: signedValue,
						Path:  "/",
					})
					isValidCookie = true
				}
			}

			// Добавляем userID и флаг валидности в контекст
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, validCookie, isValidCookie)
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

// signUserID подписывает userID с помощью HMAC-SHA256
func signUserID(userID, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(userID))
	signature := hex.EncodeToString(h.Sum(nil))
	return userID + "." + signature
}

// verifySignedUserID проверяет подпись и возвращает userID и флаг валидности
func verifySignedUserID(signedValue, secretKey string) (string, bool) {
	parts := strings.Split(signedValue, ".")
	if len(parts) != 2 {
		return "", false
	}

	userID := parts[0]
	receivedSignature := parts[1]

	// Вычисляем ожидаемую подпись
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(userID))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// Сравниваем подписи
	if !hmac.Equal([]byte(receivedSignature), []byte(expectedSignature)) {
		return "", false
	}

	return userID, true
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

// IsValidCookie проверяет, валидна ли cookie в контексте
func IsValidCookie(ctx context.Context) bool {
	value := ctx.Value(validCookie)
	if value == nil {
		return false
	}

	isValid, ok := value.(bool)
	if !ok {
		return false
	}
	return isValid
}
