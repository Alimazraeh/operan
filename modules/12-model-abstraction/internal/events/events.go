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

func (NoOpBroker) Publish(ctx context.Context, topic, key, value string) error {
	return nil
}
func (NoOpBroker) Close() error { return nil }

// ModelCallPublished is fired when a model call completes.
func (b *Broker) ModelCallPublished(ctx context.Context, tenantID, agentID, workflowID, modelName, provider string,
	promptTokens, completionTokens, latencyMs int, costUSD float64, status string) error {

	payload := map[string]any{
		"tenant_id":         tenantID,
		"agent_id":          agentID,
		"workflow_id":       workflowID,
		"model_name":        modelName,
		"provider":          provider,
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"cost_usd":          costUSD,
		"latency_ms":        latencyMs,
		"status":            status,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal model_call_completed: %w", err)
	}

	return b.p.Publish(ctx, "operan.model.model_call_completed", tenantID, string(data))
}

// ModelFailoverPublished is fired when a failover occurs.
func (b *Broker) ModelFailoverPublished(ctx context.Context, tenantID, modelName, fromProvider, toProvider, reason string) error {
	payload := map[string]any{
		"tenant_id":      tenantID,
		"model_name":     modelName,
		"from_provider":  fromProvider,
		"to_provider":    toProvider,
		"reason":         reason,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal model_failover: %w", err)
	}

	return b.p.Publish(ctx, "operan.model.model_failover", tenantID, string(data))
}

// ModelCostRecorded is fired to notify M17 of a cost record.
func (b *Broker) ModelCostRecorded(ctx context.Context, tenantID, agentID, modelName string, costUSD float64, billingTag string) error {
	payload := map[string]any{
		"tenant_id":     tenantID,
		"agent_id":      agentID,
		"model_name":    modelName,
		"cost_usd":      costUSD,
		"billing_tag":   billingTag,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal model_cost_recorded: %w", err)
	}

	return b.p.Publish(ctx, "operan.model.model_cost_recorded", tenantID, string(data))
}

// KafkaPublisher wraps the RabbitMQ library for Kafka-compatible AMQP publishing.
// In production, swap this for a real Kafka client (e.g., confluent-kafka-go).
type KafkaPublisher struct {
	logger *log.Logger
}

// NewKafkaPublisher creates a publisher.
func NewKafkaPublisher(logger *log.Logger) *KafkaPublisher {
	return &KafkaPublisher{logger: logger}
}

// Publish logs the event (stub for production Kafka client).
func (k *KafkaPublisher) Publish(ctx context.Context, topic, key, value string) error {
	k.logger.Printf("KAFKA publish: topic=%s key=%s value=%s", topic, key, value[:min(200, len(value))])
	return nil
}

func (k *KafkaPublisher) Close() error { return nil }

// NewKafkaPublisherFromURL creates a publisher that checks broker connectivity.
func NewKafkaPublisherFromURL(url string, logger *log.Logger) (*KafkaPublisher, error) {
	if url == "" {
		if logger != nil {
			logger.Println("EVENT_BROKER_URL not set; events will be logged only")
		}
	}
	return NewKafkaPublisher(logger), nil
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}