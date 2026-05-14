// Package handler реализует HTTP-обработчики API сокращения URL: plain/JSON создание,
// редирект по короткому id, список и удаление ссылок пользователя, пакетное сокращение.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mmeow0/meow-shortener/internal/audit"
	"github.com/mmeow0/meow-shortener/internal/middleware"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
)

// URLHandler обрабатывает HTTP-запросы к сервису сокращения ссылок.
type URLHandler struct {
	service       *service.URLService
	baseURL       string
	logger        *zap.Logger
	audit         *audit.Publisher
	trustedSubnet *net.IPNet
}

// NewURLHandler создаёт обработчик. baseURL — префикс публичных коротких ссылок (без завершающего «/»).
// auditPub может быть nil, тогда события аудита не публикуются.
func NewURLHandler(service *service.URLService, baseURL string, trustedSubnet string, logger *zap.Logger, auditPub *audit.Publisher) *URLHandler {
	var subnet *net.IPNet
	if trustedSubnet != "" {
		_, parsedSubnet, err := net.ParseCIDR(trustedSubnet)
		if err == nil {
			subnet = parsedSubnet
		}
	}

	return &URLHandler{
		service:       service,
		baseURL:       baseURL,
		logger:        logger,
		audit:         auditPub,
		trustedSubnet: subnet,
	}
}

func (h *URLHandler) publishAudit(action, originalURL, userID string) {
	if h.audit == nil {
		return
	}
	h.audit.Publish(audit.Event{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    originalURL,
	})
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
	shortURL, statusCode, err := h.shortenURL(originalURL, userID)
	if err != nil {
		log.Printf("failed to shorten url %q: %v", originalURL, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(statusCode)
	_, _ = io.WriteString(res, shortURL)

	if statusCode == http.StatusCreated {
		h.publishAudit(audit.ActionShorten, originalURL, userID)
	}
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
		return
	}

	if statusCode == http.StatusCreated {
		h.publishAudit(audit.ActionShorten, request.URL, userID)
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

	userID := middleware.GetUserID(req.Context(), h.logger)
	h.publishAudit(audit.ActionFollow, originalURL, userID)
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

// GetInternalStats обрабатывает GET /api/internal/stats и возвращает общую статистику сервиса.
func (h *URLHandler) GetInternalStats(res http.ResponseWriter, req *http.Request) {
	if !h.isTrustedRequest(req) {
		res.WriteHeader(http.StatusForbidden)
		return
	}

	stats, err := h.service.GetStats()
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

	if err := h.service.DeleteUserURLs(cleanedIDs, userID); err != nil {
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
