package store

import (
	"sync"

	"github.com/google/uuid"
)

// EscalationStore provides tenant-isolated storage for escalations.
type EscalationStore struct {
	mu          sync.RWMutex
	escalations map[string]*Escalation
	byTenant    map[string][]string
}

// NewEscalationStore creates an empty EscalationStore.
func NewEscalationStore() *EscalationStore {
	return &EscalationStore{
		escalations: make(map[string]*Escalation),
		byTenant:    make(map[string][]string),
	}
}

// Create stores a new open escalation.
func (s *EscalationStore) Create(e *Escalation) (*Escalation, error) {
	if e.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if e.Title == "" || !ValidEscalationSeverity(e.Severity) || !ValidEscalationCategory(e.Category) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e.ID = uuid.New().String()
	e.Status = "open"
	e.CreatedAt = timeNow()
	e.UpdatedAt = e.CreatedAt

	s.escalations[e.ID] = e
	s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID], e.ID)
	cp := *e
	return &cp, nil
}

// Get returns a tenant's escalation.
func (s *EscalationStore) Get(id, tenantID string) (*Escalation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.escalations[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

// Update applies a partial update; conflicts on resolved/closed escalations.
func (s *EscalationStore) Update(id, tenantID string, upd func(*Escalation)) (*Escalation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.escalations[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if e.Status == "resolved" || e.Status == "closed" {
		return nil, ErrConflict
	}
	upd(e)
	e.UpdatedAt = timeNow()
	cp := *e
	return &cp, nil
}

// Resolve finalizes an escalation.
func (s *EscalationStore) Resolve(id, tenantID, resolverID, notes, resolutionType string) (*Escalation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.escalations[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if e.Status == "resolved" || e.Status == "closed" {
		return nil, ErrConflict
	}
	now := timeNow()
	e.Status = "resolved"
	e.ResolvedAt = &now
	e.ResolverID = resolverID
	e.ResolutionNotes = notes
	e.ResolutionType = resolutionType
	e.UpdatedAt = now
	cp := *e
	return &cp, nil
}

// Delete removes a tenant's escalation.
func (s *EscalationStore) Delete(id, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.escalations[id]
	if !ok || e.TenantID != tenantID {
		return ErrNotFound
	}
	delete(s.escalations, id)
	list := s.byTenant[tenantID]
	for i, eid := range list {
		if eid == id {
			s.byTenant[tenantID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

// Open returns a tenant's unresolved escalations.
func (s *EscalationStore) Open(tenantID string) []Escalation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Escalation
	for _, id := range s.byTenant[tenantID] {
		e := s.escalations[id]
		if e != nil && e.Status != "resolved" && e.Status != "closed" {
			out = append(out, *e)
		}
	}
	return out
}

// All returns every escalation for a tenant (for the dashboard).
func (s *EscalationStore) All(tenantID string) []Escalation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Escalation
	for _, id := range s.byTenant[tenantID] {
		if e := s.escalations[id]; e != nil {
			out = append(out, *e)
		}
	}
	return out
}
