package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the Identity & Access Management module.
type Config struct {
	Port             int
	TokenSecret      string
	TokenExpiry      int // minutes
	EventBrokerURL   string
	TracerEnabled    bool
	OtelCollectorURL string

	// Authentik integration
	AuthentikServerURL string // Base URL of Authentik API (e.g., https://authentik.operan.internal)
	AuthentikAdminToken string // API token with admin privileges
	AuthentikTokenTTL  int    // TTL for per-tenant API tokens in minutes (0 = no expiry)
	ProvisioningMethod string // "none", "docker-compose", "helm" — "none" assumes pre-provisioned Authentik
	IngressDomain      string // Domain for Ingress resources when provisioning via Helm
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		Port:             getEnvInt("IAM_PORT", 8002),
		TokenSecret:      getEnvString("IAM_TOKEN_SECRET", "change-me-in-production"),
		TokenExpiry:      getEnvInt("IAM_TOKEN_EXPIRY_MIN", 60),
		EventBrokerURL:   getEnvString("IAM_EVENT_BROKER_URL", "amqp://mq.operan.internal:5672"),
		TracerEnabled:    getEnvBool("IAM_OTEL_ENABLED", true),
		OtelCollectorURL: getEnvString("IAM_OTEL_COLLECTOR_URL", "http://otel-collector:4317"),

		// Authentik integration
		AuthentikServerURL: getEnvString("AUTHENTIK_SERVER_URL", ""),
		AuthentikAdminToken: getEnvString("AUTHENTIK_ADMIN_API_TOKEN", ""),
		AuthentikTokenTTL:  getEnvInt("AUTHENTIK_TOKEN_TTL_MIN", 0),
		ProvisioningMethod: getEnvString("AUTHENTIK_PROVISIONING_METHOD", "none"),
		IngressDomain:      getEnvString("AUTHENTIK_INGRESS_DOMAIN", "auth.operan.internal"),
	}

	// Validate Authentik config if integration is expected
	if cfg.AuthentikServerURL == "" {
		fmt.Fprintln(os.Stderr, "FATAL: AUTHENTIK_SERVER_URL is required for Authentik-backed IAM. Set this environment variable before starting.")
		os.Exit(1)
	}
	if cfg.AuthentikAdminToken == "" {
		fmt.Fprintln(os.Stderr, "FATAL: AUTHENTIK_ADMIN_API_TOKEN is required for Authentik-backed IAM. Set this environment variable before starting.")
		os.Exit(1)
	}

	return cfg
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
