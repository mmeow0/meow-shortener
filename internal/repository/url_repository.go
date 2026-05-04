// Package repository реализует хранение URL в памяти, в файле (NDJSON) и в PostgreSQL.
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

// ErrNotFound возвращается, если короткий идентификатор не найден.
var ErrNotFound = errors.New("url not found")

// ErrAlreadyExists означает коллизию генерируемого short_id при вставке.
var ErrAlreadyExists = errors.New("url id already exists")

// ErrConflict означает, что такой original_url уже сохранён с другим short_id (уникальность в БД).
var ErrConflict = errors.New("url already exists")

// ErrDeleted означает, что запись существует, но помечена удалённой (ответ 410 Gone).
var ErrDeleted = errors.New("url has been deleted")

// InMemoryURLRepository — потокобезопасное хранилище в памяти (карта по short_id).
type InMemoryURLRepository struct {
	mu   sync.RWMutex
	urls map[string]*model.URL // ключ - ShortURL
}

// NewInMemoryURLRepository создаёт пустое in-memory хранилище.
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

	if url.IsDeleted {
		return nil, ErrDeleted
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

// GetByUserID возвращает все URL конкретного пользователя (исключая удалённые)
func (r *InMemoryURLRepository) GetByUserID(userID string) ([]*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	urls := make([]*model.URL, 0, len(r.urls))
	for _, url := range r.urls {
		if url.UserID == userID && !url.IsDeleted {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

// DeleteByIDs помечает URL как удалённые по списку коротких ID для конкретного пользователя
func (r *InMemoryURLRepository) DeleteByIDs(shortIDs []string, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, shortID := range shortIDs {
		if url, exists := r.urls[shortID]; exists && url.UserID == userID {
			url.IsDeleted = true
		}
	}

	return nil
}

// Close для in-memory репозитория ничего не делает
func (r *InMemoryURLRepository) Close() error {
	return nil
}

// FileURLRepository дополняет InMemoryURLRepository дозаписью JSON-строк в файл и перезаписью при удалении.
type FileURLRepository struct {
	*InMemoryURLRepository
	filePath string
	file     *os.File
	encoder  *json.Encoder
	mu       sync.Mutex // отдельная блокировка для операций с файлом
}

// NewFileURLRepository загружает существующий файл (если есть) и открывает его для append-записи.
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

// DeleteByIDs переопределяет метод DeleteByIDs, перезаписывая файл после мягкого удаления
func (r *FileURLRepository) DeleteByIDs(shortIDs []string, userID string) error {
	// Помечаем как удалённые в памяти (мягкое удаление)
	if err := r.InMemoryURLRepository.DeleteByIDs(shortIDs, userID); err != nil {
		return err
	}

	// Перезаписываем файл с актуальными данными (включая is_deleted флаги)
	r.mu.Lock()
	defer r.mu.Unlock()

	// Закрываем текущий файл
	if r.file != nil {
		r.file.Close()
	}

	// Открываем файл для перезаписи
	file, err := os.OpenFile(r.filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for rewrite: %w", err)
	}

	r.file = file
	r.encoder = json.NewEncoder(file)

	// Записываем все URL (включая помеченные как удалённые)
	r.InMemoryURLRepository.mu.RLock()
	for _, url := range r.InMemoryURLRepository.urls {
		if err := r.encoder.Encode(url); err != nil {
			r.InMemoryURLRepository.mu.RUnlock()
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}
	r.InMemoryURLRepository.mu.RUnlock()

	return nil
}

// Close закрывает файл
func (r *FileURLRepository) Close() error {
	if r.file != nil {
		r.mu.Lock()
		defer r.mu.Unlock()

		if err := r.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync file: %w", err)
		}
		if err := r.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		r.file = nil
	}
	return nil
}
