package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/audit"
	"github.com/mmeow0/meow-shortener/internal/facade"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
)

func setupHandler(t *testing.T) (*URLHandler, *chi.Mux) {
	tempDir := t.TempDir()
	tempFile := tempDir + "/test_storage.log"

	repo, err := repository.NewFileURLRepository(tempFile)
	if err != nil {
		t.Fatalf("не удалось создать репозиторий: %v", err)
	}

	// Создаём логер для тестов
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("не удалось создать логер: %v", err)
	}

	svc := service.NewURLService(repo)
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "", logger)

	r := chi.NewRouter()
	r.Post("/", h.CreateShortURLPlain)
	r.Post("/api/shorten", h.CreateShortURL) // JSON API
	r.Get("/{id}", h.GetOriginalURL)

	return h, r
}

func TestHandlePost_Success(t *testing.T) {
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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
	_, r := setupHandler(t)

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

func TestHandlePost_API_Shorten_Example(t *testing.T) {
	_, r := setupHandler(t)

	requestBody := `{"url":"https://practicum.yandex.ru"}`
	body := strings.NewReader(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	// Проверяем статус 201
	if res.StatusCode != http.StatusCreated {
		t.Errorf("ожидался статус 201, получен %d", res.StatusCode)
	}

	// Проверяем Content-Type
	contentType := res.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("ожидался Content-Type application/json, получен %s", contentType)
	}

	// Проверяем структуру ответа
	var response model.ShortenResponse
	err := json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		t.Fatalf("ошибка декодирования JSON ответа: %v", err)
	}

	// Проверяем, что результат содержит короткий URL
	if !strings.HasPrefix(response.Result, "http://localhost:8080/") {
		t.Errorf("ожидался короткий URL с префиксом http://localhost:8080/, получен %s", response.Result)
	}

	// Проверяем длину ID (должна быть 8 символов)
	shortID := strings.TrimPrefix(response.Result, "http://localhost:8080/")
	if len(shortID) != 8 {
		t.Errorf("ожидалась длина короткого ID = 8 символов, получен ID: %s", shortID)
	}
}

func waitAuditLine(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil && len(b) > 0 {
			return strings.TrimSpace(string(b))
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("нет записи аудита в %s", path)
	return ""
}

func TestAudit_ShortenAndFollow_FileSink(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "store.json")
	auditPath := filepath.Join(tempDir, "audit.log")

	repo, err := repository.NewFileURLRepository(storagePath)
	if err != nil {
		t.Fatalf("репозиторий: %v", err)
	}
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}

	fileObs := audit.NewFileObserver(auditPath)
	pub := audit.NewPublisher([]audit.Observer{fileObs}, zap.NewNop())
	svc := service.NewURLService(repo)
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", pub), "", logger)

	r := chi.NewRouter()
	r.Post("/api/shorten", h.CreateShortURL)
	r.Get("/{id}", h.GetOriginalURL)

	original := "https://practicum.yandex.ru/audit-test"
	postReq := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"`+original+`"}`))
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)
	if postW.Code != http.StatusCreated {
		t.Fatalf("POST статус %d", postW.Code)
	}

	var shortenResp model.ShortenResponse
	_ = json.NewDecoder(postW.Body).Decode(&shortenResp)
	shortID := strings.TrimPrefix(shortenResp.Result, "http://localhost:8080/")

	line := waitAuditLine(t, auditPath)
	var ev audit.Event
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Action != audit.ActionShorten || ev.URL != original {
		t.Fatalf("событие shorten: %+v", ev)
	}

	_ = os.Remove(auditPath)

	getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusTemporaryRedirect {
		t.Fatalf("GET статус %d", getW.Code)
	}

	line = waitAuditLine(t, auditPath)
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Action != audit.ActionFollow || ev.URL != original {
		t.Fatalf("событие follow: %+v", ev)
	}
}

func TestGetInternalStats_Success(t *testing.T) {
	tempDir := t.TempDir()
	repo, err := repository.NewFileURLRepository(filepath.Join(tempDir, "stats.json"))
	if err != nil {
		t.Fatalf("не удалось создать репозиторий: %v", err)
	}

	svc := service.NewURLService(repo)
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "192.168.1.0/24", zap.NewNop())

	if _, err := svc.ShortenURL("https://example.com/1", "user-1"); err != nil {
		t.Fatalf("не удалось создать первый URL: %v", err)
	}
	if _, err := svc.ShortenURL("https://example.com/2", "user-1"); err != nil {
		t.Fatalf("не удалось создать второй URL: %v", err)
	}
	if _, err := svc.ShortenURL("https://example.com/3", "user-2"); err != nil {
		t.Fatalf("не удалось создать третий URL: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "192.168.1.42")
	w := httptest.NewRecorder()

	h.GetInternalStats(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", res.StatusCode)
	}

	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("ожидался Content-Type application/json, получен %s", got)
	}

	var response model.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("ошибка декодирования JSON ответа: %v", err)
	}

	if response.URLs != 3 {
		t.Fatalf("ожидалось 3 URL, получено %d", response.URLs)
	}
	if response.Users != 2 {
		t.Fatalf("ожидалось 2 пользователя, получено %d", response.Users)
	}
}

func TestGetInternalStats_ForbiddenWithoutTrustedSubnet(t *testing.T) {
	svc := service.NewURLService(repository.NewInMemoryURLRepository())
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "192.168.1.42")
	w := httptest.NewRecorder()

	h.GetInternalStats(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("ожидался статус 403, получен %d", w.Code)
	}
}

func TestGetInternalStats_ForbiddenForUntrustedIP(t *testing.T) {
	svc := service.NewURLService(repository.NewInMemoryURLRepository())
	h := NewURLHandler(facade.NewURLFacade(svc, "http://localhost:8080", nil), "192.168.1.0/24", zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	w := httptest.NewRecorder()

	h.GetInternalStats(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("ожидался статус 403, получен %d", w.Code)
	}
}
