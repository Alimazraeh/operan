package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars to test defaults
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("ISSUER")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("EVENT_BROKER_URL")
	os.Unsetenv("M04_BASE_URL")

	cfg := Load()
	assert.Equal(t, "8010", cfg.HTTPPort)
	assert.Equal(t, "default-secret-key-for-development", cfg.JWTSecret)
	assert.Equal(t, "operan-tenant-control-plane", cfg.Issuer)
	assert.Equal(t, "postgres://postgres:password@localhost:5432/operan?sslmode=disable", cfg.DBDSN)
	assert.Equal(t, "localhost:9092", cfg.EventBrokerURL)
	assert.Equal(t, "http://localhost:8004", cfg.M04BaseURL)
}

func TestLoad_EnvVars(t *testing.T) {
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("JWT_SECRET", "my-secret")
	os.Setenv("ISSUER", "my-issuer")
	os.Setenv("DB_DSN", "postgres://test:test@localhost:5432/testdb")
	os.Setenv("EVENT_BROKER_URL", "kafka:9092")
	os.Setenv("M04_BASE_URL", "http://localhost:8004")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ISSUER")
		os.Unsetenv("DB_DSN")
		os.Unsetenv("EVENT_BROKER_URL")
		os.Unsetenv("M04_BASE_URL")
	}()

	cfg := Load()
	assert.Equal(t, "9090", cfg.HTTPPort)
	assert.Equal(t, "my-secret", cfg.JWTSecret)
	assert.Equal(t, "my-issuer", cfg.Issuer)
	assert.Equal(t, "postgres://test:test@localhost:5432/testdb", cfg.DBDSN)
	assert.Equal(t, "kafka:9092", cfg.EventBrokerURL)
	assert.Equal(t, "http://localhost:8004", cfg.M04BaseURL)
}

func TestValidate_Success(t *testing.T) {
	cfg := &Config{
		JWTSecret: "my-secret",
		DBDSN:     "postgres://test:test@localhost:5432/testdb",
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestValidate_DefaultSecret(t *testing.T) {
	cfg := &Config{
		JWTSecret: "default-secret-key-for-development",
		DBDSN:     "postgres://test:test@localhost:5432/testdb",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidate_MissingDSN(t *testing.T) {
	cfg := &Config{
		JWTSecret: "my-secret",
		DBDSN:     "",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN")
}