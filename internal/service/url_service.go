package service

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
)

type URLRepository interface {
	Save(url *model.URL) error
	FindByID(id string) (*model.URL, error)
}

// URLService содержит бизнес-логику работы с URL
type URLService struct {
	repo URLRepository
	rand *rand.Rand
}

func NewURLService(repo URLRepository) *URLService {
	return &URLService{
		repo: repo,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ShortenURL создаёт короткий URL из оригинального
func (s *URLService) ShortenURL(originalURL string) (string, error) {
	const maxAttempts = 5

	for i := 0; i < maxAttempts; i++ {
		shortID := s.generateShortID()

		url := &model.URL{
			ID:          shortID,
			OriginalURL: originalURL,
		}

		err := s.repo.Save(url)
		if err == nil {
			return shortID, nil
		}

		if errors.Is(err, repository.ErrAlreadyExists) {
			continue
		}

		return "", err
	}

	return "", fmt.Errorf("failed to obtain unique id after %d attempts", maxAttempts)
}

// GetOriginalURL возвращает оригинальный URL по короткому идентификатору
func (s *URLService) GetOriginalURL(shortID string) (string, error) {
	url, err := s.repo.FindByID(shortID)
	if err != nil {
		return "", err
	}

	return url.OriginalURL, nil
}

// generateShortID генерирует случайный короткий идентификатор
func (s *URLService) generateShortID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[s.rand.Intn(len(charset))]
	}
	return string(b)
}

