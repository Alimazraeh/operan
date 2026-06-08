// Package events publishes AsyncAPI events for Module 08 (Tool Execution).
// Topics:
//   operan/events/tools/tool_registered
//   operan/events/tools/tool_version_changed
//   operan/events/tools/execution/requested
//   operan/events/tools/execution/started
//   operan/events/tools/execution/completed
//   operan/events/tools/execution/failed
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

// Publisher publishes tool lifecycle and execution events.
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

func (p *Publisher) publish(topic string, payload interface{}) error {
	if p.broker == nil {
		return fmt.Errorf("broker not set")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event %s: %w", topic, err)
	}
	return p.broker.Publish(context.Background(), topic, nil, data, nil)
}

const base = "operan/events/tools"

// ─── Payloads ────────────────────────────────────────────────────────────────

// ToolRegisteredPayload is emitted when a tool is registered.
type ToolRegisteredPayload struct {
	Event     string    `json:"event"`
	ToolID    string    `json:"tool_id"`
	Name      string    `json:"name"`
	Category  string    `json:"category,omitempty"`
	Version   string    `json:"version"`
	TenantID  string    `json:"tenant_id"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ToolVersionChangedPayload is emitted when a tool's version changes.
type ToolVersionChangedPayload struct {
	Event           string    `json:"event"`
	ToolID          string    `json:"tool_id"`
	Version         string    `json:"version"`
	PreviousVersion string    `json:"previous_version,omitempty"`
	ChangeSummary   string    `json:"change_summary,omitempty"`
	TenantID        string    `json:"tenant_id"`
	ChangedAt       time.Time `json:"changed_at"`
}

// ExecutionPayload is emitted across the execution lifecycle.
type ExecutionPayload struct {
	Event           string                 `json:"event"`
	ExecutionID     string                 `json:"execution_id"`
	ToolID          string                 `json:"tool_id,omitempty"`
	Tool            string                 `json:"tool"`
	ToolVersion     string                 `json:"tool_version,omitempty"`
	AgentID         string                 `json:"agent_id"`
	TenantID        string                 `json:"tenant_id"`
	Status          string                 `json:"status"`
	Input           map[string]interface{} `json:"input,omitempty"`
	Output          map[string]interface{} `json:"output,omitempty"`
	ExecutionTimeMS int                    `json:"execution_time_ms,omitempty"`
	ErrorCode       string                 `json:"error_code,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	RetryCount      int                    `json:"retry_count"`
	Timestamp       time.Time              `json:"timestamp"`
}

// ─── Publish methods ─────────────────────────────────────────────────────────

func (p *Publisher) PublishToolRegistered(payload ToolRegisteredPayload) error {
	payload.Event = "tool_registered"
	return p.publish(base+"/tool_registered", payload)
}

func (p *Publisher) PublishToolVersionChanged(payload ToolVersionChangedPayload) error {
	payload.Event = "tool_version_changed"
	return p.publish(base+"/tool_version_changed", payload)
}

func (p *Publisher) PublishExecutionRequested(payload ExecutionPayload) error {
	payload.Event = "execution_requested"
	return p.publish(base+"/execution/requested", payload)
}

func (p *Publisher) PublishExecutionStarted(payload ExecutionPayload) error {
	payload.Event = "execution_started"
	return p.publish(base+"/execution/started", payload)
}

func (p *Publisher) PublishExecutionCompleted(payload ExecutionPayload) error {
	payload.Event = "execution_completed"
	return p.publish(base+"/execution/completed", payload)
}

func (p *Publisher) PublishExecutionFailed(payload ExecutionPayload) error {
	payload.Event = "execution_failed"
	return p.publish(base+"/execution/failed", payload)
}
