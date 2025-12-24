package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

type URLHandler struct {
	service *service.URLService
	baseURL string
}

func NewURLHandler(service *service.URLService, baseURL string) *URLHandler {
	return &URLHandler{
		service: service,
		baseURL: baseURL,
	}
}

// CreateShortURLPlain обрабатывает POST запрос для создания короткого URL (text/plain формат)
func (h *URLHandler) CreateShortURLPlain(res http.ResponseWriter, req *http.Request) {
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

	// Получаем userID из контекста
	userID := middleware.GetUserID(req.Context())

	shortID, err := h.service.ShortenURL(originalURL, userID)
	if err != nil {
		log.Printf("failed to shorten url %q: %v", originalURL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, shortID)

	if err != nil {
		log.Printf("failed to join url path: %v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusCreated)
	res.Write([]byte(shortURL))
}

// CreateShortURL обрабатывает POST запрос для создания короткого URL (JSON формат)
func (h *URLHandler) CreateShortURL(res http.ResponseWriter, req *http.Request) {
	var request model.ShortenRequest

	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&request); err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Проверяем, что URL не пустой и не состоит только из пробелов
	if strings.TrimSpace(request.URL) == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Получаем userID из контекста
	userID := middleware.GetUserID(req.Context())

	shortID, err := h.service.ShortenURL(request.URL, userID)
	if err != nil {
		log.Printf("failed to shorten url %q: %v", request.URL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, shortID)

	if err != nil {
		log.Printf("failed to join url path: %v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := model.ShortenResponse{
		Result: shortURL,
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(res)
	encoder.Encode(response)
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
		if errors.Is(err, repository.ErrNotFound) {
			res.WriteHeader(http.StatusNotFound)
			return
		}
		log.Printf("failed to get original url for id %q: %v", shortID, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Location", originalURL)
	res.WriteHeader(http.StatusTemporaryRedirect)
}

// GetUserURLs обрабатывает GET запрос для получения всех URL пользователя
func (h *URLHandler) GetUserURLs(res http.ResponseWriter, req *http.Request) {
	// Получаем userID из контекста
	userID := middleware.GetUserID(req.Context())

	urls, err := h.service.GetUserURLs(userID)
	if err != nil {
		log.Printf("failed to get user urls: %v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Если URL нет, возвращаем 204 No Content
	if len(urls) == 0 {
		res.WriteHeader(http.StatusNoContent)
		return
	}

	// Формируем ответ
	response := make([]model.UserURLsResponse, 0, len(urls))
	for _, u := range urls {
		fullShortURL, err := url.JoinPath(h.baseURL, u.ShortURL)
		if err != nil {
			log.Printf("failed to join url path: %v", err)
			continue
		}

		response = append(response, model.UserURLsResponse{
			ShortURL:    fullShortURL,
			OriginalURL: u.OriginalURL,
		})
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(res)
	encoder.Encode(response)
}
