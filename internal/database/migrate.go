package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations применяет все миграции к базе данных используя golang-migrate
func RunMigrations(db *sql.DB, migrationsPath string) error {
	// Ищем папку с миграциями
	actualPath, err := findMigrationsPath(migrationsPath)
	if err != nil {
		// Если папка не найдена, пытаемся создать схему напрямую
		return createSchemaDirectly(db)
	}

	// Создаём драйвер для PostgreSQL
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Создаём экземпляр migrate
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+actualPath,
		"postgres",
		driver,
	)
	if err != nil {
		// Если не удалось создать migrate, создаём схему напрямую
		return createSchemaDirectly(db)
	}

	// Применяем миграции
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// findMigrationsPath ищет папку с миграциями
func findMigrationsPath(basePath string) (string, error) {
	// Проверяем возможные пути
	paths := []string{
		basePath,
		filepath.Join(".", basePath),
		filepath.Join("..", basePath),
		filepath.Join("../..", basePath),
		filepath.Join("../../..", basePath),
	}

	// Получаем текущую рабочую директорию
	wd, _ := os.Getwd()
	paths = append(paths, filepath.Join(wd, basePath))

	// Проверяем путь относительно исполняемого файла
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		paths = append(paths, filepath.Join(exeDir, basePath))
		paths = append(paths, filepath.Join(exeDir, "..", basePath))
	}

	for _, path := range paths {
		if absPath, err := filepath.Abs(path); err == nil {
			if stat, err := os.Stat(absPath); err == nil && stat.IsDir() {
				return absPath, nil
			}
		}
	}

	return "", fmt.Errorf("migrations directory not found")
}

// createSchemaDirectly создаёт схему напрямую если миграции не найдены
func createSchemaDirectly(db *sql.DB) error {
	query := `
		-- Создание таблицы для хранения сокращённых URL
		CREATE TABLE IF NOT EXISTS urls (
			id SERIAL PRIMARY KEY,
			short_id VARCHAR(255) UNIQUE NOT NULL,
			original_url TEXT NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		-- Индексы для оптимизации запросов
		CREATE INDEX IF NOT EXISTS idx_urls_short_id ON urls(short_id);
		CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id);
		
		-- Уникальный индекс для original_url (для обработки конфликтов)
		CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_original_url ON urls(original_url);
	`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create schema directly: %w", err)
	}

	return nil
}
