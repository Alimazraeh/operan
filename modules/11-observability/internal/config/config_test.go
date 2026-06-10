package config

import "testing"

func TestParseConfigDefaults(t *testing.T) {
	cfg := ParseConfig()
	if cfg.Port != 8011 {
		t.Errorf("Port = %d, want 8011", cfg.Port)
	}
	if cfg.ConsumerGroup != "module11-observability" {
		t.Errorf("ConsumerGroup = %q", cfg.ConsumerGroup)
	}
	if len(cfg.ConsumeTopics) != len(DefaultConsumeTopics) {
		t.Errorf("ConsumeTopics = %d, want %d", len(cfg.ConsumeTopics), len(DefaultConsumeTopics))
	}
}

func TestParseConfigTopicOverride(t *testing.T) {
	t.Setenv("MODULE11_CONSUME_TOPICS", "a.topic, b.topic ,")
	cfg := ParseConfig()
	if len(cfg.ConsumeTopics) != 2 || cfg.ConsumeTopics[0] != "a.topic" || cfg.ConsumeTopics[1] != "b.topic" {
		t.Errorf("ConsumeTopics = %v", cfg.ConsumeTopics)
	}
}

func TestDefaultTopicsAreValidKafkaNames(t *testing.T) {
	for _, topic := range DefaultConsumeTopics {
		for _, r := range topic {
			valid := r == '.' || r == '_' || r == '-' ||
				(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			if !valid {
				t.Errorf("topic %q contains invalid Kafka character %q", topic, r)
			}
		}
	}
}

func TestValidateRejectsDefaultSecret(t *testing.T) {
	cfg := Config{JWTSecret: "change-me-in-production"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for default JWT secret")
	}
	cfg = Config{JWTSecret: ""}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty JWT secret")
	}
	cfg = Config{JWTSecret: "a-strong-secret-32-chars-long!!!"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("custom secret should pass: %v", err)
	}
}
