// Package events provides typed event structs and a publisher for the Agent Registry module.
// Event topic format: operan.registry.{entity}.{event}
package events

import (
	"encoding/json"
	"log"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/broker"
	"github.com/operan/modules/04-agent-registry/internal/config"
	"github.com/operan/modules/04-agent-registry/internal/store"
)

// ─── Event topic constants ──────────────────────────────────────────────────

const (
	TopicAgentRegistered         = "operan.registry.agent.registered"
	TopicAgentCapabilitiesUpdated = "operan.registry.agent.capabilities_updated"
	TopicAgentVersionCreated     = "operan.registry.agent.version_created"
	TopicAgentPromoted           = "operan.registry.agent.promoted"
	TopicAgentDeprecated         = "operan.registry.agent.deprecated"
	TopicAgentArchived           = "operan.registry.agent.archived"
	TopicDependencyAdded         = "operan.registry.dependency.added"
	TopicDependencyRemoved       = "operan.registry.dependency.removed"
)

// ─── Event payload structs ──────────────────────────────────────────────────
// Struct names match AsyncAPI operationIds exactly.

// AgentRegisteredPayload corresponds to AsyncAPI operationId: agentRegistered.
type AgentRegisteredPayload struct {
	AgentID              string                 `json:"agent_id"`
	TenantID             string                 `json:"tenant_id"`
	DepartmentID         *string                `json:"department_id,omitempty"`
	Name                 string                 `json:"name"`
	Role                 string                 `json:"role"`
	Objectives           []Objective            `json:"objectives"`
	Capabilities         []string               `json:"capabilities"`
	Tools                []string               `json:"tools"`
	MemoryAccess         *MemoryAccess          `json:"memory_access,omitempty"`
	EscalationRules      []string               `json:"escalation_rules"`
	GovernancePolicies   []string               `json:"governance_policies"`
	SupportedLanguages   []string               `json:"supported_languages"`
	ExecutionBudget      *ExecutionBudget       `json:"execution_budget,omitempty"`
	Version              string                 `json:"version"`
	Status               string                 `json:"status"`
	RegisteredBy         string                 `json:"registered_by"`
	RegisteredAt         time.Time              `json:"registered_at"`
}

// AgentCapabilitiesUpdatedPayload corresponds to AsyncAPI operationId: agentCapabilitiesUpdated.
type AgentCapabilitiesUpdatedPayload struct {
	AgentID            string   `json:"agent_id"`
	TenantID           string   `json:"tenant_id"`
	PreviousCapabilities []string `json:"previous_capabilities"`
	NewCapabilities    []string `json:"new_capabilities"`
	UpdatedBy          string   `json:"updated_by"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AgentVersionCreatedPayload corresponds to AsyncAPI operationId: agentVersionCreated.
type AgentVersionCreatedPayload struct {
	AgentID            string `json:"agent_id"`
	NewVersion         string `json:"new_version"`
	PreviousVersion    string `json:"previous_version"`
	ChangeSummary      string `json:"change_summary"`
	DiffFromPrevious   *string `json:"diff_from_previous,omitempty"`
	Status             string `json:"status"`
	CreatedBy          string `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
}

// AgentPromotedPayload corresponds to AsyncAPI operationId: agentPromoted.
type AgentPromotedPayload struct {
	AgentID        string    `json:"agent_id"`
	PromotedVersion string   `json:"promoted_version"`
	PromotedBy     string    `json:"promoted_by"`
	PromotedAt     time.Time `json:"promoted_at"`
	FromEnvironment string   `json:"from_environment"`
	ToEnvironment  string    `json:"to_environment"`
}

// AgentDeprecatedPayload corresponds to AsyncAPI operationId: agentDeprecated.
type AgentDeprecatedPayload struct {
	AgentID           string     `json:"agent_id"`
	DeprecatedBy      string     `json:"deprecated_by"`
	DeprecatedAt      time.Time  `json:"deprecated_at"`
	Reason            string     `json:"reason"`
	ReplacementAgentID *string   `json:"replacement_agent_id,omitempty"`
	SunsetDate        *time.Time `json:"sunset_date,omitempty"`
	Status            string     `json:"status"`
}

// AgentArchivedPayload corresponds to AsyncAPI operationId: agentArchived.
type AgentArchivedPayload struct {
	AgentID     string    `json:"agent_id"`
	ArchivedBy  string    `json:"archived_by"`
	ArchivedAt  time.Time `json:"archived_at"`
	ArchiveReason string  `json:"archive_reason"`
}

