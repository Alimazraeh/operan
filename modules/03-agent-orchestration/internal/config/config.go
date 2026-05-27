// Package config provides runtime configuration for the orchestration engine.
package config

import (
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
	LogLevel      string
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
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
