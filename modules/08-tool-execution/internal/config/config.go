package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime configuration for Module 08.
type Config struct {
	Port           int
	DBURL          string
	RedisURL       string
	JWTSecret      string
	EventBrokerURL string
	OTLPEndpoint   string
	MaxPageSize    int
	DefaultTimeout int // default tool execution timeout in ms
}

// ParseConfig reads configuration from the environment, applying defaults.
func ParseConfig() Config {
	return Config{
		Port:           envInt("MODULE08_PORT", 8008),
		DBURL:          env("MODULE08_DB_URL", ""),
		RedisURL:       env("MODULE08_REDIS_URL", ""),
		JWTSecret:      env("MODULE08_JWT_SECRET", "change-me-in-production"),
		EventBrokerURL: env("MODULE08_EVENT_BROKER_URL", ""),
		OTLPEndpoint:   env("MODULE08_OTLP_ENDPOINT", "http://localhost:4318"),
		MaxPageSize:    envInt("MODULE08_MAX_PAGE_SIZE", 100),
		DefaultTimeout: envInt("MODULE08_DEFAULT_TIMEOUT_MS", 30000),
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
		return fmt.Errorf("MODULE08_JWT_SECRET must be set")
	}
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE08_JWT_SECRET must be changed from default value in production")
	}
	return nil
}
