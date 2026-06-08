// Package store provides in-memory data stores for the Agent Registry module.
// All stores enforce tenant isolation via tenant-scoped indexes.
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

// AgentStore provides thread-safe CRUD operations for Agent entities
// with tenant isolation.
type AgentStore struct {
	mu       sync.RWMutex
	agents   map[string]*Agent
	byTenant map[string]map[string]*Agent // tenant_id -> agent_id -> Agent
}

// NewAgentStore creates a new tenant-isolated agent store.
func NewAgentStore() *AgentStore {
	return &AgentStore{
		agents:   make(map[string]*Agent),
		byTenant: make(map[string]map[string]*Agent),
	}
}

// Create adds a new agent, enforcing tenant isolation.
func (s *AgentStore) Create(ctx context.Context, agent *Agent) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.agents[agent.ID]; ok && existing.TenantID == tenantID {
		return fmt.Errorf("agent %s already exists for this tenant", agent.ID)
	}

	now := timeNow()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	if agent.Status == "" {
		agent.Status = AgentStatusActive
	}

	s.agents[agent.ID] = agent

	if s.byTenant[tenantID] == nil {
		s.byTenant[tenantID] = make(map[string]*Agent)
	}
	s.byTenant[tenantID][agent.ID] = agent

	return nil
}

// GetByID retrieves an agent by ID with tenant isolation.
func (s *AgentStore) GetByID(ctx context.Context, id string) (*Agent, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantAgents := s.byTenant[tenantID]
	if agent, ok := tenantAgents[id]; ok {
		return agent, nil
	}
	return nil, fmt.Errorf("agent not found")
}

// Patch updates fields of an existing agent with tenant isolation.
func (s *AgentStore) Patch(ctx context.Context, id string, fn func(*Agent)) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantAgents := s.byTenant[tenantID]
	agent, ok := tenantAgents[id]
	if !ok {
		return fmt.Errorf("agent not found")
	}

	fn(agent)
	return nil
}

// List returns agents with tenant isolation, pagination, and optional filters.
func (s *AgentStore) List(ctx context.Context, role, status, capability string, page, pageSize int) ([]*Agent, int, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenant context required")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantAgents := s.byTenant[tenantID]
	var filtered []*Agent

	for _, a := range tenantAgents {
		if role != "" && a.Role != role {
			continue
		}
		if status != "" && string(a.Status) != status {
			continue
		}
		if capability != "" {
			found := false
			for _, c := range a.Capabilities {
				if c == capability {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, a)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		return []*Agent{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

// Delete removes an agent from the store with tenant isolation.
func (s *AgentStore) Delete(ctx context.Context, id string) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantAgents := s.byTenant[tenantID]
	if _, ok := tenantAgents[id]; !ok {
		return fmt.Errorf("agent not found")
	}

	delete(s.agents, id)
	delete(tenantAgents, id)

	return nil
}

// Exists checks if an agent exists for the given tenant.
func (s *AgentStore) Exists(ctx context.Context, id string) (bool, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return false, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.byTenant[tenantID][id]
	return ok, nil
}
