package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the Execution Sandbox.
type Config struct {
	JWTSecret     string
	Issuer        string
	DBDSN         string
	EventBrokerURL string
	M10BaseURL    string
	HTTPPort      int
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	port := 8016
	if p, err := strconv.Atoi(envOr("HTTP_PORT", "8016")); err == nil {
		port = p
	}
	return &Config{
		JWTSecret:      envOr("JWT_SECRET", ""),
		Issuer:         envOr("JWT_ISSUER", "operan-tenant-control-plane"),
		DBDSN:          envOr("DB_DSN", ""),
		EventBrokerURL: envOr("EVENT_BROKER_URL", ""),
		M10BaseURL:     envOr("M10_BASE_URL", "http://localhost:8010"),
		HTTPPort:       port,
	}
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.JWTSecret == "change-me" {
		return fmt.Errorf("JWT_SECRET must not be the default value")
	}
	if c.DBDSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}