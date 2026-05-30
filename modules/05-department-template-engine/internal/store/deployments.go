package store

import (
	"sync"

	"github.com/google/uuid"
)

// DeploymentStore provides CRUD for template deployments.
type DeploymentStore struct {
	mu          sync.RWMutex
	deployments map[string]*TemplateDeployment
}

// NewDeploymentStore creates a new empty DeploymentStore.
func NewDeploymentStore() *DeploymentStore {
	return &DeploymentStore{
		deployments: make(map[string]*TemplateDeployment),
	}
}

// Create adds a new deployment.
func (s *DeploymentStore) Create(d *TemplateDeployment) (*TemplateDeployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.CreatedAt = timeNow()
	d.UpdatedAt = d.CreatedAt
	if d.Status == "" {
		d.Status = "select"
	}

	s.deployments[d.ID] = d
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

// ListByTemplate returns deployments for a given template, with pagination.
func (s *DeploymentStore) ListByTemplate(templateID string, page, pageSize int) ([]TemplateDeployment, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*TemplateDeployment
	for _, d := range s.deployments {
		if d.TemplateID == templateID {
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

	if _, ok := s.deployments[id]; !ok {
		return ErrNotFound
	}
	delete(s.deployments, id)
	return nil
}
