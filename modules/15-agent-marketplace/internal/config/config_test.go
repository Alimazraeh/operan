package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")

	c, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, 8015, c.HTTPPort)
	assert.Equal(t, "test-secret", c.JWTSecret)
	assert.NotEmpty(t, c.DBDSN)
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("HTTP_PORT", "9999")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("HTTP_PORT")

	c, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, 9999, c.HTTPPort)
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_EnvOverrides(t *testing.T) {
	os.Setenv("JWT_SECRET", "custom-secret")
	os.Setenv("HTTP_PORT", "8888")
	os.Setenv("M04_BASE_URL", "http://m04.custom")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("HTTP_PORT")
	defer os.Unsetenv("M04_BASE_URL")

	c, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "custom-secret", c.JWTSecret)
	assert.Equal(t, 8888, c.HTTPPort)
	assert.Equal(t, "http://m04.custom", c.M04BaseURL)
}

func TestEnvOr(t *testing.T) {
	os.Setenv("TEST_KEY", "set-value")
	defer os.Unsetenv("TEST_KEY")

	assert.Equal(t, "set-value", envOr("TEST_KEY", "fallback"))
	assert.Equal(t, "fallback", envOr("NONEXISTENT_KEY", "fallback"))
}