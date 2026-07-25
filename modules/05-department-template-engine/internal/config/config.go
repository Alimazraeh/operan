package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port             int
	DBURL            string
	RedisURL         string
	JWTSecret        string
	EventBrokerURL   string
	OTLPEndpoint     string
	TemplateCacheTTL int
	MaxPageSize      int
	DataDir          string // snapshot persistence directory (hostPath in k8s)
	SnapshotInterval int    // seconds between snapshots
	RegistryURL      string // Module 04 agent registry base URL (deploy orchestration)
	MemoryURL        string // Module 07 memory fabric base URL (deploy orchestration)
	OrchestrationURL string
	IdentityURL      string  // Module 02 — user lookup for seat binding
	CadenceHour      int     // local hour daily/weekly briefings fire
	CadenceTick      string  // e.g. "30s": test mode — fire every interval regardless of clock
	TokenRate        float64 // USD per 1M tokens for measured spend estimates
}

func ParseConfig() Config {
	return Config{
		Port:             envInt("MODULE05_PORT", 8005),
		DBURL:            env("MODULE05_DB_URL", ""),
		RedisURL:         env("MODULE05_REDIS_URL", ""),
		JWTSecret:        env("MODULE05_JWT_SECRET", "change-me-in-production"),
		EventBrokerURL:   env("MODULE05_EVENT_BROKER_URL", ""),
		OTLPEndpoint:     env("MODULE05_OTLP_ENDPOINT", "http://localhost:4318"),
		TemplateCacheTTL: envInt("MODULE05_TEMPLATE_CACHE_TTL", 300),
		MaxPageSize:      envInt("MODULE05_MAX_PAGE_SIZE", 100),
		DataDir:          env("MODULE05_DATA_DIR", "/data"),
		SnapshotInterval: envInt("MODULE05_SNAPSHOT_INTERVAL", 10),
		RegistryURL:      env("MODULE05_REGISTRY_URL", "http://agent-registry.operan.svc.cluster.local:8083"),
		MemoryURL:        env("MODULE05_MEMORY_URL", "http://memory-fabric.operan.svc.cluster.local:8007"),
		OrchestrationURL: env("MODULE05_ORCHESTRATION_URL", "http://agent-orchestration.operan.svc.cluster.local:8080/api/v1/orchestration"),
		IdentityURL:      env("MODULE05_IDENTITY_URL", "http://identity-access.operan.svc.cluster.local:8002/api/v1/iam"),
		CadenceHour:      envInt("MODULE05_CADENCE_HOUR", 7),
		CadenceTick:      env("MODULE05_CADENCE_TICK", ""),
		TokenRate:        envFloat("MODULE05_TOKEN_RATE", 3.0),
	}
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
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

func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("MODULE05_JWT_SECRET must be set")
	}
	if c.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("MODULE05_JWT_SECRET must be changed from default value in production")
	}
	return nil
}
