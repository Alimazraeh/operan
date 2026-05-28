// Package events publishes AsyncAPI events for orchestration lifecycle operations.
// Events use the multi-stack topic format: operan.orchestration.{stack}.{entity}.{event}
// where stack ∈ {langgraph, temporal, ray, celery}.
// Events are published to the configured event bus (Kafka/Pulsar) via
// the Publisher abstraction. This is a reference implementation that
// logs events; production should use a real message broker.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// StackType represents an orchestration stack backend.
type StackType string

const (
	StackLangGraph StackType = "langgraph"
	StackTemporal  StackType = "temporal"
	StackRay       StackType = "ray"
	StackCelery    StackType = "celery"
)

// Publisher handles publishing orchestration lifecycle events.
// It delegates to a Broker implementation (Kafka, AMQP, or in-memory).
type Publisher struct {
	broker Broker
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPublisher creates a new event publisher with a log-only broker (no-op).
// For production use, call NewPublisherWithBroker or set broker via SetBroker.
func NewPublisher() *Publisher {
	return &Publisher{
		broker: &logBroker{},
	}
}

// NewPublisherWithBroker creates a new event publisher backed by a real broker.
func NewPublisherWithBroker(broker Broker) *Publisher {
	return &Publisher{broker: broker}
}

// NewPublisherWithConfig creates a Publisher backed by the broker type specified in config.
func NewPublisherWithConfig(cfg BrokerConfig) (*Publisher, error) {
	factory := NewBrokerFactory()
	broker, err := factory.CreateBroker(BrokerKafka, cfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka broker: %w", err)
	}
	return &Publisher{broker: broker}, nil
}

// SetBroker replaces the underlying broker. Useful for swapping in a test broker.
func (p *Publisher) SetBroker(broker Broker) {
	p.broker = broker
}

// Close gracefully shuts down the broker.
func (p *Publisher) Close() error {
	if p.broker != nil {
		return p.broker.Close()
	}
	return nil
}

// logBroker is the default no-op broker that logs events instead of publishing.
type logBroker struct{}

func (l *logBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	log.Printf("[EVENT] %s: %s", topic, string(value))
	return nil
}

func (l *logBroker) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	return nil // not supported
}

func (l *logBroker) Close() error {
	return nil
}

// ─── topic helpers ───────────────────────────────────────────────────────────

// topic builds the AsyncAPI-compliant topic name:
// operan.orchestration.{stack}.{entity}.{event}
func (p *Publisher) topic(stack StackType, entity, event string) string {
	return fmt.Sprintf("operan.orchestration.%s.%s.%s", stack, entity, event)
}

func (p *Publisher) publish(topic string, data []byte) error {
	if p.broker == nil {
		return fmt.Errorf("broker not set")
	}
	return p.broker.Publish(context.Background(), topic, nil, data, nil)
}

func (p *Publisher) marshalAndPublish(stack StackType, entity, event string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s.%s event: %w", entity, event, err)
	}
	return p.publish(p.topic(stack, entity, event), data)
}

// ─── Workflow event payloads ─────────────────────────────────────────────────

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
	WorkflowID         string    `json:"workflow_id"`
	CancelledBy        string    `json:"cancelled_by"`
	CancelledAt        time.Time `json:"cancelled_at"`
	CancellationReason string    `json:"cancellation_reason"`
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
	ScheduleID     string    `json:"schedule_id"`
	WorkflowID     string    `json:"workflow_id"`
	TriggeredBy    string    `json:"triggered_by"`
	CronExpression string    `json:"cron_expression,omitempty"`
	NextRunAt      time.Time `json:"next_run_at"`
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
	AgentID           string   `json:"agent_id"`
	Reason            string   `json:"reason"`
	AffectedWorkflows []string `json:"affected_workflows"`
	DetectedAt        time.Time `json:"detected_at"`
}

