package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultPort(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("M12_BASE_URL")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("EVENT_BROKER_URL")

	_, err := Load()
	// Expect error on missing JWT secret (fail-closed)
	if err == nil {
		t.Fatal("expected error for missing JWT_SECRET")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("HTTP_PORT", "9999")
	defer os.Unsetenv("HTTP_PORT")
	os.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.HTTPPort)
	}
}

func TestLoad_DefaultPortRange(t *testing.T) {
	os.Setenv("HTTP_PORT", "0")
	defer os.Unsetenv("HTTP_PORT")
	os.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Port 0 is invalid, so it falls back to 8017
	if cfg.HTTPPort != 8017 {
		t.Errorf("expected default port 8017 for invalid port 0, got %d", cfg.HTTPPort)
	}
}

func TestLoad_AllEnvVars(t *testing.T) {
	os.Setenv("JWT_SECRET", "my-secret")
	os.Setenv("M12_BASE_URL", "http://m12:8012")
	os.Setenv("DB_DSN", "postgres://localhost/costgov")
	os.Setenv("EVENT_BROKER_URL", "kafka://localhost:9092")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTSecret != "my-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, "my-secret")
	}
	if cfg.M12BaseURL != "http://m12:8012" {
		t.Errorf("M12BaseURL = %q, want %q", cfg.M12BaseURL, "http://m12:8012")
	}
	if cfg.DBDSN != "postgres://localhost/costgov" {
		t.Errorf("DBDSN = %q, want %q", cfg.DBDSN, "postgres://localhost/costgov")
	}
	if cfg.EventBrokerURL != "kafka://localhost:9092" {
		t.Errorf("EventBrokerURL = %q, want %q", cfg.EventBrokerURL, "kafka://localhost:9092")
	}

	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("M12_BASE_URL")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("EVENT_BROKER_URL")
}

func TestLoad_DefaultTokenRejected(t *testing.T) {
	os.Setenv("JWT_SECRET", "change-me-in-production")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for default JWT secret")
	}

	os.Unsetenv("JWT_SECRET")
}