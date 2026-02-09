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
	"go.uber.org/zap"
)

type URLHandler struct {
	service *service.URLService
	baseURL string
	logger  *zap.Logger
}

func NewURLHandler(service *service.URLService, baseURL string, logger *zap.Logger) *URLHandler {
	return &URLHandler{
		service: service,
		baseURL: baseURL,
		logger:  logger,
	}
}

// shortenURL создаёт короткий URL с обработкой конфликтов
// Возвращает: полный короткий URL, HTTP статус код (201 или 409), ошибку
func (h *URLHandler) shortenURL(originalURL, userID string) (string, int, error) {
	shortID, err := h.service.ShortenURL(originalURL, userID)
	if err != nil {
		// Проверяем, является ли это конфликтом
		if errors.Is(err, repository.ErrConflict) {
			// URL уже существует, находим существующий короткий URL
			existingURL, findErr := h.service.FindByOriginalURL(originalURL)
			if findErr != nil {
				return "", 0, findErr
			}

			shortURL, joinErr := url.JoinPath(h.baseURL, existingURL.ShortURL)
			if joinErr != nil {
				return "", 0, joinErr
			}

			return shortURL, http.StatusConflict, nil
		}
		return "", 0, err
	}

	// Успешно создан новый URL
	shortURL, err := url.JoinPath(h.baseURL, shortID)
	if err != nil {
		return "", 0, err
	}

	return shortURL, http.StatusCreated, nil
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
	userID := middleware.GetUserID(req.Context(), h.logger)

	// Создаём короткий URL с обработкой конфликтов
	shortURL, statusCode, err := h.shortenURL(originalURL, userID)
	if err != nil {
		log.Printf("failed to shorten url %q: %v", originalURL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(statusCode)
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
	userID := middleware.GetUserID(req.Context(), h.logger)

	// Создаём короткий URL с обработкой конфликтов
	shortURL, statusCode, err := h.shortenURL(request.URL, userID)
	if err != nil {
		log.Printf("failed to shorten url %q: %v", request.URL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := model.ShortenResponse{
		Result: shortURL,
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(statusCode)

	encoder := json.NewEncoder(res)
	if err := encoder.Encode(response); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
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
		if errors.Is(err, repository.ErrDeleted) {
			// URL был удалён - возвращаем 410 Gone
			res.WriteHeader(http.StatusGone)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			res.WriteHeader(http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get original url", zap.String("shortID", shortID), zap.Error(err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Location", originalURL)
	res.WriteHeader(http.StatusTemporaryRedirect)
}

// GetUserURLs обрабатывает GET запрос для получения всех URL пользователя
func (h *URLHandler) GetUserURLs(res http.ResponseWriter, req *http.Request) {
	// Проверяем валидность cookie
	if !middleware.IsValidCookie(req.Context()) {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Получаем userID из контекста
	userID := middleware.GetUserID(req.Context(), h.logger)
	if userID == "" {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

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
	if err := encoder.Encode(response); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// DeleteUserURLs обрабатывает DELETE запрос для удаления URL пользователя
func (h *URLHandler) DeleteUserURLs(res http.ResponseWriter, req *http.Request) {
	// Проверяем валидность cookie
	if !middleware.IsValidCookie(req.Context()) {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Получаем userID из контекста
	userID := middleware.GetUserID(req.Context(), h.logger)
	if userID == "" {
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Декодируем запрос
	var shortIDs model.DeleteURLsRequest
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&shortIDs); err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Проверяем, что список не пустой
	if len(shortIDs) == 0 {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Извлекаем короткие ID из URL (если были переданы полные URL)
	cleanedIDs := make([]string, 0, len(shortIDs))
	for _, id := range shortIDs {
		// Если это полный URL, извлекаем только короткий ID
		if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
			parsedURL, err := url.Parse(id)
			if err == nil && parsedURL.Path != "" {
				// Убираем ведущий слэш
				shortID := strings.TrimPrefix(parsedURL.Path, "/")
				if shortID != "" {
					cleanedIDs = append(cleanedIDs, shortID)
				}
			}
		} else {
			// Это уже короткий ID
			cleanedIDs = append(cleanedIDs, id)
		}
	}

	if err := h.service.DeleteUserURLs(cleanedIDs, userID); err != nil {
		h.logger.Error("failed to queue delete task", zap.Error(err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusAccepted)
}

// CreateShortURLBatch обрабатывает POST запрос для пакетного создания коротких URL
func (h *URLHandler) CreateShortURLBatch(res http.ResponseWriter, req *http.Request) {
	var batchRequest []model.BatchShortenRequest

	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&batchRequest); err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Проверяем, что батч не пустой
	if len(batchRequest) == 0 {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	// Проверяем валидность всех URL
	for _, item := range batchRequest {
		if strings.TrimSpace(item.OriginalURL) == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(item.CorrelationID) == "" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// Получаем userID из контекста
	userID := middleware.GetUserID(req.Context(), h.logger)

	// Подготавливаем данные для сервиса
	items := make([]struct {
		CorrelationID string
		OriginalURL   string
	}, len(batchRequest))

	for i, item := range batchRequest {
		items[i].CorrelationID = item.CorrelationID
		items[i].OriginalURL = item.OriginalURL
	}

	// Создаём короткие URL
	results, err := h.service.BatchShortenURL(items, userID)
	if err != nil {
		log.Printf("failed to batch shorten urls: %v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	response := make([]model.BatchShortenResponse, 0, len(results))
	for _, result := range results {
		fullShortURL, err := url.JoinPath(h.baseURL, result.ShortURL)
		if err != nil {
			log.Printf("failed to join url path: %v", err)
			continue
		}

		response = append(response, model.BatchShortenResponse{
			CorrelationID: result.CorrelationID,
			ShortURL:      fullShortURL,
		})
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(res)
	if err := encoder.Encode(response); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
