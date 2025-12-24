package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"

	"github.com/caarlos0/env/v6"
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

func NewConfig() (*Config, error) {
	cfg := &Config{}

	// Дополнительный флаг для порта
	var serverPort string

	// Флаги командной строки
	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Базовый адрес сокращённого URL")
	flag.StringVar(&cfg.LogLevel, "l", "FATAL", "Уровень логирования")
	flag.StringVar(&cfg.FileStoragePath, "f", "/tmp/short-url-db.json", "Путь к файлу для хранения URL")
	flag.StringVar(&serverPort, "server-port", "", "Порт для запуска сервера")

	flag.Parse()

	// Переменные окружения имеют больший приоритет
	// и перезаписвают флаги
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Обработка SERVER_PORT - если задан, он перезаписывает ServerAddress и BaseURL
	if port := os.Getenv("SERVER_PORT"); port != "" && serverPort == "" {
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
