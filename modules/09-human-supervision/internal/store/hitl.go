package store

import (
	"sync"

	"github.com/google/uuid"
)

// HitlStore records human-in-the-loop answers, keyed by request ID.
type HitlStore struct {
	mu      sync.RWMutex
	answers map[string]*HitlAnswer // tenantID+"|"+requestID -> answer
}

// NewHitlStore creates an empty HitlStore.
func NewHitlStore() *HitlStore {
	return &HitlStore{answers: make(map[string]*HitlAnswer)}
}

// Submit records an answer for a request. Conflicts when one exists already.
func (s *HitlStore) Submit(a *HitlAnswer) (*HitlAnswer, error) {
	if a.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if a.RequestID == "" || a.Answer == "" {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := a.TenantID + "|" + a.RequestID
	if _, exists := s.answers[key]; exists {
		return nil, ErrConflict
	}

	a.ID = uuid.New().String()
	a.CreatedAt = timeNow()
	s.answers[key] = a
	cp := *a
	return &cp, nil
}

// Get returns the answer recorded for a request, if any.
func (s *HitlStore) Get(requestID, tenantID string) (*HitlAnswer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.answers[tenantID+"|"+requestID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *a
	return &cp, nil
}
