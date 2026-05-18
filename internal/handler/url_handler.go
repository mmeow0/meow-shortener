// Package handler реализует HTTP-обработчики API сокращения URL: plain/JSON создание,
// редирект по короткому id, список и удаление ссылок пользователя, пакетное сокращение.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/facade"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"go.uber.org/zap"
)

// URLHandler обрабатывает HTTP-запросы к сервису сокращения ссылок.
type URLHandler struct {
	facade        *facade.URLFacade
	logger        *zap.Logger
	trustedSubnet *net.IPNet
}

// NewURLHandler создаёт HTTP-обработчик поверх общего фасада приложения.
func NewURLHandler(appFacade *facade.URLFacade, trustedSubnet string, logger *zap.Logger) *URLHandler {
	var subnet *net.IPNet
	if trustedSubnet != "" {
		_, parsedSubnet, err := net.ParseCIDR(trustedSubnet)
		if err == nil {
			subnet = parsedSubnet
		}
	}

	return &URLHandler{
		facade:        appFacade,
		logger:        logger,
		trustedSubnet: subnet,
	}
}

// shortenURL создаёт короткий URL с обработкой конфликтов.
func (h *URLHandler) shortenURL(reqCtx context.Context, originalURL, userID string) (string, error) {
	return h.facade.ShortenURL(reqCtx, originalURL, userID)
}

// CreateShortURLPlain обрабатывает POST «/» с телом text/plain — одна строка с оригинальным URL.
// Успех: 201 Created и тело с полной короткой ссылкой; конфликт дубликата original_url: 409 Conflict.
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
	shortURL, err := h.shortenURL(req.Context(), originalURL, userID)
	if err != nil {
		var conflictErr *facade.ConflictError
		if errors.As(err, &conflictErr) {
			res.Header().Set("Content-Type", "text/plain")
			res.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(res, conflictErr.Result)
			return
		}

		log.Printf("failed to shorten url %q: %v", originalURL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(res, shortURL)

}

// CreateShortURL обрабатывает POST «/api/shorten» с JSON model.ShortenRequest.
// Ответ — model.ShortenResponse; коды 201, 409 или ошибки 400/500.
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
	shortURL, err := h.shortenURL(req.Context(), request.URL, userID)
	if err != nil {
		var conflictErr *facade.ConflictError
		if errors.As(err, &conflictErr) {
			res.Header().Set("Content-Type", "application/json")
			res.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(res).Encode(model.ShortenResponse{Result: conflictErr.Result})
			return
		}

		log.Printf("failed to shorten url %q: %v", request.URL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := model.ShortenResponse{
		Result: shortURL,
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(res)
	if err := encoder.Encode(response); err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}

}

// GetOriginalURL обрабатывает GET «/{id}»: редирект 307 Temporary Redirect с заголовком Location.
// 404 — не найдено; 410 Gone — запись помечена удалённой.
func (h *URLHandler) GetOriginalURL(res http.ResponseWriter, req *http.Request) {
	shortID := chi.URLParam(req, "id")

	if shortID == "" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	userID := middleware.GetUserID(req.Context(), h.logger)
	originalURL, err := h.facade.ExpandURL(req.Context(), shortID, userID)
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

// GetUserURLs обрабатывает GET «/api/user/urls». Требуется валидная подписанная cookie user_id.
// 200 OK и JSON-массив model.UserURLsResponse; 204 No Content если ссылок нет; 401 при невалидной cookie.
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

	urls, err := h.facade.ListUserURLs(req.Context(), userID)
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
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(res)
	if err := encoder.Encode(urls); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// GetInternalStats обрабатывает GET /api/internal/stats и возвращает общую статистику сервиса.
func (h *URLHandler) GetInternalStats(res http.ResponseWriter, req *http.Request) {
	if !h.isTrustedRequest(req) {
		res.WriteHeader(http.StatusForbidden)
		return
	}

	stats, err := h.facade.GetStats(req.Context())
	if err != nil {
		h.logger.Error("failed to get internal stats", zap.Error(err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(res).Encode(stats); err != nil {
		h.logger.Error("failed to encode internal stats response", zap.Error(err))
	}
}

func (h *URLHandler) isTrustedRequest(req *http.Request) bool {
	if h.trustedSubnet == nil {
		return false
	}

	realIP := strings.TrimSpace(req.Header.Get("X-Real-IP"))
	if realIP == "" {
		return false
	}

	clientIP := net.ParseIP(realIP)
	if clientIP == nil {
		return false
	}

	return h.trustedSubnet.Contains(clientIP)
}

// DeleteUserURLs обрабатывает DELETE «/api/user/urls» с телом model.DeleteURLsRequest (JSON-массив строк).
// Элементы могут быть короткими id или полными URL; постановка на удаление асинхронна, ответ 202 Accepted.
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

	if err := h.facade.DeleteUserURLs(req.Context(), cleanedIDs, userID); err != nil {
		h.logger.Error("failed to queue delete task", zap.Error(err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusAccepted)
}

// CreateShortURLBatch обрабатывает POST «/api/shorten/batch» с JSON-массивом model.BatchShortenRequest.
// Ответ 201 Created и массив model.BatchShortenResponse с полными короткими URL.
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

	results, err := h.facade.BatchShortenURL(req.Context(), batchRequest, userID)
	if err != nil {
		log.Printf("failed to batch shorten urls: %v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(res)
	if err := encoder.Encode(results); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
