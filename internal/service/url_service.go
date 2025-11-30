package service

import (
	"math/rand"
	"time"

	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
)

// URLService содержит бизнес-логику работы с URL
type URLService struct {
	repo repository.URLRepository
	rand *rand.Rand
}

func NewURLService(repo repository.URLRepository) *URLService {
	return &URLService{
		repo: repo,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ShortenURL создаёт короткий URL из оригинального
func (s *URLService) ShortenURL(originalURL string) (string, error) {
	shortID := s.generateShortID()

	url := &model.URL{
		ID:          shortID,
		OriginalURL: originalURL,
	}

	err := s.repo.Save(url)
	if err != nil {
		return "", err
	}

	return shortID, nil
}

// GetOriginalURL возвращает оригинальный URL по короткому идентификатору
func (s *URLService) GetOriginalURL(shortID string) (string, error) {
	url, err := s.repo.FindByID(shortID)
	if err != nil {
		return "", err
	}

	if url == nil {
		return "", nil
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

