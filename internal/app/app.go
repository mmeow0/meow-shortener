package app

import (
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
}

func InitializeApp() (*App, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, err
	}

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		return nil, err
	}

	// Используем файловое хранилище
	urlRepo, err := repository.NewFileURLRepository(cfg.FileStoragePath)
	if err != nil {
		return nil, err
	}
	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService, cfg.BaseURL)
	rt := router.NewRouter(urlHandler)

	return &App{
		cfg:    cfg,
		router: rt,
		repo:   urlRepo,
	}, nil
}

func (a *App) Run() error {
	logger.Log.Info("Running server", zap.String("address", a.cfg.ServerAddress))
	defer a.repo.Close() // Закрываем файл при завершении работы
	return http.ListenAndServe(a.cfg.ServerAddress, a.router)
}
