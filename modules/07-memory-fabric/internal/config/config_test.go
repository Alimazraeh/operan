package config

import "testing"

func TestParseConfigDefaults(t *testing.T) {
	cfg := ParseConfig()
	if cfg.Port != 8007 {
		t.Errorf("Port = %d, want 8007", cfg.Port)
	}
	if cfg.MaxPageSize != 100 {
		t.Errorf("MaxPageSize = %d, want 100", cfg.MaxPageSize)
	}
	if cfg.GCBatchSize != 1000 {
		t.Errorf("GCBatchSize = %d, want 1000", cfg.GCBatchSize)
	}
	if cfg.EventBrokerURL != "" {
		t.Errorf("EventBrokerURL = %q, want empty (log-only default)", cfg.EventBrokerURL)
	}
}

func TestParseConfigEnvOverrides(t *testing.T) {
	t.Setenv("MODULE07_PORT", "9999")
	t.Setenv("MODULE07_JWT_SECRET", "test-secret-of-sufficient-length!")
	t.Setenv("MODULE07_EVENT_BROKER_URL", "localhost:9092")
	cfg := ParseConfig()
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
	if cfg.EventBrokerURL != "localhost:9092" {
		t.Errorf("EventBrokerURL = %q, want localhost:9092", cfg.EventBrokerURL)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate with custom secret should pass, got: %v", err)
	}
}

func TestValidateRejectsDefaultSecret(t *testing.T) {
	cfg := Config{JWTSecret: "change-me-in-production"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for default JWT secret, got nil")
	}
}

func TestValidateRejectsEmptySecret(t *testing.T) {
	cfg := Config{JWTSecret: ""}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty JWT secret, got nil")
	}
}
