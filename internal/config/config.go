// Package config читает настройки из флагов командной строки и переменных окружения.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// Config описывает параметры запуска бинарника shortener.
type Config struct {
	// Адрес запуска HTTP-сервера
	ServerAddress string `env:"SERVER_ADDRESS" envDefault:"localhost:8080" json:"server_address"`

	// Базовый адрес результирующего сокращённого URL
	BaseURL string `env:"BASE_URL" envDefault:"http://localhost:8080" json:"base_url"`

	// Уровень логирования
	LogLevel string `env:"LOG_LEVEL" envDefault:"FATAL" json:"log_level"`

	// Путь к файлу для хранения URL
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:"/tmp/short-url-db.json" json:"file_storage_path"`

	// Строка подключения к базе данных PostgreSQL
	DatabaseDSN string `env:"DATABASE_DSN" json:"database_dsn"`

	// Секретный ключ для подписи кук
	SecretKey string `env:"SECRET_KEY" json:"secret_key"`

	// Путь к файлу логов аудита (пусто — запись в файл отключена)
	AuditFile string `env:"AUDIT_FILE" json:"audit_file"`

	// URL удалённого приёмника аудита POST (пусто — отправка отключена)
	AuditURL string `env:"AUDIT_URL" json:"audit_url"`

	// Флаг включения debug/pprof эндпоинтов (по умолчанию выключены)
	EnablePprof bool `env:"ENABLE_PPROF" envDefault:"false" json:"enable_pprof"`

	// Флаг включения HTTPS-сервера
	EnableHTTPS bool `env:"ENABLE_HTTPS" envDefault:"false" json:"enable_https"`
}

const defaultBaseURL = "http://localhost:8080"

var (
	flagServerAddress   = flag.String("a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flagBaseURL         = flag.String("b", defaultBaseURL, "Базовый адрес сокращённого URL")
	flagLogLevel        = flag.String("l", "FATAL", "Уровень логирования")
	flagFileStoragePath = flag.String("f", "/tmp/short-url-db.json", "Путь к файлу для хранения URL")
	flagServerPort      = flag.String("server-port", "", "Порт для запуска сервера")
	flagDatabaseDSN     = flag.String("d", "", "Строка подключения к базе данных")
	flagDatabaseDSNLong = flag.String("database-dsn", "", "Строка подключения к базе данных")
	flagEnableHTTPS     = flag.Bool("s", false, "Включить HTTPS-сервер")
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
		cfg.ServerAddress = val
	}
	if val, ok := os.LookupEnv("BASE_URL"); ok {
		cfg.BaseURL = val
	}
	if val, ok := os.LookupEnv("LOG_LEVEL"); ok {
		cfg.LogLevel = val
	}
	if val, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.FileStoragePath = val
	}
	if val, ok := os.LookupEnv("DATABASE_DSN"); ok {
		cfg.DatabaseDSN = val
	}
	if val, ok := os.LookupEnv("SECRET_KEY"); ok {
		cfg.SecretKey = val
	}
	if val, ok := os.LookupEnv("AUDIT_FILE"); ok {
		cfg.AuditFile = val
	}
	if val, ok := os.LookupEnv("AUDIT_URL"); ok {
		cfg.AuditURL = val
	}
	if val, ok := os.LookupEnv("ENABLE_PPROF"); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid ENABLE_PPROF value %q: %w", val, err)
		}
		cfg.EnablePprof = enabled
	}
	if val, ok := os.LookupEnv("ENABLE_HTTPS"); ok {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("invalid ENABLE_HTTPS value %q: %w", val, err)
		}
		cfg.EnableHTTPS = enabled
	}

	// Обработка SERVER_PORT - если задан, он перезаписывает ServerAddress и BaseURL
	if flagWasSet(setFlags, "server-port") {
		serverPort = *flagServerPort
	}
	if port, ok := os.LookupEnv("SERVER_PORT"); ok {
		serverPort = port
	}

	// Если секретный ключ не задан, генерируем случайный
	if cfg.SecretKey == "" {
		cfg.SecretKey = generateSecretKey()
	}

	// Если указан serverPort (из флага или переменной окружения), он перезаписывает ServerAddress и BaseURL
	if serverPort != "" {
		cfg.ServerAddress = fmt.Sprintf("localhost:%s", serverPort)
		// Обновляем BaseURL только если он имеет значение по умолчанию
		if cfg.BaseURL == defaultBaseURL {
			scheme := "http"
			if cfg.EnableHTTPS {
				scheme = "https"
			}
			cfg.BaseURL = fmt.Sprintf("%s://localhost:%s", scheme, serverPort)
		}
	} else if cfg.EnableHTTPS && cfg.BaseURL == defaultBaseURL {
		cfg.BaseURL = "https://localhost:8080"
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         defaultBaseURL,
		LogLevel:        "FATAL",
		FileStoragePath: "/tmp/short-url-db.json",
	}
}

func loadConfigFile(path string, cfg *Config) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	fileCfg := struct {
		*Config
		ServerPort string `json:"server_port"`
	}{
		Config: cfg,
	}

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
		cfg.ServerAddress = *flagServerAddress
	}
	if flagWasSet(setFlags, "b") {
		cfg.BaseURL = *flagBaseURL
	}
	if flagWasSet(setFlags, "l") {
		cfg.LogLevel = *flagLogLevel
	}
	if flagWasSet(setFlags, "f") {
		cfg.FileStoragePath = *flagFileStoragePath
	}
	if flagWasSet(setFlags, "d") {
		cfg.DatabaseDSN = *flagDatabaseDSN
	}
	if flagWasSet(setFlags, "database-dsn") {
		cfg.DatabaseDSN = *flagDatabaseDSNLong
	}
	if flagWasSet(setFlags, "s") {
		cfg.EnableHTTPS = *flagEnableHTTPS
	}
	if flagWasSet(setFlags, "secret-key") {
		cfg.SecretKey = *flagSecretKey
	}
	if flagWasSet(setFlags, "audit-file") {
		cfg.AuditFile = *flagAuditFile
	}
	if flagWasSet(setFlags, "audit-url") {
		cfg.AuditURL = *flagAuditURL
	}
	if flagWasSet(setFlags, "enable-pprof") {
		cfg.EnablePprof = *flagEnablePprof
	}
}

func (c *Config) validate() error {
	if c.ServerAddress == "" {
		return errors.New("server address is empty")
	}

	if c.BaseURL == "" {
		return errors.New("base URL is empty")
	}

	parsedURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return errors.New("base URL is invalid")
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("base URL must contain scheme and host")
	}

	if c.FileStoragePath == "" {
		return errors.New("file storage path is empty")
	}

	return nil
}

// generateSecretKey генерирует случайный секретный ключ
func generateSecretKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
