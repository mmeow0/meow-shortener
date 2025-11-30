package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
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

// CreateShortURL обрабатывает POST запрос для создания короткого URL
func (h *URLHandler) CreateShortURL(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	shortID, err := h.service.ShortenURL(originalURL)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	shortURL := fmt.Sprintf("http://localhost:8080/%s", shortID)

	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusCreated)
	res.Write([]byte(shortURL))
}

// GetOriginalURL обрабатывает GET запрос для получения оригинального URL
func (h *URLHandler) GetOriginalURL(res http.ResponseWriter, req *http.Request) {
	shortID := chi.URLParam(req, "id")

	if shortID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	originalURL, err := h.service.GetOriginalURL(shortID)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if originalURL == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	res.Header().Set("Location", originalURL)
	res.WriteHeader(http.StatusTemporaryRedirect)
}
