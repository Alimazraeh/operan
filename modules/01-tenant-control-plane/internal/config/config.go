package config

import (
	"fmt"
	"os"
)

const (
	DefaultListenAddr    = ":8080"
	DefaultOTLPEndpoint  = "http://localhost:4318"
	DefaultLogEnv        = "production"
	DefaultVersion       = "1.0.0"
	DefaultEventBusHost  = "events.operan.internal"
	DefaultEventBusPort  = "9092"
	DefaultEventBusProto = "kafka"
	DefaultJWTSecret     = "default-jwt-secret-change-in-production-min-32-chars!"
	DefaultIssuer        = "operan-tenant-control-plane"
)

// Config holds the runtime configuration for tenant-control-plane.
type Config struct {
	ListenAddr    string
	OTLPEndpoint  string
	LogEnv        string
	Version       string
	EventBusHost  string
	EventBusPort  string
	EventBusProto string
	LogLevel      string
	JWTSecret     string
	Issuer        string
}

// ParseConfig reads configuration from environment variables with defaults.
func ParseConfig() Config {
	return Config{
		ListenAddr:    getEnvOrDefault("LISTEN_ADDR", DefaultListenAddr),
		OTLPEndpoint:  getEnvOrDefault("OTLP_ENDPOINT", DefaultOTLPEndpoint),
		LogEnv:        getEnvOrDefault("LOG_ENV", DefaultLogEnv),
		Version:       getEnvOrDefault("MODULE_VERSION", DefaultVersion),
		EventBusHost:  getEnvOrDefault("EVENT_BUS_HOST", DefaultEventBusHost),
		EventBusPort:  getEnvOrDefault("EVENT_BUS_PORT", DefaultEventBusPort),
		EventBusProto: getEnvOrDefault("EVENT_BUS_PROTO", DefaultEventBusProto),
		LogLevel: func() string {
			switch getEnvOrDefault("LOG_ENV", DefaultLogEnv) {
			case "debug":
				return "debug"
			default:
				return "info"
			}
		}(),
		JWTSecret: getEnvOrDefault("JWT_SECRET", DefaultJWTSecret),
		Issuer:    getEnvOrDefault("JWT_ISSUER", DefaultIssuer),
	}
}

// Validate checks that required configuration values are set to safe values.
// Returns an error if JWT_SECRET is empty or still set to the default value.
func (c *Config) Validate() error {
	if c.JWTSecret == "" || c.JWTSecret == DefaultJWTSecret {
		return fmt.Errorf("JWT_SECRET is empty or set to default value; set a secure value via JWT_SECRET env var")
	}
	return nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
