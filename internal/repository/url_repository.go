package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/mmeow0/meow-shortener/internal/model"
)

var ErrNotFound = errors.New("url not found")
var ErrAlreadyExists = errors.New("url id already exists")

// FileURLRepository реализация хранилища URL с сохранением в файл
type FileURLRepository struct {
	mu       sync.Mutex
	urls     map[string]*model.URL // ключ - ShortURL
	filePath string
	file     *os.File
	encoder  *json.Encoder
}

func NewFileURLRepository(filePath string) (*FileURLRepository, error) {
	repo := &FileURLRepository{
		urls:     make(map[string]*model.URL),
		filePath: filePath,
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
		r.urls[url.ShortURL] = &url
	}

	return scanner.Err()
}

// Save сохраняет URL в хранилище и в файл
func (r *FileURLRepository) Save(url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.urls[url.ShortURL]; exists {
		return ErrAlreadyExists
	}

	r.urls[url.ShortURL] = url

	// Записываем в файл
	if err := r.encoder.Encode(url); err != nil {
		return err
	}

	return nil
}

// FindByID находит URL по короткому идентификатору
func (r *FileURLRepository) FindByID(id string) (*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	url, exists := r.urls[id]
	if !exists {
		return nil, ErrNotFound
	}

	return url, nil
}

// GetAll возвращает все URL из хранилища
func (r *FileURLRepository) GetAll() ([]*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	urls := make([]*model.URL, 0, len(r.urls))
	for _, url := range r.urls {
		urls = append(urls, url)
	}

	return urls, nil
}

// GetByUserID возвращает все URL конкретного пользователя
func (r *FileURLRepository) GetByUserID(userID string) ([]*model.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	urls := make([]*model.URL, 0)
	for _, url := range r.urls {
		if url.UserID == userID {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

// Close закрывает файл
func (r *FileURLRepository) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
