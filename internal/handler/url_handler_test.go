package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

func setupHandler() *URLHandler {
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewURLService(repo)
	return NewURLHandler(svc)
}

func TestHandlePost_Success(t *testing.T) {
	handler := setupHandler()

	body := strings.NewReader("https://practicum.yandex.ru/")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Errorf("ожидался статус 201, получен %d", res.StatusCode)
	}

	contentType := res.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("ожидался Content-Type text/plain, получен %s", contentType)
	}

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ошибка чтения ответа: %v", err)
	}

	responseStr := string(responseBody)
	if !strings.HasPrefix(responseStr, "http://localhost:8080/") {
		t.Errorf("ожидался короткий URL с префиксом http://localhost:8080/, получен %s", responseStr)
	}

	if len(responseStr) != len("http://localhost:8080/")+8 {
		t.Errorf("ожидалась длина короткого ID = 8 символов, получен URL: %s", responseStr)
	}
}

func TestHandlePost_EmptyBody(t *testing.T) {
	handler := setupHandler()

	body := strings.NewReader("")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для пустого тела, получен %d", res.StatusCode)
	}
}

func TestHandlePost_WhitespaceBody(t *testing.T) {
	handler := setupHandler()

	body := strings.NewReader("   \n\t   ")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для пустого URL, получен %d", res.StatusCode)
	}
}

func TestHandlePost_InvalidPath(t *testing.T) {
	handler := setupHandler()

	body := strings.NewReader("https://example.com")
	req := httptest.NewRequest(http.MethodPost, "/some/path", body)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для невалидного пути, получен %d", res.StatusCode)
	}
}

func TestHandleGet_Success(t *testing.T) {
	handler := setupHandler()

	originalURL := "https://practicum.yandex.ru/"

	postBody := strings.NewReader(originalURL)
	postReq := httptest.NewRequest(http.MethodPost, "/", postBody)
	postW := httptest.NewRecorder()
	handler.HandleRoot(postW, postReq)

	postRes := postW.Result()
	defer postRes.Body.Close()

	responseBody, _ := io.ReadAll(postRes.Body)
	shortURL := string(responseBody)
	shortID := strings.TrimPrefix(shortURL, "http://localhost:8080/")

	getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	getW := httptest.NewRecorder()
	handler.HandleRoot(getW, getReq)

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
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для несуществующего ID, получен %d", res.StatusCode)
	}
}

func TestHandleGet_RootPath(t *testing.T) {
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для GET / без ID, получен %d", res.StatusCode)
	}
}

func TestHandleGet_NestedPath(t *testing.T) {
	handler := setupHandler()

	req := httptest.NewRequest(http.MethodGet, "/some/nested/path", nil)
	w := httptest.NewRecorder()

	handler.HandleRoot(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался статус 400 для вложенного пути, получен %d", res.StatusCode)
	}
}

func TestHandleRoot_UnsupportedMethod(t *testing.T) {
	handler := setupHandler()

	methods := []string{
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			handler.HandleRoot(w, req)

			res := w.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusBadRequest {
				t.Errorf("ожидался статус 400 для метода %s, получен %d", method, res.StatusCode)
			}
		})
	}
}

func TestHandlePost_MultipleURLs(t *testing.T) {
	handler := setupHandler()

	urls := []string{
		"https://practicum.yandex.ru/",
		"https://golang.org/",
		"https://github.com/",
	}

	shortIDs := make([]string, 0, len(urls))

	for _, url := range urls {
		body := strings.NewReader(url)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		handler.HandleRoot(w, req)

		res := w.Result()
		responseBody, _ := io.ReadAll(res.Body)
		res.Body.Close()

		shortURL := string(responseBody)
		shortID := strings.TrimPrefix(shortURL, "http://localhost:8080/")
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

		handler.HandleRoot(w, req)

		res := w.Result()
		location := res.Header.Get("Location")
		res.Body.Close()

		if location != urls[i] {
			t.Errorf("для короткого ID %s ожидался URL %s, получен %s", shortID, urls[i], location)
		}
	}
}

