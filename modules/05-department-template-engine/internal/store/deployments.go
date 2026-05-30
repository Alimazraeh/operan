package store

import (
	"sync"

	"github.com/google/uuid"
)

// DeploymentStore provides tenant-isolated CRUD for template deployments.
type DeploymentStore struct {
	mu          sync.RWMutex
	deployments map[string]*TemplateDeployment
	byTemplate  map[string][]string  // templateID -> []deploymentIDs
	byTenant    map[string][]string  // tenantID -> []deploymentIDs
}

// NewDeploymentStore creates a new empty DeploymentStore.
func NewDeploymentStore() *DeploymentStore {
	return &DeploymentStore{
		deployments: make(map[string]*TemplateDeployment),
		byTemplate:  make(map[string][]string),
		byTenant:    make(map[string][]string),
	}
}

// Create adds a new deployment with tenant isolation.
func (s *DeploymentStore) Create(d *TemplateDeployment) (*TemplateDeployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	d.CreatedAt = timeNow()
	d.UpdatedAt = d.CreatedAt
	if d.Status == "" {
		d.Status = "select"
	}

	s.deployments[d.ID] = d
	s.byTemplate[d.TemplateID] = append(s.byTemplate[d.TemplateID], d.ID)
	s.byTenant[d.TenantID] = append(s.byTenant[d.TenantID], d.ID)
	return d, nil
}

// GetByID retrieves a deployment by ID.
func (s *DeploymentStore) GetByID(id string) (*TemplateDeployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

// GetByIDAndTenant retrieves a deployment by ID and verifies tenant ownership.
func (s *DeploymentStore) GetByIDAndTenant(id, tenantID string) (*TemplateDeployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.deployments[id]
	if !ok || d.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

// GetByIDAndTemplate retrieves a deployment, verifying it belongs to the given template.
func (s *DeploymentStore) GetByIDAndTemplate(id, templateID string) (*TemplateDeployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.deployments[id]
	if !ok || d.TemplateID != templateID {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

// ListByTemplate returns deployments for a given template and tenant, with pagination.
func (s *DeploymentStore) ListByTemplate(templateID, tenantID string, page, pageSize int) ([]TemplateDeployment, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTemplate[templateID]
	if !ok {
		return nil, 0, false
	}

	var all []*TemplateDeployment
	for _, id := range ids {
		if d, exists := s.deployments[id]; exists && d.TenantID == tenantID {
			all = append(all, d)
		}
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

	result := make([]TemplateDeployment, 0, end-start)
	for i := start; i < end; i++ {
		cp := *all[i]
		result = append(result, cp)
	}

	return result, total, hasMore
}

// UpdateStatus updates a deployment's status and tracks timing.
func (s *DeploymentStore) UpdateStatus(id string, status string, updatedBy string) (*TemplateDeployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, ErrNotFound
	}

	d.Status = status
	d.UpdatedAt = timeNow()
	if updatedBy != "" {
		d.DeployedBy = updatedBy
	}

	if status == "operational" {
		now := timeNow()
		d.CompletedAt = &now
	} else if status == "failed" {
		now := timeNow()
		d.CompletedAt = &now
	}

	return d, nil
}

// Delete removes a deployment by ID.
func (s *DeploymentStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return ErrNotFound
	}

	delete(s.deployments, id)
	// Remove from byTemplate index
	if ids, exists := s.byTemplate[d.TemplateID]; exists {
		for i, tid := range ids {
			if tid == id {
				s.byTemplate[d.TemplateID] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}
	// Remove from byTenant index
	if ids, exists := s.byTenant[d.TenantID]; exists {
		for i, tid := range ids {
			if tid == id {
				s.byTenant[d.TenantID] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	return nil
}
