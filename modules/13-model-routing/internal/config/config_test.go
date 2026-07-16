package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultPort(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Unsetenv("PORT")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, 8013, cfg.Port)
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("PORT", "9999")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
	}()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, 9999, cfg.Port)
}

func TestLoad_InvalidPort(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("PORT", "not-a-number")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
	}()

	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("PORT")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET must be set")
}

func TestLoad_WithAllEnv(t *testing.T) {
	os.Setenv("JWT_SECRET", "my-secret")
	os.Setenv("PORT", "8013")
	os.Setenv("M12_BASE_URL", "http://m12:8012")
	os.Setenv("DB_DSN", "postgres://localhost/testdb")
	os.Setenv("EVENT_BROKER_URL", "kafka://localhost:9092")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
		os.Unsetenv("M12_BASE_URL")
		os.Unsetenv("DB_DSN")
		os.Unsetenv("EVENT_BROKER_URL")
	}()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "my-secret", cfg.JWTSecret)
	assert.Equal(t, "http://m12:8012", cfg.M12BaseURL)
	assert.Equal(t, "postgres://localhost/testdb", cfg.DBDSN)
	assert.Equal(t, "kafka://localhost:9092", cfg.EventBrokerURL)
}

func TestLoad_FailClosed(t *testing.T) {
	// Verify fail-closed: empty string is not accepted
	os.Setenv("JWT_SECRET", "")
	defer os.Unsetenv("JWT_SECRET")

	_, err := Load()
	assert.Error(t, err)
}