package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultPort(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	cfg := Load()
	if cfg.HTTPPort != 8018 {
		t.Errorf("expected port 8018, got %d", cfg.HTTPPort)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("HTTP_PORT", "9999")
	defer os.Unsetenv("HTTP_PORT")
	cfg := Load()
	if cfg.HTTPPort != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.HTTPPort)
	}
}

func TestLoad_CustomPortInvalid(t *testing.T) {
	os.Setenv("HTTP_PORT", "notanumber")
	defer os.Unsetenv("HTTP_PORT")
	cfg := Load()
	if cfg.HTTPPort != 8018 {
		t.Errorf("expected default port 8018 for invalid port, got %d", cfg.HTTPPort)
	}
}

func TestValidate_MissingJWTSecret(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing JWT_SECRET")
	}
}

func TestValidate_DefaultSecretRejected(t *testing.T) {
	cfg := &Config{JWTSecret: "change-me"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for default JWT_SECRET value")
	}
}

func TestValidate_MissingDBDSN(t *testing.T) {
	cfg := &Config{JWTSecret: "valid-secret"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing DB_DSN")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{JWTSecret: "valid-secret", DBDSN: "postgres://test"}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}