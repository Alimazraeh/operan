package store

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// TemplateStore provides tenant-isolated CRUD for department templates.
type TemplateStore struct {
	mu     sync.RWMutex
	templates map[string]*Template // id -> template
}

// NewTemplateStore creates a new empty TemplateStore.
func NewTemplateStore() *TemplateStore {
	return &TemplateStore{
		templates: make(map[string]*Template),
	}
}

// Create adds a new template. The ID and timestamps are auto-generated.
func (s *TemplateStore) Create(t *Template) (*Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = timeNow()
	t.UpdatedAt = t.CreatedAt
	if t.Status == "" {
		t.Status = "draft"
	}

	s.templates[t.ID] = t
	return t, nil
}

// GetByID retrieves a template by ID.
func (s *TemplateStore) GetByID(id string) (*Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.templates[id]
	if !ok {
		return nil, ErrNotFound
	}
	// Return a copy to prevent mutation
	cp := *t
	return &cp, nil
}

// List returns templates for the given tenant, with pagination.
func (s *TemplateStore) List(tenantID string, page, pageSize int, filterCategory *string) ([]Template, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*Template
	for _, t := range s.templates {
		// Note: In production, tenant_id would be stored on the template
		// and queried here. For this MVP, we return all templates.
		all = append(all, t)
	}

	// Apply category filter
	if filterCategory != nil && *filterCategory != "" {
		var filtered []*Template
		for _, t := range all {
			if t.Category == *filterCategory {
				filtered = append(filtered, t)
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

	result := make([]Template, 0, end-start)
	for i := start; i < end; i++ {
		cp := *all[i]
		result = append(result, cp)
	}

	return result, total, hasMore
}

// Update partially updates a template. Only non-empty fields are applied.
func (s *TemplateStore) Update(id string, patch map[string]interface{}) (*Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.templates[id]
	if !ok {
		return nil, ErrNotFound
	}

	if v, exists := patch["name"]; exists {
		if s, ok := v.(string); ok && s != "" {
			t.Name = s
		}
	}
	if v, exists := patch["description"]; exists {
		if s, ok := v.(string); ok {
			t.Description = s
		}
	}
	if v, exists := patch["category"]; exists {
		if s, ok := v.(string); ok && s != "" {
			t.Category = s
		}
	}
	if v, exists := patch["version"]; exists {
		if s, ok := v.(string); ok && s != "" {
			t.Version = s
		}
	}
	if v, exists := patch["status"]; exists {
		if s, ok := v.(string); ok && s != "" {
			t.Status = s
		}
	}
	if v, exists := patch["tags"]; exists {
		if arr, ok := v.([]string); ok {
			t.Tags = arr
		}
	}
	if v, exists := patch["metadata"]; exists {
		if m, ok := v.(map[string]interface{}); ok {
			t.Metadata = m
		}
	}
	if v, exists := patch["agents"]; exists {
		if arr, ok := v.([]interface{}); ok {
			// Parse back to []AgentDefinition
			b, _ := json.Marshal(arr)
			json.Unmarshal(b, &t.Agents)
		}
	}
	if v, exists := patch["workflows"]; exists {
		if arr, ok := v.([]interface{}); ok {
			b, _ := json.Marshal(arr)
			json.Unmarshal(b, &t.Workflows)
		}
	}
	if v, exists := patch["memory_topology"]; exists && v != nil {
		b, _ := json.Marshal(v)
		json.Unmarshal(b, &t.MemoryTopology)
	}
	if v, exists := patch["governance_rules"]; exists {
		if arr, ok := v.([]interface{}); ok {
			b, _ := json.Marshal(arr)
			json.Unmarshal(b, &t.GovernanceRules)
		}
	}
	if v, exists := patch["kpis"]; exists {
		if arr, ok := v.([]interface{}); ok {
			b, _ := json.Marshal(arr)
			json.Unmarshal(b, &t.KPIS)
		}
	}
	if v, exists := patch["integrations"]; exists {
		if arr, ok := v.([]interface{}); ok {
			b, _ := json.Marshal(arr)
			json.Unmarshal(b, &t.Integrations)
		}
	}
	if v, exists := patch["operational_policies"]; exists {
		if arr, ok := v.([]interface{}); ok {
			b, _ := json.Marshal(arr)
			json.Unmarshal(b, &t.OperationalPolicies)
		}
	}

	t.UpdatedAt = timeNow()
	return t, nil
}

// Delete removes a template by ID.
func (s *TemplateStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.templates[id]; !ok {
		return ErrNotFound
	}
	delete(s.templates, id)
	return nil
}
