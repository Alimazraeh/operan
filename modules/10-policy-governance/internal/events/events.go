package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

// Publisher publishes policy events to Kafka.
type Publisher struct {
	writer *kafka.Writer
}

// EvaluationEvent represents a policy evaluation event.
type EvaluationEvent struct {
	TenantID     string  `json:"tenant_id"`
	RequestID    string  `json:"request_id"`
	AgentID      string  `json:"agent_id"`
	Resource     string  `json:"resource"`
	ActionType   string  `json:"action_type"`
	Result       string  `json:"result"`
	PolicyName   string  `json:"policy_name"`
	EvaluationMS int64   `json:"evaluation_ms"`
	DataClass    string  `json:"data_class,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
}

// ViolationEvent represents a policy violation event.
type ViolationEvent struct {
	TenantID   string `json:"tenant_id"`
	RequestID  string `json:"request_id"`
	AgentID    string `json:"agent_id"`
	Resource   string `json:"resource"`
	ActionType string `json:"action_type"`
	PolicyName string `json:"policy_name"`
	Result     string `json:"result"`
}

// PolicyUpdatedEvent represents a policy update event.
type PolicyUpdatedEvent struct {
	TenantID  string `json:"tenant_id"`
	PolicyID  string `json:"policy_id"`
	Name      string `json:"name"`
	UpdatedBy string `json:"updated_by"`
}

// GroupUpdatedEvent represents a policy group update event.
type GroupUpdatedEvent struct {
	TenantID  string `json:"tenant_id"`
	GroupID   string `json:"group_id"`
	Name      string `json:"name"`
	UpdatedBy string `json:"updated_by"`
}

// NewPublisher creates a new Kafka event publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokerURL),
			Balancer:     &kafka.LeastBytes{},
			Async:        true,
			Completion:   func(messages []kafka.Message, err error) {
				if err != nil {
					log.Printf("kafka publish error: %v", err)
				}
			},
		},
	}
}

// PublishEvaluation publishes a policy evaluation event.
func (p *Publisher) PublishEvaluation(ctx context.Context, event EvaluationEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal evaluation event: %v", err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(event.TenantID),
		Value: data,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("failed to publish evaluation event: %v", err)
	}
}

// PublishViolation publishes a policy violation event.
func (p *Publisher) PublishViolation(ctx context.Context, event ViolationEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal violation event: %v", err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(event.TenantID),
		Value: data,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("failed to publish violation event: %v", err)
	}
}

// PublishPolicyUpdated publishes a policy update event.
func (p *Publisher) PublishPolicyUpdated(ctx context.Context, event PolicyUpdatedEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal policy updated event: %v", err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(event.TenantID),
		Value: data,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("failed to publish policy updated event: %v", err)
	}
}

// PublishGroupUpdated publishes a policy group update event.
func (p *Publisher) PublishGroupUpdated(ctx context.Context, event GroupUpdatedEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal group updated event: %v", err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(event.TenantID),
		Value: data,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("failed to publish group updated event: %v", err)
	}
}

// Close closes the Kafka writer.
func (p *Publisher) Close() error {
	return p.writer.Close()
}

// IsHealthy checks if the Kafka broker is reachable.
func (p *Publisher) IsHealthy(ctx context.Context) error {
	if p.writer == nil {
		return fmt.Errorf("kafka writer not initialized")
	}
	return nil
}