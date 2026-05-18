package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/facade"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
)

type benchNoopURLRepo struct{}

func (benchNoopURLRepo) Save(*model.URL) error        { return nil }
func (benchNoopURLRepo) BatchSave([]*model.URL) error { return nil }
func (benchNoopURLRepo) FindByID(string) (*model.URL, error) {
	return &model.URL{OriginalURL: "https://example.com"}, nil
}
func (benchNoopURLRepo) FindByOriginalURL(string) (*model.URL, error) {
	return nil, repository.ErrNotFound
}
func (benchNoopURLRepo) GetAll() ([]*model.URL, error)            { return nil, nil }
func (benchNoopURLRepo) GetByUserID(string) ([]*model.URL, error) { return nil, nil }
func (benchNoopURLRepo) DeleteByIDs([]string, string) error       { return nil }
func (benchNoopURLRepo) Close() error                             { return nil }

// BenchmarkCreateShortURLPlain_chi измеряет цепочку gzip + auth + сокращение (text/plain).
func BenchmarkCreateShortURLPlain_chi(b *testing.B) {
	log := zap.NewNop()
	svc := service.NewURLService(benchNoopURLRepo{})
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "", log)

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.AuthMiddleware("bench-secret-key-32bytes-long!!", log))
	r.Post("/", h.CreateShortURLPlain)

	const payload = "https://example.com/very/long/path/for/benchmark"

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkCreateShortURL_JSON — JSON /api/shorten (без Accept-Encoding: gzip).
func BenchmarkCreateShortURL_JSON(b *testing.B) {
	log := zap.NewNop()
	svc := service.NewURLService(benchNoopURLRepo{})
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "", log)

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.AuthMiddleware("bench-secret-key-32bytes-long!!", log))
	r.Post("/api/shorten", h.CreateShortURL)

	body := `{"url":"https://example.com/long/url/for/bench"}`

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkCreateShortURL_JSON_gzip — тот же путь с сжатием ответа (крупные аллокации gzip).
func BenchmarkCreateShortURL_JSON_gzip(b *testing.B) {
	log := zap.NewNop()
	svc := service.NewURLService(benchNoopURLRepo{})
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "", log)

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.AuthMiddleware("bench-secret-key-32bytes-long!!", log))
	r.Post("/api/shorten", h.CreateShortURL)

	body := `{"url":"https://example.com/long/url/for/bench"}`

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
