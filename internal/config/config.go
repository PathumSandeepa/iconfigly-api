package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	return Config{
		DatabaseURL: databaseURL,
		Port:        port,
	}, nil
}