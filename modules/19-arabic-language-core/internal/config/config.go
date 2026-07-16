package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration.
type Config struct {
	HTTPPort      int
	JWTSecret     string
	M12BaseURL    string
	DBDSN         string
	EventBrokerURL string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	port := parsePort(8019)
	jwtSecret := os.Getenv("JWT_SECRET")
	m12URL := os.Getenv("M12_BASE_URL")
	dbDSN := os.Getenv("DB_DSN")
	eventBroker := os.Getenv("EVENT_BROKER_URL")

	cfg := &Config{
		HTTPPort:      port,
		JWTSecret:     jwtSecret,
		M12BaseURL:    m12URL,
		DBDSN:         dbDSN,
		EventBrokerURL: eventBroker,
	}

	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		return nil, errors.New("JWT_SECRET is not set or is the default value — fail-closed")
	}

	return cfg, nil
}

func parsePort(defaultPort int) int {
	if v := os.Getenv("HTTP_PORT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil && val >= 1 && val <= 65535 {
			return val
		}
	}
	return defaultPort
}