package config

import (
	"errors"
	"flag"
	"net/url"

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
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:""`
}

func NewConfig() (*Config, error) {
	cfg := &Config{}

	// Флаги командной строки
	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Базовый адрес сокращённого URL")
	flag.StringVar(&cfg.LogLevel, "l", "FATAL", "Уровень логирования")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "Путь к файлу для хранения URL")

	flag.Parse()

	// Переменные окружения имеют больший приоритет
	// и перезаписвают флаги
	if err := env.Parse(cfg); err != nil {
		return nil, err
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
