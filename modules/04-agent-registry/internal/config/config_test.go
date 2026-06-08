// Package config tests for the Agent Registry module.
package config

import (
	"os"
	"testing"
)

func TestParseConfig_Defaults(t *testing.T) {
	// Ensure all env vars are unset.
	keys := []string{
		"AGENT_REGISTRY_PORT",
		"AGENT_REGISTRY_MODULE_ID",
		"JWT_SECRET",
		"EVENT_BUS_HOST",
		"EVENT_BUS_PORT",
		"EVENT_BUS_PROTO",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_MAX_OPEN",
		"DB_MAX_IDLE",
		"OTLP_ENDPOINT",
		"LOG_LEVEL",
		"AGENT_REGISTRY_DATABASE_DSN",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}

	cfg, err := ParseConfig()
	if err != nil {
		// Expected error because JWT_SECRET defaults to "change-me-in-production"
		// and Validate() rejects it. But we can still check the returned config.
		t.Logf("ParseConfig returned error (expected): %v", err)
	}

	if cfg.ListenAddr != DefaultListenAddr {
		t.Errorf("expected ListenAddr %q, got %q", DefaultListenAddr, cfg.ListenAddr)
	}
	if cfg.ModuleID != DefaultModuleID {
		t.Errorf("expected ModuleID %q, got %q", DefaultModuleID, cfg.ModuleID)
	}
	if cfg.JWTSecret != DefaultJWTSecret {
		t.Errorf("expected JWTSecret %q, got %q", DefaultJWTSecret, cfg.JWTSecret)
	}
	if cfg.EventBusHost != DefaultEventBusHost {
		t.Errorf("expected EventBusHost %q, got %q", DefaultEventBusHost, cfg.EventBusHost)
	}
	if cfg.EventBusPort != DefaultEventBusPort {
		t.Errorf("expected EventBusPort %q, got %q", DefaultEventBusPort, cfg.EventBusPort)
	}
	if cfg.EventBusProto != DefaultEventBusProto {
		t.Errorf("expected EventBusProto %q, got %q", DefaultEventBusProto, cfg.EventBusProto)
	}
	if cfg.DBHost != DefaultDBHost {
		t.Errorf("expected DBHost %q, got %q", DefaultDBHost, cfg.DBHost)
	}
	if cfg.DBPort != DefaultDBPort {
		t.Errorf("expected DBPort %q, got %q", DefaultDBPort, cfg.DBPort)
	}
	if cfg.DBUser != DefaultDBUser {
		t.Errorf("expected DBUser %q, got %q", DefaultDBUser, cfg.DBUser)
	}
	if cfg.DBPassword != DefaultDBPassword {
		t.Errorf("expected DBPassword %q, got %q", DefaultDBPassword, cfg.DBPassword)
	}
	if cfg.DBName != DefaultDBName {
		t.Errorf("expected DBName %q, got %q", DefaultDBName, cfg.DBName)
	}
	if cfg.DBMaxOpen != DefaultDBMaxOpen {
		t.Errorf("expected DBMaxOpen %d, got %d", DefaultDBMaxOpen, cfg.DBMaxOpen)
	}
	if cfg.DBMaxIdle != DefaultDBMaxIdle {
		t.Errorf("expected DBMaxIdle %d, got %d", DefaultDBMaxIdle, cfg.DBMaxIdle)
	}
	if cfg.OTLPEndpoint != DefaultOTLPEndpoint {
		t.Errorf("expected OTLPEndpoint %q, got %q", DefaultOTLPEndpoint, cfg.OTLPEndpoint)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("expected LogLevel %q, got %q", DefaultLogLevel, cfg.LogLevel)
	}
}

