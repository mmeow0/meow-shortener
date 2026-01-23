package app

import (
	"fmt"
	"net/http"

	"github.com/mmeow0/meow-shortener/internal/config"
	"github.com/mmeow0/meow-shortener/internal/database"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/logger"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/router"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
)

type App struct {
	cfg    *config.Config
	router http.Handler
	repo   repository.URLRepository
	db     *database.DB
	logger *zap.Logger
}

func InitializeApp() (*App, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Создаём логгер
	log, err := logger.NewLogger(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Инициализируем подключение к базе данных (если DSN указан)
	var db *database.DB
	var urlRepo repository.URLRepository

	// Приоритет хранилищ: БД > Файл > Память
	if cfg.DatabaseDSN != "" {
		// Используем PostgreSQL
		db, err = database.NewDB(cfg.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}

		// Применяем миграции из папки migrations
		if err := database.RunMigrations(db.DB, "migrations"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		urlRepo = repository.NewPostgresURLRepository(db.DB)
		log.Info("Using PostgreSQL storage", zap.String("dsn", maskDSN(cfg.DatabaseDSN)))
	} else if cfg.FileStoragePath != "" {
		// Используем файловое хранилище
		urlRepo, err = repository.NewFileURLRepository(cfg.FileStoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize a URL file repository: %w", err)
		}
		log.Info("Using file storage", zap.String("path", cfg.FileStoragePath))
	} else {
		// Используем in-memory хранилище
		urlRepo = repository.NewInMemoryURLRepository()
		log.Info("Using in-memory storage")
	}

	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService, cfg.BaseURL, log)
	pingHandler := handler.NewPingHandler(db, log)
	rt := router.NewRouter(urlHandler, pingHandler, log)

	return &App{
		cfg:    cfg,
		router: rt,
		repo:   urlRepo,
		db:     db,
		logger: log,
	}, nil
}

// maskDSN маскирует пароль в строке подключения для безопасного логирования
func maskDSN(dsn string) string {
	// показываем только схему и хост
	if len(dsn) > 20 {
		return dsn[:20] + "..."
	}
	return "***"
}

func (a *App) Run() error {
	a.logger.Info("Running server", zap.String("address", a.cfg.ServerAddress))
	
	// Закрываем ресурсы при завершении работы
	defer func() {
		if a.repo != nil {
			a.repo.Close()
		}
		if a.db != nil {
			a.db.Close()
		}
	}()
	
	return http.ListenAndServe(a.cfg.ServerAddress, a.router)
}
