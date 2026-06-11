package store

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// InterventionStore provides tenant-isolated storage for interventions.
type InterventionStore struct {
	mu            sync.RWMutex
	interventions map[string]*Intervention
	byTenant      map[string][]string
}

// NewInterventionStore creates an empty InterventionStore.
func NewInterventionStore() *InterventionStore {
	return &InterventionStore{
		interventions: make(map[string]*Intervention),
		byTenant:      make(map[string][]string),
	}
}

// Create stores a new active intervention; expires_at derives from
// duration_minutes when set.
func (s *InterventionStore) Create(iv *Intervention) (*Intervention, error) {
	if iv.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if iv.TargetAgentID == "" || iv.Reason == "" || !ValidInterventionAction(iv.Action) {
		return nil, ErrValidation
	}
	if iv.Scope != nil && !ValidScopeType(iv.Scope.Type) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	iv.ID = uuid.New().String()
	iv.Status = "active"
	iv.IssuedAt = timeNow()
	iv.UpdatedAt = iv.IssuedAt
	if iv.DurationMinutes > 0 {
		exp := iv.IssuedAt.Add(time.Duration(iv.DurationMinutes) * time.Minute)
		iv.ExpiresAt = &exp
	}

	s.interventions[iv.ID] = iv
	s.byTenant[iv.TenantID] = append(s.byTenant[iv.TenantID], iv.ID)
	cp := *iv
	return &cp, nil
}

// Get returns a tenant's intervention, expiring it first when past deadline.
func (s *InterventionStore) Get(id, tenantID string) (*Intervention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	iv, ok := s.interventions[id]
	if !ok || iv.TenantID != tenantID {
		return nil, ErrNotFound
	}
	s.expireLocked(iv)
	cp := *iv
	return &cp, nil
}

// Update applies a partial update; conflicts unless the intervention is active.
func (s *InterventionStore) Update(id, tenantID string, upd func(*Intervention)) (*Intervention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	iv, ok := s.interventions[id]
	if !ok || iv.TenantID != tenantID {
		return nil, ErrNotFound
	}
	s.expireLocked(iv)
	if iv.Status != "active" {
		return nil, ErrConflict
	}
	upd(iv)
	if iv.DurationMinutes > 0 {
		exp := iv.IssuedAt.Add(time.Duration(iv.DurationMinutes) * time.Minute)
		iv.ExpiresAt = &exp
	}
	iv.UpdatedAt = timeNow()
	cp := *iv
	return &cp, nil
}

// Revoke lifts an active intervention.
func (s *InterventionStore) Revoke(id, tenantID, revokedBy string) (*Intervention, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	iv, ok := s.interventions[id]
	if !ok || iv.TenantID != tenantID {
		return nil, ErrNotFound
	}
	s.expireLocked(iv)
	if iv.Status != "active" {
		return nil, ErrConflict
	}
	now := timeNow()
	iv.Status = "revoked"
	iv.RevokedAt = &now
	iv.RevokedBy = revokedBy
	iv.UpdatedAt = now
	cp := *iv
	return &cp, nil
}

// Delete removes a tenant's intervention.
func (s *InterventionStore) Delete(id, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	iv, ok := s.interventions[id]
	if !ok || iv.TenantID != tenantID {
		return ErrNotFound
	}
	delete(s.interventions, id)
	list := s.byTenant[tenantID]
	for i, ivid := range list {
		if ivid == id {
			s.byTenant[tenantID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

// Active returns a tenant's currently active interventions.
func (s *InterventionStore) Active(tenantID string) []Intervention {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Intervention
	for _, id := range s.byTenant[tenantID] {
		iv := s.interventions[id]
		if iv == nil {
			continue
		}
		s.expireLocked(iv)
		if iv.Status == "active" {
			out = append(out, *iv)
		}
	}
	return out
}

// All returns every intervention for a tenant (for the dashboard).
func (s *InterventionStore) All(tenantID string) []Intervention {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Intervention
	for _, id := range s.byTenant[tenantID] {
		if iv := s.interventions[id]; iv != nil {
			s.expireLocked(iv)
			out = append(out, *iv)
		}
	}
	return out
}

func (s *InterventionStore) expireLocked(iv *Intervention) {
	if iv.Status == "active" && iv.ExpiresAt != nil && iv.ExpiresAt.Before(timeNow()) {
		iv.Status = "expired"
		iv.UpdatedAt = timeNow()
	}
}