// DependencyAddedPayload corresponds to AsyncAPI operationId: dependencyAdded.
type DependencyAddedPayload struct {
	AgentID         string `json:"agent_id"`
	DependencyID    string `json:"dependency_id"`
	DependencyType  string `json:"dependency_type"`
	VersionConstraint *string `json:"version_constraint,omitempty"`
	AddedBy         string `json:"added_by"`
	AddedAt         time.Time `json:"added_at"`
}

// DependencyRemovedPayload corresponds to AsyncAPI operationId: dependencyRemoved.
type DependencyRemovedPayload struct {
	AgentID      string    `json:"agent_id"`
	DependencyID string    `json:"dependency_id"`
	RemovedBy    string    `json:"removed_by"`
	RemovedAt    time.Time `json:"removed_at"`
	Reason       string    `json:"reason"`
}

// ─── Embedded domain types ──────────────────────────────────────────────────

// Objective represents an agent objective with a metric and weight.
type Objective struct {
	Description string  `json:"description"`
	Metric      string  `json:"metric"`
	Weight      float64 `json:"weight"`
	Tier        string  `json:"tier"`
}

// MemoryAccess represents agent memory configuration.
type MemoryAccess struct {
	Scope            string   `json:"scope"`
	IsolatedStores   []string `json:"isolated_stores"`
	AllowedTypes     []string `json:"allowed_types"`
	IsolationLevel   string   `json:"isolation_level"`
}

// ExecutionBudget represents agent execution budget constraints.
type ExecutionBudget struct {
	DailyTokenLimit    int     `json:"daily_token_limit"`
	MaxRunSeconds      int     `json:"max_run_seconds"`
	MonthlyExecutionCap int    `json:"monthly_execution_cap"`
	MonthlyBudgetUSD   float64 `json:"monthly_budget_usd"`
}

// ─── Broker interface ───────────────────────────────────────────────────────

// Broker is the interface for publishing events to a message broker.
type Broker interface {
	Produce(topic, key string, value []byte) error
	Close() error
}

// logBroker is a no-op broker that logs events for debugging.
type logBroker struct{}

func (l *logBroker) Produce(topic, key string, value []byte) error {
	log.Printf("[event-producer] topic=%s key=%s payload=%s", topic, key, string(value))
	return nil
}

func (l *logBroker) Close() error {
	return nil
}

// ─── Publisher ──────────────────────────────────────────────────────────────

// Publisher handles publishing typed events to the message broker.
type Publisher struct {
	broker Broker
}

// NewPublisher creates a publisher with a log broker (no-op, logs to stdout).
func NewPublisher() *Publisher {
	return &Publisher{
		broker: &logBroker{},
	}
}

// NewPublisherWithConfig creates a publisher with a Kafka broker configured from the service config.
func NewPublisherWithConfig(cfg config.Config) *Publisher {
	kafkaCfg := broker.Config{
		Host:  cfg.EventBusHost,
		Port:  cfg.EventBusPort,
		Proto: cfg.EventBusProto,
	}
	return &Publisher{
		broker: broker.NewKafkaProducer(kafkaCfg),
	}
}

// NewPublisherWithBroker creates a publisher with a real broker.
func NewPublisherWithBroker(broker Broker) *Publisher {
	return &Publisher{
		broker: broker,
	}
}

// SetBroker replaces the current broker.
func (p *Publisher) SetBroker(broker Broker) {
	p.broker = broker
}

// Close shuts down the broker connection.
func (p *Publisher) Close() error {
	if p.broker != nil {
		return p.broker.Close()
	}
	return nil
}

// PublishAgentRegistered publishes an agent registered event.
func (p *Publisher) PublishAgentRegistered(agentID, tenantID, name, role, version, status, registeredBy string, objectives []store.Objective, capabilities, tools, escalation, governance, languages []string, budget *store.ExecutionBudget, deptID *string, memAccess *store.MemoryAccess, ts time.Time) error {
	payload := AgentRegisteredPayload{
		AgentID:             agentID,
		TenantID:            tenantID,
		DepartmentID:        deptID,
		Name:                name,
		Role:                role,
		Objectives:          toEventObjectives(objectives),
		Capabilities:        capabilities,
		Tools:               tools,
		MemoryAccess:        toEventMemoryAccess(memAccess),
		EscalationRules:     escalation,
		GovernancePolicies:  governance,
		SupportedLanguages:  languages,
		Version:             version,
		Status:              status,
		RegisteredBy:        registeredBy,
		RegisteredAt:        ts,
	}
	if budget != nil {
		payload.ExecutionBudget = toEventExecutionBudget(budget)
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicAgentRegistered, agentID, value)
}

