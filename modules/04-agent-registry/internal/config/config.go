// Package config provides configuration parsing for the Agent Registry module.
package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	DefaultListenAddr     = ":8083"
	DefaultModuleID       = "04-agent-registry"
	DefaultJWTSecret      = "change-me-in-production"
	DefaultJWKSURL        = ""
	DefaultJWKSRefreshRate = 3600
	DefaultEventBusHost   = "events.operan.internal"
	DefaultEventBusPort   = "9092"
	DefaultEventBusProto  = "kafka"
	DefaultDBHost         = "localhost"
	DefaultDBPort         = "5432"
	DefaultDBName         = "agent_registry"
	DefaultDBUser         = "postgres"
	DefaultDBPassword     = "postgres"
	DefaultOTLPEndpoint   = "http://localhost:4318"
	DefaultLogLevel       = "production"
	DefaultDBMaxOpen      = 25
	DefaultDBMaxIdle      = 5
)

// Config holds the runtime configuration for the Agent Registry service.
type Config struct {
	ListenAddr         string
	ModuleID           string
	JWTSecret          string
	JWKSURL            string
	JWKSRefreshRate    int
	EventBusHost       string
	EventBusPort       string
	EventBusProto      string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBMaxOpen          int
	DBMaxIdle          int
	OTLPEndpoint       string
	LogLevel           string
	DatabaseDSN        string
}

// ParseConfig reads configuration from environment variables with defaults.
func ParseConfig() (Config, error) {
	cfg := Config{
		ListenAddr:       getEnvOrDefault("AGENT_REGISTRY_PORT", DefaultListenAddr),
		ModuleID:         getEnvOrDefault("AGENT_REGISTRY_MODULE_ID", DefaultModuleID),
		JWTSecret:        getEnvOrDefault("JWT_SECRET", DefaultJWTSecret),
		JWKSURL:          getEnvOrDefault("JWKS_URL", DefaultJWKSURL),
		JWKSRefreshRate:  getEnvIntOrDefault("JWKS_REFRESH_RATE", DefaultJWKSRefreshRate),
		EventBusHost:     getEnvOrDefault("EVENT_BUS_HOST", DefaultEventBusHost),
		EventBusPort:     getEnvOrDefault("EVENT_BUS_PORT", DefaultEventBusPort),
		EventBusProto:    getEnvOrDefault("EVENT_BUS_PROTO", DefaultEventBusProto),
		DBHost:           getEnvOrDefault("DB_HOST", DefaultDBHost),
		DBPort:           getEnvOrDefault("DB_PORT", DefaultDBPort),
		DBUser:           getEnvOrDefault("DB_USER", DefaultDBUser),
		DBPassword:       getEnvOrDefault("DB_PASSWORD", DefaultDBPassword),
		DBName:           getEnvOrDefault("DB_NAME", DefaultDBName),
		DBMaxOpen:        getEnvIntOrDefault("DB_MAX_OPEN", DefaultDBMaxOpen),
		DBMaxIdle:        getEnvIntOrDefault("DB_MAX_IDLE", DefaultDBMaxIdle),
		OTLPEndpoint:     getEnvOrDefault("OTLP_ENDPOINT", DefaultOTLPEndpoint),
		LogLevel:         getEnvOrDefault("LOG_LEVEL", DefaultLogLevel),
		DatabaseDSN:      os.Getenv("AGENT_REGISTRY_DATABASE_DSN"),
	}

	return cfg, cfg.Validate()
}

// Validate checks configuration for required fields and security issues.
func (c *Config) Validate() error {
	if c.ModuleID == "" {
		return fmt.Errorf("module ID is required")
	}
	if c.JWTSecret == DefaultJWTSecret {
		return fmt.Errorf("JWT_SECRET must be changed from default value in production")
	}
	if c.DBPassword == DefaultDBPassword {
		return fmt.Errorf("DB_PASSWORD is set to default value; set via DB_PASSWORD env var")
	}
	return nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
