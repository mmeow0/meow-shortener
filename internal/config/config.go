// Package config читает настройки из флагов командной строки и переменных окружения.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Config описывает параметры запуска бинарника shortener.
type Config struct {
	Server    ServerConfig    `json:"server"`
	Storage   StorageConfig   `json:"storage"`
	Logging   LoggingConfig   `json:"logging"`
	Security  SecurityConfig  `json:"security"`
	Audit     AuditConfig     `json:"audit"`
	Profiling ProfilingConfig `json:"profiling"`
}

// ServerConfig описывает сетевые настройки HTTP-сервера.
type ServerConfig struct {
	Address       string        `json:"address"`
	BaseURL       string        `json:"base_url"`
	EnableHTTPS   bool          `json:"enable_https"`
	TrustedSubnet string        `json:"trusted_subnet"`
	Timeout       time.Duration `json:"-"`
}

// StorageConfig описывает настройки хранилища.
type StorageConfig struct {
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
}

// LoggingConfig описывает параметры логирования.
type LoggingConfig struct {
	Level string `json:"log_level"`
}

// SecurityConfig описывает параметры безопасности.
type SecurityConfig struct {
	SecretKey string `json:"secret_key"`
}

// AuditConfig описывает настройки аудита.
type AuditConfig struct {
	File string `json:"file"`
	URL  string `json:"url"`
}

// ProfilingConfig описывает отладочные настройки.
type ProfilingConfig struct {
	EnablePprof bool `json:"enable_pprof"`
}

const defaultBaseURL = "http://localhost:8080"
const defaultServerTimeout = 10 * time.Second

