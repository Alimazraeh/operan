package config

import (
	"fmt"
	"os"
)

// Config holds all configuration loaded from environment variables.
type Config struct {
	HTTPPort       string
	JWTSecret      string
	Issuer         string
	DBDSN          string
	EventBrokerURL string
	M04BaseURL     string
}

// Load reads all config values from environment variables.
func Load() *Config {
	return &Config{
		HTTPPort:       getEnv("HTTP_PORT", "8010"),
		JWTSecret:      getEnv("JWT_SECRET", "default-secret-key-for-development"),
		Issuer:         getEnv("ISSUER", "operan-tenant-control-plane"),
		DBDSN:          getEnv("DB_DSN", "postgres://postgres:password@localhost:5432/operan?sslmode=disable"),
		EventBrokerURL: getEnv("EVENT_BROKER_URL", "localhost:9092"),
		M04BaseURL:     getEnv("M04_BASE_URL", "http://localhost:8004"),
	}
}

// Validate checks that required config values are set.
func (c *Config) Validate() error {
	if c.JWTSecret == "default-secret-key-for-development" {
		return fmt.Errorf("JWT_SECRET must be set in production (fail-closed)")
	}
	if c.DBDSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}