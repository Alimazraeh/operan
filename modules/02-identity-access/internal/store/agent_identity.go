package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ErrAgentIdentityNotFound is returned when an agent identity is not found.
var ErrAgentIdentityNotFound = errors.New("agent identity not found")

// AgentIdentityStore provides in-memory CRUD operations for agent identities with tenant isolation.
type AgentIdentityStore struct {
	mu        sync.Mutex // upgraded to full mutex for atomic create-lookup
	idByAgent map[string]*models.AgentIdentity // key: agentID
	idByID    map[string]*models.AgentIdentity   // key: agent identity ID
}

// NewAgentIdentityStore creates a new in-memory agent identity store.
func NewAgentIdentityStore() *AgentIdentityStore {
	return &AgentIdentityStore{
		idByAgent: make(map[string]*models.AgentIdentity),
		idByID:    make(map[string]*models.AgentIdentity),
	}
}

// Create creates a new agent identity.
func (s *AgentIdentityStore) Create(identity *models.AgentIdentity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	if identity.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if identity.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}

	// Store capabilities as JSON (before lock, to validate)
	if len(identity.Capabilities) > 0 {
		data, err := json.Marshal(identity.Capabilities)
		if err != nil {
			return fmt.Errorf("marshal capabilities: %w", err)
		}
		identity.CapabilitiesJSON = string(data)
	}

	// Store memory scope as JSON
	if len(identity.MemoryScope) > 0 {
		data, err := json.Marshal(identity.MemoryScope)
		if err != nil {
			return fmt.Errorf("marshal memory scope: %w", err)
		}
		identity.MemoryScopeJSON = string(data)
	}

	// Store allowed tools as JSON
	if len(identity.AllowedTools) > 0 {
		data, err := json.Marshal(identity.AllowedTools)
		if err != nil {
			return fmt.Errorf("marshal allowed tools: %w", err)
		}
		identity.AllowedToolsJSON = string(data)
	}

	// Store escalation targets as JSON
	if len(identity.EscalationTargets) > 0 {
		data, err := json.Marshal(identity.EscalationTargets)
		if err != nil {
			return fmt.Errorf("marshal escalation targets: %w", err)
		}
		identity.EscalationTargetsJSON = string(data)
	}

	identity.CreatedAt = time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check uniqueness per tenant inside the lock to prevent race
	if existing, exists := s.idByAgent[identity.AgentID]; exists && existing.TenantID == identity.TenantID {
		return fmt.Errorf("agent identity for agent %s already exists in tenant %s", identity.AgentID, identity.TenantID)
	}

	s.idByID[identity.ID] = identity
	s.idByAgent[identity.AgentID] = identity

	return nil
}

// GetByAgent retrieves an agent identity by agent ID within a tenant.
func (s *AgentIdentityStore) GetByAgent(agentID string) (*models.AgentIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, exists := s.idByAgent[agentID]
	if !exists {
		return nil, ErrAgentIdentityNotFound
	}

	result := *identity
	result.Capabilities = unmarshalString(identity.CapabilitiesJSON)
	result.MemoryScope = unmarshalString(identity.MemoryScopeJSON)
	result.AllowedTools = unmarshalString(identity.AllowedToolsJSON)
	result.EscalationTargets = unmarshalString(identity.EscalationTargetsJSON)
	return &result, nil
}

// GetByID retrieves an agent identity by ID.
func (s *AgentIdentityStore) GetByID(id string) (*models.AgentIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, exists := s.idByID[id]
	if !exists {
		return nil, ErrAgentIdentityNotFound
	}

	result := *identity
	result.Capabilities = unmarshalString(identity.CapabilitiesJSON)
	result.MemoryScope = unmarshalString(identity.MemoryScopeJSON)
	result.AllowedTools = unmarshalString(identity.AllowedToolsJSON)
	result.EscalationTargets = unmarshalString(identity.EscalationTargetsJSON)
	return &result, nil
}

// ListByTenant returns all agent identities for a tenant.
func (s *AgentIdentityStore) ListByTenant(tenantID string) ([]models.AgentIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []models.AgentIdentity
	for _, identity := range s.idByID {
		if identity.TenantID == tenantID {
			r := *identity
			r.Capabilities = unmarshalString(identity.CapabilitiesJSON)
			r.MemoryScope = unmarshalString(identity.MemoryScopeJSON)
			r.AllowedTools = unmarshalString(identity.AllowedToolsJSON)
			r.EscalationTargets = unmarshalString(identity.EscalationTargetsJSON)
			result = append(result, r)
		}
	}

	return result, nil
}
