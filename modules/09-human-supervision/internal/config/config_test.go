package config

import "testing"

func TestParseConfigDefaults(t *testing.T) {
	cfg := ParseConfig()
	if cfg.Port != 8009 {
		t.Errorf("Port = %d, want 8009", cfg.Port)
	}
	if cfg.MaxPageSize != 100 {
		t.Errorf("MaxPageSize = %d, want 100", cfg.MaxPageSize)
	}
	if cfg.EventBrokerURL != "" {
		t.Errorf("EventBrokerURL = %q, want empty", cfg.EventBrokerURL)
	}
}

func TestParseConfigEnvOverrides(t *testing.T) {
	t.Setenv("MODULE09_PORT", "9999")
	t.Setenv("MODULE09_JWT_SECRET", "a-strong-secret-32-chars-long!!!")
	cfg := ParseConfig()
	if cfg.Port != 9999 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidateRejectsBadSecrets(t *testing.T) {
	for _, secret := range []string{"", "change-me-in-production"} {
		cfg := Config{JWTSecret: secret}
		if err := cfg.Validate(); err == nil {
			t.Errorf("secret %q should fail validation", secret)
		}
	}
}
