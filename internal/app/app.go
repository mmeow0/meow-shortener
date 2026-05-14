// Package app собирает конфигурацию, хранилище, сервис, хендлеры и HTTP-роутер приложения.
package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"time"

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
	log, err := logger.NewLogger(cfg.Logging.Level)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Инициализируем подключение к базе данных (если DSN указан)
	var db *database.DB
	var urlRepo service.URLRepository

	// Приоритет хранилищ: БД > Файл > Память
	if cfg.Storage.DatabaseDSN != "" {
		// Используем PostgreSQL
		db, err = database.NewDB(cfg.Storage.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}

		// Применяем миграции из папки migrations
		if err := database.RunMigrations(db.DB, "migrations"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		urlRepo = repository.NewPostgresURLRepository(db.DB)
		log.Info("Using PostgreSQL storage", zap.String("dsn", maskDSN(cfg.Storage.DatabaseDSN)))
	} else if cfg.Storage.FileStoragePath != "" {
		// Используем файловое хранилище
		urlRepo, err = repository.NewFileURLRepository(cfg.Storage.FileStoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize a URL file repository: %w", err)
		}
		log.Info("Using file storage", zap.String("path", cfg.Storage.FileStoragePath))
	} else {
		// Используем in-memory хранилище
		urlRepo = repository.NewInMemoryURLRepository()
		log.Info("Using in-memory storage")
	}

	urlService := service.NewURLService(urlRepo)

	var auditObservers []audit.Observer
	if cfg.Audit.File != "" {
		auditObservers = append(auditObservers, audit.NewFileObserver(cfg.Audit.File))
		log.Info("Audit file sink enabled", zap.String("path", cfg.Audit.File))
	}
	if cfg.Audit.URL != "" {
		auditObservers = append(auditObservers, audit.NewHTTPObserver(cfg.Audit.URL))
		log.Info("Audit HTTP sink enabled", zap.String("url", cfg.Audit.URL))
	}
	var auditPublisher *audit.Publisher
	if len(auditObservers) > 0 {
		auditPublisher = audit.NewPublisher(auditObservers, log)
	}

	urlHandler := handler.NewURLHandler(urlService, cfg.Server.BaseURL, cfg.Server.TrustedSubnet, log, auditPublisher)
	pingHandler := handler.NewPingHandler(db, log)
	rt := router.NewRouter(urlHandler, pingHandler, cfg.Security.SecretKey, log)

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

// Run слушает a.cfg.Server.Address до отмены ctx и штатно завершает сервер.
func (a *App) Run(ctx context.Context) error {
	var pprofServer *http.Server
	if a.cfg.Profiling.EnablePprof {
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		pprofMux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		pprofMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		pprofMux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))

		const addr = "127.0.0.1:6060"
		pprofServer = &http.Server{
			Addr:    addr,
			Handler: pprofMux,
		}

		go func() {
			a.logger.Info("pprof server started", zap.String("addr", addr), zap.String("heap", "http://"+addr+"/debug/pprof/heap"))
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.logger.Error("pprof server stopped", zap.Error(err))
			}
		}()
	}

	server := &http.Server{
		Addr:              a.cfg.Server.Address,
		Handler:           a.router,
		ReadHeaderTimeout: a.cfg.Server.Timeout,
		ReadTimeout:       a.cfg.Server.Timeout,
		WriteTimeout:      a.cfg.Server.Timeout,
		IdleTimeout:       a.cfg.Server.Timeout * 2,
	}
	errCh := make(chan error, 1)

	a.logger.Info("Starting server", zap.String("address", a.cfg.Server.Address), zap.Bool("https", a.cfg.Server.EnableHTTPS), zap.Duration("timeout", a.cfg.Server.Timeout))
	if a.cfg.Server.EnableHTTPS {
		tlsConfig, err := newTLSConfig(a.cfg.Server.Address)
		if err != nil {
			return fmt.Errorf("failed to initialize TLS config: %w", err)
		}

		listener, err := tls.Listen("tcp", a.cfg.Server.Address, tlsConfig)
		if err != nil {
			return err
		}

		go func() {
			errCh <- server.Serve(listener)
		}()
	} else {
		go func() {
			errCh <- server.ListenAndServe()
		}()
	}

	select {
	case <-ctx.Done():
		a.logger.Info("Shutting down server")
		return a.shutdownServers(server, pprofServer)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func (a *App) shutdownServers(server *http.Server, pprofServer *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	if pprofServer != nil {
		if err := pprofServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown pprof server: %w", err)
		}
	}

	return nil
}

func newTLSConfig(addr string) (*tls.Config, error) {
	cert, err := newSelfSignedCertificate(addr)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func newSelfSignedCertificate(addr string) (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return tls.X509KeyPair(certPEM, keyPEM)
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
