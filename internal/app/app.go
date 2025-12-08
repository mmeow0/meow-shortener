package app

import (
	"net/http"

	"github.com/mmeow0/meow-shortener/internal/config"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/router"
	"github.com/mmeow0/meow-shortener/internal/service"
)

type App struct {
	cfg    *config.Config
	router http.Handler
}

func InitializeApp() (*App, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, err
	}

	urlRepo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(urlRepo)
	urlHandler := handler.NewURLHandler(urlService, cfg.BaseURL)
	rt := router.NewRouter(urlHandler)

	return &App{
		cfg:    cfg,
		router: rt,
	}, nil
}

func (a *App) Run() error {
	return http.ListenAndServe(a.cfg.ServerAddress, a.router)
}
