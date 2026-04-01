// Package app собирает конфигурацию, хранилище, сервис, хендлеры и HTTP-роутер приложения.
package app

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"

	"github.com/mmeow0/meow-shortener/internal/audit"
	"github.com/mmeow0/meow-shortener/internal/config"
	"github.com/mmeow0/meow-shortener/internal/database"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/logger"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/router"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
)

// App держит зависимости запущенного сервера и реализует graceful закрытие ресурсов.
type App struct {
	cfg    *config.Config
	router http.Handler
	repo   service.URLRepository
	db     *database.DB
	logger *zap.Logger
}

// InitializeApp загружает конфигурацию, подключает хранилище (PostgreSQL, файл или память),
// строит роутер и опционально включает аудит по файлу или HTTP.
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
	var urlRepo service.URLRepository

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

	var auditObservers []audit.Observer
	if cfg.AuditFile != "" {
		auditObservers = append(auditObservers, audit.NewFileObserver(cfg.AuditFile))
		log.Info("Audit file sink enabled", zap.String("path", cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		auditObservers = append(auditObservers, audit.NewHTTPObserver(cfg.AuditURL))
		log.Info("Audit HTTP sink enabled", zap.String("url", cfg.AuditURL))
	}
	var auditPublisher *audit.Publisher
	if len(auditObservers) > 0 {
		auditPublisher = audit.NewPublisher(auditObservers, log)
	}

	urlHandler := handler.NewURLHandler(urlService, cfg.BaseURL, log, auditPublisher)
	pingHandler := handler.NewPingHandler(db, log)
	rt := router.NewRouter(urlHandler, pingHandler, cfg.SecretKey, log)

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
	// Парсим DSN как URL
	u, err := url.Parse(dsn)
	if err != nil {
		return "***" // Если не удалось распарсить, скрываем всё
	}

	// Маскируем пароль
	if u.User != nil {
		username := u.User.Username()
		u.User = url.UserPassword(username, "***")
	}

	return u.String()
}

// Run поднимает сервер pprof на 127.0.0.1:6060 и слушает a.cfg.ServerAddress.
func (a *App) Run() error {
	pprofMux := http.NewServeMux()
	pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
	pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	pprofMux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	pprofMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	pprofMux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))

	go func() {
		const addr = "127.0.0.1:6060"
		a.logger.Info("pprof server started", zap.String("addr", addr), zap.String("heap", "http://"+addr+"/debug/pprof/heap"))
		if err := http.ListenAndServe(addr, pprofMux); err != nil {
			a.logger.Error("pprof server stopped", zap.Error(err))
		}
	}()

	a.logger.Info("Starting server", zap.String("address", a.cfg.ServerAddress))
	return http.ListenAndServe(a.cfg.ServerAddress, a.router)
}

// Close закрывает репозиторий и пул соединений БД (если были инициализированы).
func (a *App) Close() error {
	if a.repo != nil {
		if err := a.repo.Close(); err != nil {
			a.logger.Error("Failed to close repository", zap.Error(err))
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Error("Failed to close database", zap.Error(err))
		}
	}

	return nil
}
