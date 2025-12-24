package middleware_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

func setupTestHandler() http.Handler {
	repo, _ := repository.NewFileURLRepository("../../logs.log")
	svc := service.NewURLService(repo)
	h := handler.NewURLHandler(svc, "http://localhost:8080")

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware)
	r.Post("/api/shorten", h.CreateShortURL)
	r.Post("/", h.CreateShortURLPlain)

	return r
}

func TestGzipCompression(t *testing.T) {
	handler := setupTestHandler()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	requestBody := `{
		"url": "https://practicum.yandex.ru/"
	}`

	t.Run("sends_gzip", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		zb := gzip.NewWriter(buf)
		_, err := zb.Write([]byte(requestBody))
		if err != nil {
			t.Fatalf("failed to write gzip data: %v", err)
		}
		err = zb.Close()
		if err != nil {
			t.Fatalf("failed to close gzip writer: %v", err)
		}

		r := httptest.NewRequest("POST", srv.URL+"/api/shorten", buf)
		r.RequestURI = ""
		r.Header.Set("Content-Encoding", "gzip")
		r.Header.Set("Accept-Encoding", "")

		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("failed to send request: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		var response model.ShortenResponse
		err = json.Unmarshal(b, &response)
		if err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Проверяем что ответ содержит короткий URL
		if !strings.HasPrefix(response.Result, "http://localhost:8080/") {
			t.Errorf("expected result to start with http://localhost:8080/, got %s", response.Result)
		}
	})

	t.Run("accepts_gzip", func(t *testing.T) {
		buf := bytes.NewBufferString(requestBody)
		r := httptest.NewRequest("POST", srv.URL+"/api/shorten", buf)
		r.RequestURI = ""
		r.Header.Set("Accept-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("failed to send request: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		defer resp.Body.Close()

		// Проверяем что ответ сжат
		if resp.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("expected Content-Encoding gzip, got %s", resp.Header.Get("Content-Encoding"))
		}

		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}

		b, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("failed to read gzip data: %v", err)
		}

		var response model.ShortenResponse
		err = json.Unmarshal(b, &response)
		if err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Проверяем что ответ содержит короткий URL
		if !strings.HasPrefix(response.Result, "http://localhost:8080/") {
			t.Errorf("expected result to start with http://localhost:8080/, got %s", response.Result)
		}
	})

	t.Run("no_compression_for_plain_text", func(t *testing.T) {
		buf := bytes.NewBufferString("https://example.com")
		r := httptest.NewRequest("POST", srv.URL+"/", buf)
		r.RequestURI = ""
		r.Header.Set("Accept-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("failed to send request: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}

		defer resp.Body.Close()

		// Проверяем что ответ НЕ сжат для text/plain
		if resp.Header.Get("Content-Encoding") == "gzip" {
			t.Errorf("expected no Content-Encoding for text/plain, but got gzip")
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		responseStr := string(b)

		// Проверяем что ответ содержит короткий URL и не сжат
		if !strings.HasPrefix(responseStr, "http://localhost:8080/") {
			t.Errorf("expected result to start with http://localhost:8080/, got %s", responseStr)
		}
	})
}