var (
	flagServerAddress   = flag.String("a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flagBaseURL         = flag.String("b", defaultBaseURL, "Базовый адрес сокращённого URL")
	flagLogLevel        = flag.String("l", "FATAL", "Уровень логирования")
	flagFileStoragePath = flag.String("f", "/tmp/short-url-db.json", "Путь к файлу для хранения URL")
	flagServerPort      = flag.String("server-port", "", "Порт для запуска сервера")
	flagDatabaseDSN     = flag.String("d", "", "Строка подключения к базе данных")
	flagDatabaseDSNLong = flag.String("database-dsn", "", "Строка подключения к базе данных")
	flagEnableHTTPS     = flag.Bool("s", false, "Включить HTTPS-сервер")
	flagTrustedSubnet   = flag.String("t", "", "Доверенная подсеть в формате CIDR для внутренней статистики")
	flagServerTimeout   = flag.Duration("server-timeout", defaultServerTimeout, "Таймаут HTTP-сервера")
	flagSecretKey       = flag.String("secret-key", "", "Секретный ключ для подписи кук")
	flagConfigPath      = flag.String("c", "", "Путь к JSON-файлу конфигурации")
	flagConfigPathLong  = flag.String("config", "", "Путь к JSON-файлу конфигурации")
	flagAuditFile       = flag.String("audit-file", "", "Путь к файлу-приёмнику логов аудита (пусто — отключено)")
	flagAuditURL        = flag.String("audit-url", "", "Полный URL удалённого приёмника аудита POST (пусто — отключено)")
	flagEnablePprof     = flag.Bool("enable-pprof", false, "Включить debug/pprof сервер на 127.0.0.1:6060")
)

// NewConfig разбирает flag.Parse() (если ещё не вызывали), затем переопределяет поля из окружения.
// Пустой SECRET_KEY заменяется случайно сгенерированным значением.
func NewConfig() (*Config, error) {
	// Парсим флаги только если они ещё не распарсены
	if !flag.Parsed() {
		flag.Parse()
	}

	setFlags := visitedFlags()

	cfg := defaultConfig()
	serverPort := ""

	configPath := configPathFromFlags(setFlags)
	if val, ok := os.LookupEnv("CONFIG"); ok {
		configPath = val
	}
	if configPath != "" {
		fileServerPort, err := loadConfigFile(configPath, cfg)
		if err != nil {
			return nil, err
		}
		serverPort = fileServerPort
	}

	applyFlags(cfg, setFlags)

	// Переменные окружения перезаписывают флаги, если они установлены
	if val, ok := os.LookupEnv("SERVER_ADDRESS"); ok {
		cfg.Server.Address = val
	}
	if val, ok := os.LookupEnv("BASE_URL"); ok {
		cfg.Server.BaseURL = val
	}
	if val, ok := os.LookupEnv("TRUSTED_SUBNET"); ok {
		cfg.Server.TrustedSubnet = val
	}
	if val, ok := os.LookupEnv("LOG_LEVEL"); ok {
		cfg.Logging.Level = val
	}
	if val, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.Storage.FileStoragePath = val
	}
	if val, ok := os.LookupEnv("DATABASE_DSN"); ok {
		cfg.Storage.DatabaseDSN = val
	}
	if val, ok := os.LookupEnv("SECRET_KEY"); ok {
		cfg.Security.SecretKey = val
	}
	if val, ok := os.LookupEnv("AUDIT_FILE"); ok {
		cfg.Audit.File = val
	}
	if val, ok := os.LookupEnv("AUDIT_URL"); ok {
		cfg.Audit.URL = val
	}
	if val, ok := os.LookupEnv("ENABLE_PPROF"); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid ENABLE_PPROF value %q: %w", val, err)
		}
		cfg.Profiling.EnablePprof = enabled
	}
	if val, ok := os.LookupEnv("ENABLE_HTTPS"); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid ENABLE_HTTPS value %q: %w", val, err)
		}
		cfg.Server.EnableHTTPS = enabled
	}
	if val, ok := os.LookupEnv("SERVER_TIMEOUT"); ok {
		timeout, err := time.ParseDuration(val)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVER_TIMEOUT value %q: %w", val, err)
		}
		cfg.Server.Timeout = timeout
	}

	// Обработка SERVER_PORT - если задан, он перезаписывает ServerAddress и BaseURL
	if flagWasSet(setFlags, "server-port") {
		serverPort = *flagServerPort
	}
	if port, ok := os.LookupEnv("SERVER_PORT"); ok {
		serverPort = port
	}

	// Если секретный ключ не задан, генерируем случайный
	if cfg.Security.SecretKey == "" {
		cfg.Security.SecretKey = generateSecretKey()
	}

	// Если указан serverPort (из флага или переменной окружения), он перезаписывает ServerAddress и BaseURL
	if serverPort != "" {
		cfg.Server.Address = fmt.Sprintf("localhost:%s", serverPort)
		// Обновляем BaseURL только если он имеет значение по умолчанию
		if cfg.Server.BaseURL == defaultBaseURL {
			scheme := "http"
			if cfg.Server.EnableHTTPS {
				scheme = "https"
			}
			cfg.Server.BaseURL = fmt.Sprintf("%s://localhost:%s", scheme, serverPort)
		}
	} else if cfg.Server.EnableHTTPS && cfg.Server.BaseURL == defaultBaseURL {
		cfg.Server.BaseURL = "https://localhost:8080"
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address: "localhost:8080",
			BaseURL: defaultBaseURL,
			Timeout: defaultServerTimeout,
		},
		Storage: StorageConfig{
			FileStoragePath: "/tmp/short-url-db.json",
		},
		Logging: LoggingConfig{
			Level: "FATAL",
		},
	}
}

