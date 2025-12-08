package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

func setupHandler() (*URLHandler, *chi.Mux) {
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewURLService(repo)
	h := NewURLHandler(svc, "http://localhost:8080")

	r := chi.NewRouter()
	r.Post("/", h.CreateShortURLPlain)
	r.Post("/api/shorten", h.CreateShortURL) // JSON API
	r.Get("/{id}", h.GetOriginalURL)

	return h, r
}

func TestHandlePost_Success(t *testing.T) {
	_, r := setupHandler()

	requestBody := `{"url":"https://practicum.yandex.ru/"}`
	body := strings.NewReader(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Errorf("ожидался статус 201, получен %d", res.StatusCode)
	}

	contentType := res.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("ожидался Content-Type application/json, получен %s", contentType)
	}

	var response model.ShortenResponse
	err := json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		t.Fatalf("ошибка декодирования JSON ответа: %v", err)
	}

	if !strings.HasPrefix(response.Result, "http://localhost:8080/") {
		t.Errorf("ожидался короткий URL с префиксом http://localhost:8080/, получен %s", response.Result)
	}

	shortID := strings.TrimPrefix(response.Result, "http://localhost:8080/")
	if len(shortID) != 8 {
		t.Errorf("ожидалась длина короткого ID = 8 символов, получен ID: %s", shortID)
	}
}

func TestHandlePost_EmptyBody(t *testing.T) {
	_, r := setupHandler()

	body := strings.NewReader("")
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для пустого тела, получен %d", res.StatusCode)
	}
}

func TestHandlePost_WhitespaceBody(t *testing.T) {
	_, r := setupHandler()

	requestBody := `{"url":"   \n\t   "}`
	body := strings.NewReader(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для пустого URL, получен %d", res.StatusCode)
	}
}

func TestHandlePost_InvalidPath(t *testing.T) {
	_, r := setupHandler()

	requestBody := `{"url":"https://example.com"}`
	body := strings.NewReader(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/some/path", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался статус 404 для невалидного пути, получен %d", res.StatusCode)
	}
}

func TestHandleGet_Success(t *testing.T) {
	_, r := setupHandler()

	originalURL := "https://practicum.yandex.ru/"

	requestBody := `{"url":"` + originalURL + `"}`
	postBody := strings.NewReader(requestBody)
	postReq := httptest.NewRequest(http.MethodPost, "/api/shorten", postBody)
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	postRes := postW.Result()
	defer postRes.Body.Close()

	var response model.ShortenResponse
	json.NewDecoder(postRes.Body).Decode(&response)
	shortID := strings.TrimPrefix(response.Result, "http://localhost:8080/")

	getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	getRes := getW.Result()
	defer getRes.Body.Close()

	if getRes.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("ожидался статус 307, получен %d", getRes.StatusCode)
	}

	location := getRes.Header.Get("Location")
	if location != originalURL {
		t.Errorf("ожидался Location: %s, получен %s", originalURL, location)
	}
}

func TestHandleGet_NotFound(t *testing.T) {
	_, r := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался статус 404 для несуществующего ID, получен %d", res.StatusCode)
	}
}

func TestHandleGet_RootPath(t *testing.T) {
	_, r := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("ожидался статус 405 для GET / без ID, получен %d", res.StatusCode)
	}
}

func TestHandleGet_NestedPath(t *testing.T) {
	_, r := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/some/nested/path", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался статус 404 для вложенного пути, получен %d", res.StatusCode)
	}
}

func TestHandleRoot_UnsupportedMethod(t *testing.T) {
	_, r := setupHandler()

	methods := []string{
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/shorten", nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("ожидался статус 405 для метода %s, получен %d", method, res.StatusCode)
			}
		})
	}
}

func TestHandlePost_MultipleURLs(t *testing.T) {
	_, r := setupHandler()

	urls := []string{
		"https://practicum.yandex.ru/",
		"https://golang.org/",
		"https://github.com/",
	}

	shortIDs := make([]string, 0, len(urls))

	for _, url := range urls {
		requestBody := `{"url":"` + url + `"}`
		body := strings.NewReader(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		res := w.Result()
		var response model.ShortenResponse
		json.NewDecoder(res.Body).Decode(&response)
		res.Body.Close()

		shortID := strings.TrimPrefix(response.Result, "http://localhost:8080/")
		shortIDs = append(shortIDs, shortID)
	}

	if len(shortIDs) != len(urls) {
		t.Fatalf("ожидалось создание %d коротких URL, создано %d", len(urls), len(shortIDs))
	}

	for i, shortID := range shortIDs {
		for j, otherID := range shortIDs {
			if i != j && shortID == otherID {
				t.Errorf("обнаружены одинаковые ID: %s (индексы %d и %d)", shortID, i, j)
			}
		}
	}

	for i, shortID := range shortIDs {
		req := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		res := w.Result()
		location := res.Header.Get("Location")
		res.Body.Close()

		if location != urls[i] {
			t.Errorf("для короткого ID %s ожидался URL %s, получен %s", shortID, urls[i], location)
		}
	}
}

func TestHandlePost_PlainText_Success(t *testing.T) {
	_, r := setupHandler()

	body := strings.NewReader("https://practicum.yandex.ru/")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Errorf("ожидался статус 201, получен %d", res.StatusCode)
	}

	contentType := res.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("ожидался Content-Type text/plain, получен %s", contentType)
	}

	responseStr := w.Body.String()

	if !strings.HasPrefix(responseStr, "http://localhost:8080/") {
		t.Errorf("ожидался короткий URL с префиксом http://localhost:8080/, получен %s", responseStr)
	}

	shortID := strings.TrimPrefix(responseStr, "http://localhost:8080/")
	if len(shortID) != 8 {
		t.Errorf("ожидалась длина короткого ID = 8 символов, получен ID: %s", shortID)
	}
}

func TestHandlePost_PlainText_EmptyBody(t *testing.T) {
	_, r := setupHandler()

	body := strings.NewReader("")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для пустого тела, получен %d", res.StatusCode)
	}
}

func TestHandlePost_PlainText_WhitespaceBody(t *testing.T) {
	_, r := setupHandler()

	body := strings.NewReader("   \n\t   ")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для пустого URL, получен %d", res.StatusCode)
	}
}

func TestHandlePost_PlainText_Integration(t *testing.T) {
	_, r := setupHandler()

	// Создаем короткий URL через text/plain API
	originalURL := "https://practicum.yandex.ru/"
	postBody := strings.NewReader(originalURL)
	postReq := httptest.NewRequest(http.MethodPost, "/", postBody)
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	postRes := postW.Result()
	defer postRes.Body.Close()

	responseBody := postW.Body.String()
	shortID := strings.TrimPrefix(responseBody, "http://localhost:8080/")

	// Проверяем, что можем получить оригинальный URL
	getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	getRes := getW.Result()
	defer getRes.Body.Close()

	if getRes.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("ожидался статус 307, получен %d", getRes.StatusCode)
	}

	location := getRes.Header.Get("Location")
	if location != originalURL {
		t.Errorf("ожидался Location: %s, получен %s", originalURL, location)
	}
}
