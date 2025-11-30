package config

import (
	"flag"
)

type Config struct {
	// ServerAddress - адрес запуска HTTP-сервера
	ServerAddress string
	// BaseURL - базовый адрес результирующего сокращённого URL
	BaseURL string
}

func NewConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Базовый адрес результирующего сокращённого URL")

	flag.Parse()

	return cfg
}

