package service

import (
	"testing"

	"github.com/mmeow0/meow-shortener/internal/model"
)

type mockURLRepo struct{}

func (m *mockURLRepo) Save(url *model.URL) error {
	return nil
}

func (m *mockURLRepo) BatchSave(urls []*model.URL) error {
	return nil
}

func (m *mockURLRepo) FindByID(id string) (*model.URL, error) {
	return &model.URL{OriginalURL: "https://example.com"}, nil
}

func (m *mockURLRepo) FindByOriginalURL(originalURL string) (*model.URL, error) {
	return nil, nil
}

func (m *mockURLRepo) GetAll() ([]*model.URL, error) {
	return nil, nil
}

func (m *mockURLRepo) GetByUserID(userID string) ([]*model.URL, error) {
	return nil, nil
}

func (m *mockURLRepo) DeleteByIDs(shortIDs []string, userID string) error {
	return nil
}

func (m *mockURLRepo) Close() error {
	return nil
}

// --- Бэнчмарки ---

func BenchmarkShortenURL(b *testing.B) {
	repo := &mockURLRepo{}
	service := NewURLService(repo)

	for b.Loop() {
		_, _ = service.ShortenURL("https://example.com", "user1")
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	repo := &mockURLRepo{}
	service := NewURLService(repo)

	for b.Loop() {
		_, _ = service.GetOriginalURL("abc123")
	}
}

func BenchmarkBatchShortenURL(b *testing.B) {
	repo := &mockURLRepo{}
	service := NewURLService(repo)

	items := make([]struct {
		CorrelationID string
		OriginalURL   string
	}, 10)

	for i := range items {
		items[i].CorrelationID = "id"
		items[i].OriginalURL = "https://example.com"
	}

	

	for b.Loop() {
		_, _ = service.BatchShortenURL(items, "user1")
	}
}
