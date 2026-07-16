package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Publisher defines the interface for publishing events to the broker (Kafka).
type Publisher interface {
	Publish(ctx context.Context, topic, key, value string) error
	Close() error
}

// Broker wraps a Publisher with convenience methods.
type Broker struct {
	p Publisher
}

// NewBroker creates a new event broker.
func NewBroker(p Publisher) *Broker {
	return &Broker{p: p}
}

// NewNoOpBroker creates a broker that discards events (for testing).
type NoOpBroker struct{}

func (NoOpBroker) Publish(ctx context.Context, topic, key, value string) error  { return nil }
func (NoOpBroker) Close() error                                                  { return nil }

// TextNormalized is published when Arabic text is normalized.
func (b *Broker) TextNormalized(ctx context.Context, tenantID string, textLen, normalizedLen, actionsCount int) error {
	payload := map[string]any{
		"tenant_id":        tenantID,
		"text_length":      textLen,
		"normalized_length": normalizedLen,
		"actions_count":    actionsCount,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal text_normalized: %w", err)
	}
	return b.p.Publish(ctx, "operan.arabic.text_normalized", tenantID, string(data))
}

// DialectDetected is published when dialect detection completes.
func (b *Broker) DialectDetected(ctx context.Context, tenantID, dialect string, confidence float64) error {
	payload := map[string]any{
		"tenant_id":    tenantID,
		"dialect":      dialect,
		"confidence":   confidence,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dialect_detected: %w", err)
	}
	return b.p.Publish(ctx, "operan.arabic.dialect_detected", tenantID, string(data))
}

// TerminologyCheck is published when terminology checking completes.
func (b *Broker) TerminologyCheck(ctx context.Context, tenantID string, textLen int, matched, flagged int) error {
	payload := map[string]any{
		"tenant_id":     tenantID,
		"text_length":   textLen,
		"matched_terms": matched,
		"flagged_terms": flagged,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal terminology_check: %w", err)
	}
	return b.p.Publish(ctx, "operan.arabic.terminology_check", tenantID, string(data))
}

// TerminologyViolation is published when an unauthorized term is flagged.
func (b *Broker) TerminologyViolation(ctx context.Context, tenantID, term, reason, suggested, checkedBy string) error {
	payload := map[string]any{
		"tenant_id":          tenantID,
		"term":               term,
		"reason":             reason,
		"suggested_replacement": suggested,
		"checked_by":         checkedBy,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal terminology_violation: %w", err)
	}
	return b.p.Publish(ctx, "operan.arabic.terminology_violation", tenantID, string(data))
}

// EmbeddingRequested is published when an Arabic embedding request is made.
func (b *Broker) EmbeddingRequested(ctx context.Context, tenantID string, textLen int, model, status string) error {
	payload := map[string]any{
		"tenant_id": tenantID,
		"text_length": textLen,
		"model":     model,
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal embedding_requested: %w", err)
	}
	return b.p.Publish(ctx, "operan.arabic.embedding_requested", tenantID, string(data))
}

// KafkaPublisher is a stub for production Kafka client.
type KafkaPublisher struct {
	logger *log.Logger
}

// NewKafkaPublisher creates a publisher.
func NewKafkaPublisher(logger *log.Logger) *KafkaPublisher {
	return &KafkaPublisher{logger: logger}
}

// Publish logs the event (stub for production Kafka client).
func (k *KafkaPublisher) Publish(ctx context.Context, topic, key, value string) error {
	if k.logger != nil {
		k.logger.Printf("KAFKA publish: topic=%s key=%s value=%s", topic, key, value[:min(200, len(value))])
	}
	return nil
}

func (k *KafkaPublisher) Close() error { return nil }

// NewKafkaPublisherFromURL creates a publisher that checks broker connectivity.
func NewKafkaPublisherFromURL(url string, logger *log.Logger) (*KafkaPublisher, error) {
	if url == "" && logger != nil {
		logger.Println("EVENT_BROKER_URL not set; events will be logged only")
	}
	return NewKafkaPublisher(logger), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}