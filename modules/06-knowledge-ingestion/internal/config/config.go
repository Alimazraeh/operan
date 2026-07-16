package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
)

// Config holds all runtime configuration.
type Config struct {
	HTTPPort        int               `json:"http_port"`
	JWTSecret       string            `json:"jwt_secret"`
	M12BaseURL      string            `json:"m12_base_url"`
	M07BaseURL      string            `json:"m07_base_url"`
	M19BaseURL      string            `json:"m19_base_url"`
	DBDSN           string            `json:"db_dsn"`
	EventBrokerURL  string            `json:"event_broker_url"`
	ProviderAPIKeys map[string]string `json:"provider_api_keys"`
	// ServiceToken is the service-to-service JWT used for inter-module calls.
	ServiceToken string `json:"service_token"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	port := parsePort()
	jwtSecret := os.Getenv("IAM_TOKEN_SECRET")
	eventBroker := os.Getenv("EVENT_BROKER_URL")
	dbDSN := os.Getenv("DB_DSN")
	m12BaseURL := os.Getenv("M12_BASE_URL")
	m07BaseURL := os.Getenv("M07_BASE_URL")
	m19BaseURL := os.Getenv("M19_BASE_URL")
	serviceToken := os.Getenv("SERVICE_TOKEN")

	cfg := &Config{
		HTTPPort:        port,
		JWTSecret:       jwtSecret,
		M12BaseURL:      m12BaseURL,
		M07BaseURL:      m07BaseURL,
		M19BaseURL:      m19BaseURL,
		DBDSN:           dbDSN,
		EventBrokerURL:  eventBroker,
		ProviderAPIKeys: make(map[string]string),
		ServiceToken:    serviceToken,
	}

	// Parse PROVIDER_API_KEYS JSON array
	providerKeysJSON := os.Getenv("PROVIDER_API_KEYS")
	if providerKeysJSON != "" {
		var keys []map[string]string
		if err := json.Unmarshal([]byte(providerKeysJSON), &keys); err != nil {
			return nil, errors.New("invalid PROVIDER_API_KEYS: " + err.Error())
		}
		for _, k := range keys {
			cfg.ProviderAPIKeys[k["name"]] = k["key"]
		}
	}

	// Fail-closed: JWT secret must not be the default
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		return nil, errors.New("IAM_TOKEN_SECRET is not set or is the default value — fail-closed")
	}

	return cfg, nil
}

func parsePort() int {
	p := 8006
	if v := os.Getenv("HTTP_PORT"); v != "" {
		val, err := strconv.Atoi(v)
		if err == nil && val >= 1 && val <= 65535 {
			p = val
		}
	}
	return p
}