// PublishAgentCapabilitiesUpdated publishes an agent capabilities updated event.
func (p *Publisher) PublishAgentCapabilitiesUpdated(agentID, tenantID, updatedBy string, prevCaps, newCaps []string, ts time.Time) error {
	payload := AgentCapabilitiesUpdatedPayload{
		AgentID:              agentID,
		TenantID:             tenantID,
		PreviousCapabilities: prevCaps,
		NewCapabilities:      newCaps,
		UpdatedBy:            updatedBy,
		UpdatedAt:            ts,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicAgentCapabilitiesUpdated, agentID, value)
}

// PublishAgentVersionCreated publishes an agent version created event.
func (p *Publisher) PublishAgentVersionCreated(agentID, newVersion, previousVersion, changeSummary, status, createdBy string, diff *string, ts time.Time) error {
	payload := AgentVersionCreatedPayload{
		AgentID:          agentID,
		NewVersion:       newVersion,
		PreviousVersion:  previousVersion,
		ChangeSummary:    changeSummary,
		DiffFromPrevious: diff,
		Status:           status,
		CreatedBy:        createdBy,
		CreatedAt:        ts,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicAgentVersionCreated, agentID, value)
}

// PublishAgentPromoted publishes an agent promoted event.
func (p *Publisher) PublishAgentPromoted(agentID, promotedVersion, promotedBy, fromEnv, toEnv string, ts time.Time) error {
	payload := AgentPromotedPayload{
		AgentID:         agentID,
		PromotedVersion: promotedVersion,
		PromotedBy:      promotedBy,
		PromotedAt:      ts,
		FromEnvironment: fromEnv,
		ToEnvironment:   toEnv,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicAgentPromoted, agentID, value)
}

// PublishAgentDeprecated publishes an agent deprecated event.
func (p *Publisher) PublishAgentDeprecated(agentID, deprecatedBy, reason, status string, replacementID *string, sunsetDate *time.Time, ts time.Time) error {
	payload := AgentDeprecatedPayload{
		AgentID:            agentID,
		DeprecatedBy:       deprecatedBy,
		DeprecatedAt:       ts,
		Reason:             reason,
		ReplacementAgentID: replacementID,
		SunsetDate:         sunsetDate,
		Status:             status,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicAgentDeprecated, agentID, value)
}

// PublishAgentArchived publishes an agent archived event.
func (p *Publisher) PublishAgentArchived(agentID, archivedBy, archiveReason string, ts time.Time) error {
	payload := AgentArchivedPayload{
		AgentID:       agentID,
		ArchivedBy:    archivedBy,
		ArchivedAt:    ts,
		ArchiveReason: archiveReason,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicAgentArchived, agentID, value)
}

// PublishDependencyAdded publishes a dependency added event.
func (p *Publisher) PublishDependencyAdded(agentID, dependencyID, depType, versionConstraint, addedBy string, ts time.Time) error {
	payload := DependencyAddedPayload{
		AgentID:           agentID,
		DependencyID:      dependencyID,
		DependencyType:    depType,
		VersionConstraint: &versionConstraint,
		AddedBy:           addedBy,
		AddedAt:           ts,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicDependencyAdded, agentID, value)
}

// PublishDependencyRemoved publishes a dependency removed event.
func (p *Publisher) PublishDependencyRemoved(agentID, dependencyID, removedBy, reason string, ts time.Time) error {
	payload := DependencyRemovedPayload{
		AgentID:      agentID,
		DependencyID: dependencyID,
		RemovedBy:    removedBy,
		RemovedAt:    ts,
		Reason:       reason,
	}
	value, _ := serialize(payload)
	return p.broker.Produce(TopicDependencyRemoved, agentID, value)
}

// ─── Conversion helpers ─────────────────────────────────────────────────────

func toEventObjectives(s []store.Objective) []Objective {
	if s == nil {
		return nil
	}
	e := make([]Objective, len(s))
	for i, o := range s {
		e[i] = Objective{Description: o.Description, Metric: o.Metric, Weight: o.Weight, Tier: string(o.Tier)}
	}
	return e
}

func toEventMemoryAccess(s *store.MemoryAccess) *MemoryAccess {
	if s == nil {
		return nil
	}
	return &MemoryAccess{Scope: s.Scope, IsolatedStores: s.IsolatedStores, AllowedTypes: s.AllowedTypes, IsolationLevel: string(s.IsolationLevel)}
}

func toEventExecutionBudget(s *store.ExecutionBudget) *ExecutionBudget {
	if s == nil {
		return nil
	}
	return &ExecutionBudget{DailyTokenLimit: s.DailyTokenLimit, MaxRunSeconds: s.MaxRunSeconds, MonthlyExecutionCap: s.MonthlyExecutionCap, MonthlyBudgetUSD: s.MonthlyBudgetUSD}
}

// serialize encodes a payload to JSON bytes.
func serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
