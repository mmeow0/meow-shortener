package repository

import (
	"sync"

	"github.com/mmeow0/meow-shortener/internal/model"
)

type URLRepository interface {
	Save(url *model.URL) error
	FindByID(id string) (*model.URL, error)
}

// InMemoryURLRepository реализация хранилища URL в памяти
type InMemoryURLRepository struct {
	mu   sync.RWMutex
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

	r.urls[url.ID] = url
	return nil
}

// FindByID находит URL по идентификатору
func (r *InMemoryURLRepository) FindByID(id string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	url, exists := r.urls[id]
	if !exists {
		return nil, nil
	}

	return url, nil
}

