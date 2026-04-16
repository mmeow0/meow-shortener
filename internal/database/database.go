// Package database оборачивает подключение к PostgreSQL (драйвер lib/pq).
package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB представляет обёртку над database/sql.DB
type DB struct {
	*sql.DB
}

// NewDB создаёт новое подключение к базе данных PostgreSQL
func NewDB(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, nil // Если DSN не указан, возвращаем nil (БД не используется)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

// Close закрывает соединение с базой данных
func (db *DB) Close() error {
	if db.DB != nil {
		return db.DB.Close()
	}
	return nil
}

// Ping проверяет соединение с базой данных
func (db *DB) Ping() error {
	if db.DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	return db.DB.Ping()
}
