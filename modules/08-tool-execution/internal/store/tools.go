package store

import (
	"sort"
	"sync"

	"github.com/google/uuid"
)

// ToolStore provides tenant-isolated CRUD for tools.
type ToolStore struct {
	mu       sync.RWMutex
	tools    map[string]*Tool    // id -> tool
	byTenant map[string][]string // tenantID -> []toolID
}

// NewToolStore creates an empty ToolStore.
func NewToolStore() *ToolStore {
	return &ToolStore{
		tools:    make(map[string]*Tool),
		byTenant: make(map[string][]string),
	}
}

// Create registers a new tool, auto-generating ID/timestamps/defaults.
func (s *ToolStore) Create(t *Tool) (*Tool, error) {
	if t.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if t.Name == "" {
		return nil, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Version == "" {
		t.Version = "1.0.0"
	}
	if t.Status == "" {
		t.Status = "active"
	}
	t.CreatedAt = timeNow()
	t.UpdatedAt = t.CreatedAt

	s.tools[t.ID] = t
	s.byTenant[t.TenantID] = append(s.byTenant[t.TenantID], t.ID)
	cp := *t
	return &cp, nil
}

// GetByIDAndTenant returns a tool scoped to a tenant.
func (s *ToolStore) GetByIDAndTenant(id, tenantID string) (*Tool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tools[id]
	if !ok || t.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

// List returns a tenant's tools, optionally filtered by category/status, paginated.
func (s *ToolStore) List(tenantID string, page, pageSize int, category, status *string) ([]Tool, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*Tool
	for _, id := range s.byTenant[tenantID] {
		t, ok := s.tools[id]
		if !ok {
			continue
		}
		if category != nil && t.Category != *category {
			continue
		}
		if status != nil && t.Status != *status {
			continue
		}
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]Tool, 0, end-start)
	for _, t := range all[start:end] {
		out = append(out, *t)
	}
	return out, total, end < total
}

// Update applies a partial update to a tenant-owned tool.
func (s *ToolStore) Update(id, tenantID string, apply func(*Tool)) (*Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tools[id]
	if !ok || t.TenantID != tenantID {
		return nil, ErrNotFound
	}
	apply(t)
	t.UpdatedAt = timeNow()
	cp := *t
	return &cp, nil
}
