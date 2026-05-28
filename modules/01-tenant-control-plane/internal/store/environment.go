package store

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Environment types ───────────────────────────────────────────────────────

// EnvironmentType represents the type of environment.
type EnvironmentType string

const (
	EnvironmentTypeProduction EnvironmentType = "production"
	EnvironmentTypeStaging    EnvironmentType = "staging"
	EnvironmentTypeDevelopment EnvironmentType = "development"
	EnvironmentTypeTesting    EnvironmentType = "testing"
)

// EnvironmentIsolationLevel represents the isolation strictness.
type EnvironmentIsolationLevel string

const (
	IsolationLevelFull    EnvironmentIsolationLevel = "full"
	IsolationLevelPartial EnvironmentIsolationLevel = "partial"
	IsolationLevelLogical EnvironmentIsolationLevel = "logical"
)

// EnvironmentState represents the current state of an environment.
type EnvironmentState string

const (
	EnvironmentStateCreating EnvironmentState = "creating"
	EnvironmentStateActive   EnvironmentState = "active"
	EnvironmentStateUpdating EnvironmentState = "updating"
	EnvironmentStateStopping EnvironmentState = "stopping"
	EnvironmentStateStopped  EnvironmentState = "stopped"
	EnvironmentStateError    EnvironmentState = "error"
)

// EnvironmentIsolationConfig contains the configuration for environment isolation.
type EnvironmentIsolationConfig struct {
	DataIsolation   bool                              `json:"data_isolation"`
	NetworkIsolation bool                             `json:"network_isolation"`
	ResourceQuota   EnvironmentResourceQuota          `json:"resource_quota"`
	BackupPolicy    EnvironmentBackupPolicy           `json:"backup_policy"`
	ComplianceLevel string                            `json:"compliance_level,omitempty"`
	Metadata        map[string]interface{}            `json:"metadata,omitempty"`
}

// EnvironmentResourceQuota defines resource limits for an environment.
type EnvironmentResourceQuota struct {
	MaxCPUs         int     `json:"max_cpus"`
	MaxMemoryGB     float64 `json:"max_memory_gb"`
	MaxStorageGB    float64 `json:"max_storage_gb"`
	MaxConnections  int     `json:"max_connections"`
	MaxDeployments  int     `json:"max_deployments"`
	MaxAgents       int     `json:"max_agents"`
}

// EnvironmentBackupPolicy defines backup configuration.
type EnvironmentBackupPolicy struct {
	Enabled              bool      `json:"enabled"`
	Frequency            string    `json:"frequency"` // e.g., "daily", "weekly"
	RetentionDays        int       `json:"retention_days"`
	EncryptionEnabled    bool      `json:"encryption_enabled"`
	BackupRegion         string    `json:"backup_region,omitempty"`
}

