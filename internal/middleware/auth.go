package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	validCookie contextKey = "validCookie"
)

// AuthMiddleware проверяет/создаёт cookie с ID пользователя
func AuthMiddleware(secretKey string, logger *zap.Logger) func(next http.Handler) http.Handler {
	key := []byte(secretKey)
	macPool := &sync.Pool{
		New: func() any {
			return hmac.New(sha256.New, key)
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пытаемся получить cookie
			cookie, err := r.Cookie("user_id")

			var userID string
			var isValidCookie bool

			if err != nil || cookie.Value == "" {
				// Cookie нет, создаём новый ID пользователя
				userID = generateUserID()
				signedValue := signUserID(userID, macPool)

				// Устанавливаем cookie
				http.SetCookie(w, &http.Cookie{
					Name:  "user_id",
					Value: signedValue,
					Path:  "/",
				})
				isValidCookie = true
			} else {
				// Проверяем подпись cookie
				userID, isValidCookie = verifySignedUserID(cookie.Value, macPool)

				if !isValidCookie {
					// Cookie невалидна, создаём новую
					logger.Warn("invalid cookie signature, creating new user ID")
					userID = generateUserID()
					signedValue := signUserID(userID, macPool)

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
	var b [16]byte
	_, _ = rand.Read(b[:])
	var dst [32]byte
	hex.Encode(dst[:], b[:])
	return string(dst[:])
}

// signUserID подписывает userID с помощью HMAC-SHA256
func signUserID(userID string, macPool *sync.Pool) string {
	h := macPool.Get().(hash.Hash)
	h.Reset()
	_, _ = h.Write([]byte(userID))
	var sumBuf [sha256.Size]byte
	sum := h.Sum(sumBuf[:0])
	var hexBuf [sha256.Size * 2]byte
	hex.Encode(hexBuf[:], sum)
	macPool.Put(h)
	return userID + "." + string(hexBuf[:])
}

// verifySignedUserID проверяет подпись и возвращает userID и флаг валидности
func verifySignedUserID(signedValue string, macPool *sync.Pool) (string, bool) {
	dot := strings.IndexByte(signedValue, '.')
	if dot <= 0 || dot == len(signedValue)-1 {
		return "", false
	}

	userID := signedValue[:dot]
	receivedSignature := signedValue[dot+1:]

	h := macPool.Get().(hash.Hash)
	h.Reset()
	_, _ = h.Write([]byte(userID))
	var sumBuf [sha256.Size]byte
	sum := h.Sum(sumBuf[:0])
	var hexBuf [sha256.Size * 2]byte
	hex.Encode(hexBuf[:], sum)
	macPool.Put(h)

	if len(receivedSignature) != len(hexBuf) || !hmac.Equal([]byte(receivedSignature), hexBuf[:]) {
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
