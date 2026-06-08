package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port             int
	DBURL            string
	RedisURL         string
	JWTSecret        string
	EventBrokerURL   string
	OTLPEndpoint     string
	TemplateCacheTTL int
	MaxPageSize      int
}

func ParseConfig() Config {
	return Config{
		Port:             envInt("MODULE05_PORT", 8005),
		DBURL:            env("MODULE05_DB_URL", ""),
		RedisURL:         env("MODULE05_REDIS_URL", ""),
		JWTSecret:        env("MODULE05_JWT_SECRET", "change-me-in-production"),
		EventBrokerURL:   env("MODULE05_EVENT_BROKER_URL", ""),
		OTLPEndpoint:     env("MODULE05_OTLP_ENDPOINT", "http://localhost:4318"),
		TemplateCacheTTL: envInt("MODULE05_TEMPLATE_CACHE_TTL", 300),
		MaxPageSize:      envInt("MODULE05_MAX_PAGE_SIZE", 100),
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

func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("MODULE05_JWT_SECRET must be set")
	}
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE05_JWT_SECRET must be changed from default value in production")
	}
	return nil
}
