package store

import (
	"sync"

	"github.com/google/uuid"
)

// VersionStore provides immutable version storage for templates.
type VersionStore struct {
	mu        sync.RWMutex
	versions  map[string]*TemplateVersion // id -> version
	byTemplate map[string][]*TemplateVersion // templateID -> versions
}

// NewVersionStore creates a new empty VersionStore.
func NewVersionStore() *VersionStore {
	return &VersionStore{
		versions:   make(map[string]*TemplateVersion),
		byTemplate: make(map[string][]*TemplateVersion),
	}
}

// Create adds a new immutable version snapshot.
func (s *VersionStore) Create(v *TemplateVersion) (*TemplateVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	v.CreatedAt = timeNow()

	s.versions[v.ID] = v
	s.byTemplate[v.TemplateID] = append(s.byTemplate[v.TemplateID], v)
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

// ListByTemplate returns all versions for a template, ordered by creation time.
func (s *VersionStore) ListByTemplate(templateID string) []TemplateVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vers, ok := s.byTemplate[templateID]
	if !ok || len(vers) == 0 {
		return nil
	}

	result := make([]TemplateVersion, 0, len(vers))
	for _, v := range vers {
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

// CreateFromTemplate creates a version snapshot from a template.
func (s *VersionStore) CreateFromTemplate(templateID, version string, templateData map[string]interface{}) (*TemplateVersion, error) {
	snapshot := make(map[string]interface{})
	for k, v := range templateData {
		snapshot[k] = v
	}

	v := &TemplateVersion{
		TemplateID: templateID,
		Version:    version,
		Snapshot:   snapshot,
	}

	return s.Create(v)
}
