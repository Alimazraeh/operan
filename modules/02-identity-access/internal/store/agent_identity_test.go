package store

import (
	"testing"

	"github.com/operan/modules/02-identity-access/internal/models"
)

func TestAgentIdentityStoreCreate(t *testing.T) {
	s := NewAgentIdentityStore()

	identity := &models.AgentIdentity{
		TenantID:  "tenant-1",
		AgentID:   "agent-123",
		Capabilities: []string{"read", "write"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if identity.ID == "" {
		t.Error("Create() should auto-generate identity ID")
	}
}

func TestAgentIdentityStoreCreateDuplicate(t *testing.T) {
	s := NewAgentIdentityStore()

	identity1 := &models.AgentIdentity{
		TenantID:  "tenant-1",
		AgentID:   "agent-123",
		Capabilities: []string{"read"},
	}
	if err := s.Create(identity1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	identity2 := &models.AgentIdentity{
		TenantID:  "tenant-1",
		AgentID:   "agent-123",
		Capabilities: []string{"write"},
	}
	err := s.Create(identity2)
	if err == nil {
		t.Error("Create() should return error for duplicate agent_id in same tenant")
	}
}

func TestAgentIdentityStoreCreateMissingFields(t *testing.T) {
	s := NewAgentIdentityStore()

	// Missing agent_id
	identity1 := &models.AgentIdentity{TenantID: "tenant-1"}
	if err := s.Create(identity1); err == nil {
		t.Error("Create() should error when agent_id is empty")
	}

	// Missing tenant_id
	identity2 := &models.AgentIdentity{AgentID: "agent-123"}
	if err := s.Create(identity2); err == nil {
		t.Error("Create() should error when tenant_id is empty")
	}
}

func TestAgentIdentityStoreGetByAgent(t *testing.T) {
	s := NewAgentIdentityStore()

	identity := &models.AgentIdentity{
		TenantID:        "tenant-1",
		AgentID:         "agent-456",
		Capabilities:    []string{"read", "execute"},
		MemoryScope:     []string{"project-a"},
		AllowedTools:    []string{"search", "api-call"},
		EscalationTargets: []string{"admin-1", "admin-2"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByAgent("agent-456")
	if err != nil {
		t.Fatalf("GetByAgent() error = %v", err)
	}
	if got.TenantID != "tenant-1" {
		t.Errorf("GetByAgent() tenant_id = %v, want tenant-1", got.TenantID)
	}
	if len(got.Capabilities) != 2 {
		t.Errorf("GetByAgent() capabilities = %v, want [read execute]", got.Capabilities)
	}
	if len(got.AllowedTools) != 2 {
		t.Errorf("GetByAgent() allowed_tools = %v, want [search api-call]", got.AllowedTools)
	}
}

func TestAgentIdentityStoreGetByAgentNotFound(t *testing.T) {
	s := NewAgentIdentityStore()
	_, err := s.GetByAgent("nonexistent")
	if err != ErrAgentIdentityNotFound {
		t.Errorf("GetByAgent() error = %v, want ErrAgentIdentityNotFound", err)
	}
}

func TestAgentIdentityStoreGetByID(t *testing.T) {
	s := NewAgentIdentityStore()

	identity := &models.AgentIdentity{
		TenantID:       "tenant-1",
		AgentID:        "agent-789",
		Capabilities:   []string{"execute"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.GetByID(identity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.AgentID != "agent-789" {
		t.Errorf("GetByID() agent_id = %v, want agent-789", got.AgentID)
	}
}

func TestAgentIdentityStoreGetByIDNotFound(t *testing.T) {
	s := NewAgentIdentityStore()
	_, err := s.GetByID("nonexistent")
	if err != ErrAgentIdentityNotFound {
		t.Errorf("GetByID() error = %v, want ErrAgentIdentityNotFound", err)
	}
}

func TestAgentIdentityStoreListByTenant(t *testing.T) {
	s := NewAgentIdentityStore()

	// Create 3 agents for tenant-1
	for i := 0; i < 3; i++ {
		identity := &models.AgentIdentity{
			TenantID:     "tenant-1",
			AgentID:      "agent-" + string(rune('a'+i)),
			Capabilities: []string{"read"},
		}
		if err := s.Create(identity); err != nil {
			t.Fatalf("Create(%d) error = %v", i, err)
		}
	}

	// Create 2 agents for tenant-2
	for i := 0; i < 2; i++ {
		identity := &models.AgentIdentity{
			TenantID:     "tenant-2",
			AgentID:      "tenant2-agent-" + string(rune('0'+i)),
			Capabilities: []string{"write"},
		}
		if err := s.Create(identity); err != nil {
			t.Fatalf("Create tenant-2(%d) error = %v", i, err)
		}
	}

	// List tenant-1 agents
	agents, err := s.ListByTenant("tenant-1")
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(agents) != 3 {
		t.Errorf("ListByTenant() tenant-1 count = %v, want 3", len(agents))
	}

	// List tenant-2 agents
	agents, err = s.ListByTenant("tenant-2")
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("ListByTenant() tenant-2 count = %v, want 2", len(agents))
	}
}

func TestAgentIdentityStoreListByTenantEmpty(t *testing.T) {
	s := NewAgentIdentityStore()

	agents, err := s.ListByTenant("nonexistent")
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("ListByTenant() count = %v, want 0", len(agents))
	}
}

func TestAgentIdentityStoreCreateStoresJSONFields(t *testing.T) {
	s := NewAgentIdentityStore()

	identity := &models.AgentIdentity{
		TenantID:          "tenant-1",
		AgentID:           "agent-json",
		Capabilities:      []string{"read", "write"},
		MemoryScope:       []string{"scope-a", "scope-b"},
		AllowedTools:      []string{"tool-1"},
		EscalationTargets: []string{"escalation-1"},
	}

	if err := s.Create(identity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if identity.CapabilitiesJSON == "" {
		t.Error("Create() should store capabilities as JSON")
	}
	if identity.MemoryScopeJSON == "" {
		t.Error("Create() should store memory_scope as JSON")
	}
	if identity.AllowedToolsJSON == "" {
		t.Error("Create() should store allowed_tools as JSON")
	}
	if identity.EscalationTargetsJSON == "" {
		t.Error("Create() should store escalation_targets as JSON")
	}
}
