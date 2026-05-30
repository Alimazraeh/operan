package store

import (
	"sync"

	"github.com/google/uuid"
)

// CustomTemplateStore provides tenant-isolated CRUD for custom templates.
type CustomTemplateStore struct {
	mu       sync.RWMutex
	templates map[string]*CustomTemplate
}

// NewCustomTemplateStore creates a new empty CustomTemplateStore.
func NewCustomTemplateStore() *CustomTemplateStore {
	return &CustomTemplateStore{
		templates: make(map[string]*CustomTemplate),
	}
}

// Create adds a new custom template.
func (s *CustomTemplateStore) Create(ct *CustomTemplate) (*CustomTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ct.ID == "" {
		ct.ID = uuid.New().String()
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

// List returns custom templates for the given tenant, with pagination.
func (s *CustomTemplateStore) List(tenantID string, page, pageSize int, filterCategory *string) ([]CustomTemplate, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*CustomTemplate
	for _, ct := range s.templates {
		all = append(all, ct)
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
func (s *CustomTemplateStore) Update(id string, patch map[string]interface{}) (*CustomTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ct, ok := s.templates[id]
	if !ok {
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

// Delete removes a custom template by ID.
func (s *CustomTemplateStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.templates[id]; !ok {
		return ErrNotFound
	}
	delete(s.templates, id)
	return nil
}
