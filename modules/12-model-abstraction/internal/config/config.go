package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
)

// ProviderAPIKey represents a single provider key from PROVIDER_API_KEYS JSON.
type ProviderAPIKey struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// Config holds all runtime configuration.
type Config struct {
	HTTPPort        int               `json:"http_port"`
	JWTSecret       string            `json:"jwt_secret"`
	EventBrokerURL  string            `json:"event_broker_url"`
	DBDSN           string            `json:"db_dsn"`
	ProviderAPIKeys map[string]string `json:"provider_api_keys"`
}

// Load reads configuration from environment variables.
// PROVIDER_API_KEYS is an optional JSON array like:
//
//	[{"name":"openai","key":"sk-..."},{"name":"anthropic","key":"sk-ant-..."}]
func Load() (*Config, error) {
	port := parsePort()
	jwtSecret := os.Getenv("IAM_TOKEN_SECRET")
	eventBroker := os.Getenv("EVENT_BROKER_URL")
	dbDSN := os.Getenv("DB_DSN")

	cfg := &Config{
		HTTPPort:        port,
		JWTSecret:       jwtSecret,
		EventBrokerURL:  eventBroker,
		DBDSN:           dbDSN,
		ProviderAPIKeys: make(map[string]string),
	}

	// Parse PROVIDER_API_KEYS JSON array
	providerKeysJSON := os.Getenv("PROVIDER_API_KEYS")
	if providerKeysJSON != "" {
		var keys []ProviderAPIKey
		if err := json.Unmarshal([]byte(providerKeysJSON), &keys); err != nil {
			return nil, errors.New("invalid PROVIDER_API_KEYS: " + err.Error())
		}
		for _, k := range keys {
			cfg.ProviderAPIKeys[k.Name] = k.Key
		}
	}

	// Fail-closed: JWT secret must not be the default
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		return nil, errors.New("IAM_TOKEN_SECRET is not set or is the default value — fail-closed")
	}

	return cfg, nil
}

func parsePort() int {
	p := 8012
	if v := os.Getenv("HTTP_PORT"); v != "" {
		val, err := strconv.Atoi(v)
		if err == nil && val >= 1 && val <= 65535 {
			p = val
		}
	}
	return p
}