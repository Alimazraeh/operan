package store

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Deployment types ────────────────────────────────────────────────────────

// DeploymentStatus represents a deployment lifecycle state.
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusProvisioning DeploymentStatus = "provisioning"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusRollingBack DeploymentStatus = "rolling_back"
	DeploymentStatusError      DeploymentStatus = "error"
	DeploymentStatusStopped    DeploymentStatus = "stopped"
	DeploymentStatusDeprecated DeploymentStatus = "deprecated"
)

// DeploymentStrategy represents how a deployment should be performed.
type DeploymentStrategy string

const (
	DeploymentStrategyRolling  DeploymentStrategy = "rolling"
	DeploymentStrategyBlueGreen DeploymentStrategy = "blue_green"
	DeploymentStrategyRecreate DeploymentStrategy = "recreate"
)

// Deployment represents a tenant deployment version.
type Deployment struct {
	ID             string               `json:"id"`
	TenantID       string               `json:"tenant_id"`
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	Status         DeploymentStatus     `json:"status"`
	Strategy       DeploymentStrategy   `json:"strategy"`
	Manifest       []byte               `json:"manifest"` // JSON manifest for the deployment
	DesiredState   map[string]interface{} `json:"desired_state"`
	CurrentState   map[string]interface{} `json:"current_state"`
	Error          string               `json:"error,omitempty"`
	ResourceRefs   []string             `json:"resource_refs,omitempty"` // IDs of resources managed by this deployment
	NamespaceID    string               `json:"namespace_id,omitempty"`
	PreviousID     *string              `json:"previous_id,omitempty"` // For rollback
	CreatedBy      string               `json:"created_by,omitempty"`
	Notes          string               `json:"notes,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	DeployedAt     *time.Time           `json:"deployed_at,omitempty"`
	DeprecatedAt   *time.Time           `json:"deprecated_at,omitempty"`
}

// DeploymentPatchRequest for partial updates.
type DeploymentPatchRequest struct {
	Name         string               `json:"name,omitempty"`
	Notes        string               `json:"notes,omitempty"`
	DesiredState map[string]interface{} `json:"desired_state,omitempty"`
	Status       DeploymentStatus     `json:"status,omitempty"`
}

// DeploymentTransitionRequest for status transitions.
type DeploymentTransitionRequest struct {
	NewStatus DeploymentStatus `json:"new_status"`
	Notes     string           `json:"notes,omitempty"`
}

// ─── DeploymentStore ─────────────────────────────────────────────────────────

// DeploymentStore manages tenant deployments.
type DeploymentStore struct {
	mu          sync.RWMutex
	deployments map[string]*Deployment
	byTenant    map[string][]string // keyed by TenantID -> DeploymentIDs
}

// NewDeploymentStore creates a new DeploymentStore.
func NewDeploymentStore() *DeploymentStore {
	return &DeploymentStore{
		deployments: make(map[string]*Deployment),
		byTenant:    make(map[string][]string),
	}
}

// Create adds a new deployment.
func (s *DeploymentStore) Create(d *Deployment) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.Status == "" {
		d.Status = DeploymentStatusPending
	}
	if d.Strategy == "" {
		d.Strategy = DeploymentStrategyRolling
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = timeNow()
	}
	d.UpdatedAt = timeNow()

	s.deployments[d.ID] = d
	s.byTenant[d.TenantID] = append(s.byTenant[d.TenantID], d.ID)

	return d, nil
}

// GetByID retrieves a deployment by ID.
func (s *DeploymentStore) GetByID(id string) (*Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}
	cpy := *d
	return &cpy, nil
}

// Patch updates a deployment.
func (s *DeploymentStore) Patch(id string, req DeploymentPatchRequest) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}

	if req.Name != "" {
		d.Name = req.Name
	}
	if req.Notes != "" {
		d.Notes = req.Notes
	}
	if req.DesiredState != nil {
		if d.DesiredState == nil {
			d.DesiredState = make(map[string]interface{})
		}
		for k, v := range req.DesiredState {
			d.DesiredState[k] = v
		}
	}
	if req.Status != "" {
		d.Status = req.Status
	}

	d.UpdatedAt = timeNow()

	return d, nil
}

// Delete removes a deployment.
func (s *DeploymentStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %s not found", id)
	}

	// Remove from byTenant
	tenantIDs, ok := s.byTenant[d.TenantID]
	if ok {
		idx := slices.Index(tenantIDs, id)
		if idx >= 0 {
			s.byTenant[d.TenantID] = append(tenantIDs[:idx], tenantIDs[idx+1:]...)
		}
	}

	delete(s.deployments, id)

	return nil
}

// ListByTenant returns all deployments for a tenant.
func (s *DeploymentStore) ListByTenant(tenantID string, page, pageSize int) ([]*Deployment, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Deployment, 0, len(ids))
	for _, id := range ids {
		d, ok := s.deployments[id]
		if !ok {
			continue
		}
		cpy := *d
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Deployment) int {
		return strings.Compare(a.Name, b.Name)
	})

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := items[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// CountTotal returns the total number of deployments.
func (s *DeploymentStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.deployments)
}

// Deploy transitions a deployment to running state.
func (s *DeploymentStore) Deploy(id string) (*Deployment, error) {
	return s.transition(id, DeploymentStatusRunning, nil)
}

// Stop transitions a deployment to stopped state.
func (s *DeploymentStore) Stop(id string) (*Deployment, error) {
	return s.transition(id, DeploymentStatusStopped, nil)
}

// Rollback creates a new deployment based on a previous version.
func (s *DeploymentStore) Rollback(deploymentID string, createdBy string) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.deployments[deploymentID]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", deploymentID)
	}

	if original.PreviousID == nil {
		return nil, fmt.Errorf("no previous deployment to rollback to for deployment %s", deploymentID)
	}

	prev, ok := s.deployments[*original.PreviousID]
	if !ok {
		return nil, fmt.Errorf("previous deployment %s not found", *original.PreviousID)
	}

	// Create rollback deployment
	rollback := &Deployment{
		TenantID:        original.TenantID,
		Name:            original.Name + "-rollback",
		Version:         prev.Version,
		Status:          DeploymentStatusPending,
		Strategy:        DeploymentStrategyRecreate,
		Manifest:        prev.Manifest,
		DesiredState:    prev.DesiredState,
		ResourceRefs:    prev.ResourceRefs,
		NamespaceID:     original.NamespaceID,
		PreviousID:      &deploymentID,
		CreatedBy:       createdBy,
		Notes:           fmt.Sprintf("Rollback from %s to %s", original.Version, prev.Version),
	}

	// Set ID and timestamps
	rollback.ID = uuid.New().String()
	rollback.CreatedAt = timeNow()
	rollback.UpdatedAt = timeNow()

	s.deployments[rollback.ID] = rollback
	s.byTenant[rollback.TenantID] = append(s.byTenant[rollback.TenantID], rollback.ID)

	return rollback, nil
}

// Deprecate marks a deployment as deprecated.
func (s *DeploymentStore) Deprecate(id string) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}

	now := timeNow()
	d.Status = DeploymentStatusDeprecated
	d.DeprecatedAt = &now
	d.UpdatedAt = now

	return d, nil
}

// transition performs a status transition on a deployment.
func (s *DeploymentStore) transition(id string, newStatus DeploymentStatus, notes *string) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[id]
	if !ok {
		return nil, fmt.Errorf("deployment %s not found", id)
	}

	if !canDeploymentTransition(d.Status, newStatus) {
		return nil, fmt.Errorf("invalid deployment status transition from %s to %s", d.Status, newStatus)
	}

	d.Status = newStatus
	if notes != nil && *notes != "" {
		d.Notes = *notes
	}
	if newStatus == DeploymentStatusRunning {
		now := timeNow()
		d.DeployedAt = &now
	}
	d.UpdatedAt = timeNow()

	return d, nil
}

// canDeploymentTransition checks if a transition is valid.
func canDeploymentTransition(from, to DeploymentStatus) bool {
	switch from {
	case DeploymentStatusPending:
		return to == DeploymentStatusProvisioning || to == DeploymentStatusError || to == DeploymentStatusStopped
	case DeploymentStatusProvisioning:
		return to == DeploymentStatusRunning || to == DeploymentStatusError || to == DeploymentStatusStopped
	case DeploymentStatusRunning:
		return to == DeploymentStatusRollingBack || to == DeploymentStatusStopped || to == DeploymentStatusError || to == DeploymentStatusDeprecated
	case DeploymentStatusRollingBack:
		return to == DeploymentStatusRunning || to == DeploymentStatusError || to == DeploymentStatusStopped
	case DeploymentStatusError:
		return to == DeploymentStatusPending || to == DeploymentStatusStopped || to == DeploymentStatusDeprecated
	case DeploymentStatusStopped:
		return to == DeploymentStatusPending || to == DeploymentStatusDeprecated
	case DeploymentStatusDeprecated:
		return false
	default:
		return false
	}
}
