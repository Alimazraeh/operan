package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration for Module 11.
type Config struct {
	Port           int
	JWTSecret      string
	EventBrokerURL string
	OTLPEndpoint   string
	MaxPageSize    int
	ConsumerGroup  string
	ConsumeTopics  []string
}

// DefaultConsumeTopics is the platform topic set Module 11 ingests into
// traces and metrics when no MODULE11_CONSUME_TOPICS override is given.
// It covers every topic the implemented modules (01–08) publish.
var DefaultConsumeTopics = []string{
	// Module 01 — tenant control plane
	"operan.tenant.provisioned", "operan.tenant.suspended",
	"operan.tenant.deprovisioned", "operan.tenant.quota_exceeded",
	// Module 02 — identity & access
	"operan.iam.user.created", "operan.iam.user.updated", "operan.iam.user.suspended",
	"operan.iam.identity.rotated", "operan.iam.permission.granted", "operan.iam.permission.revoked",
	"operan.iam.session.created", "operan.iam.session.expired", "operan.iam.mfa.enrolled",
	"operan.iam.sso.login",
	// Module 04 — agent registry
	"operan.registry.agent.registered", "operan.registry.agent.capabilities_updated",
	"operan.registry.agent.version_created", "operan.registry.agent.promoted",
	"operan.registry.agent.deprecated", "operan.registry.agent.archived",
	"operan.registry.dependency.added", "operan.registry.dependency.removed",
	// Module 05 — department templates
	"operan.templates.template.created", "operan.templates.template.updated",
	"operan.templates.template.deployed", "operan.templates.template.deployment_failed",
	"operan.templates.template.undeployed", "operan.templates.template.deleted",
	"operan.templates.template.versioned", "operan.templates.template.cloned",
	// Module 07 — memory fabric
	"operan.memory.vector.ingested", "operan.memory.vector.searched",
	"operan.memory.vector.updated", "operan.memory.vector.deleted",
	"operan.memory.vector.garbage_collected",
	// Module 08 — tool execution
	"operan.tools.tool_registered", "operan.tools.tool_version_changed",
	"operan.tools.execution.requested", "operan.tools.execution.started",
	"operan.tools.execution.completed", "operan.tools.execution.failed",
	// Module 09 — human supervision
	"operan.supervision.gate.raised", "operan.supervision.gate.responded",
	"operan.supervision.gate.escalated", "operan.supervision.gate.timeout",
	"operan.supervision.policy.violation_detected",
}

// ParseConfig reads configuration from the environment, applying defaults.
func ParseConfig() Config {
	return Config{
		Port:           envInt("MODULE11_PORT", 8011),
		JWTSecret:      env("MODULE11_JWT_SECRET", "change-me-in-production"),
		EventBrokerURL: env("MODULE11_EVENT_BROKER_URL", ""),
		OTLPEndpoint:   env("MODULE11_OTLP_ENDPOINT", "http://localhost:4318"),
		MaxPageSize:    envInt("MODULE11_MAX_PAGE_SIZE", 100),
		ConsumerGroup:  env("MODULE11_CONSUMER_GROUP", "module11-observability"),
		ConsumeTopics:  envList("MODULE11_CONSUME_TOPICS", DefaultConsumeTopics),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var out []string
	for _, t := range strings.Split(v, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// Validate ensures required production settings are present.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("MODULE11_JWT_SECRET must be set")
	}
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE11_JWT_SECRET must be changed from default value in production")
	}
	return nil
}
