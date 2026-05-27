// Package events publishes AsyncAPI events for orchestration lifecycle operations.
// Events are published to the configured event bus (Kafka/Pulsar) via
// the Publisher abstraction. This is a reference implementation that
// logs events; production should use a real message broker.
package events

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Publisher handles publishing orchestration lifecycle events.
type Publisher struct{}

// NewPublisher creates a new event publisher.
func NewPublisher() *Publisher {
	return &Publisher{}
}

// ─── Event payloads ──────────────────────────────────────────────────────────

// WorkflowCreatedPayload matches the AsyncAPI WorkflowCreatedPayload schema.
type WorkflowCreatedPayload struct {
	WorkflowID   string                 `json:"workflow_id"`
	TenantID     string                 `json:"tenant_id"`
	DepartmentID string                 `json:"department_id"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	CreatedBy    string                 `json:"created_by"`
	CreatedAt    time.Time              `json:"created_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowStartedPayload matches the AsyncAPI WorkflowStartedPayload schema.
type WorkflowStartedPayload struct {
	WorkflowID    string    `json:"workflow_id"`
	StartedBy     string    `json:"started_by"`
	StartedAt     time.Time `json:"started_at"`
	InitialNodes  []string  `json:"initial_nodes"`
}

// WorkflowPausedPayload matches the AsyncAPI WorkflowPausedPayload schema.
type WorkflowPausedPayload struct {
	WorkflowID string    `json:"workflow_id"`
	PausedBy   string    `json:"paused_by"`
	PausedAt   time.Time `json:"paused_at"`
	Reason     string    `json:"reason"`
}

// WorkflowResumedPayload matches the AsyncAPI WorkflowResumedPayload schema.
type WorkflowResumedPayload struct {
	WorkflowID string    `json:"workflow_id"`
	ResumedBy  string    `json:"resumed_by"`
	ResumedAt  time.Time `json:"resumed_at"`
}

// WorkflowCompletedPayload matches the AsyncAPI WorkflowCompletedPayload schema.
type WorkflowCompletedPayload struct {
	WorkflowID    string                 `json:"workflow_id"`
	CompletedAt   time.Time              `json:"completed_at"`
	DurationMs    int                    `json:"duration_ms"`
	FinalStatus   string                 `json:"final_status"`
	Outcome       map[string]interface{} `json:"outcome,omitempty"`
}

// WorkflowFailedPayload matches the AsyncAPI WorkflowFailedPayload schema.
type WorkflowFailedPayload struct {
	WorkflowID     string    `json:"workflow_id"`
	FailedAt       time.Time `json:"failed_at"`
	ErrorCode      string    `json:"error_code"`
	ErrorMessage   string    `json:"error_message"`
	FailedNodeID   string    `json:"failed_node_id"`
}

// WorkflowCancelledPayload matches the AsyncAPI WorkflowCancelledPayload schema.
type WorkflowCancelledPayload struct {
	WorkflowID        string    `json:"workflow_id"`
	CancelledBy       string    `json:"cancelled_by"`
	CancelledAt       time.Time `json:"cancelled_at"`
	CancellationReason string   `json:"cancellation_reason"`
}

// WorkflowCheckpointedPayload matches the AsyncAPI WorkflowCheckpointedPayload schema.
type WorkflowCheckpointedPayload struct {
	WorkflowID        string    `json:"workflow_id"`
	CheckpointID      string    `json:"checkpoint_id"`
	NodeID            string    `json:"node_id"`
	Timestamp         time.Time `json:"timestamp"`
	StateSnapshotSize int       `json:"state_snapshot_size"`
}

// WorkflowReplayedPayload matches the AsyncAPI WorkflowReplayedPayload schema.
type WorkflowReplayedPayload struct {
	WorkflowID       string    `json:"workflow_id"`
	ReplayID         string    `json:"replay_id"`
	FromCheckpointID string    `json:"from_checkpoint_id"`
	ReplayedBy       string    `json:"replayed_by"`
	StartedAt        time.Time `json:"started_at"`
}

// ScheduleTriggeredPayload matches the AsyncAPI ScheduleTriggeredPayload schema.
type ScheduleTriggeredPayload struct {
	ScheduleID      string    `json:"schedule_id"`
	WorkflowID      string    `json:"workflow_id"`
	TriggeredBy     string    `json:"triggered_by"`
	CronExpression  string    `json:"cron_expression,omitempty"`
	NextRunAt       time.Time `json:"next_run_at"`
}

// AgentAssignedPayload matches the AsyncAPI AgentAssignedPayload schema.
type AgentAssignedPayload struct {
	AssignmentID string                 `json:"assignment_id"`
	WorkflowID   string                 `json:"workflow_id"`
	NodeID       string                 `json:"node_id"`
	AgentID      string                 `json:"agent_id"`
	AssignedAt   time.Time              `json:"assigned_at"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

// AgentUnavailablePayload matches the AsyncAPI AgentUnavailablePayload schema.
type AgentUnavailablePayload struct {
	AgentID            string    `json:"agent_id"`
	Reason             string    `json:"reason"`
	AffectedWorkflows  []string  `json:"affected_workflows"`
	DetectedAt         time.Time `json:"detected_at"`
}

// ─── Publish methods ─────────────────────────────────────────────────────────

// PublishWorkflowCreated emits an orchestration.workflow.created event.
func (p *Publisher) PublishWorkflowCreated(payload WorkflowCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.created event: %w", err)
	}
	p.publish("orchestration.workflow.created", data)
	return nil
}

// PublishWorkflowStarted emits an orchestration.workflow.started event.
func (p *Publisher) PublishWorkflowStarted(payload WorkflowStartedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.started event: %w", err)
	}
	p.publish("orchestration.workflow.started", data)
	return nil
}

// PublishWorkflowPaused emits an orchestration.workflow.paused event.
func (p *Publisher) PublishWorkflowPaused(payload WorkflowPausedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.paused event: %w", err)
	}
	p.publish("orchestration.workflow.paused", data)
	return nil
}

// PublishWorkflowResumed emits an orchestration.workflow.resumed event.
func (p *Publisher) PublishWorkflowResumed(payload WorkflowResumedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.resumed event: %w", err)
	}
	p.publish("orchestration.workflow.resumed", data)
	return nil
}

// PublishWorkflowCompleted emits an orchestration.workflow.completed event.
func (p *Publisher) PublishWorkflowCompleted(payload WorkflowCompletedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.completed event: %w", err)
	}
	p.publish("orchestration.workflow.completed", data)
	return nil
}

// PublishWorkflowFailed emits an orchestration.workflow.failed event.
func (p *Publisher) PublishWorkflowFailed(payload WorkflowFailedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.failed event: %w", err)
	}
	p.publish("orchestration.workflow.failed", data)
	return nil
}

// PublishWorkflowCancelled emits an orchestration.workflow.cancelled event.
func (p *Publisher) PublishWorkflowCancelled(payload WorkflowCancelledPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.cancelled event: %w", err)
	}
	p.publish("orchestration.workflow.cancelled", data)
	return nil
}

// PublishWorkflowCheckpointed emits an orchestration.workflow.checkpointed event.
func (p *Publisher) PublishWorkflowCheckpointed(payload WorkflowCheckpointedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.checkpointed event: %w", err)
	}
	p.publish("orchestration.workflow.checkpointed", data)
	return nil
}

// PublishWorkflowReplayed emits an orchestration.workflow.replayed event.
func (p *Publisher) PublishWorkflowReplayed(payload WorkflowReplayedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal workflow.replayed event: %w", err)
	}
	p.publish("orchestration.workflow.replayed", data)
	return nil
}

// PublishScheduleTriggered emits an orchestration.schedule.triggered event.
func (p *Publisher) PublishScheduleTriggered(payload ScheduleTriggeredPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal schedule.triggered event: %w", err)
	}
	p.publish("orchestration.schedule.triggered", data)
	return nil
}

// PublishAgentAssigned emits an orchestration.agent.assigned event.
func (p *Publisher) PublishAgentAssigned(payload AgentAssignedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal agent.assigned event: %w", err)
	}
	p.publish("orchestration.agent.assigned", data)
	return nil
}

// PublishAgentUnavailable emits an orchestration.agent.unavailable event.
func (p *Publisher) PublishAgentUnavailable(payload AgentUnavailablePayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal agent.unavailable event: %w", err)
	}
	p.publish("orchestration.agent.unavailable", data)
	return nil
}

// publish sends raw event data to the configured event bus.
func (p *Publisher) publish(topic string, data []byte) {
	log.Printf("[EVENT] %s: %s", topic, string(data))
}
