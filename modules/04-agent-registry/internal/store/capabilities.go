// Package store provides in-memory data stores for the Agent Registry module.
// This file contains the CapabilityStore with tenant isolation.
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

// CapabilityStore provides thread-safe CRUD operations for capability
// entries with tenant isolation.
type CapabilityStore struct {
	mu       sync.RWMutex
	entries  map[string][]*CapabilityEntry // agent_id -> [capability entries]
	byTenant map[string]map[string]bool    // tenant_id -> agent_id -> exists
}

// NewCapabilityStore creates a new tenant-isolated capability store.
func NewCapabilityStore() *CapabilityStore {
	return &CapabilityStore{
		entries:  make(map[string][]*CapabilityEntry),
		byTenant: make(map[string]map[string]bool),
	}
}

// Get returns the first capability entry for an agent within tenant scope.
// For listing all capabilities, use ListAll.
func (s *CapabilityStore) Get(ctx context.Context, agentID string) (*CapabilityEntry, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.byTenant[tenantID][agentID] {
		return nil, fmt.Errorf("capabilities not found for agent")
	}
	entries := s.entries[agentID]
	if len(entries) == 0 {
		return nil, fmt.Errorf("capabilities not found for agent")
	}
	return entries[0], nil
}

// Upsert adds a capability entry for an agent within tenant scope.
func (s *CapabilityStore) Upsert(ctx context.Context, entry *CapabilityEntry) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byTenant[tenantID] == nil {
		s.byTenant[tenantID] = make(map[string]bool)
	}

	s.entries[entry.AgentID] = append(s.entries[entry.AgentID], entry)
	s.byTenant[tenantID][entry.AgentID] = true

	return nil
}

// ListAll returns all capability entries for an agent within tenant scope.
// Returns an error if the agent does not exist within the tenant scope.
func (s *CapabilityStore) ListAll(ctx context.Context, agentID string) ([]*CapabilityEntry, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.byTenant[tenantID][agentID] {
		return nil, fmt.Errorf("agent not found")
	}
	return s.entries[agentID], nil
}

// Index marks the last evaluation time for all of an agent's capabilities.
func (s *CapabilityStore) Index(ctx context.Context, agentID string) error {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant context required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.byTenant[tenantID][agentID] {
		return fmt.Errorf("capabilities not found for agent")
	}
	entries := s.entries[agentID]
	for _, entry := range entries {
		entry.LastEvaluated = timeNow()
	}
	return nil
}

// Exists checks if capabilities exist for the given tenant and agent.
func (s *CapabilityStore) Exists(ctx context.Context, agentID string) (bool, error) {
	tenantID := ctxkeys.GetTenantID(ctx)
	if tenantID == "" {
		return false, fmt.Errorf("tenant context required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byTenant[tenantID][agentID], nil
}
