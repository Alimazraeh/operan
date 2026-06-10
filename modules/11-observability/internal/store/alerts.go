package store

import (
	"sort"
	"sync"

	"github.com/google/uuid"
)

// AlertStore provides tenant-isolated storage for alerts.
type AlertStore struct {
	mu       sync.RWMutex
	alerts   map[string]*Alert
	byTenant map[string][]string
}

// NewAlertStore creates an empty AlertStore.
func NewAlertStore() *AlertStore {
	return &AlertStore{
		alerts:   make(map[string]*Alert),
		byTenant: make(map[string][]string),
	}
}

// Fire creates a new unresolved alert.
func (s *AlertStore) Fire(a *Alert) (*Alert, error) {
	if a.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if a.AlertName == "" || !ValidAlertSeverity(string(a.Severity)) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.TriggeredAt = timeNow()
	a.ResolvedAt = nil

	s.alerts[a.ID] = a
	s.byTenant[a.TenantID] = append(s.byTenant[a.TenantID], a.ID)
	cp := *a
	return &cp, nil
}

// Resolve marks a tenant's alert resolved by the given principal.
func (s *AlertStore) Resolve(id, tenantID, resolvedBy string) (*Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.alerts[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	now := timeNow()
	a.ResolvedAt = &now
	a.ResolvedBy = resolvedBy
	cp := *a
	return &cp, nil
}

// List returns a tenant's alerts, optionally filtered by severity and
// resolved state, paginated, newest first.
func (s *AlertStore) List(tenantID string, page, pageSize int, severity *string, resolved *bool) ([]Alert, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*Alert
	for _, id := range s.byTenant[tenantID] {
		a := s.alerts[id]
		if a == nil {
			continue
		}
		if severity != nil && string(a.Severity) != *severity {
			continue
		}
		if resolved != nil && (a.ResolvedAt != nil) != *resolved {
			continue
		}
		matched = append(matched, a)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].TriggeredAt.After(matched[j].TriggeredAt) })

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]Alert, 0, end-start)
	for _, a := range matched[start:end] {
		out = append(out, *a)
	}
	return out, total, end < total
}

// UnresolvedBySeverity returns counts of unresolved alerts per severity.
func (s *AlertStore) UnresolvedBySeverity(tenantID string) map[AlertSeverity]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[AlertSeverity]int{}
	for _, id := range s.byTenant[tenantID] {
		a := s.alerts[id]
		if a != nil && a.ResolvedAt == nil {
			counts[a.Severity]++
		}
	}
	return counts
}