// AgentOnlinePayload matches the AsyncAPI AgentOnlinePayload schema.
type AgentOnlinePayload struct {
	AgentID      string   `json:"agent_id"`
	DetectedAt   time.Time `json:"detected_at"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// AgentOfflinePayload matches the AsyncAPI AgentOfflinePayload schema.
type AgentOfflinePayload struct {
	AgentID  string    `json:"agent_id"`
	DetectedAt time.Time `json:"detected_at"`
	Reason   string    `json:"reason"` // user_initiated, heartbeat_timeout, system_shutdown
}

// EscalationCreatedPayload matches the AsyncAPI EscalationCreatedPayload schema.
type EscalationCreatedPayload struct {
	EscalationID string    `json:"escalation_id"`
	WorkflowID   string    `json:"workflow_id"`
	NodeID       string    `json:"node_id"`
	Severity     string    `json:"severity"` // low, medium, high, critical
	Reason       string    `json:"reason"`
	EscalatedTo  string    `json:"escalated_to,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// EscalationAcknowledgedPayload matches the AsyncAPI EscalationAcknowledgedPayload schema.
type EscalationAcknowledgedPayload struct {
	EscalationID  string    `json:"escalation_id"`
	AcknowledgedBy string   `json:"acknowledged_by"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

// EscalationResolvedPayload matches the AsyncAPI EscalationResolvedPayload schema.
type EscalationResolvedPayload struct {
	EscalationID   string    `json:"escalation_id"`
	ResolvedBy     string    `json:"resolved_by"`
	ResolvedAt     time.Time `json:"resolved_at"`
	ResolutionNotes string   `json:"resolution_notes,omitempty"`
}

// RetryRequestedPayload matches the AsyncAPI RetryRequestedPayload schema.
type RetryRequestedPayload struct {
	WorkflowID    string    `json:"workflow_id"`
	NodeID        string    `json:"node_id"`
	AttemptNumber int       `json:"attempt_number"`
	RequestedAt   time.Time `json:"requested_at"`
}

// RetryCompletedPayload matches the AsyncAPI RetryCompletedPayload schema.
type RetryCompletedPayload struct {
	WorkflowID    string    `json:"workflow_id"`
	NodeID        string    `json:"node_id"`
	AttemptNumber int       `json:"attempt_number"`
	Status        string    `json:"status"` // success, exhausted
	CompletedAt   time.Time `json:"completed_at"`
}

// WorkflowPriorityChangedPayload matches the AsyncAPI WorkflowPriorityChangedPayload schema.
type WorkflowPriorityChangedPayload struct {
	WorkflowID string    `json:"workflow_id"`
	OldPriority int      `json:"old_priority"`
	NewPriority int      `json:"new_priority"`
	ChangedBy  string    `json:"changed_by"`
	ChangedAt  time.Time `json:"changed_at"`
}

// WorkflowDelegationPayload matches the AsyncAPI WorkflowDelegationPayload schema.
type WorkflowDelegationPayload struct {
	DelegationID    string    `json:"delegation_id"`
	WorkflowID      string    `json:"workflow_id"`
	NodeID          string    `json:"node_id"`
	OriginalAgentID string    `json:"original_agent_id"`
	DelegatedAgentID string   `json:"delegated_agent_id"`
	Status          string    `json:"status"` // pending, accepted, rejected
	Reason          string    `json:"reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// DelegationCompletedPayload matches the AsyncAPI DelegationCompletedPayload schema.
type DelegationCompletedPayload struct {
	DelegationID string    `json:"delegation_id"`
	WorkflowID   string    `json:"workflow_id"`
	NodeID       string    `json:"node_id"`
	Status       string    `json:"status"` // completed, rejected
	CompletedAt  time.Time `json:"completed_at"`
}

// NodeStartedPayload matches the AsyncAPI NodeStartedPayload schema.
type NodeStartedPayload struct {
	WorkflowID string    `json:"workflow_id"`
	NodeID     string    `json:"node_id"`
	AgentID    string    `json:"agent_id"`
	StartedAt  time.Time `json:"started_at"`
}

// NodeCompletedPayload matches the AsyncAPI NodeCompletedPayload schema.
type NodeCompletedPayload struct {
	WorkflowID  string                 `json:"workflow_id"`
	NodeID      string                 `json:"node_id"`
	AgentID     string                 `json:"agent_id"`
	Status      string                 `json:"status"` // success, skipped
	Output      map[string]interface{} `json:"output,omitempty"`
	DurationMs  int                    `json:"duration_ms"`
	CompletedAt time.Time              `json:"completed_at"`
}

// NodeFailedPayload matches the AsyncAPI NodeFailedPayload schema.
type NodeFailedPayload struct {
	WorkflowID  string    `json:"workflow_id"`
	NodeID      string    `json:"node_id"`
	AgentID     string    `json:"agent_id"`
	ErrorCode   string    `json:"error_code"`
	ErrorMessage string   `json:"error_message"`
	RetryCount  int       `json:"retry_count"`
	FailedAt    time.Time `json:"failed_at"`
}

// ─── LangGraph event payloads ────────────────────────────────────────────────

// LangGraphGraphRegisteredPayload matches the AsyncAPI LangGraphGraphRegistered schema.
type LangGraphGraphRegisteredPayload struct {
	GraphID     string                 `json:"graph_id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	NodeCount   int                    `json:"node_count"`
	EdgeCount   int                    `json:"edge_count"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LangGraphStateUpdatedPayload matches the AsyncAPI LangGraphStateUpdated schema.
type LangGraphStateUpdatedPayload struct {
	GraphID     string                 `json:"graph_id"`
	NodeID      string                 `json:"node_id"`
	State       map[string]interface{} `json:"state"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Revision    int                    `json:"revision"`
}

// LangGraphGraphDeployedPayload matches the AsyncAPI LangGraphGraphDeployed schema.
type LangGraphGraphDeployedPayload struct {
	GraphID     string    `json:"graph_id"`
	TenantID    string    `json:"tenant_id"`
	DeployedBy  string    `json:"deployed_by"`
	DeployedAt  time.Time `json:"deployed_at"`
	Status      string    `json:"status"`
}

// ─── Temporal event payloads ─────────────────────────────────────────────────

// TemporalWorkflowRegisteredPayload matches the AsyncAPI TemporalWorkflowRegistered schema.
type TemporalWorkflowRegisteredPayload struct {
	WorkflowID   string    `json:"workflow_id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	RegisterdBy  string    `json:"registered_by"`
	RegisteredAt time.Time `json:"registered_at"`
}

// TemporalCheckpointCreatedPayload matches the AsyncAPI TemporalCheckpointCreated schema.
type TemporalCheckpointCreatedPayload struct {
	CheckpointID string                 `json:"checkpoint_id"`
	WorkflowID   string                 `json:"workflow_id"`
	TenantID     string                 `json:"tenant_id"`
	CreatedAt    time.Time              `json:"created_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TemporalWorkflowReplayedPayload matches the AsyncAPI TemporalWorkflowReplayed schema.
type TemporalWorkflowReplayedPayload struct {
	WorkflowID     string    `json:"workflow_id"`
	TenantID       string    `json:"tenant_id"`
	ReplayID       string    `json:"replay_id"`
	FromCheckpoint string    `json:"from_checkpoint"`
	ReplayedBy     string    `json:"replayed_by"`
	ReplayedAt     time.Time `json:"replayed_at"`
}

// ─── Ray event payloads ──────────────────────────────────────────────────────

// RayWorkerPooledPayload matches the AsyncAPI RayWorkerPooled schema.
type RayWorkerPooledPayload struct {
	PoolID     string                 `json:"pool_id"`
	TenantID   string                 `json:"tenant_id"`
	WorkerCount int                   `json:"worker_count"`
	CreatedAt  time.Time              `json:"created_at"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// RayTaskSubmittedPayload matches the AsyncAPI RayTaskSubmitted schema.
type RayTaskSubmittedPayload struct {
	TaskID     string                 `json:"task_id"`
	PoolID     string                 `json:"pool_id"`
	TenantID   string                 `json:"tenant_id"`
	SubmittedBy string                `json:"submitted_by"`
	SubmittedAt time.Time              `json:"submitted_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// RayTaskCompletedPayload matches the AsyncAPI RayTaskCompleted schema.
type RayTaskCompletedPayload struct {
	TaskID      string    `json:"task_id"`
	PoolID      string    `json:"pool_id"`
	TenantID    string    `json:"tenant_id"`
	CompletedAt time.Time `json:"completed_at"`
	DurationMs  int       `json:"duration_ms"`
	Success     bool      `json:"success"`
}

// RayWorkerStatusPayload matches the AsyncAPI RayWorkerStatus schema.
type RayWorkerStatusPayload struct {
	WorkerID  string                 `json:"worker_id"`
	PoolID    string                 `json:"pool_id"`
	TenantID  string                 `json:"tenant_id"`
	Status    string                 `json:"status"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ─── Celery event payloads ───────────────────────────────────────────────────

// CeleryQueueCreatedPayload matches the AsyncAPI CeleryQueueCreated schema.
type CeleryQueueCreatedPayload struct {
	QueueID   string                 `json:"queue_id"`
	TenantID  string                 `json:"tenant_id"`
	Name      string                 `json:"name"`
	CreatedAt time.Time              `json:"created_at"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

// CeleryTaskPublishedPayload matches the AsyncAPI CeleryTaskPublished schema.
type CeleryTaskPublishedPayload struct {
	TaskID      string    `json:"task_id"`
	QueueID     string    `json:"queue_id"`
	TenantID    string    `json:"tenant_id"`
	PublishedAt time.Time `json:"published_at"`
	PublishedBy string    `json:"published_by"`
}

// CeleryTaskConsumedPayload matches the AsyncAPI CeleryTaskConsumed schema.
type CeleryTaskConsumedPayload struct {
	TaskID     string    `json:"task_id"`
	QueueID    string    `json:"queue_id"`
	WorkerID   string    `json:"worker_id"`
	TenantID   string    `json:"tenant_id"`
	ConsumedAt time.Time `json:"consumed_at"`
}

// CeleryTaskCompletedPayload matches the AsyncAPI CeleryTaskCompleted schema.
type CeleryTaskCompletedPayload struct {
	TaskID      string    `json:"task_id"`
	QueueID     string    `json:"queue_id"`
	TenantID    string    `json:"tenant_id"`
	CompletedAt time.Time `json:"completed_at"`
	DurationMs  int       `json:"duration_ms"`
	Success     bool      `json:"success"`
}

// CeleryWorkerHeartbeatPayload matches the AsyncAPI CeleryWorkerHeartbeat schema.
type CeleryWorkerHeartbeatPayload struct {
	WorkerID  string    `json:"worker_id"`
	QueueID   string    `json:"queue_id"`
	TenantID  string    `json:"tenant_id"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// ─── Stack Health event payload ──────────────────────────────────────────────

// StackHealthPayload matches the AsyncAPI StackHealth schema.
type StackHealthPayload struct {
	StackType   string                 `json:"stack_type"`
	StackName   string                 `json:"stack_name"`
	TenantID    string                 `json:"tenant_id"`
	Status      string                 `json:"status"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ─── Publish methods: Workflow (stack-agnostic) ──────────────────────────────

// PublishWorkflowCreated emits an orchestration.workflow.created event.
func (p *Publisher) PublishWorkflowCreated(stack StackType, payload WorkflowCreatedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "created", payload)
}

// PublishWorkflowStarted emits an orchestration.workflow.started event.
func (p *Publisher) PublishWorkflowStarted(stack StackType, payload WorkflowStartedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "started", payload)
}

// PublishWorkflowPaused emits an orchestration.workflow.paused event.
func (p *Publisher) PublishWorkflowPaused(stack StackType, payload WorkflowPausedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "paused", payload)
}

// PublishWorkflowResumed emits an orchestration.workflow.resumed event.
func (p *Publisher) PublishWorkflowResumed(stack StackType, payload WorkflowResumedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "resumed", payload)
}

// PublishWorkflowCompleted emits an orchestration.workflow.completed event.
func (p *Publisher) PublishWorkflowCompleted(stack StackType, payload WorkflowCompletedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "completed", payload)
}

// PublishWorkflowFailed emits an orchestration.workflow.failed event.
func (p *Publisher) PublishWorkflowFailed(stack StackType, payload WorkflowFailedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "failed", payload)
}

// PublishWorkflowCancelled emits an orchestration.workflow.cancelled event.
func (p *Publisher) PublishWorkflowCancelled(stack StackType, payload WorkflowCancelledPayload) error {
	return p.marshalAndPublish(stack, "workflow", "cancelled", payload)
}

// PublishWorkflowCheckpointed emits an orchestration.workflow.checkpointed event.
func (p *Publisher) PublishWorkflowCheckpointed(stack StackType, payload WorkflowCheckpointedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "checkpointed", payload)
}

// PublishWorkflowReplayed emits an orchestration.workflow.replayed event.
func (p *Publisher) PublishWorkflowReplayed(stack StackType, payload WorkflowReplayedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "replayed", payload)
}

// PublishScheduleTriggered emits an orchestration.schedule.triggered event.
func (p *Publisher) PublishScheduleTriggered(stack StackType, payload ScheduleTriggeredPayload) error {
	return p.marshalAndPublish(stack, "schedule", "triggered", payload)
}

// PublishAgentAssigned emits an orchestration.agent.assigned event.
func (p *Publisher) PublishAgentAssigned(stack StackType, payload AgentAssignedPayload) error {
	return p.marshalAndPublish(stack, "agent", "assigned", payload)
}

// PublishAgentUnavailable emits an orchestration.agent.unavailable event.
func (p *Publisher) PublishAgentUnavailable(stack StackType, payload AgentUnavailablePayload) error {
	return p.marshalAndPublish(stack, "agent", "unavailable", payload)
}

// PublishAgentOnline emits an orchestration.agent.online event.
func (p *Publisher) PublishAgentOnline(stack StackType, payload AgentOnlinePayload) error {
	return p.marshalAndPublish(stack, "agent", "online", payload)
}

// PublishAgentOffline emits an orchestration.agent.offline event.
func (p *Publisher) PublishAgentOffline(stack StackType, payload AgentOfflinePayload) error {
	return p.marshalAndPublish(stack, "agent", "offline", payload)
}

// PublishEscalationCreated emits an orchestration.workflow.escalation.created event.
func (p *Publisher) PublishEscalationCreated(stack StackType, payload EscalationCreatedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "escalation.created", payload)
}

// PublishEscalationAcknowledged emits an orchestration.workflow.escalation.acknowledged event.
func (p *Publisher) PublishEscalationAcknowledged(stack StackType, payload EscalationAcknowledgedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "escalation.acknowledged", payload)
}

// PublishEscalationResolved emits an orchestration.workflow.escalation.resolved event.
func (p *Publisher) PublishEscalationResolved(stack StackType, payload EscalationResolvedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "escalation.resolved", payload)
}

// PublishRetryRequested emits an orchestration.workflow.retry.requested event.
func (p *Publisher) PublishRetryRequested(stack StackType, payload RetryRequestedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "retry.requested", payload)
}

// PublishRetryCompleted emits an orchestration.workflow.retry.completed event.
func (p *Publisher) PublishRetryCompleted(stack StackType, payload RetryCompletedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "retry.completed", payload)
}

// PublishWorkflowPriorityChanged emits an orchestration.workflow.priority_changed event.
func (p *Publisher) PublishWorkflowPriorityChanged(stack StackType, payload WorkflowPriorityChangedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "priority_changed", payload)
}

// PublishWorkflowDelegation emits an orchestration.workflow.delegate event.
func (p *Publisher) PublishWorkflowDelegation(stack StackType, payload WorkflowDelegationPayload) error {
	return p.marshalAndPublish(stack, "workflow", "delegate", payload)
}

// PublishDelegationCompleted emits an orchestration.workflow.delegate.completed event.
func (p *Publisher) PublishDelegationCompleted(stack StackType, payload DelegationCompletedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "delegate.completed", payload)
}

// PublishNodeStarted emits an orchestration.workflow.node.started event.
func (p *Publisher) PublishNodeStarted(stack StackType, payload NodeStartedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "node.started", payload)
}

// PublishNodeCompleted emits an orchestration.workflow.node.completed event.
func (p *Publisher) PublishNodeCompleted(stack StackType, payload NodeCompletedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "node.completed", payload)
}

// PublishNodeFailed emits an orchestration.workflow.node.failed event.
func (p *Publisher) PublishNodeFailed(stack StackType, payload NodeFailedPayload) error {
	return p.marshalAndPublish(stack, "workflow", "node.failed", payload)
}

// ─── Publish methods: LangGraph ──────────────────────────────────────────────

// PublishLangGraphGraphRegistered emits operan.orchestration.langgraph.graph.registered.
func (p *Publisher) PublishLangGraphGraphRegistered(payload LangGraphGraphRegisteredPayload) error {
	return p.marshalAndPublish(StackLangGraph, "graph", "registered", payload)
}

// PublishLangGraphStateUpdated emits operan.orchestration.langgraph.state.updated.
func (p *Publisher) PublishLangGraphStateUpdated(payload LangGraphStateUpdatedPayload) error {
	return p.marshalAndPublish(StackLangGraph, "state", "updated", payload)
}

// PublishLangGraphGraphDeployed emits operan.orchestration.langgraph.graph.deployed.
func (p *Publisher) PublishLangGraphGraphDeployed(payload LangGraphGraphDeployedPayload) error {
	return p.marshalAndPublish(StackLangGraph, "graph", "deployed", payload)
}

// ─── Publish methods: Temporal ───────────────────────────────────────────────

// PublishTemporalWorkflowRegistered emits operan.orchestration.temporal.workflow.registered.
func (p *Publisher) PublishTemporalWorkflowRegistered(payload TemporalWorkflowRegisteredPayload) error {
	return p.marshalAndPublish(StackTemporal, "workflow", "registered", payload)
}

// PublishTemporalCheckpointCreated emits operan.orchestration.temporal.checkpoint.created.
func (p *Publisher) PublishTemporalCheckpointCreated(payload TemporalCheckpointCreatedPayload) error {
	return p.marshalAndPublish(StackTemporal, "checkpoint", "created", payload)
}

// PublishTemporalWorkflowReplayed emits operan.orchestration.temporal.workflow.replayed.
func (p *Publisher) PublishTemporalWorkflowReplayed(payload TemporalWorkflowReplayedPayload) error {
	return p.marshalAndPublish(StackTemporal, "workflow", "replayed", payload)
}

// ─── Publish methods: Ray ────────────────────────────────────────────────────

// PublishRayWorkerPooled emits operan.orchestration.ray.worker.pooled.
func (p *Publisher) PublishRayWorkerPooled(payload RayWorkerPooledPayload) error {
	return p.marshalAndPublish(StackRay, "worker", "pooled", payload)
}

// PublishRayTaskSubmitted emits operan.orchestration.ray.task.submitted.
func (p *Publisher) PublishRayTaskSubmitted(payload RayTaskSubmittedPayload) error {
	return p.marshalAndPublish(StackRay, "task", "submitted", payload)
}

// PublishRayTaskCompleted emits operan.orchestration.ray.task.completed.
func (p *Publisher) PublishRayTaskCompleted(payload RayTaskCompletedPayload) error {
	return p.marshalAndPublish(StackRay, "task", "completed", payload)
}

// PublishRayWorkerStatus emits operan.orchestration.ray.worker.status.
func (p *Publisher) PublishRayWorkerStatus(payload RayWorkerStatusPayload) error {
	return p.marshalAndPublish(StackRay, "worker", "status", payload)
}

// ─── Publish methods: Celery ─────────────────────────────────────────────────

// PublishCeleryQueueCreated emits operan.orchestration.celery.queue.created.
func (p *Publisher) PublishCeleryQueueCreated(payload CeleryQueueCreatedPayload) error {
	return p.marshalAndPublish(StackCelery, "queue", "created", payload)
}

// PublishCeleryTaskPublished emits operan.orchestration.celery.task.published.
func (p *Publisher) PublishCeleryTaskPublished(payload CeleryTaskPublishedPayload) error {
	return p.marshalAndPublish(StackCelery, "task", "published", payload)
}

// PublishCeleryTaskConsumed emits operan.orchestration.celery.task.consumed.
func (p *Publisher) PublishCeleryTaskConsumed(payload CeleryTaskConsumedPayload) error {
	return p.marshalAndPublish(StackCelery, "task", "consumed", payload)
}

// PublishCeleryTaskCompleted emits operan.orchestration.celery.task.completed.
func (p *Publisher) PublishCeleryTaskCompleted(payload CeleryTaskCompletedPayload) error {
	return p.marshalAndPublish(StackCelery, "task", "completed", payload)
}

// PublishCeleryWorkerHeartbeat emits operan.orchestration.celery.worker.heartbeat.
func (p *Publisher) PublishCeleryWorkerHeartbeat(payload CeleryWorkerHeartbeatPayload) error {
	return p.marshalAndPublish(StackCelery, "worker", "heartbeat", payload)
}

// ─── Publish methods: Stack Health ───────────────────────────────────────────

// PublishStackHealth emits operan.orchestration.stack.health.
func (p *Publisher) PublishStackHealth(payload StackHealthPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal stack.health event: %w", err)
	}
	return p.publish("operan.orchestration.stack.health", data)
}
