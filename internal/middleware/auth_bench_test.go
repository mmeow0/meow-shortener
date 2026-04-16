package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// BenchmarkAuthMiddleware — стоимость проверки/выдачи cookie на каждый запрос.
func BenchmarkAuthMiddleware(b *testing.B) {
	log := zap.NewNop()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := AuthMiddleware("test1-secret-key-2kgfes9-9dsvkaa", log)(next)

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}
