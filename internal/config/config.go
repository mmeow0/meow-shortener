package config

import (
	"errors"
	"flag"
	"net/url"
)

type Config struct {
	// ServerAddress - адрес запуска HTTP-сервера
	ServerAddress string
	// BaseURL - базовый адрес результирующего сокращённого URL
	BaseURL string
}

func NewConfig() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Базовый адрес результирующего сокращённого URL")

	flag.Parse()

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

	return nil
}
