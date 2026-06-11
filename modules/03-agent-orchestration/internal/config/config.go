// Package config provides runtime configuration for the orchestration engine.
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
	DefaultEventBusTLS   = false
	DefaultEventBusSASL  = false
	DefaultJWTSecret     = "change-me-in-production"
	DefaultDBHost        = "localhost"
	DefaultDBPort        = "5432"
	DefaultDBUser        = "postgres"
	DefaultDBPassword    = "postgres"
	DefaultDBName        = "orchestration"
	DefaultDBMaxOpen     = 25
	DefaultDBMaxIdle     = 5
)

// Config holds the runtime configuration for the orchestration engine.
type Config struct {
	ListenAddr    string
	OTLPEndpoint  string
	LogEnv        string
	Version       string
	EventBusHost  string
	EventBusPort  string
	EventBusProto string
	EventBusTLS   bool
	EventBusSASL  bool
	EventBusUser  string
	EventBusPass  string
	LogLevel      string
	JWTSecret     string
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBMaxOpen     int
	DBMaxIdle     int
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
		EventBusTLS:   getEnvBool("EVENT_BUS_TLS", DefaultEventBusTLS),
		EventBusSASL:  getEnvBool("EVENT_BUS_SASL", DefaultEventBusSASL),
		EventBusUser:  getEnvOrDefault("EVENT_BUS_USER", ""),
		EventBusPass:  getEnvOrDefault("EVENT_BUS_PASS", ""),
		JWTSecret:     getEnvOrDefault("JWT_SECRET", DefaultJWTSecret),
		LogLevel: func() string {
			switch getEnvOrDefault("LOG_ENV", DefaultLogEnv) {
			case "debug":
				return "debug"
			default:
				return "info"
			}
		}(),
		DBHost:     getEnvOrDefault("DB_HOST", DefaultDBHost),
		DBPort:     getEnvOrDefault("DB_PORT", DefaultDBPort),
		DBUser:     getEnvOrDefault("DB_USER", DefaultDBUser),
		DBPassword: getEnvOrDefault("DB_PASSWORD", DefaultDBPassword),
		DBName:     getEnvOrDefault("DB_NAME", DefaultDBName),
		DBMaxOpen: func() int {
			v := getEnvOrDefault("DB_MAX_OPEN", "")
			if v == "" {
				return DefaultDBMaxOpen
			}
			var n int
			_, _ = fmt.Sscanf(v, "%d", &n)
			if n < 1 {
				return DefaultDBMaxOpen
			}
			return n
		}(),
		DBMaxIdle: func() int {
			v := getEnvOrDefault("DB_MAX_IDLE", "")
			if v == "" {
				return DefaultDBMaxIdle
			}
			var n int
			_, _ = fmt.Sscanf(v, "%d", &n)
			if n < 1 {
				return DefaultDBMaxIdle
			}
			return n
		}(),
	}
}

// Validate checks that required configuration values are set to safe defaults.
// Returns an error if JWT_SECRET uses the default production value.
func (c *Config) Validate() error {
	if c.JWTSecret == DefaultJWTSecret {
		return fmt.Errorf("JWT_SECRET is set to default value; set a secure value via JWT_SECRET env var")
	}
	return nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return fallback
	}
}
