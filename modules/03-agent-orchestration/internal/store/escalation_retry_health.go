// Package store provides in-memory stores for the orchestration engine.
package store

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Agent ───────────────────────────────────────────────────────────────────

// Agent represents a registered agent with its capabilities and status.
type Agent struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Capabilities []string       `json:"capabilities"`
	Status       AgentStatus    `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ─── Escalation ──────────────────────────────────────────────────────────────

// EscalationStatus represents the lifecycle state of an escalation.
type EscalationStatus string

const (
	EscalationPending    EscalationStatus = "pending"
	EscalationAcknowledged EscalationStatus = "acknowledged"
	EscalationResolved   EscalationStatus = "resolved"
	EscalationExpired    EscalationStatus = "expired"
)

// EscalationSeverity represents the severity of an escalation.
type EscalationSeverity string

const (
	EscalationLow    EscalationSeverity = "low"
	EscalationMedium EscalationSeverity = "medium"
	EscalationHigh   EscalationSeverity = "high"
	EscalationCritical EscalationSeverity = "critical"
)

// Escalation represents an escalation triggered by a failed node or timeout.
type Escalation struct {
	ID             string            `json:"id"`
	WorkflowID     string            `json:"workflow_id"`
	NodeID         string            `json:"node_id"`
	TenantID       string            `json:"tenant_id"`
	DepartmentID   string            `json:"department_id,omitempty"`
	Status         EscalationStatus  `json:"status"`
	Severity       EscalationSeverity `json:"severity"`
	Reason         string            `json:"reason"`
	EscalatedTo    string            `json:"escalated_to,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	AcknowledgedAt *time.Time        `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty"`
}

// EscalationStore provides CRUD operations for escalations.
type EscalationStore struct {
	mu       sync.RWMutex
	items    map[string]*Escalation // key: escalation ID
	byWorkflow map[string][]*Escalation // key: workflow ID
}

// NewEscalationStore creates a new EscalationStore.
func NewEscalationStore() *EscalationStore {
	return &EscalationStore{
		items:      make(map[string]*Escalation),
		byWorkflow: make(map[string][]*Escalation),
	}
}

// Create creates a new escalation.
func (s *EscalationStore) Create(e *Escalation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	s.items[e.ID] = e
	s.byWorkflow[e.WorkflowID] = append(s.byWorkflow[e.WorkflowID], e)
}

// GetByID returns an escalation by ID.
func (s *EscalationStore) GetByID(id string) (*Escalation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[id]
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutation
	ec := *e
	return &ec, true
}

// ListByWorkflow returns all escalations for a workflow.
func (s *EscalationStore) ListByWorkflow(workflowID string) []*Escalation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	escs := s.byWorkflow[workflowID]
	result := make([]*Escalation, len(escs))
	for i, e := range escs {
		ec := *e
		result[i] = &ec
	}
	return result
}

// Acknowledge marks an escalation as acknowledged.
func (s *EscalationStore) Acknowledge(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok {
		return false
	}
	if e.Status != EscalationPending {
		return false
	}
	now := time.Now().UTC()
	e.Status = EscalationAcknowledged
	e.AcknowledgedAt = &now
	return true
}

// Resolve marks an escalation as resolved.
func (s *EscalationStore) Resolve(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok {
		return false
	}
	now := time.Now().UTC()
	e.Status = EscalationResolved
	e.ResolvedAt = &now
	return true
}

// ─── RetryRecord ─────────────────────────────────────────────────────────────

// RetryStatus represents the lifecycle state of a retry attempt.
type RetryStatus string

const (
	RetryPending   RetryStatus = "pending"
	RetryInProgress RetryStatus = "in_progress"
	RetrySuccess   RetryStatus = "success"
	RetryExhausted RetryStatus = "exhausted"
)

// RetryRecord represents a retry attempt for a failed workflow node.
type RetryRecord struct {
	ID            string     `json:"id"`
	WorkflowID    string     `json:"workflow_id"`
	NodeID        string     `json:"node_id"`
	TenantID      string     `json:"tenant_id"`
	AttemptNumber int        `json:"attempt_number"`
	Status        RetryStatus `json:"status"`
	ErrorCode     string     `json:"error_code,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// RetryRecordStore provides CRUD operations for retry records.
type RetryRecordStore struct {
	mu         sync.RWMutex
	items      map[string]*RetryRecord
	byWorkflow map[string][]*RetryRecord
}

// NewRetryRecordStore creates a new RetryRecordStore.
func NewRetryRecordStore() *RetryRecordStore {
	return &RetryRecordStore{
		items:      make(map[string]*RetryRecord),
		byWorkflow: make(map[string][]*RetryRecord),
	}
}

