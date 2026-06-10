package store

import (
	"sync"

	"github.com/google/uuid"
)

// OperationStore tracks garbage-collection job statuses per tenant.
type OperationStore struct {
	mu       sync.RWMutex
	ops      map[string]*OperationStatus
	byTenant map[string][]string
}

// NewOperationStore creates an empty OperationStore.
func NewOperationStore() *OperationStore {
	return &OperationStore{
		ops:      make(map[string]*OperationStatus),
		byTenant: make(map[string][]string),
	}
}

// Start records a new processing operation for a tenant and returns it.
func (s *OperationStore) Start(tenantID string) *OperationStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeNow()
	op := &OperationStatus{
		ID:        uuid.New().String(),
		Status:    "processing",
		StartedAt: &now,
	}
	s.ops[op.ID] = op
	s.byTenant[tenantID] = append(s.byTenant[tenantID], op.ID)
	cp := *op
	return &cp
}

// Complete marks an operation finished with the given batch size.
func (s *OperationStore) Complete(id string, batchSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, ok := s.ops[id]
	if !ok {
		return
	}
	now := timeNow()
	op.Status = "completed"
	op.BatchSize = batchSize
	op.CompletedAt = &now
}

// Fail marks an operation failed with an error message.
func (s *OperationStore) Fail(id, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, ok := s.ops[id]
	if !ok {
		return
	}
	now := timeNow()
	op.Status = "failed"
	op.ErrorMessage = msg
	op.CompletedAt = &now
}

// Get returns an operation by ID.
func (s *OperationStore) Get(id string) (*OperationStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	op, ok := s.ops[id]
	if !ok {
		return nil, false
	}
	cp := *op
	return &cp, true
}
