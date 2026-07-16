package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the Agent Marketplace.
type Config struct {
	HTTPPort       int
	JWTSecret      string
	DBDSN          string
	EventBrokerURL string
	M04BaseURL     string
	M03BaseURL     string
	M12BaseURL     string
}

// Load reads configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	port, _ := strconv.Atoi(envOr("HTTP_PORT", "8015"))

	c := &Config{
		HTTPPort:       port,
		JWTSecret:      envOr("JWT_SECRET", ""),
		DBDSN:          envOr("DB_DSN", "postgresql://localhost:5432/operan_marketplace?sslmode=disable"),
		EventBrokerURL: envOr("EVENT_BROKER_URL", ""),
		M04BaseURL:     envOr("M04_BASE_URL", "http://localhost:8004"),
		M03BaseURL:     envOr("M03_BASE_URL", "http://localhost:8003"),
		M12BaseURL:     envOr("M12_BASE_URL", "http://localhost:8012"),
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks critical configuration values.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET must be set")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}