func loadConfigFile(path string, cfg *Config) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	fileCfg := struct {
		Server *struct {
			Address       string `json:"address"`
			BaseURL       string `json:"base_url"`
			EnableHTTPS   *bool  `json:"enable_https"`
			TrustedSubnet string `json:"trusted_subnet"`
			Timeout       string `json:"timeout"`
		} `json:"server"`
		Storage *struct {
			FileStoragePath string `json:"file_storage_path"`
			DatabaseDSN     string `json:"database_dsn"`
		} `json:"storage"`
		Logging *struct {
			Level string `json:"log_level"`
		} `json:"logging"`
		Security *struct {
			SecretKey string `json:"secret_key"`
		} `json:"security"`
		Audit *struct {
			File string `json:"file"`
			URL  string `json:"url"`
		} `json:"audit"`
		Profiling *struct {
			EnablePprof *bool `json:"enable_pprof"`
		} `json:"profiling"`
		ServerAddress   string `json:"server_address"`
		BaseURL         string `json:"base_url"`
		LogLevel        string `json:"log_level"`
		FileStoragePath string `json:"file_storage_path"`
		DatabaseDSN     string `json:"database_dsn"`
		SecretKey       string `json:"secret_key"`
		AuditFile       string `json:"audit_file"`
		AuditURL        string `json:"audit_url"`
		EnablePprof     *bool  `json:"enable_pprof"`
		EnableHTTPS     *bool  `json:"enable_https"`
		TrustedSubnet   string `json:"trusted_subnet"`
		ServerTimeout   string `json:"server_timeout"`
		ServerPort      string `json:"server_port"`
	}{}

	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return "", fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	return fileCfg.ServerPort, nil
}

func visitedFlags() map[string]bool {
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})
	return setFlags
}

func flagWasSet(setFlags map[string]bool, name string) bool {
	return setFlags[name]
}

func configPathFromFlags(setFlags map[string]bool) string {
	configPath := ""
	if flagWasSet(setFlags, "c") {
		configPath = *flagConfigPath
	}
	if flagWasSet(setFlags, "config") {
		configPath = *flagConfigPathLong
	}
	return configPath
}

func applyFlags(cfg *Config, setFlags map[string]bool) {
	if flagWasSet(setFlags, "a") {
		cfg.Server.Address = *flagServerAddress
	}
	if flagWasSet(setFlags, "b") {
		cfg.Server.BaseURL = *flagBaseURL
	}
	if flagWasSet(setFlags, "t") {
		cfg.Server.TrustedSubnet = *flagTrustedSubnet
	}
	if flagWasSet(setFlags, "l") {
		cfg.Logging.Level = *flagLogLevel
	}
	if flagWasSet(setFlags, "f") {
		cfg.Storage.FileStoragePath = *flagFileStoragePath
	}
	if flagWasSet(setFlags, "d") {
		cfg.Storage.DatabaseDSN = *flagDatabaseDSN
	}
	if flagWasSet(setFlags, "database-dsn") {
		cfg.Storage.DatabaseDSN = *flagDatabaseDSNLong
	}
	if flagWasSet(setFlags, "s") {
		cfg.Server.EnableHTTPS = *flagEnableHTTPS
	}
	if flagWasSet(setFlags, "server-timeout") {
		cfg.Server.Timeout = *flagServerTimeout
	}
	if flagWasSet(setFlags, "secret-key") {
		cfg.Security.SecretKey = *flagSecretKey
	}
	if flagWasSet(setFlags, "audit-file") {
		cfg.Audit.File = *flagAuditFile
	}
	if flagWasSet(setFlags, "audit-url") {
		cfg.Audit.URL = *flagAuditURL
	}
	if flagWasSet(setFlags, "enable-pprof") {
		cfg.Profiling.EnablePprof = *flagEnablePprof
	}
}

func (c *Config) validate() error {
	if c.Server.Address == "" {
		return errors.New("server address is empty")
	}

	if c.Server.BaseURL == "" {
		return errors.New("base URL is empty")
	}

	parsedURL, err := url.Parse(c.Server.BaseURL)
	if err != nil {
		return errors.New("base URL is invalid")
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("base URL must contain scheme and host")
	}

	if c.Storage.FileStoragePath == "" {
		return errors.New("file storage path is empty")
	}

	if c.Server.Timeout <= 0 {
		return errors.New("server timeout must be positive")
	}

	if c.Server.TrustedSubnet != "" {
		if _, _, err := net.ParseCIDR(c.Server.TrustedSubnet); err != nil {
			return fmt.Errorf("trusted subnet is invalid: %w", err)
		}
	}

	return nil
}

// generateSecretKey генерирует случайный секретный ключ
func generateSecretKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
