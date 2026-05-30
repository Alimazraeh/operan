package store

import (
	"sync"

	"github.com/google/uuid"
)

// VersionStore provides immutable, tenant-isolated version storage for templates.
type VersionStore struct {
	mu         sync.RWMutex
	versions   map[string]*TemplateVersion // id -> version
	byTemplate map[string][]*TemplateVersion // templateID -> versions
	byTenant   map[string][]*TemplateVersion // tenantID -> versions
}

// NewVersionStore creates a new empty VersionStore.
func NewVersionStore() *VersionStore {
	return &VersionStore{
		versions:   make(map[string]*TemplateVersion),
		byTemplate: make(map[string][]*TemplateVersion),
		byTenant:   make(map[string][]*TemplateVersion),
	}
}

// Create adds a new immutable version snapshot with tenant isolation.
func (s *VersionStore) Create(v *TemplateVersion) (*TemplateVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	v.CreatedAt = timeNow()

	s.versions[v.ID] = v
	s.byTemplate[v.TemplateID] = append(s.byTemplate[v.TemplateID], v)
	s.byTenant[v.TenantID] = append(s.byTenant[v.TenantID], v)
	return v, nil
}

// GetByID retrieves a version by ID.
func (s *VersionStore) GetByID(id string) (*TemplateVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.versions[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *v
	cp.Snapshot = make(map[string]interface{})
	for k, val := range v.Snapshot {
		cp.Snapshot[k] = val
	}
	return &cp, nil
}

// GetByIDAndTenant retrieves a version by ID and verifies tenant ownership.
func (s *VersionStore) GetByIDAndTenant(id, tenantID string) (*TemplateVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.versions[id]
	if !ok || v.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *v
	cp.Snapshot = make(map[string]interface{})
	for k, val := range v.Snapshot {
		cp.Snapshot[k] = val
	}
	return &cp, nil
}

// ListByTemplate returns all versions for a template and tenant, ordered by creation time.
func (s *VersionStore) ListByTemplate(templateID, tenantID string) []TemplateVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vers, ok := s.byTemplate[templateID]
	if !ok || len(vers) == 0 {
		return nil
	}

	result := make([]TemplateVersion, 0, len(vers))
	for _, v := range vers {
		if v.TenantID != tenantID {
			continue
		}
		cp := *v
		cp.Snapshot = make(map[string]interface{})
		for k, val := range v.Snapshot {
			cp.Snapshot[k] = val
		}
		result = append(result, cp)
	}
	return result
}

// GetByVersion retrieves a version for a template by version string.
func (s *VersionStore) GetByVersion(templateID, version string) (*TemplateVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vers, ok := s.byTemplate[templateID]
	if !ok {
		return nil, ErrNotFound
	}

	for _, v := range vers {
		if v.Version == version {
			cp := *v
			cp.Snapshot = make(map[string]interface{})
			for k, val := range v.Snapshot {
				cp.Snapshot[k] = val
			}
			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

// CreateFromTemplate creates a version snapshot from a template with tenant isolation.
func (s *VersionStore) CreateFromTemplate(templateID, version, tenantID string, templateData map[string]interface{}) (*TemplateVersion, error) {
	snapshot := make(map[string]interface{})
	for k, v := range templateData {
		snapshot[k] = v
	}

	v := &TemplateVersion{
		TenantID:   tenantID,
		TemplateID: templateID,
		Version:    version,
		Snapshot:   snapshot,
	}

	return s.Create(v)
}
