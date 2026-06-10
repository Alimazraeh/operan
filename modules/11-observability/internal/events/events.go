// Package events publishes AsyncAPI events for Module 11 (Observability).
// Topics follow the platform standard: operan.observability.{entity}.{event}.
// Per the AsyncAPI contract, the message envelope (correlationId, tenantId,
// messageId, timestamp) is carried in message headers, not the payload.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

const topicPrefix = "operan.observability."

// Broker is the interface for event publishing.
type Broker interface {
	Publish(ctx context.Context, topic string, key, value []byte, headers map[string]string) error
	Close() error
}

// logBroker is the default no-op broker that logs events instead of publishing.
type logBroker struct{}

func (l *logBroker) Publish(_ context.Context, topic string, _, value []byte, _ map[string]string) error {
	log.Printf("[EVENT] %s: %s", topic, string(value))
	return nil
}

func (l *logBroker) Close() error { return nil }

// Publisher publishes observability lifecycle events.
type Publisher struct {
	broker Broker
}

// NewPublisher creates a publisher with a log-only broker.
func NewPublisher() *Publisher { return &Publisher{broker: &logBroker{}} }

// NewPublisherWithBroker creates a publisher backed by a real broker.
func NewPublisherWithBroker(b Broker) *Publisher { return &Publisher{broker: b} }

// SetBroker replaces the underlying broker.
func (p *Publisher) SetBroker(b Broker) { p.broker = b }

// Close shuts down the broker.
func (p *Publisher) Close() error {
	if p.broker != nil {
		return p.broker.Close()
	}
	return nil
}

func (p *Publisher) publish(topic, tenantID, correlationID string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", topic, err)
	}
	if correlationID == "" {
		correlationID = uuid.New().String()
	}
	headers := map[string]string{
		"correlationId": correlationID,
		"tenantId":      tenantID,
		"messageId":     uuid.New().String(),
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"content-type":  "application/json",
	}
	if err := p.broker.Publish(context.Background(), topicPrefix+topic, []byte(tenantID), data, headers); err != nil {
		log.Printf("[WARN] publish %s%s failed: %v", topicPrefix, topic, err)
		return err
	}
	return nil
}

// ─── Payloads (match asyncapi-11-observability.yaml) ─────────────────────────

// MetricRecordedPayload matches the MetricRecorded message.
type MetricRecordedPayload struct {
	MetricID    string                 `json:"metric_id"`
	TenantID    string                 `json:"tenant_id"`
	MetricName  string                 `json:"metric_name"`
	MetricValue float64                `json:"metric_value"`
	MetricType  string                 `json:"metric_type"`
	Labels      map[string]interface{} `json:"labels"`
	SourceID    string                 `json:"source_id"`
	RecordedAt  string                 `json:"recorded_at"`
}

// TraceSpanPayload matches the TraceSpan message.
type TraceSpanPayload struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	TenantID   string                 `json:"tenant_id"`
	WorkflowID string                 `json:"workflow_id"`
	AgentID    string                 `json:"agent_id"`
	SpanName   string                 `json:"span_name"`
	SpanType   string                 `json:"span_type"`
	StartTime  string                 `json:"start_time"`
	EndTime    string                 `json:"end_time"`
	DurationMs int                    `json:"duration_ms"`
	Status     string                 `json:"status"`
	Tags       map[string]interface{} `json:"tags"`
}

// TraceFlushPayload matches the TraceFlush message.
type TraceFlushPayload struct {
	TenantID  string `json:"tenant_id"`
	FlushID   string `json:"flush_id"`
	SpanCount int    `json:"span_count"`
	FlushedAt string `json:"flushed_at"`
}

// AlertFiredPayload matches the AlertFired message.
type AlertFiredPayload struct {
	AlertID              string  `json:"alert_id"`
	TenantID             string  `json:"tenant_id"`
	AlertName            string  `json:"alert_name"`
	Severity             string  `json:"severity"`
	ConditionDescription string  `json:"condition_description"`
	CurrentValue         float64 `json:"current_value"`
	Threshold            float64 `json:"threshold"`
	TriggeredAt          string  `json:"triggered_at"`
	ResolvedAt           *string `json:"resolved_at"`
}

// HealthStatusChangePayload matches the HealthStatusChange message.
type HealthStatusChangePayload struct {
	TenantID       string `json:"tenant_id"`
	ComponentID    string `json:"component_id"`
	ComponentType  string `json:"component_type"`
	PreviousStatus string `json:"previous_status"`
	NewStatus      string `json:"new_status"`
	Reason         string `json:"reason"`
	ChangedAt      string `json:"changed_at"`
}

// ─── Typed publish methods ───────────────────────────────────────────────────

// PublishMetricRecorded emits operan.observability.metric.recorded.
func (p *Publisher) PublishMetricRecorded(pl MetricRecordedPayload, correlationID string) error {
	return p.publish("metric.recorded", pl.TenantID, correlationID, pl)
}

// PublishTraceSpan emits operan.observability.trace.span.
func (p *Publisher) PublishTraceSpan(pl TraceSpanPayload, correlationID string) error {
	return p.publish("trace.span", pl.TenantID, correlationID, pl)
}

// PublishTraceFlush emits operan.observability.trace.flush.
func (p *Publisher) PublishTraceFlush(pl TraceFlushPayload, correlationID string) error {
	return p.publish("trace.flush", pl.TenantID, correlationID, pl)
}

// PublishAlertFired emits operan.observability.alert.fired.
func (p *Publisher) PublishAlertFired(pl AlertFiredPayload, correlationID string) error {
	return p.publish("alert.fired", pl.TenantID, correlationID, pl)
}

// PublishHealthStatusChange emits operan.observability.health.status_change.
func (p *Publisher) PublishHealthStatusChange(pl HealthStatusChangePayload, correlationID string) error {
	return p.publish("health.status_change", pl.TenantID, correlationID, pl)
}
