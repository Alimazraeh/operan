package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultPort(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("M12_BASE_URL")
	os.Unsetenv("EVENT_BROKER_URL")

	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 8019 {
		t.Errorf("expected default port 8019, got %d", cfg.HTTPPort)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("expected JWT_SECRET='test-secret', got '%s'", cfg.JWTSecret)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("HTTP_PORT", "9999")
	defer os.Unsetenv("HTTP_PORT")
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.HTTPPort)
	}
}

func TestLoad_DefaultPortOutOfBounds(t *testing.T) {
	os.Setenv("HTTP_PORT", "99999") // invalid
	defer os.Unsetenv("HTTP_PORT")
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 8019 {
		t.Errorf("expected fallback to default 8019 for invalid port, got %d", cfg.HTTPPort)
	}
}

func TestLoad_FailClosed(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is empty")
	}
}

func TestLoad_FailClosedDefaultSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "change-me-in-production")
	defer os.Unsetenv("JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is the default value")
	}
}

func TestLoad_AllEnvVars(t *testing.T) {
	os.Setenv("HTTP_PORT", "8019")
	os.Setenv("JWT_SECRET", "my-secret")
	os.Setenv("M12_BASE_URL", "http://m12:8012")
	os.Setenv("DB_DSN", "postgres://user:pass@localhost/testdb")
	os.Setenv("EVENT_BROKER_URL", "kafka://localhost:9092")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("M12_BASE_URL")
		os.Unsetenv("DB_DSN")
		os.Unsetenv("EVENT_BROKER_URL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.M12BaseURL != "http://m12:8012" {
		t.Errorf("M12_BASE_URL = %s, want http://m12:8012", cfg.M12BaseURL)
	}
	if cfg.DBDSN != "postgres://user:pass@localhost/testdb" {
		t.Errorf("DB_DSN = %s", cfg.DBDSN)
	}
	if cfg.EventBrokerURL != "kafka://localhost:9092" {
		t.Errorf("EVENT_BROKER_URL = %s", cfg.EventBrokerURL)
	}
}