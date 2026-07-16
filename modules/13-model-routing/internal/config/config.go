package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the model-routing service.
type Config struct {
	Port         int
	JWTSecret    string
	M12BaseURL   string
	DBDSN        string
	EventBrokerURL string
}

// Load reads configuration from environment variables with sensible defaults.
// Port defaults to 8013. The JWT secret must be set to a non-empty value
// (fail-closed startup).
func Load() (*Config, error) {
	port := 8013
	if p := os.Getenv("PORT"); p != "" {
		var err error
		port, err = strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("port %q: %w", p, err)
		}
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET must be set (fail-closed)")
	}

	return &Config{
		Port:         port,
		JWTSecret:    jwtSecret,
		M12BaseURL:   os.Getenv("M12_BASE_URL"),
		DBDSN:        os.Getenv("DB_DSN"),
		EventBrokerURL: os.Getenv("EVENT_BROKER_URL"),
	}, nil
}