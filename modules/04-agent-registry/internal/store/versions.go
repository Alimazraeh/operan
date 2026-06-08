// Package store provides in-memory data stores for the Agent Registry module.
// This file contains the VersionStore with tenant isolation.
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

// VersionStore provides thread-safe CRUD operations for AgentVersion entities
// with tenant isolation and agent-scoped access.
type VersionStore struct {
	mu       sync.RWMutex
	versions map[string]*AgentVersion
	byTenant map[string]map[string]*AgentVersion // tenant_id -> version_id -> AgentVersion
	byAgent  map[string]map[string]*AgentVersion // agent_id -> version_id -> AgentVersion
}

// NewVersionStore creates a new tenant-isolated version store.
func NewVersionStore() *VersionStore {
	return &VersionStore{
		versions: make(map[string]*AgentVersion),
		byTenant: make(map[string]map[string]*AgentVersion),
		byAgent:  make(map[string]map[string]*AgentVersion),
	}
}

// Create adds a new version with tenant isolation.
func (s *VersionStore) Create(ctx context.Context, version *AgentVersion) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.versions[version.ID]; ok && existing.TenantID == tenantID {
		return fmt.Errorf("version %s already exists for this tenant", version.ID)
	}

	s.versions[version.ID] = version

	if s.byTenant[tenantID] == nil {
		s.byTenant[tenantID] = make(map[string]*AgentVersion)
	}
	s.byTenant[tenantID][version.ID] = version

	if s.byAgent[version.AgentID] == nil {
		s.byAgent[version.AgentID] = make(map[string]*AgentVersion)
	}
	s.byAgent[version.AgentID][version.ID] = version

	return nil
}

// GetByID retrieves a version by ID with tenant isolation.
func (s *VersionStore) GetByID(ctx context.Context, id string) (*AgentVersion, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantVersions := s.byTenant[tenantID]
	if version, ok := tenantVersions[id]; ok {
		return version, nil
	}
	return nil, fmt.Errorf("version not found")
}

// ListByAgent returns all versions for an agent within the tenant scope.
// Returns an error if the agent does not exist within the tenant scope.
func (s *VersionStore) ListByAgent(ctx context.Context, agentID string) ([]*AgentVersion, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check tenant isolation: verify the agent belongs to this tenant
	tenantVersions := s.byTenant[tenantID]
	agentExists := false
	for _, v := range tenantVersions {
		if v.AgentID == agentID {
			agentExists = true
			break
		}
	}
	if !agentExists {
		return nil, fmt.Errorf("agent not found")
	}

	agentVersions := s.byAgent[agentID]
	if agentVersions == nil {
		return []*AgentVersion{}, nil
	}

	result := make([]*AgentVersion, 0, len(agentVersions))
	for _, v := range agentVersions {
		// Verify tenant ownership
		if _, ok := tenantVersions[v.ID]; ok {
			result = append(result, v)
		}
	}
	return result, nil
}

// ListByAgentAndStatus returns versions for an agent filtered by status.
func (s *VersionStore) ListByAgentAndStatus(ctx context.Context, agentID string, status string) ([]*AgentVersion, error) {
	versions, err := s.ListByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	if status == "" {
		return versions, nil
	}

	result := make([]*AgentVersion, 0)
	for _, v := range versions {
		if string(v.Status) == status {
			result = append(result, v)
		}
	}
	return result, nil
}

// Patch updates fields of an existing version with tenant isolation.
func (s *VersionStore) Patch(ctx context.Context, id string, fn func(*AgentVersion)) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantVersions := s.byTenant[tenantID]
	version, ok := tenantVersions[id]
	if !ok {
		return fmt.Errorf("version not found")
	}

	fn(version)
	return nil
}

// SetPromoted marks a version as promoted to a specific environment.
// The promotedVersionID parameter is the version ID that represents
// the promoted target in this environment (typically the same as id).
func (s *VersionStore) SetPromoted(ctx context.Context, id, env, promotedVersionID string) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantVersions := s.byTenant[tenantID]
	version, ok := tenantVersions[id]
	if !ok {
		return fmt.Errorf("version not found")
	}

	if version.PromotedTo == nil {
		version.PromotedTo = make(map[string]string)
	}
	version.PromotedTo[env] = promotedVersionID
	return nil
}

// Exists checks if a version exists for the given tenant.
func (s *VersionStore) Exists(ctx context.Context, id string) (bool, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return false, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.byTenant[tenantID][id]
	return ok, nil
}
