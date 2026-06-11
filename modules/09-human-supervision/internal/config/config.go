package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime configuration for Module 09.
type Config struct {
	Port           int
	JWTSecret      string
	EventBrokerURL string
	OTLPEndpoint   string
	MaxPageSize    int
}

// ParseConfig reads configuration from the environment, applying defaults.
func ParseConfig() Config {
	return Config{
		Port:           envInt("MODULE09_PORT", 8009),
		JWTSecret:      env("MODULE09_JWT_SECRET", "change-me-in-production"),
		EventBrokerURL: env("MODULE09_EVENT_BROKER_URL", ""),
		OTLPEndpoint:   env("MODULE09_OTLP_ENDPOINT", "http://localhost:4318"),
		MaxPageSize:    envInt("MODULE09_MAX_PAGE_SIZE", 100),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// Validate ensures required production settings are present.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("MODULE09_JWT_SECRET must be set")
	}
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE09_JWT_SECRET must be changed from default value in production")
	}
	return nil
}
