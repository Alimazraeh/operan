package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration.
type Config struct {
	HTTPPort       int    `json:"http_port"`
	JWTSecret      string `json:"jwt_secret"`
	M12BaseURL     string `json:"m12_base_url"`
	DBDSN          string `json:"db_dsn"`
	EventBrokerURL string `json:"event_broker_url"`
}

var ErrMissingSecret = errors.New("JWT_SECRET must be set to a non-default value")

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	port := parsePort()
	jwtSecret := os.Getenv("JWT_SECRET")
	m12BaseURL := os.Getenv("M12_BASE_URL")
	dbDSN := os.Getenv("DB_DSN")
	eventBrokerURL := os.Getenv("EVENT_BROKER_URL")

	if jwtSecret == "" || jwtSecret == "change-me-in-production" {
		return nil, ErrMissingSecret
	}

	return &Config{
		HTTPPort:       port,
		JWTSecret:      jwtSecret,
		M12BaseURL:     m12BaseURL,
		DBDSN:          dbDSN,
		EventBrokerURL: eventBrokerURL,
	}, nil
}

func parsePort() int {
	p := 8017
	if v := os.Getenv("HTTP_PORT"); v != "" {
		val, err := strconv.Atoi(v)
		if err == nil && val >= 1 && val <= 65535 {
			p = val
		}
	}
	return p
}