// Create creates a new retry record.
func (s *RetryRecordStore) Create(r *RetryRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.items[r.ID] = r
	s.byWorkflow[r.WorkflowID] = append(s.byWorkflow[r.WorkflowID], r)
}

// GetByID returns a retry record by ID.
func (s *RetryRecordStore) GetByID(id string) (*RetryRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.items[id]
	if !ok {
		return nil, false
	}
	rc := *r
	return &rc, true
}

// ListByWorkflow returns all retry records for a workflow.
func (s *RetryRecordStore) ListByWorkflow(workflowID string) []*RetryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := s.byWorkflow[workflowID]
	result := make([]*RetryRecord, len(records))
	for i, r := range records {
		rc := *r
		result[i] = &rc
	}
	return result
}

// UpdateStatus updates the status of a retry record.
func (s *RetryRecordStore) UpdateStatus(id string, status RetryStatus, errorCode, errorMessage string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.items[id]
	if !ok {
		return false
	}
	r.Status = status
	r.ErrorCode = errorCode
	r.ErrorMessage = errorMessage
	now := time.Now().UTC()
	r.CompletedAt = &now
	return true
}

// ─── Stack Health ────────────────────────────────────────────────────────────

// StackHealthStatus represents the health of an orchestration stack module.
type StackHealthStatus string

const (
	StackHealthy   StackHealthStatus = "healthy"
	StackDegraded  StackHealthStatus = "degraded"
	StackUnhealthy StackHealthStatus = "unhealthy"
	StackUnknown   StackHealthStatus = "unknown"
)

// StackHealthModule represents health status for a single stack module.
type StackHealthModule struct {
	Status       StackHealthStatus `json:"status"`
	Version      string            `json:"version,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	UptimeSeconds int              `json:"uptime_seconds,omitempty"`
	LastCheck    time.Time         `json:"last_check"`
}

// StackHealthEntry represents the complete health status for all orchestration stacks.
type StackHealthEntry struct {
	ID        string                          `json:"id"`
	TenantID  string                          `json:"tenant_id"`
	Stacks    map[string]*StackHealthModule   `json:"stacks"`
	At        time.Time                       `json:"at"`
	// Optional fields for individual stack instance management
	StackType  string                         `json:"stack_type,omitempty"`
	StackName  string                         `json:"stack_name,omitempty"`
	Status     StackHealthStatus              `json:"status,omitempty"`
	Config     map[string]interface{}         `json:"config,omitempty"`
	Metadata   map[string]interface{}         `json:"metadata,omitempty"`
	GraphDef   map[string]interface{}         `json:"graph_def,omitempty"`
}

// StackHealthStore provides CRUD operations for stack health entries.
type StackHealthStore struct {
	mu      sync.RWMutex
	entries map[string]*StackHealthEntry // key: entry ID
}

// NewStackHealthStore creates a new StackHealthStore.
func NewStackHealthStore() *StackHealthStore {
	return &StackHealthStore{
		entries: make(map[string]*StackHealthEntry),
	}
}

// Create creates a new stack health entry.
func (s *StackHealthStore) Create(e *StackHealthEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	s.entries[e.ID] = e
}

// GetByID returns a stack health entry by ID.
func (s *StackHealthStore) GetByID(id string) (*StackHealthEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[id]
	if !ok {
		return nil, false
	}
	ec := *e
	return &ec, true
}

// GetLatest returns the most recent stack health entry.
func (s *StackHealthStore) GetLatest() *StackHealthEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *StackHealthEntry
	for _, e := range s.entries {
		if latest == nil || e.At.After(latest.At) {
			ec := *e
			latest = &ec
		}
	}
	return latest
}

// Update updates an existing stack health entry.
func (s *StackHealthStore) Update(e *StackHealthEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID != "" {
		s.entries[e.ID] = e
	}
}

// Delete deletes a stack health entry by ID.
func (s *StackHealthStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[id]; !ok {
		return false
	}
	delete(s.entries, id)
	return true
}

// ListByStack returns all entries for a tenant and stack type.
// For simplicity, filters by tenantID and checks if any stack in the entry
// matches the given stackType.
func (s *StackHealthStore) ListByStack(tenantID, stackType string) []*StackHealthEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*StackHealthEntry
	for _, e := range s.entries {
		if e.TenantID != tenantID {
			continue
		}
		// Return a copy filtered to only the matching stack type
		filtered := *e
		if stackType != "" {
			filtered.Stacks = make(map[string]*StackHealthModule)
			if m, ok := e.Stacks[stackType]; ok {
				mc := *m
				filtered.Stacks[stackType] = &mc
			}
		}
		ec := filtered
		result = append(result, &ec)
	}
	return result
}
