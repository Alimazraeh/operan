// Package store provides in-memory data stores for the Agent Registry module.
// This file contains the DependencyStore with tenant isolation.
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

// DependencyStore provides thread-safe CRUD operations for agent dependencies
// with tenant isolation.
type DependencyStore struct {
	mu       sync.RWMutex
	deps     map[string]*AgentDependency // id -> dependency
	byTenant map[string]map[string]*AgentDependency // tenant_id -> dep_id -> AgentDependency
	byAgent  map[string]map[string]*AgentDependency // agent_id -> dep_id -> AgentDependency
}

// NewDependencyStore creates a new tenant-isolated dependency store.
func NewDependencyStore() *DependencyStore {
	return &DependencyStore{
		deps:     make(map[string]*AgentDependency),
		byTenant: make(map[string]map[string]*AgentDependency),
		byAgent:  make(map[string]map[string]*AgentDependency),
	}
}

// Add adds a dependency with tenant isolation.
func (s *DependencyStore) Add(ctx context.Context, dep *AgentDependency) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byTenant[tenantID] == nil {
		s.byTenant[tenantID] = make(map[string]*AgentDependency)
	}

	// Check for duplicate
	if existing, ok := s.deps[dep.ID]; ok && existing.TenantID == tenantID {
		return fmt.Errorf("dependency %s already exists for this tenant", dep.ID)
	}

	s.deps[dep.ID] = dep
	s.byTenant[tenantID][dep.ID] = dep

	if s.byAgent[dep.AgentID] == nil {
		s.byAgent[dep.AgentID] = make(map[string]*AgentDependency)
	}
	s.byAgent[dep.AgentID][dep.ID] = dep

	return nil
}

// GetByID retrieves a dependency by ID with tenant isolation.
func (s *DependencyStore) GetByID(ctx context.Context, id string) (*AgentDependency, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantDeps := s.byTenant[tenantID]
	if dep, ok := tenantDeps[id]; ok {
		return dep, nil
	}
	return nil, fmt.Errorf("dependency not found")
}

// ListByAgent returns all dependencies for an agent within tenant scope.
// Returns an error if the agent is not found within the tenant scope.
func (s *DependencyStore) ListByAgent(ctx context.Context, agentID string) ([]*AgentDependency, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Cross-reference through tenant to verify agent exists
	tenantDeps := s.byTenant[tenantID]
	agentExists := false
	for _, dep := range tenantDeps {
		if dep.AgentID == agentID {
			agentExists = true
			break
		}
	}
	if !agentExists {
		return nil, fmt.Errorf("agent not found")
	}

	var result []*AgentDependency
	for _, dep := range tenantDeps {
		if dep.AgentID == agentID {
			result = append(result, dep)
		}
	}
	return result, nil
}

// Remove deletes a dependency within tenant scope.
func (s *DependencyStore) Remove(ctx context.Context, id string) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantDeps := s.byTenant[tenantID]
	dep, ok := tenantDeps[id]
	if !ok {
		return fmt.Errorf("dependency not found")
	}

	delete(s.deps, id)
	delete(tenantDeps, id)

	// Clean up byAgent index
	if agentDeps, ok := s.byAgent[dep.AgentID]; ok {
		delete(agentDeps, id)
	}

	return nil
}

// Exists checks if a dependency exists for the given tenant.
func (s *DependencyStore) Exists(ctx context.Context, id string) (bool, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return false, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.byTenant[tenantID][id]
	return ok, nil
}