func TestParseConfig_EnvOverrides(t *testing.T) {
	os.Setenv("AGENT_REGISTRY_PORT", ":9090")
	os.Setenv("AGENT_REGISTRY_MODULE_ID", "test-module")
	os.Setenv("JWT_SECRET", "custom-secret")
	os.Setenv("EVENT_BUS_HOST", "custom-host")
	os.Setenv("EVENT_BUS_PORT", "9999")
	os.Setenv("DB_HOST", "custom-db")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_NAME", "custom_db")
	os.Setenv("DB_PASSWORD", "custom-db-pass")
	os.Setenv("DB_MAX_OPEN", "50")
	os.Setenv("DB_MAX_IDLE", "10")
	os.Setenv("OTLP_ENDPOINT", "http://custom:4319")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("AGENT_REGISTRY_DATABASE_DSN", "custom-dsn")

	cfg, err := ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig returned unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":9090" {
		t.Errorf("expected ListenAddr :9090, got %q", cfg.ListenAddr)
	}
	if cfg.ModuleID != "test-module" {
		t.Errorf("expected ModuleID test-module, got %q", cfg.ModuleID)
	}
	if cfg.JWTSecret != "custom-secret" {
		t.Errorf("expected JWTSecret custom-secret, got %q", cfg.JWTSecret)
	}
	if cfg.EventBusHost != "custom-host" {
		t.Errorf("expected EventBusHost custom-host, got %q", cfg.EventBusHost)
	}
	if cfg.EventBusPort != "9999" {
		t.Errorf("expected EventBusPort 9999, got %q", cfg.EventBusPort)
	}
	if cfg.DBHost != "custom-db" {
		t.Errorf("expected DBHost custom-db, got %q", cfg.DBHost)
	}
	if cfg.DBPort != "5433" {
		t.Errorf("expected DBPort 5433, got %q", cfg.DBPort)
	}
	if cfg.DBName != "custom_db" {
		t.Errorf("expected DBName custom_db, got %q", cfg.DBName)
	}
	if cfg.DBMaxOpen != 50 {
		t.Errorf("expected DBMaxOpen 50, got %d", cfg.DBMaxOpen)
	}
	if cfg.DBMaxIdle != 10 {
		t.Errorf("expected DBMaxIdle 10, got %d", cfg.DBMaxIdle)
	}
	if cfg.OTLPEndpoint != "http://custom:4319" {
		t.Errorf("expected OTLPEndpoint http://custom:4319, got %q", cfg.OTLPEndpoint)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug, got %q", cfg.LogLevel)
	}
	if cfg.DatabaseDSN != "custom-dsn" {
		t.Errorf("expected DatabaseDSN custom-dsn, got %q", cfg.DatabaseDSN)
	}

	// Cleanup
	os.Unsetenv("AGENT_REGISTRY_PORT")
	os.Unsetenv("AGENT_REGISTRY_MODULE_ID")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("EVENT_BUS_HOST")
	os.Unsetenv("EVENT_BUS_PORT")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_MAX_OPEN")
	os.Unsetenv("DB_MAX_IDLE")
	os.Unsetenv("OTLP_ENDPOINT")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("AGENT_REGISTRY_DATABASE_DSN")
}

func TestValidate_EmptyModuleID(t *testing.T) {
	cfg := Config{ModuleID: ""}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty ModuleID")
	}
	if err.Error() != "module ID is required" {
		t.Errorf("expected 'module ID is required', got %q", err.Error())
	}
}

func TestValidate_DefaultJWTSecret(t *testing.T) {
	cfg := Config{ModuleID: "test", JWTSecret: DefaultJWTSecret}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for default JWT_SECRET")
	}
	if err.Error() != "JWT_SECRET must be changed from default value in production" {
		t.Errorf("expected 'JWT_SECRET must be changed from default value in production', got %q", err.Error())
	}
}

func TestValidate_DefaultDBPassword(t *testing.T) {
	cfg := Config{ModuleID: "test", JWTSecret: "custom-secret", DBPassword: DefaultDBPassword}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for default DB_PASSWORD")
	}
	if err.Error() != "DB_PASSWORD is set to default value; set via DB_PASSWORD env var" {
		t.Errorf("expected 'DB_PASSWORD is set to default value; set via DB_PASSWORD env var', got %q", err.Error())
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := Config{ModuleID: "test", JWTSecret: "my-secret", DBPassword: "custom-db-pass"}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetEnvIntOrDefault_Invalid(t *testing.T) {
	os.Setenv("DB_MAX_OPEN", "not-a-number")
	v := getEnvIntOrDefault("DB_MAX_OPEN", 25)
	if v != 25 {
		t.Errorf("expected 25 for invalid int, got %d", v)
	}
	os.Unsetenv("DB_MAX_OPEN")
}

func TestGetEnvIntOrDefault_Empty(t *testing.T) {
	os.Unsetenv("DB_MAX_OPEN")
	v := getEnvIntOrDefault("DB_MAX_OPEN", 25)
	if v != 25 {
		t.Errorf("expected 25 for empty env, got %d", v)
	}
}

func TestGetEnvIntOrDefault_Valid(t *testing.T) {
	os.Setenv("DB_MAX_OPEN", "100")
	v := getEnvIntOrDefault("DB_MAX_OPEN", 25)
	if v != 100 {
		t.Errorf("expected 100 for valid env, got %d", v)
	}
	os.Unsetenv("DB_MAX_OPEN")
}
