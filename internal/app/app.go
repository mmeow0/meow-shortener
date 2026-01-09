package app

import (
	"fmt"
	"net/http"

	"github.com/mmeow0/meow-shortener/internal/config"
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
	repo   *repository.FileURLRepository
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

	// Используем файловое хранилище
	urlRepo, err := repository.NewFileURLRepository(cfg.FileStoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a URL file repository: %w", err)
	}
	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService, cfg.BaseURL, log)
	rt := router.NewRouter(urlHandler, log)

	return &App{
		cfg:    cfg,
		router: rt,
		repo:   urlRepo,
		logger: log,
	}, nil
}

func (a *App) Run() error {
	a.logger.Info("Running server", zap.String("address", a.cfg.ServerAddress))
	defer a.repo.Close() // Закрываем файл при завершении работы
	return http.ListenAndServe(a.cfg.ServerAddress, a.router)
}
