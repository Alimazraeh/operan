package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port              int
	DBURL             string
	RedisURL          string
	JWTSecret         string
	EventBrokerURL    string
	OTLPEndpoint      string
	TemplateCacheTTL  int
	MaxPageSize       int
	JWKSURL           string
	PolicyEngineURL   string
	Module03Endpoint  string
	Module04Endpoint  string
	Module07Endpoint  string
	Module10Endpoint  string
	Module11Endpoint  string
	Module18Endpoint  string
	SovereignEndpoint string
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
		JWKSURL:          env("MODULE05_JWKS_URL", ""),
		PolicyEngineURL:  env("MODULE05_POLICY_ENGINE_URL", ""),
		Module03Endpoint: env("MODULE03_ENDPOINT", ""),
		Module04Endpoint: env("MODULE04_ENDPOINT", ""),
		Module07Endpoint: env("MODULE07_ENDPOINT", ""),
		Module10Endpoint: env("MODULE10_ENDPOINT", ""),
		Module11Endpoint: env("MODULE11_ENDPOINT", ""),
		Module18Endpoint: env("MODULE18_ENDPOINT", ""),
		SovereignEndpoint: env("MODULE20_ENDPOINT", ""),
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
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE05_JWT_SECRET must be set in production")
	}
	return nil
}
