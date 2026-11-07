package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
)

const (
	defaultPort              = "8080"
	defaultSessionCookieName = "iconfigly_session"
	defaultSessionTTLHours   = 24
	developmentEnv           = "development"
	masterKeyBytes           = 32
	developmentMasterKeySeed = "iconfigly-development-master-key"
)

type Config struct {
	DatabaseURL       string
	Port              string
	Env               string
	SessionCookieName string
	SessionTTLHours   int
	MasterKeyHex      string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	env := getEnv("ICONFIGLY_ENV", developmentEnv)

	sessionTTLHours, err := loadSessionTTLHours()
	if err != nil {
		return Config{}, err
	}

	masterKeyHex, err := loadMasterKeyHex(env)
	if err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL:       databaseURL,
		Port:              getEnv("PORT", defaultPort),
		Env:               env,
		SessionCookieName: getEnv("ICONFIGLY_SESSION_COOKIE", defaultSessionCookieName),
		SessionTTLHours:   sessionTTLHours,
		MasterKeyHex:      masterKeyHex,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func loadSessionTTLHours() (int, error) {
	raw := os.Getenv("ICONFIGLY_SESSION_TTL_HOURS")

	if raw == "" {
		return defaultSessionTTLHours, nil
	}

	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return 0, errors.New("ICONFIGLY_SESSION_TTL_HOURS must be a positive integer")
	}

	return hours, nil
}

func loadMasterKeyHex(env string) (string, error) {
	raw := os.Getenv("ICONFIGLY_MASTER_KEY")

	if raw == "" {
		return defaultMasterKeyHex(env)
	}

	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != masterKeyBytes {
		return "", errors.New("ICONFIGLY_MASTER_KEY must be a 64-character hex string")
	}

	return raw, nil
}

func defaultMasterKeyHex(env string) (string, error) {
	if env != developmentEnv {
		return "", errors.New("ICONFIGLY_MASTER_KEY is required outside the development environment")
	}

	sum := sha256.Sum256([]byte(developmentMasterKeySeed))

	return hex.EncodeToString(sum[:]), nil
}
