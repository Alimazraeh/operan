package store

import (
	"sort"
	"sync"
)

// HealthStore tracks component health per tenant.
type HealthStore struct {
	mu         sync.RWMutex
	components map[string]map[string]*HealthStatus // tenantID -> componentID -> status
}

// NewHealthStore creates an empty HealthStore.
func NewHealthStore() *HealthStore {
	return &HealthStore{components: make(map[string]map[string]*HealthStatus)}
}

// Upsert records a component's status. It returns the resulting status and
// whether the state actually changed (for status_change event publishing).
func (s *HealthStore) Upsert(tenantID, componentID, componentType string, status HealthState, reason string) (*HealthStatus, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.components[tenantID] == nil {
		s.components[tenantID] = make(map[string]*HealthStatus)
	}

	prev := s.components[tenantID][componentID]
	if prev != nil && prev.NewStatus == status {
		return copyStatus(prev), false
	}

	hs := &HealthStatus{
		TenantID:      tenantID,
		ComponentID:   componentID,
		ComponentType: componentType,
		NewStatus:     status,
		ChangedAt:     timeNow(),
		Reason:        reason,
	}
	if prev != nil {
		hs.PreviousStatus = prev.NewStatus
	}
	s.components[tenantID][componentID] = hs
	return copyStatus(hs), true
}

// Get returns one component's status for a tenant.
func (s *HealthStore) Get(tenantID, componentID string) (*HealthStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hs := s.components[tenantID][componentID]
	if hs == nil {
		return nil, ErrNotFound
	}
	return copyStatus(hs), nil
}

// Overview returns all of a tenant's components (sorted by ID) and the
// rolled-up overall status: unhealthy beats degraded beats healthy.
func (s *HealthStore) Overview(tenantID string) ([]HealthStatus, HealthState) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	overall := Healthy
	var out []HealthStatus
	for _, hs := range s.components[tenantID] {
		out = append(out, *hs)
		switch hs.NewStatus {
		case Unhealthy:
			overall = Unhealthy
		case Degraded:
			if overall == Healthy {
				overall = Degraded
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ComponentID < out[j].ComponentID })
	return out, overall
}

func copyStatus(hs *HealthStatus) *HealthStatus {
	cp := *hs
	return &cp
}
