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
	BatchSave(urls []*model.URL) error
	FindByID(id string) (*model.URL, error)
	FindByOriginalURL(originalURL string) (*model.URL, error)
	GetAll() ([]*model.URL, error)
	GetByUserID(userID string) ([]*model.URL, error)
	Close() error
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

// BatchShortenURL создаёт несколько коротких URL за одну операцию
func (s *URLService) BatchShortenURL(items []struct {
	CorrelationID string
	OriginalURL   string
}, userID string) ([]struct {
	CorrelationID string
	ShortURL      string
}, error) {
	const maxAttempts = 5

	urls := make([]*model.URL, 0, len(items))
	results := make([]struct {
		CorrelationID string
		ShortURL      string
	}, 0, len(items))

	// Генерируем короткие ID для всех URL
	for _, item := range items {
		var shortID string
		var generated bool

		// Пытаемся сгенерировать уникальный ID
		for range maxAttempts {
			shortID = s.generateShortID()
			
			// Проверяем, что ID уникален в текущем батче
			duplicate := false
			for _, u := range urls {
				if u.ShortURL == shortID {
					duplicate = true
					break
				}
			}
			
			if !duplicate {
				generated = true
				break
			}
		}

		if !generated {
			return nil, fmt.Errorf("failed to generate unique id for correlation_id: %s", item.CorrelationID)
		}

		uuid := s.generateUUID()
		url := &model.URL{
			UUID:        uuid,
			ShortURL:    shortID,
			OriginalURL: item.OriginalURL,
			UserID:      userID,
		}

		urls = append(urls, url)
		results = append(results, struct {
			CorrelationID string
			ShortURL      string
		}{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortID,
		})
	}

	// Сохраняем все URL одной операцией
	if err := s.repo.BatchSave(urls); err != nil {
		return nil, fmt.Errorf("failed to save batch: %w", err)
	}

	return results, nil
}

// GetOriginalURL возвращает оригинальный URL по короткому идентификатору
func (s *URLService) GetOriginalURL(shortID string) (string, error) {
	url, err := s.repo.FindByID(shortID)
	if err != nil {
		return "", err
	}

	return url.OriginalURL, nil
}

// FindByOriginalURL находит URL по оригинальному URL
func (s *URLService) FindByOriginalURL(originalURL string) (*model.URL, error) {
	return s.repo.FindByOriginalURL(originalURL)
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
