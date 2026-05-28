package store

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Delegation ──────────────────────────────────────────────────────────────

// DelegationStatus represents the lifecycle state of a delegation.
type DelegationStatus string

const (
	DelegationPending   DelegationStatus = "pending"
	DelegationAccepted  DelegationStatus = "accepted"
	DelegationRejected  DelegationStatus = "rejected"
	DelegationCompleted DelegationStatus = "completed"
)

// Delegation represents a task delegation from one agent to another.
type Delegation struct {
	ID              string            `json:"id"`
	WorkflowID      string            `json:"workflow_id"`
	NodeID          string            `json:"node_id"`
	OriginalAgentID string            `json:"original_agent_id"`
	DelegatedAgentID string           `json:"delegated_agent_id"`
	TenantID        string            `json:"tenant_id"`
	DepartmentID    string            `json:"department_id,omitempty"`
	Status          DelegationStatus  `json:"status"`
	Reason          string            `json:"reason"`
	CreatedAt       time.Time         `json:"created_at"`
}

// DelegationStore provides CRUD operations for delegations.
type DelegationStore struct {
	mu         sync.RWMutex
	items      map[string]*Delegation   // key: delegation ID
	byWorkflow map[string][]*Delegation // key: workflow ID
}

// NewDelegationStore creates a new DelegationStore.
func NewDelegationStore() *DelegationStore {
	return &DelegationStore{
		items:      make(map[string]*Delegation),
		byWorkflow: make(map[string][]*Delegation),
	}
}

// Create creates a new delegation.
func (s *DelegationStore) Create(d *Delegation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	s.items[d.ID] = d
	s.byWorkflow[d.WorkflowID] = append(s.byWorkflow[d.WorkflowID], d)
}

// GetByID returns a delegation by ID.
func (s *DelegationStore) GetByID(id string) (*Delegation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.items[id]
	if !ok {
		return nil, false
	}
	// Return a copy to prevent mutation
	cp := *d
	return &cp, true
}

// ListByWorkflow returns all delegations for a workflow.
func (s *DelegationStore) ListByWorkflow(workflowID string) []*Delegation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	delegations := s.byWorkflow[workflowID]
	result := make([]*Delegation, len(delegations))
	for i, d := range delegations {
		cp := *d
		result[i] = &cp
	}
	return result
}

// UpdateStatus updates the status of a delegation.
func (s *DelegationStore) UpdateStatus(id string, status DelegationStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.items[id]
	if !ok {
		return false
	}
	d.Status = status
	return true
}
