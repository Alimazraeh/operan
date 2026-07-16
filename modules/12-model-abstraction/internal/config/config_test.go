package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultPort(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("IAM_TOKEN_SECRET")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("PROVIDER_API_KEYS")

	// Must fail without JWT secret.
	_, err := Load()
	if err == nil {
		t.Fatal("expected error without JWT secret")
	}
}

func TestLoad_FailClosed(t *testing.T) {
	os.Setenv("HTTP_PORT", "8012")
	os.Setenv("IAM_TOKEN_SECRET", "change-me-in-production")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("PROVIDER_API_KEYS")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("IAM_TOKEN_SECRET")
		os.Unsetenv("EVENT_BROKER_URL")
		os.Unsetenv("DB_DSN")
		os.Unsetenv("PROVIDER_API_KEYS")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected fail-closed error")
	}
}

func TestLoad_Success(t *testing.T) {
	os.Setenv("IAM_TOKEN_SECRET", "test-secret-key")
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("PROVIDER_API_KEYS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 8012 {
		t.Errorf("expected port 8012, got %d", cfg.HTTPPort)
	}
	if cfg.JWTSecret != "test-secret-key" {
		t.Errorf("expected JWT secret 'test-secret-key', got %q", cfg.JWTSecret)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("HTTP_PORT", "9999")
	os.Setenv("IAM_TOKEN_SECRET", "test-secret")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("PROVIDER_API_KEYS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.HTTPPort)
	}
}

func TestLoad_ProviderAPIKeys(t *testing.T) {
	os.Setenv("IAM_TOKEN_SECRET", "test-secret")
	os.Setenv("PROVIDER_API_KEYS", `[{"name":"openai","key":"sk-abc"},{"name":"anthropic","key":"sk-ant-xyz"}]`)
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("DB_DSN")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ProviderAPIKeys["openai"] != "sk-abc" {
		t.Errorf("expected openai key 'sk-abc', got %q", cfg.ProviderAPIKeys["openai"])
	}
	if cfg.ProviderAPIKeys["anthropic"] != "sk-ant-xyz" {
		t.Errorf("expected anthropic key 'sk-ant-xyz', got %q", cfg.ProviderAPIKeys["anthropic"])
	}
}

func TestLoad_InvalidProviderAPIKeys(t *testing.T) {
	os.Setenv("IAM_TOKEN_SECRET", "test-secret")
	os.Setenv("PROVIDER_API_KEYS", "not-json")
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("DB_DSN")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid PROVIDER_API_KEYS")
	}
}