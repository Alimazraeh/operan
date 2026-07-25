// Package store provides in-memory data stores for the Agent Registry module.
// All stores enforce tenant isolation via tenant-scoped indexes.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

// unmarshalJSON is shared with persist.go; it exists so this package does not
// import encoding/json in three places for one call each.
func unmarshalJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

// AgentStore provides thread-safe CRUD operations for Agent entities
// with tenant isolation.
type AgentStore struct {
	mu       sync.RWMutex
	agents   map[string]*Agent
	byTenant map[string]map[string]*Agent // tenant_id -> agent_id -> Agent
	// sink, when set by Persist, makes writes durable. Nil means memory only.
	sink *sink
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
	s.save(ctx, agent)

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
	agent.UpdatedAt = timeNow()
	s.save(ctx, agent)
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

	// Map iteration order is random, so slicing it for pagination returned
	// overlapping and missing agents between calls to page 1 and page 2.
	// Oldest first, id breaking ties, so paging is stable.
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		}
		return filtered[i].ID < filtered[j].ID
	})

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

	if s.sink != nil {
		if err := s.sink.db.DeleteAgent(ctx, tenantID, id); err != nil {
			// The agent is gone from memory, so the request succeeded; the row
			// surviving would resurrect it at the next restart.
			return fmt.Errorf("agent deleted but its record remains: %w", err)
		}
	}
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
