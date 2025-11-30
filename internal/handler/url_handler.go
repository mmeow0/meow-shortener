package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mmeow0/meow-shortener/internal/service"
)

type URLHandler struct {
	service *service.URLService
}

func NewURLHandler(service *service.URLService) *URLHandler {
	return &URLHandler{
		service: service,
	}
}

// HandleRoot обрабатывает запросы к корневому эндпоинту
func (h *URLHandler) HandleRoot(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		h.handleGet(res, req)
	case http.MethodPost:
		h.handlePost(res, req)
	default:
		res.WriteHeader(http.StatusBadRequest)
	}
}

// handlePost обрабатывает POST запрос для создания короткого URL
func (h *URLHandler) handlePost(res http.ResponseWriter, req *http.Request) {
	// Проверяем, что путь корректный (только "/")
	if req.URL.Path != "/" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Получаем URL из тела запроса
	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Создаём короткий URL
	shortID, err := h.service.ShortenURL(originalURL)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Формируем короткий URL
	shortURL := fmt.Sprintf("http://localhost:8080/%s", shortID)

	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusCreated)
	res.Write([]byte(shortURL))
}

// handleGet обрабатывает GET запрос для получения оригинального URL
func (h *URLHandler) handleGet(res http.ResponseWriter, req *http.Request) {
	// Извлекаем ID из пути
	path := req.URL.Path
	if path == "/" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Убираем начальный слеш
	shortID := strings.TrimPrefix(path, "/")

	// Проверяем, что нет вложенных путей
	if strings.Contains(shortID, "/") {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Получаем оригинальный URL через сервис
	originalURL, err := h.service.GetOriginalURL(shortID)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Если URL не найден
	if originalURL == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Возвращаем редирект
	res.Header().Set("Location", originalURL)
	res.WriteHeader(http.StatusTemporaryRedirect)
}

