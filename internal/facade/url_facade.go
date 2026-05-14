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

func (f *URLFacade) ShortenURL(_ context.Context, originalURL, userID string) (string, int, error) {
	shortID, err := f.service.ShortenURL(originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			existingURL, findErr := f.service.FindByOriginalURL(originalURL)
			if findErr != nil {
				return "", 0, findErr
			}

			shortURL, joinErr := url.JoinPath(f.baseURL, existingURL.ShortURL)
			if joinErr != nil {
				return "", 0, joinErr
			}

			return shortURL, 409, nil
		}
		return "", 0, err
	}

	shortURL, err := url.JoinPath(f.baseURL, shortID)
	if err != nil {
		return "", 0, err
	}

	f.publishAudit(audit.ActionShorten, originalURL, userID)
	return shortURL, 201, nil
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
		fullShortURL, joinErr := url.JoinPath(f.baseURL, item.ShortURL)
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
