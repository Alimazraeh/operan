package store

import (
	"sync"

	"github.com/google/uuid"
)

// CustomTemplateStore provides tenant-isolated CRUD for custom templates.
type CustomTemplateStore struct {
	mu       sync.RWMutex
	templates map[string]*CustomTemplate
	byTenant  map[string][]string  // tenantID -> []customTemplateIDs
}

// NewCustomTemplateStore creates a new empty CustomTemplateStore.
func NewCustomTemplateStore() *CustomTemplateStore {
	return &CustomTemplateStore{
		templates: make(map[string]*CustomTemplate),
		byTenant:  make(map[string][]string),
	}
}

// Create adds a new custom template with tenant isolation.
func (s *CustomTemplateStore) Create(ct *CustomTemplate) (*CustomTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ct.ID == "" {
		ct.ID = uuid.New().String()
	}
	if ct.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	ct.CreatedAt = timeNow()
	ct.UpdatedAt = ct.CreatedAt
	if ct.Status == "" {
		ct.Status = "draft"
	}
	if ct.Content == nil {
		ct.Content = make(map[string]interface{})
	}

	s.templates[ct.ID] = ct
	s.byTenant[ct.TenantID] = append(s.byTenant[ct.TenantID], ct.ID)
	return ct, nil
}

// GetByID retrieves a custom template by ID.
func (s *CustomTemplateStore) GetByID(id string) (*CustomTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ct, ok := s.templates[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *ct
	if ct.Content != nil {
		cp.Content = make(map[string]interface{})
		for k, v := range ct.Content {
			cp.Content[k] = v
		}
	}
	return &cp, nil
}

// GetByIDAndTenant retrieves a custom template by ID, verifying tenant ownership.
func (s *CustomTemplateStore) GetByIDAndTenant(id, tenantID string) (*CustomTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ct, ok := s.templates[id]
	if !ok {
		return nil, ErrNotFound
	}
	if ct.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *ct
	if ct.Content != nil {
		cp.Content = make(map[string]interface{})
		for k, v := range ct.Content {
			cp.Content[k] = v
		}
	}
	return &cp, nil
}

// List returns custom templates for the given tenant, with pagination.
func (s *CustomTemplateStore) List(tenantID string, page, pageSize int, filterCategory *string) ([]CustomTemplate, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	var all []*CustomTemplate
	for _, id := range ids {
		if ct, exists := s.templates[id]; exists {
			all = append(all, ct)
		}
	}

	if filterCategory != nil && *filterCategory != "" {
		var filtered []*CustomTemplate
		for _, ct := range all {
			if ct.Category == *filterCategory {
				filtered = append(filtered, ct)
			}
		}
		all = filtered
	}

	total := len(all)
	hasMore := false
	start := (page - 1) * pageSize
	end := start + pageSize

	if end > total {
		end = total
	}
	if end < total {
		hasMore = true
	}

	result := make([]CustomTemplate, 0, end-start)
	for i := start; i < end; i++ {
		cp := *all[i]
		if all[i].Content != nil {
			cp.Content = make(map[string]interface{})
			for k, v := range all[i].Content {
				cp.Content[k] = v
			}
		}
		result = append(result, cp)
	}

	return result, total, hasMore
}

// Update partially updates a custom template.
// NOTE: This method does NOT verify tenant ownership. Use UpdateByTenant for
// handlers that need tenant isolation.
func (s *CustomTemplateStore) Update(id string, patch map[string]interface{}) (*CustomTemplate, error) {
	return s.UpdateByTenant(id, "", patch)
}

// UpdateByTenant partially updates a custom template with tenant verification.
func (s *CustomTemplateStore) UpdateByTenant(id, tenantID string, patch map[string]interface{}) (*CustomTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ct, ok := s.templates[id]
	if !ok {
		return nil, ErrNotFound
	}
	if ct.TenantID != tenantID {
		return nil, ErrNotFound
	}

	if v, exists := patch["name"]; exists {
		if s, ok := v.(string); ok && s != "" {
			ct.Name = s
		}
	}
	if v, exists := patch["description"]; exists {
		if s, ok := v.(string); ok {
			ct.Description = s
		}
	}
	if v, exists := patch["category"]; exists {
		if s, ok := v.(string); ok && s != "" {
			ct.Category = s
		}
	}
	if v, exists := patch["content"]; exists && v != nil {
		if m, ok := v.(map[string]interface{}); ok {
			ct.Content = m
		}
	}
	if v, exists := patch["shared_with"]; exists {
		if arr, ok := v.([]string); ok {
			ct.SharedWith = arr
		}
	}
	if v, exists := patch["status"]; exists {
		if s, ok := v.(string); ok && s != "" {
			ct.Status = s
		}
	}

	ct.UpdatedAt = timeNow()
	return ct, nil
}

// Delete removes a custom template by ID, verifying tenant ownership.
func (s *CustomTemplateStore) Delete(id, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ct, ok := s.templates[id]
	if !ok {
		return ErrNotFound
	}
	if ct.TenantID != tenantID {
		return ErrNotFound
	}

	delete(s.templates, id)
	// Remove from byTenant index
	if ids, exists := s.byTenant[tenantID]; exists {
		for i, tid := range ids {
			if tid == id {
				s.byTenant[tenantID] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	return nil
}
