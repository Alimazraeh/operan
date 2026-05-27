package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the Identity & Access Management module.
type Config struct {
	Port             int
	DBDriver         string
	DBSource         string
	TokenSecret      string
	TokenExpiry      int // minutes
	EventBrokerURL   string
	TracerEnabled    bool
	OtelCollectorURL string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:             getEnvInt("IAM_PORT", 8002),
		DBDriver:         getEnvString("IAM_DB_DRIVER", "memory"),
		DBSource:         getEnvString("IAM_DB_SOURCE", ""),
		TokenSecret:      getEnvString("IAM_TOKEN_SECRET", "change-me-in-production"),
		TokenExpiry:      getEnvInt("IAM_TOKEN_EXPIRY_MIN", 60),
		EventBrokerURL:   getEnvString("IAM_EVENT_BROKER_URL", "amqp://mq.operan.internal:5672"),
		TracerEnabled:    getEnvBool("IAM_OTEL_ENABLED", true),
		OtelCollectorURL: getEnvString("IAM_OTEL_COLLECTOR_URL", "http://otel-collector:4317"),
	}
}

func getEnvString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1"
	}
	return fallback
}

// Assert compile-time type usage
var _ = fmt.Sprintf
