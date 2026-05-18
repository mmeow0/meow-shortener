// Package middleware предоставляет HTTP middleware: подписанная cookie пользователя и gzip для запроса/ответа.
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

// Authenticator проверяет и выпускает пользовательские токены,
// используемые в cookie HTTP и metadata gRPC.
type Authenticator struct {
	logger  *zap.Logger
	macPool *sync.Pool
}

// NewAuthenticator создаёт Authenticator с HMAC-подписью на основе secretKey.
func NewAuthenticator(secretKey string, logger *zap.Logger) *Authenticator {
	key := []byte(secretKey)
	return &Authenticator{
		logger: logger,
		macPool: &sync.Pool{
			New: func() any {
				return hmac.New(sha256.New, key)
			},
		},
	}
}

// AuthMiddleware проверяет/создаёт cookie с ID пользователя
func AuthMiddleware(secretKey string, logger *zap.Logger) func(next http.Handler) http.Handler {
	authenticator := NewAuthenticator(secretKey, logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ""
			cookie, err := r.Cookie("user_id")
			if err == nil {
				token = cookie.Value
			}

			userID, signedValue, needsRefresh := authenticator.ResolveUser(token)
			if needsRefresh {
				http.SetCookie(w, &http.Cookie{
					Name:  "user_id",
					Value: signedValue,
					Path:  "/",
				})
			}

			ctx := WithUserAuth(r.Context(), userID, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ResolveUser нормализует токен, проверяет подпись и при необходимости выпускает новый.
func (a *Authenticator) ResolveUser(token string) (userID string, signedToken string, needsRefresh bool) {
	normalized := normalizeAuthorizationToken(token)
	if normalized == "" {
		return a.newSignedUser()
	}

	userID, valid := verifySignedUserID(normalized, a.macPool)
	if valid {
		return userID, normalized, false
	}

	if a.logger != nil {
		a.logger.Warn("invalid auth token signature, creating new user ID")
	}

	return a.newSignedUser()
}

// WithUserAuth сохраняет данные пользователя в контексте.
func WithUserAuth(ctx context.Context, userID string, isValid bool) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, validCookie, isValid)
}

func (a *Authenticator) newSignedUser() (userID string, signedToken string, needsRefresh bool) {
	userID = generateUserID()
	return userID, signUserID(userID, a.macPool), true
}

func normalizeAuthorizationToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) >= len("Bearer ") && strings.EqualFold(token[:len("Bearer ")], "Bearer ") {
		token = strings.TrimSpace(token[len("Bearer "):])
	}
	return token
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

// GetUserID возвращает идентификатор пользователя, установленный AuthMiddleware в контексте запроса.
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

// IsValidCookie сообщает, была ли у запроса валидная подпись cookie user_id (см. AuthMiddleware).
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
