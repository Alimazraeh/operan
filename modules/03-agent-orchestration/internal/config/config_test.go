package config

import (
	"os"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	// Clear any env vars that might interfere
	for _, key := range []string{"LISTEN_ADDR", "OTLP_ENDPOINT", "LOG_ENV", "MODULE_VERSION",
		"EVENT_BUS_HOST", "EVENT_BUS_PORT", "EVENT_BUS_PROTO", "EVENT_BUS_TLS",
		"EVENT_BUS_SASL", "EVENT_BUS_USER", "EVENT_BUS_PASS", "JWT_SECRET"} {
		os.Unsetenv(key)
	}

	cfg := ParseConfig()

	if cfg.ListenAddr != DefaultListenAddr {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, DefaultListenAddr)
	}
	if cfg.EventBusHost != DefaultEventBusHost {
		t.Errorf("EventBusHost = %q, want %q", cfg.EventBusHost, DefaultEventBusHost)
	}
	if cfg.EventBusPort != DefaultEventBusPort {
		t.Errorf("EventBusPort = %q, want %q", cfg.EventBusPort, DefaultEventBusPort)
	}
	if cfg.EventBusProto != DefaultEventBusProto {
		t.Errorf("EventBusProto = %q, want %q", cfg.EventBusProto, DefaultEventBusProto)
	}
	if cfg.EventBusTLS != DefaultEventBusTLS {
		t.Errorf("EventBusTLS = %v, want %v", cfg.EventBusTLS, DefaultEventBusTLS)
	}
	if cfg.EventBusSASL != DefaultEventBusSASL {
		t.Errorf("EventBusSASL = %v, want %v", cfg.EventBusSASL, DefaultEventBusSASL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestParseConfigEnvOverrides(t *testing.T) {
	// Clear all relevant env vars
	for _, key := range []string{"LISTEN_ADDR", "OTLP_ENDPOINT", "LOG_ENV", "MODULE_VERSION",
		"EVENT_BUS_HOST", "EVENT_BUS_PORT", "EVENT_BUS_PROTO", "EVENT_BUS_TLS",
		"EVENT_BUS_SASL", "EVENT_BUS_USER", "EVENT_BUS_PASS", "JWT_SECRET"} {
		os.Unsetenv(key)
	}

	os.Setenv("LISTEN_ADDR", ":9090")
	os.Setenv("EVENT_BUS_HOST", "kafka.events.prod")
	os.Setenv("EVENT_BUS_PORT", "9093")
	os.Setenv("EVENT_BUS_PROTO", "kafka")
	os.Setenv("EVENT_BUS_TLS", "true")
	os.Setenv("EVENT_BUS_SASL", "true")
	os.Setenv("EVENT_BUS_USER", "kafka-user")
	os.Setenv("EVENT_BUS_PASS", "kafka-pass")
	os.Setenv("LOG_ENV", "debug")
	os.Setenv("JWT_SECRET", "secure-secret")

	cfg := ParseConfig()

	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.EventBusHost != "kafka.events.prod" {
		t.Errorf("EventBusHost = %q, want %q", cfg.EventBusHost, "kafka.events.prod")
	}
	if cfg.EventBusPort != "9093" {
		t.Errorf("EventBusPort = %q, want %q", cfg.EventBusPort, "9093")
	}
	if !cfg.EventBusTLS {
		t.Error("EventBusTLS = false, want true")
	}
	if !cfg.EventBusSASL {
		t.Error("EventBusSASL = false, want true")
	}
	if cfg.EventBusUser != "kafka-user" {
		t.Errorf("EventBusUser = %q, want %q", cfg.EventBusUser, "kafka-user")
	}
	if cfg.EventBusPass != "kafka-pass" {
		t.Errorf("EventBusPass = %q, want %q", cfg.EventBusPass, "kafka-pass")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback bool
		want     bool
	}{
		{"true string", "true", false, true},
		{"1 string", "1", false, true},
		{"yes string", "yes", false, true},
		{"false string", "false", true, false},
		{"0 string", "0", true, false},
		{"no string", "no", true, false},
		{"empty falls back", "", true, true},
		{"invalid falls back", "invalid", false, false},
		{"TRUE uppercase", "TRUE", false, false}, // case-sensitive, expects lowercase
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("TEST_BOOL")
			if tt.value != "" {
				os.Setenv("TEST_BOOL", tt.value)
			}
			got := getEnvBool("TEST_BOOL", tt.fallback)
			if got != tt.want {
				t.Errorf("getEnvBool(\"%s\", %v) = %v, want %v", tt.value, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		jwt      string
		wantErr  bool
	}{
		{"default jwt fails", DefaultJWTSecret, true},
		{"secure jwt passes", "my-secure-jwt-key", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{JWTSecret: tt.jwt}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
