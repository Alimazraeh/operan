// Package events publishes AsyncAPI events for tenant lifecycle operations.
// Events are published to the configured event bus via the Broker
// abstraction: Kafka in production, log-only when no broker is configured.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Broker is the interface for event publishing.
type Broker interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error
	Close() error
}

// logBroker is the default no-op broker that logs events instead of publishing.
type logBroker struct{}

func (l *logBroker) Publish(_ context.Context, topic string, _, value []byte, _ map[string]string) error {
	log.Printf("[EVENT] %s: %s", topic, string(value))
	return nil
}

func (l *logBroker) Close() error { return nil }

// Publisher handles publishing tenant lifecycle events.
type Publisher struct {
	broker Broker
}

// NewPublisher creates a new event publisher with a log-only broker.
func NewPublisher() *Publisher {
	return &Publisher{broker: &logBroker{}}
}

// NewPublisherWithBroker creates a publisher backed by a real broker.
func NewPublisherWithBroker(broker Broker) *Publisher {
	return &Publisher{broker: broker}
}

// Close gracefully shuts down the broker.
func (p *Publisher) Close() error {
	if p.broker != nil {
		return p.broker.Close()
	}
	return nil
}

// ─── Event types ─────────────────────────────────────────────────────────────

// TenantProvisionedEvent is published when a tenant is successfully provisioned.
type TenantProvisionedEvent struct {
	TenantID   string                 `json:"tenant_id"`
	TenantName string                 `json:"tenant_name"`
	Plan       string                 `json:"plan"`
	Region     string                 `json:"region"`
	IsolationLevel string            `json:"isolation_level"`
	ProvisionedResources []string      `json:"provisioned_resources,omitempty"`
	Source     string                 `json:"source"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// TenantSuspendedEvent is published when a tenant is suspended.
type TenantSuspendedEvent struct {
	TenantID   string                 `json:"tenant_id"`
	TenantName string                 `json:"tenant_name"`
	Reason     string                 `json:"reason"`
	Source     string                 `json:"source"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// TenantDeprovisionedEvent is published when a tenant is fully deprovisioned.
type TenantDeprovisionedEvent struct {
	TenantID           string                 `json:"tenant_id"`
	TenantName         string                 `json:"tenant_name"`
	DeprovisionedAt    time.Time              `json:"deprovisioned_at"`
	Source             string                 `json:"source"`
	Timestamp          time.Time              `json:"timestamp"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// TenantQuotaExceededEvent is published when a tenant exceeds quota limits.
type TenantQuotaExceededEvent struct {
	TenantID   string                 `json:"tenant_id"`
	TenantName string                 `json:"tenant_name"`
	QuotaType  string                 `json:"quota_type"`
	Limit      int                    `json:"limit"`
	Current    int                    `json:"current"`
	Source     string                 `json:"source"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ─── Publish methods ─────────────────────────────────────────────────────────

// PublishTenantProvisioned emits a tenant.provisioned event.
func (p *Publisher) PublishTenantProvisioned(evt TenantProvisionedEvent) error {
	evt.Source = "tenant-control-plane"
	evt.Timestamp = time.Now()

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal tenant.provisioned event: %w", err)
	}

	p.publish("tenant.provisioned", data)
	return nil
}

// PublishTenantSuspended emits a tenant.suspended event.
func (p *Publisher) PublishTenantSuspended(evt TenantSuspendedEvent) error {
	evt.Source = "tenant-control-plane"
	evt.Timestamp = time.Now()

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal tenant.suspended event: %w", err)
	}

	p.publish("tenant.suspended", data)
	return nil
}

// PublishTenantDeprovisioned emits a tenant.deprovisioned event.
func (p *Publisher) PublishTenantDeprovisioned(evt TenantDeprovisionedEvent) error {
	evt.Source = "tenant-control-plane"
	evt.Timestamp = time.Now()

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal tenant.deprovisioned event: %w", err)
	}

	p.publish("tenant.deprovisioned", data)
	return nil
}

// PublishTenantQuotaExceeded emits a tenant.quota_exceeded event.
func (p *Publisher) PublishTenantQuotaExceeded(evt TenantQuotaExceededEvent) error {
	evt.Source = "tenant-control-plane"
	evt.Timestamp = time.Now()

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal tenant.quota_exceeded event: %w", err)
	}

	p.publish("tenant.quota_exceeded", data)
	return nil
}

// publish sends raw event data to the configured event bus.
func (p *Publisher) publish(topic string, data []byte) {
	if err := p.broker.Publish(context.Background(), topic, nil, data, nil); err != nil {
		log.Printf("[WARN] publish %s failed: %v", topic, err)
	}
}
