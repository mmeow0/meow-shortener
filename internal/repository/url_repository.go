package repository

import (
	"errors"
	"sync"

	"github.com/mmeow0/meow-shortener/internal/model"
)

var ErrNotFound = errors.New("url not found")
var ErrAlreadyExists = errors.New("url id already exists")

// InMemoryURLRepository реализация хранилища URL в памяти
type InMemoryURLRepository struct {
	mu   sync.Mutex
	urls map[string]*model.URL
}

func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		urls: make(map[string]*model.URL),
	}
}

// Save сохраняет URL в хранилище
func (r *InMemoryURLRepository) Save(url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.urls[url.ID]; exists {
		return ErrAlreadyExists
	}

	r.urls[url.ID] = url
	return nil
}

// FindByID находит URL по идентификатору
func (r *InMemoryURLRepository) FindByID(id string) (*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	url, exists := r.urls[id]
	if !exists {
		return nil, ErrNotFound
	}

	return url, nil
}
