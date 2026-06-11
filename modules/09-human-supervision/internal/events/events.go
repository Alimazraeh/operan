// Package events publishes AsyncAPI events for Module 09 (Human Supervision).
// Topics follow the platform standard: operan.supervision.{entity}.{event}.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

const topicPrefix = "operan.supervision."

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

// Publisher publishes supervision lifecycle events.
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

// ─── Payloads (match asyncapi-09-human-supervision.yaml) ─────────────────────

// GateRaisedPayload matches the GateRaised message.
type GateRaisedPayload struct {
	GateID            string   `json:"gate_id"`
	TenantID          string   `json:"tenant_id"`
	WorkflowID        string   `json:"workflow_id"`
	NodeID            string   `json:"node_id"`
	GateType          string   `json:"gate_type"` // human_approval | human_input | human_review | human_override
	HumanApproverID   *string  `json:"human_approver_id"`
	EscalationReason  string   `json:"escalation_reason,omitempty"`
	RaisedBy          string   `json:"raised_by"`
	RaisedAt          string   `json:"raised_at"`
	Priority          string   `json:"priority,omitempty"` // low | medium | high | urgent
	ApprovalThreshold *float64 `json:"approval_threshold"`
}

// GateRespondedPayload matches the GateResponded message.
type GateRespondedPayload struct {
	ResponseID string                 `json:"response_id"`
	GateID     string                 `json:"gate_id"`
	TenantID   string                 `json:"tenant_id"`
	Response   string                 `json:"response"` // approve | reject | request_revision | escalate
	ResponseBy string                 `json:"response_by"`
	ResponseAt string                 `json:"response_at"`
	Comments   *string                `json:"comments"`
	Conditions map[string]interface{} `json:"conditions"`
}

// GateEscalatedPayload matches the GateEscalated message.
type GateEscalatedPayload struct {
	GateID             string `json:"gate_id"`
	TenantID           string `json:"tenant_id"`
	PreviousApproverID string `json:"previous_approver_id"`
	NewApproverID      string `json:"new_approver_id"`
	EscalationReason   string `json:"escalation_reason"`
	EscalatedAt        string `json:"escalated_at"`
	EscalationLevel    int    `json:"escalation_level"`
}

// GateTimeoutPayload matches the GateTimeout message.
type GateTimeoutPayload struct {
	GateID           string `json:"gate_id"`
	TenantID         string `json:"tenant_id"`
	TimeoutAction    string `json:"timeout_action"`
	TimedOutAt       string `json:"timed_out_at"`
	OriginalDeadline string `json:"original_deadline"`
}

// PolicyViolationDetectedPayload matches the PolicyViolationDetected message.
type PolicyViolationDetectedPayload struct {
	ViolationID   string                 `json:"violation_id"`
	TenantID      string                 `json:"tenant_id"`
	AgentID       string                 `json:"agent_id"`
	WorkflowID    string                 `json:"workflow_id"`
	PolicyID      string                 `json:"policy_id"`
	ViolationType string                 `json:"violation_type"`
	Severity      string                 `json:"severity"`
	Details       map[string]interface{} `json:"details"`
	DetectedAt    string                 `json:"detected_at"`
}

// ─── Typed publish methods ───────────────────────────────────────────────────

// PublishGateRaised emits operan.supervision.gate.raised.
func (p *Publisher) PublishGateRaised(pl GateRaisedPayload, correlationID string) error {
	return p.publish("gate.raised", pl.TenantID, correlationID, pl)
}

// PublishGateResponded emits operan.supervision.gate.responded.
func (p *Publisher) PublishGateResponded(pl GateRespondedPayload, correlationID string) error {
	return p.publish("gate.responded", pl.TenantID, correlationID, pl)
}

// PublishGateEscalated emits operan.supervision.gate.escalated.
func (p *Publisher) PublishGateEscalated(pl GateEscalatedPayload, correlationID string) error {
	return p.publish("gate.escalated", pl.TenantID, correlationID, pl)
}

// PublishGateTimeout emits operan.supervision.gate.timeout.
func (p *Publisher) PublishGateTimeout(pl GateTimeoutPayload, correlationID string) error {
	return p.publish("gate.timeout", pl.TenantID, correlationID, pl)
}

// PublishPolicyViolationDetected emits operan.supervision.policy.violation_detected.
func (p *Publisher) PublishPolicyViolationDetected(pl PolicyViolationDetectedPayload, correlationID string) error {
	return p.publish("policy.violation_detected", pl.TenantID, correlationID, pl)
}
