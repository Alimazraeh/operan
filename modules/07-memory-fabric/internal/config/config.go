package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime configuration for Module 07.
type Config struct {
	Port           int
	DBURL          string
	RedisURL       string
	JWTSecret      string
	EventBrokerURL string
	OTLPEndpoint   string
	MaxPageSize    int
	GCBatchSize    int // max vectors removed per garbage-collection run
}

// ParseConfig reads configuration from the environment, applying defaults.
func ParseConfig() Config {
	return Config{
		Port:           envInt("MODULE07_PORT", 8007),
		DBURL:          env("MODULE07_DB_URL", ""),
		RedisURL:       env("MODULE07_REDIS_URL", ""),
		JWTSecret:      env("MODULE07_JWT_SECRET", "change-me-in-production"),
		EventBrokerURL: env("MODULE07_EVENT_BROKER_URL", ""),
		OTLPEndpoint:   env("MODULE07_OTLP_ENDPOINT", "http://localhost:4318"),
		MaxPageSize:    envInt("MODULE07_MAX_PAGE_SIZE", 100),
		GCBatchSize:    envInt("MODULE07_GC_BATCH_SIZE", 1000),
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
		return fmt.Errorf("MODULE07_JWT_SECRET must be set")
	}
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE07_JWT_SECRET must be changed from default value in production")
	}
	return nil
}
