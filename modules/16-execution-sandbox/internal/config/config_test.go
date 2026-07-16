package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("M10_BASE_URL")

	cfg := Load()
	if cfg.HTTPPort != 8016 {
		t.Errorf("expected port 8016, got %d", cfg.HTTPPort)
	}
	if cfg.Issuer != "operan-tenant-control-plane" {
		t.Errorf("expected issuer 'operan-tenant-control-plane', got %s", cfg.Issuer)
	}
	if cfg.DBDSN != "" {
		t.Error("expected empty DB_DSN, got non-empty")
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

func TestValidate_MissingSecret(t *testing.T) {
	cfg := &Config{JWTSecret: "", DBDSN: "postgres://test"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing JWT_SECRET")
	}
}

func TestValidate_DefaultSecret(t *testing.T) {
	cfg := &Config{JWTSecret: "change-me", DBDSN: "postgres://test"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for default JWT_SECRET")
	}
}

func TestValidate_MissingDSN(t *testing.T) {
	cfg := &Config{JWTSecret: "my-secret", DBDSN: ""}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing DB_DSN")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{JWTSecret: "my-secret", DBDSN: "postgres://test"}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}