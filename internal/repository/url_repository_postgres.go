package repository

import (
	"database/sql"
	"fmt"

	"github.com/mmeow0/meow-shortener/internal/model"
)

// PostgresURLRepository реализация хранилища URL в PostgreSQL
type PostgresURLRepository struct {
	db *sql.DB
}

// NewPostgresURLRepository создаёт новый PostgreSQL репозиторий
func NewPostgresURLRepository(db *sql.DB) *PostgresURLRepository {
	return &PostgresURLRepository{
		db: db,
	}
}

// Save сохраняет URL в базу данных
func (r *PostgresURLRepository) Save(url *model.URL) error {
	query := `
		INSERT INTO urls (short_id, original_url, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`

	_, err := r.db.Exec(query, url.ShortURL, url.OriginalURL, url.UserID)
	if err != nil {
		// Проверяем на дубликат
		if err.Error() == "pq: duplicate key value violates unique constraint \"urls_short_id_key\"" {
			return fmt.Errorf("%w: %s", ErrAlreadyExists, url.ShortURL)
		}
		return fmt.Errorf("failed to insert url: %w", err)
	}

	return nil
}

// FindByID находит URL по короткому идентификатору
func (r *PostgresURLRepository) FindByID(id string) (*model.URL, error) {
	query := `
		SELECT short_id, original_url, user_id
		FROM urls
		WHERE short_id = $1
	`

	url := &model.URL{}
	err := r.db.QueryRow(query, id).Scan(&url.ShortURL, &url.OriginalURL, &url.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query url: %w", err)
	}

	return url, nil
}

// GetAll возвращает все URL из базы данных
func (r *PostgresURLRepository) GetAll() ([]*model.URL, error) {
	query := `
		SELECT short_id, original_url, user_id
		FROM urls
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query urls: %w", err)
	}
	defer rows.Close()

	urls := make([]*model.URL, 0)
	for rows.Next() {
		url := &model.URL{}
		if err := rows.Scan(&url.ShortURL, &url.OriginalURL, &url.UserID); err != nil {
			continue
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return urls, nil
}

// GetByUserID возвращает все URL конкретного пользователя
func (r *PostgresURLRepository) GetByUserID(userID string) ([]*model.URL, error) {
	query := `
		SELECT short_id, original_url, user_id
		FROM urls
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query urls by user id: %w", err)
	}
	defer rows.Close()

	urls := make([]*model.URL, 0)
	for rows.Next() {
		url := &model.URL{}
		if err := rows.Scan(&url.ShortURL, &url.OriginalURL, &url.UserID); err != nil {
			continue
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return urls, nil
}

// Close для PostgreSQL репозитория ничего не делает (соединение управляется на уровне приложения)
func (r *PostgresURLRepository) Close() error {
	return nil
}

