package config

import (
	"testing"
)

func TestValidateRejectsDefaultSecret(t *testing.T) {
	cfg := Config{JWTSecret: DefaultJWTSecret}
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

func TestValidateAcceptsCustomSecret(t *testing.T) {
	cfg := Config{JWTSecret: "a-strong-secret-set-via-environment-variable"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected nil for custom JWT secret, got: %v", err)
	}
}

func TestParseConfigDefaultSecretFailsValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	cfg := ParseConfig()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected ParseConfig without JWT_SECRET to fail validation")
	}
}

func TestParseConfigEnvSecretPassesValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", "env-provided-secret-for-tests-32-chars!")
	cfg := ParseConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected ParseConfig with JWT_SECRET to pass validation, got: %v", err)
	}
}
