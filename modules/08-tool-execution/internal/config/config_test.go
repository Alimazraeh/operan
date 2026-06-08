package config

import (
	"os"
	"testing"
)

func TestParseConfig_Defaults(t *testing.T) {
	os.Clearenv()
	c := ParseConfig()
	if c.Port != 8008 {
		t.Errorf("Port default = %d, want 8008", c.Port)
	}
	if c.MaxPageSize != 100 || c.DefaultTimeout != 30000 {
		t.Errorf("defaults wrong: %+v", c)
	}
}

func TestParseConfig_EnvOverride(t *testing.T) {
	os.Setenv("MODULE08_PORT", "9999")
	os.Setenv("MODULE08_MAX_PAGE_SIZE", "50")
	defer os.Clearenv()
	c := ParseConfig()
	if c.Port != 9999 || c.MaxPageSize != 50 {
		t.Errorf("env override failed: %+v", c)
	}
}

func TestValidate(t *testing.T) {
	if err := (&Config{JWTSecret: ""}).Validate(); err == nil {
		t.Error("empty secret should fail")
	}
	if err := (&Config{JWTSecret: "change-me-in-production"}).Validate(); err == nil {
		t.Error("default secret should fail")
	}
	if err := (&Config{JWTSecret: "a-real-secret"}).Validate(); err != nil {
		t.Errorf("valid secret should pass, got %v", err)
	}
}
