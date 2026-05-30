package config

import (
	"os"
	"testing"
)

func TestParseConfig_Defaults(t *testing.T) {
	// Clear all relevant env vars
	os.Unsetenv("MODULE05_PORT")
	os.Unsetenv("MODULE05_JWT_SECRET")

	cfg := ParseConfig()

	if cfg.Port != 8005 {
		t.Errorf("expected port 8005, got %d", cfg.Port)
	}

	if cfg.JWTSecret != "change-me-in-production" {
		t.Errorf("expected default JWT secret 'change-me-in-production', got %s", cfg.JWTSecret)
	}

	if cfg.MaxPageSize != 100 {
		t.Errorf("expected MaxPageSize 100, got %d", cfg.MaxPageSize)
	}
}

func TestParseConfig_EnvVars(t *testing.T) {
	os.Setenv("MODULE05_PORT", "9090")
	os.Setenv("MODULE05_JWT_SECRET", "test-secret-123")
	os.Setenv("MODULE05_EVENT_BROKER_URL", "amqp://localhost:5672")
	os.Setenv("MODULE05_MAX_PAGE_SIZE", "50")
	defer func() {
		os.Unsetenv("MODULE05_PORT")
		os.Unsetenv("MODULE05_JWT_SECRET")
		os.Unsetenv("MODULE05_EVENT_BROKER_URL")
		os.Unsetenv("MODULE05_MAX_PAGE_SIZE")
	}()

	cfg := ParseConfig()

	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}

	if cfg.JWTSecret != "test-secret-123" {
		t.Errorf("expected JWT secret 'test-secret-123', got %s", cfg.JWTSecret)
	}

	if cfg.EventBrokerURL != "amqp://localhost:5672" {
		t.Errorf("expected event broker URL 'amqp://localhost:5672', got %s", cfg.EventBrokerURL)
	}

	if cfg.MaxPageSize != 50 {
		t.Errorf("expected MaxPageSize 50, got %d", cfg.MaxPageSize)
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := Config{
		Port:      8080,
		JWTSecret: "custom-secret",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidate_DefaultJWTSecret(t *testing.T) {
	cfg := Config{
		Port:      8080,
		JWTSecret: "change-me-in-production",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for default JWT secret")
	}
}

func TestValidate_DefaultConfig(t *testing.T) {
	cfg := ParseConfig()

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for default config (unsecured JWT secret)")
	}
}

func TestConfig_Defaults(t *testing.T) {
	os.Unsetenv("MODULE05_PORT")

	cfg := ParseConfig()

	if cfg.Port != 8005 {
		t.Errorf("expected default port 8005, got %d", cfg.Port)
	}

	if cfg.TemplateCacheTTL != 300 {
		t.Errorf("expected default TemplateCacheTTL 300, got %d", cfg.TemplateCacheTTL)
	}
}

func TestConfig_EnvIntParsing(t *testing.T) {
	os.Setenv("MODULE05_PORT", "invalid")
	defer os.Unsetenv("MODULE05_PORT")

	cfg := ParseConfig()

	// Should fall back to default on invalid int
	if cfg.Port != 8005 {
		t.Errorf("expected default port 8005 on invalid input, got %d", cfg.Port)
	}
}

func TestConfig_OtherDefaults(t *testing.T) {
	os.Unsetenv("MODULE05_OTLP_ENDPOINT")
	os.Unsetenv("MODULE05_DB_URL")
	os.Unsetenv("MODULE05_REDIS_URL")

	cfg := ParseConfig()

	if cfg.OTLPEndpoint != "http://localhost:4318" {
		t.Errorf("expected default OTLP endpoint 'http://localhost:4318', got %s", cfg.OTLPEndpoint)
	}

	if cfg.DBURL != "" {
		t.Errorf("expected empty DB URL, got %s", cfg.DBURL)
	}

	if cfg.RedisURL != "" {
		t.Errorf("expected empty Redis URL, got %s", cfg.RedisURL)
	}
}
