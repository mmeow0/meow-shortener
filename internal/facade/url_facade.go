package facade

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/mmeow0/meow-shortener/internal/audit"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/service"
)

// ConflictError сообщает, что сущность уже существует, и возвращает канонический результат.
type ConflictError struct {
	Result string
}

func (e *ConflictError) Error() string {
	return "resource already exists"
}

// URLFacade инкапсулирует общую бизнес-логику, переиспользуемую HTTP и gRPC transport-слоями.
type URLFacade struct {
	service *service.URLService
	baseURL string
	audit   *audit.Publisher
}

func NewURLFacade(service *service.URLService, baseURL string, auditPub *audit.Publisher) *URLFacade {
	return &URLFacade{
		service: service,
		baseURL: baseURL,
		audit:   auditPub,
	}
}

func (f *URLFacade) ShortenURL(_ context.Context, originalURL, userID string) (string, error) {
	shortID, err := f.service.ShortenURL(originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			existingURL, findErr := f.service.FindByOriginalURL(originalURL)
			if findErr != nil {
				return "", findErr
			}

			shortURL, joinErr := f.buildShortURL(existingURL.ShortURL)
			if joinErr != nil {
				return "", joinErr
			}

			return "", &ConflictError{Result: shortURL}
		}
		return "", err
	}

	shortURL, err := f.buildShortURL(shortID)
	if err != nil {
		return "", err
	}

	f.publishAudit(audit.ActionShorten, originalURL, userID)
	return shortURL, nil
}

func (f *URLFacade) ExpandURL(_ context.Context, shortID, userID string) (string, error) {
	originalURL, err := f.service.GetOriginalURL(shortID)
	if err != nil {
		return "", err
	}

	f.publishAudit(audit.ActionFollow, originalURL, userID)
	return originalURL, nil
}

func (f *URLFacade) ListUserURLs(_ context.Context, userID string) ([]model.UserURLsResponse, error) {
	urls, err := f.service.GetUserURLs(userID)
	if err != nil {
		return nil, err
	}

	response := make([]model.UserURLsResponse, 0, len(urls))
	for _, item := range urls {
		fullShortURL, joinErr := f.buildShortURL(item.ShortURL)
		if joinErr != nil {
			continue
		}

		response = append(response, model.UserURLsResponse{
			ShortURL:    fullShortURL,
			OriginalURL: item.OriginalURL,
		})
	}

	return response, nil
}

func (f *URLFacade) GetStats(_ context.Context) (model.StatsResponse, error) {
	return f.service.GetStats()
}

func (f *URLFacade) DeleteUserURLs(_ context.Context, shortIDs []string, userID string) error {
	return f.service.DeleteUserURLs(shortIDs, userID)
}

func (f *URLFacade) BatchShortenURL(_ context.Context, items []model.BatchShortenRequest, userID string) ([]model.BatchShortenResponse, error) {
	serviceItems := make([]struct {
		CorrelationID string
		OriginalURL   string
	}, len(items))

	for i, item := range items {
		serviceItems[i].CorrelationID = item.CorrelationID
		serviceItems[i].OriginalURL = item.OriginalURL
	}

	results, err := f.service.BatchShortenURL(serviceItems, userID)
	if err != nil {
		return nil, err
	}

	response := make([]model.BatchShortenResponse, 0, len(results))
	for _, result := range results {
		fullShortURL, joinErr := f.buildShortURL(result.ShortURL)
		if joinErr != nil {
			continue
		}

		response = append(response, model.BatchShortenResponse{
			CorrelationID: result.CorrelationID,
			ShortURL:      fullShortURL,
		})
	}

	return response, nil
}

func (f *URLFacade) buildShortURL(shortID string) (string, error) {
	return url.JoinPath(f.baseURL, shortID)
}

func (f *URLFacade) publishAudit(action, originalURL, userID string) {
	if f.audit == nil {
		return
	}

	f.audit.Publish(audit.Event{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    originalURL,
	})
}