// Environment represents a tenant environment configuration.
type Environment struct {
	ID              string                       `json:"id"`
	TenantID        string                       `json:"tenant_id"`
	Name            string                       `json:"name"`
	Type            EnvironmentType              `json:"type"`
	State           EnvironmentState             `json:"state"`
	IsolationLevel  EnvironmentIsolationLevel    `json:"isolation_level"`
	IsolationConfig EnvironmentIsolationConfig   `json:"isolation_config"`
	Resources       []string                     `json:"resources,omitempty"` // IDs of resources in this environment
	NetworkConfig   map[string]interface{}       `json:"network_config,omitempty"`
	CreatedBy       string                       `json:"created_by,omitempty"`
	Notes           string                       `json:"notes,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
	ActivatedAt     *time.Time                   `json:"activated_at,omitempty"`
	DeactivatedAt   *time.Time                   `json:"deactivated_at,omitempty"`
}

// EnvironmentPatchRequest for partial updates.
type EnvironmentPatchRequest struct {
	Name            string                     `json:"name,omitempty"`
	Notes           string                     `json:"notes,omitempty"`
	IsolationLevel  EnvironmentIsolationLevel  `json:"isolation_level,omitempty"`
	IsolationConfig EnvironmentIsolationConfig `json:"isolation_config,omitempty"`
}

// ─── EnvironmentStore ────────────────────────────────────────────────────────

// EnvironmentStore manages tenant environments.
type EnvironmentStore struct {
	mu             sync.RWMutex
	environments   map[string]*Environment
	byTenant       map[string][]string // keyed by TenantID -> EnvironmentIDs
	byType         map[string][]string // keyed by Type -> EnvironmentIDs
}

// NewEnvironmentStore creates a new EnvironmentStore.
func NewEnvironmentStore() *EnvironmentStore {
	return &EnvironmentStore{
		environments: make(map[string]*Environment),
		byTenant:     make(map[string][]string),
		byType:       make(map[string][]string),
	}
}

// Create adds a new environment.
func (s *EnvironmentStore) Create(e *Environment) (*Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.State == "" {
		e.State = EnvironmentStateCreating
	}
	if e.IsolationLevel == "" {
		e.IsolationLevel = IsolationLevelLogical
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = timeNow()
	}
	e.UpdatedAt = timeNow()

	s.environments[e.ID] = e
	s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID], e.ID)
	s.byType[string(e.Type)] = append(s.byType[string(e.Type)], e.ID)

	return e, nil
}

// GetByID retrieves an environment by ID.
func (s *EnvironmentStore) GetByID(id string) (*Environment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.environments[id]
	if !ok {
		return nil, fmt.Errorf("environment %s not found", id)
	}
	cpy := *e
	return &cpy, nil
}

// Patch updates an environment.
func (s *EnvironmentStore) Patch(id string, req EnvironmentPatchRequest) (*Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.environments[id]
	if !ok {
		return nil, fmt.Errorf("environment %s not found", id)
	}

	if req.Name != "" {
		e.Name = req.Name
	}
	if req.Notes != "" {
		e.Notes = req.Notes
	}
	if req.IsolationLevel != "" {
		e.IsolationLevel = req.IsolationLevel
	}
	if req.IsolationConfig.DataIsolation || req.IsolationConfig.NetworkIsolation || req.IsolationConfig.ResourceQuota.MaxCPUs > 0 || req.IsolationConfig.ResourceQuota.MaxMemoryGB > 0 || req.IsolationConfig.ResourceQuota.MaxStorageGB > 0 || req.IsolationConfig.ResourceQuota.MaxConnections > 0 || req.IsolationConfig.ResourceQuota.MaxDeployments > 0 || req.IsolationConfig.ResourceQuota.MaxAgents > 0 || req.IsolationConfig.ComplianceLevel != "" || len(req.IsolationConfig.Metadata) > 0 {
		// Merge rather than replace
		if e.IsolationConfig.ResourceQuota.MaxCPUs == 0 && req.IsolationConfig.ResourceQuota.MaxCPUs > 0 {
			e.IsolationConfig.ResourceQuota.MaxCPUs = req.IsolationConfig.ResourceQuota.MaxCPUs
		}
		if e.IsolationConfig.ResourceQuota.MaxMemoryGB == 0 && req.IsolationConfig.ResourceQuota.MaxMemoryGB > 0 {
			e.IsolationConfig.ResourceQuota.MaxMemoryGB = req.IsolationConfig.ResourceQuota.MaxMemoryGB
		}
		if e.IsolationConfig.ResourceQuota.MaxStorageGB == 0 && req.IsolationConfig.ResourceQuota.MaxStorageGB > 0 {
			e.IsolationConfig.ResourceQuota.MaxStorageGB = req.IsolationConfig.ResourceQuota.MaxStorageGB
		}
		if e.IsolationConfig.ResourceQuota.MaxConnections == 0 && req.IsolationConfig.ResourceQuota.MaxConnections > 0 {
			e.IsolationConfig.ResourceQuota.MaxConnections = req.IsolationConfig.ResourceQuota.MaxConnections
		}
		if e.IsolationConfig.ResourceQuota.MaxDeployments == 0 && req.IsolationConfig.ResourceQuota.MaxDeployments > 0 {
			e.IsolationConfig.ResourceQuota.MaxDeployments = req.IsolationConfig.ResourceQuota.MaxDeployments
		}
		if e.IsolationConfig.ResourceQuota.MaxAgents == 0 && req.IsolationConfig.ResourceQuota.MaxAgents > 0 {
			e.IsolationConfig.ResourceQuota.MaxAgents = req.IsolationConfig.ResourceQuota.MaxAgents
		}
		if req.IsolationConfig.DataIsolation {
			e.IsolationConfig.DataIsolation = true
		}
		if req.IsolationConfig.NetworkIsolation {
			e.IsolationConfig.NetworkIsolation = true
		}
		if e.IsolationConfig.ComplianceLevel == "" && req.IsolationConfig.ComplianceLevel != "" {
			e.IsolationConfig.ComplianceLevel = req.IsolationConfig.ComplianceLevel
		}
	}

	e.UpdatedAt = timeNow()

	return e, nil
}

// Delete removes an environment.
func (s *EnvironmentStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.environments[id]
	if !ok {
		return fmt.Errorf("environment %s not found", id)
	}

	// Remove from byTenant
	tenantIDs, ok := s.byTenant[e.TenantID]
	if ok {
		idx := slices.Index(tenantIDs, id)
		if idx >= 0 {
			s.byTenant[e.TenantID] = append(tenantIDs[:idx], tenantIDs[idx+1:]...)
		}
	}

	// Remove from byType
	typeIDs, ok := s.byType[string(e.Type)]
	if ok {
		idx := slices.Index(typeIDs, id)
		if idx >= 0 {
			s.byType[string(e.Type)] = append(typeIDs[:idx], typeIDs[idx+1:]...)
		}
	}

	delete(s.environments, id)

	return nil
}

// ListByTenant returns all environments for a tenant.
func (s *EnvironmentStore) ListByTenant(tenantID string, page, pageSize int) ([]*Environment, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Environment, 0, len(ids))
	for _, id := range ids {
		e, ok := s.environments[id]
		if !ok {
			continue
		}
		cpy := *e
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Environment) int {
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

// CountTotal returns the total number of environments.
func (s *EnvironmentStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.environments)
}

// ListByType returns all environments of a specific type.
func (s *EnvironmentStore) ListByType(envType EnvironmentType, page, pageSize int) ([]*Environment, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byType[string(envType)]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Environment, 0, len(ids))
	for _, id := range ids {
		e, ok := s.environments[id]
		if !ok {
			continue
		}
		cpy := *e
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Environment) int {
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

// Activate transitions an environment to active state.
func (s *EnvironmentStore) Activate(id string) (*Environment, error) {
	return s.transition(id, EnvironmentStateActive, nil)
}

// Deactivate transitions an environment to stopped state.
func (s *EnvironmentStore) Deactivate(id string) (*Environment, error) {
	return s.transition(id, EnvironmentStateStopped, nil)
}

// transition performs a state transition on an environment.
func (s *EnvironmentStore) transition(id string, newState EnvironmentState, notes *string) (*Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.environments[id]
	if !ok {
		return nil, fmt.Errorf("environment %s not found", id)
	}

	if !canEnvironmentTransition(e.State, newState) {
		return nil, fmt.Errorf("invalid environment state transition from %s to %s", e.State, newState)
	}

	e.State = newState
	now := timeNow()
	if notes != nil && *notes != "" {
		e.Notes = *notes
	}
	if newState == EnvironmentStateActive {
		e.ActivatedAt = &now
	} else if newState == EnvironmentStateStopped {
		e.DeactivatedAt = &now
	}
	e.UpdatedAt = now

	return e, nil
}

// canEnvironmentTransition checks if a transition is valid.
func canEnvironmentTransition(from, to EnvironmentState) bool {
	switch from {
	case EnvironmentStateCreating:
		return to == EnvironmentStateActive || to == EnvironmentStateError
	case EnvironmentStateActive:
		return to == EnvironmentStateUpdating || to == EnvironmentStateStopping || to == EnvironmentStateError
	case EnvironmentStateUpdating:
		return to == EnvironmentStateActive || to == EnvironmentStateError
	case EnvironmentStateStopping:
		return to == EnvironmentStateStopped || to == EnvironmentStateError
	case EnvironmentStateStopped:
		return to == EnvironmentStateCreating || to == EnvironmentStateError
	case EnvironmentStateError:
		return to == EnvironmentStateCreating || to == EnvironmentStateStopped
	default:
		return false
	}
}
