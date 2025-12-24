package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	// Адрес запуска HTTP-сервера
	ServerAddress string `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`

	// Базовый адрес результирующего сокращённого URL
	BaseURL string `env:"BASE_URL" envDefault:"http://localhost:8080"`

	// Уровень логирования
	LogLevel string `env:"LOG_LEVEL" envDefault:"FATAL"`

	// Путь к файлу для хранения URL
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:"/tmp/short-url-db.json"`
}

var (
	flagServerAddress   = flag.String("a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flagBaseURL         = flag.String("b", "http://localhost:8080", "Базовый адрес сокращённого URL")
	flagLogLevel        = flag.String("l", "FATAL", "Уровень логирования")
	flagFileStoragePath = flag.String("f", "/tmp/short-url-db.json", "Путь к файлу для хранения URL")
	flagServerPort      = flag.String("server-port", "", "Порт для запуска сервера")
)

func NewConfig() (*Config, error) {
	// Парсим флаги только если они ещё не распарсены
	if !flag.Parsed() {
		flag.Parse()
	}

	cfg := &Config{
		ServerAddress:   *flagServerAddress,
		BaseURL:         *flagBaseURL,
		LogLevel:        *flagLogLevel,
		FileStoragePath: *flagFileStoragePath,
	}

	// Переменные окружения перезаписывают флаги, если они установлены 
	if val := os.Getenv("SERVER_ADDRESS"); val != "" {
		cfg.ServerAddress = val
	}
	if val := os.Getenv("BASE_URL"); val != "" {
		cfg.BaseURL = val
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.LogLevel = val
	}
	if val := os.Getenv("FILE_STORAGE_PATH"); val != "" {
		cfg.FileStoragePath = val
	}

	// Обработка SERVER_PORT - если задан, он перезаписывает ServerAddress и BaseURL
	serverPort := *flagServerPort
	if port := os.Getenv("SERVER_PORT"); port != "" {
		serverPort = port
	}

	// Если указан serverPort (из флага или переменной окружения), он перезаписывает ServerAddress и BaseURL
	if serverPort != "" {
		cfg.ServerAddress = fmt.Sprintf("localhost:%s", serverPort)
		// Обновляем BaseURL только если он имеет значение по умолчанию
		if cfg.BaseURL == "http://localhost:8080" {
			cfg.BaseURL = fmt.Sprintf("http://localhost:%s", serverPort)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
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
