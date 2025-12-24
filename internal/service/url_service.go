package service

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
)

type URLRepository interface {
	Save(url *model.URL) error
	FindByID(id string) (*model.URL, error)
	GetAll() ([]*model.URL, error)
	GetByUserID(userID string) ([]*model.URL, error)
}

// URLService содержит бизнес-логику работы с URL
type URLService struct {
	repo    URLRepository
	rand    *rand.Rand
	counter uint64 // атомарный счётчик для UUID
}

func NewURLService(repo URLRepository) *URLService {
	return &URLService{
		repo:    repo,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
		counter: 0,
	}
}

// ShortenURL создаёт короткий URL из оригинального
func (s *URLService) ShortenURL(originalURL, userID string) (string, error) {
	const maxAttempts = 5

	for i := 0; i < maxAttempts; i++ {
		shortID := s.generateShortID()
		uuid := s.generateUUID()

		url := &model.URL{
			UUID:        uuid,
			ShortURL:    shortID,
			OriginalURL: originalURL,
			UserID:      userID,
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

// GetAllURLs возвращает все сохранённые URL
func (s *URLService) GetAllURLs() ([]*model.URL, error) {
	return s.repo.GetAll()
}

// GetUserURLs возвращает все URL конкретного пользователя
func (s *URLService) GetUserURLs(userID string) ([]*model.URL, error) {
	return s.repo.GetByUserID(userID)
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

// generateUUID генерирует простой UUID на основе счётчика
func (s *URLService) generateUUID() string {
	id := atomic.AddUint64(&s.counter, 1)
	return strconv.FormatUint(id, 10)
}
