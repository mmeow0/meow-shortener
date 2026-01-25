package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/mmeow0/meow-shortener/internal/model"
)

var ErrNotFound = errors.New("url not found")
var ErrAlreadyExists = errors.New("url id already exists")
var ErrConflict = errors.New("url already exists") // Конфликт - URL уже существует с другим short_id

// URLRepository интерфейс для работы с URL
type URLRepository interface {
	Save(url *model.URL) error
	BatchSave(urls []*model.URL) error
	FindByID(id string) (*model.URL, error)
	FindByOriginalURL(originalURL string) (*model.URL, error)
	GetAll() ([]*model.URL, error)
	GetByUserID(userID string) ([]*model.URL, error)
	Close() error
}

// InMemoryURLRepository базовая реализация хранилища URL в памяти
type InMemoryURLRepository struct {
	mu   sync.RWMutex
	urls map[string]*model.URL // ключ - ShortURL
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

	if _, exists := r.urls[url.ShortURL]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, url.ShortURL)
	}

	r.urls[url.ShortURL] = url
	return nil
}

// BatchSave сохраняет несколько URL за одну операцию
func (r *InMemoryURLRepository) BatchSave(urls []*model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем на дубликаты
	for _, url := range urls {
		if _, exists := r.urls[url.ShortURL]; exists {
			return fmt.Errorf("%w: %s", ErrAlreadyExists, url.ShortURL)
		}
	}

	// Сохраняем все URL
	for _, url := range urls {
		r.urls[url.ShortURL] = url
	}

	return nil
}

// FindByID находит URL по короткому идентификатору
func (r *InMemoryURLRepository) FindByID(id string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	url, exists := r.urls[id]
	if !exists {
		return nil, ErrNotFound
	}

	return url, nil
}

// FindByOriginalURL находит URL по оригинальному URL
func (r *InMemoryURLRepository) FindByOriginalURL(originalURL string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, url := range r.urls {
		if url.OriginalURL == originalURL {
			return url, nil
		}
	}

	return nil, ErrNotFound
}

// GetAll возвращает все URL из хранилища
func (r *InMemoryURLRepository) GetAll() ([]*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	urls := make([]*model.URL, 0, len(r.urls))
	for _, url := range r.urls {
		urls = append(urls, url)
	}

	return urls, nil
}

// GetByUserID возвращает все URL конкретного пользователя
func (r *InMemoryURLRepository) GetByUserID(userID string) ([]*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	urls := make([]*model.URL, 0)
	for _, url := range r.urls {
		if url.UserID == userID {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

// Close для in-memory репозитория ничего не делает
func (r *InMemoryURLRepository) Close() error {
	return nil
}

// FileURLRepository декоратор над InMemoryURLRepository с сохранением в файл
type FileURLRepository struct {
	*InMemoryURLRepository
	filePath string
	file     *os.File
	encoder  *json.Encoder
	mu       sync.Mutex // отдельная блокировка для операций с файлом
}

func NewFileURLRepository(filePath string) (*FileURLRepository, error) {
	inMemoryRepo := NewInMemoryURLRepository()

	repo := &FileURLRepository{
		InMemoryURLRepository: inMemoryRepo,
		filePath:              filePath,
	}

	// Загружаем существующие данные из файла
	if err := repo.loadFromFile(); err != nil {
		return nil, err
	}

	// Открываем файл для записи (append режим)
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	repo.file = file
	repo.encoder = json.NewEncoder(file)

	return repo, nil
}

// loadFromFile загружает данные из файла при старте
func (r *FileURLRepository) loadFromFile() error {
	file, err := os.Open(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Файл не существует - это нормально
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var url model.URL
		if err := json.Unmarshal(scanner.Bytes(), &url); err != nil {
			continue // Пропускаем битые записи
		}
		// Используем прямой доступ к map, т.к. это загрузка при инициализации
		r.InMemoryURLRepository.urls[url.ShortURL] = &url
	}

	return scanner.Err()
}

// Save переопределяет метод Save, добавляя запись в файл
func (r *FileURLRepository) Save(url *model.URL) error {
	// Сначала сохраняем в памяти
	if err := r.InMemoryURLRepository.Save(url); err != nil {
		return err
	}

	// Затем записываем в файл
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.encoder.Encode(url); err != nil {
		// Если не удалось записать в файл, нужно откатить изменения в памяти
		r.InMemoryURLRepository.mu.Lock()
		delete(r.InMemoryURLRepository.urls, url.ShortURL)
		r.InMemoryURLRepository.mu.Unlock()
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// BatchSave переопределяет метод BatchSave, добавляя запись в файл
func (r *FileURLRepository) BatchSave(urls []*model.URL) error {
	// Сначала сохраняем в памяти
	if err := r.InMemoryURLRepository.BatchSave(urls); err != nil {
		return err
	}

	// Затем записываем все URL в файл
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, url := range urls {
		if err := r.encoder.Encode(url); err != nil {
			// При ошибке откатываем все изменения в памяти
			r.InMemoryURLRepository.mu.Lock()
			for _, u := range urls {
				delete(r.InMemoryURLRepository.urls, u.ShortURL)
			}
			r.InMemoryURLRepository.mu.Unlock()
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return nil
}

// Close закрывает файл
func (r *FileURLRepository) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
