package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/operan/modules/04-agent-registry/internal/ctxkeys"
)

func uid() string      { return uuid.New().String() }
func ctx() context.Context { return context.Background() }
func ctxWithTenant(ctx context.Context, tenantID string) context.Context {
	return ctxkeys.SetTenantID(ctx, tenantID)
}

func TestAgentStore_CreateAndGetByID(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	agent := &Agent{
		ID:         uid(),
		Name:       "Test Agent",
		Role:       "coder",
		TenantID:   tenantID,
		Capabilities: []string{"code_generation", "testing"},
	}

	if err := store.Create(ctxT, agent); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if agent.ID == "" {
		t.Fatal("created agent should have an ID")
	}
	if agent.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if agent.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	found, err := store.GetByID(ctxT, agent.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found.Name != "Test Agent" {
		t.Errorf("expected name 'Test Agent', got %q", found.Name)
	}
	if found.TenantID != "tenant-1" {
		t.Errorf("expected tenant 'tenant-1', got %q", found.TenantID)
	}

	// Different tenant should NOT see this agent
	ctxOther := ctxWithTenant(ctx(), "tenant-2")
	_, err = store.GetByID(ctxOther, agent.ID)
	if err == nil {
		t.Fatal("expected error for other tenant, got nil")
	}
}

func TestAgentStore_Create_DuplicateName(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	agent1 := &Agent{ID: uid(), Name: "Dup Agent", Role: "researcher", TenantID: tenantID, Capabilities: []string{"research"}}
	agent2 := &Agent{ID: uid(), Name: "Dup Agent", Role: "researcher", TenantID: tenantID, Capabilities: []string{"research"}}

	if err := store.Create(ctxT, agent1); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if err := store.Create(ctxT, agent2); err != nil {
		t.Fatalf("second create failed: %v", err)
	}

	if agent1.ID == agent2.ID {
		t.Fatal("expected different IDs for created agents")
	}
}

func TestAgentStore_Create_DuplicateAcrossTenants(t *testing.T) {
	store := NewAgentStore()

	// Same name, different tenants — both should succeed
	tenant1 := ctxWithTenant(ctx(), "tenant-1")
	tenant2 := ctxWithTenant(ctx(), "tenant-2")

	agent1 := &Agent{ID: uid(), Name: "Dup Agent", Role: "researcher", TenantID: "tenant-1", Capabilities: []string{"research"}}
	agent2 := &Agent{ID: uid(), Name: "Dup Agent", Role: "researcher", TenantID: "tenant-2", Capabilities: []string{"research"}}

	if err := store.Create(tenant1, agent1); err != nil {
		t.Fatalf("create in tenant-1 failed: %v", err)
	}
	if err := store.Create(tenant2, agent2); err != nil {
		t.Fatalf("create in tenant-2 failed: %v", err)
	}

	if agent1.ID == agent2.ID {
		t.Fatal("expected different IDs across tenants")
	}

	// Verify tenant isolation
	found1, _ := store.GetByID(tenant1, agent1.ID)
	if found1.TenantID != "tenant-1" {
		t.Errorf("tenant-1 agent should have tenant-1")
	}
	found2, _ := store.GetByID(tenant2, agent2.ID)
	if found2.TenantID != "tenant-2" {
		t.Errorf("tenant-2 agent should have tenant-2")
	}
}

func TestAgentStore_GetByID_NotFound(t *testing.T) {
	store := NewAgentStore()
	ctxT := ctxWithTenant(ctx(), "tenant-1")
	_, err := store.GetByID(ctxT, "non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}
}

func TestAgentStore_Patch(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	agent := &Agent{ID: uid(), Name: "Patch Agent", Role: "coder", TenantID: tenantID, Capabilities: []string{"coding"}}
	if err := store.Create(ctxT, agent); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err := store.Patch(ctxT, agent.ID, func(a *Agent) {
		a.Name = "Patched Agent"
		a.Role = "fullstack"
		a.Status = AgentStatusActive
	})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}

	found, err := store.GetByID(ctxT, agent.ID)
	if err != nil {
		t.Fatalf("GetByID after Patch failed: %v", err)
	}
	if found.Name != "Patched Agent" {
		t.Errorf("expected name 'Patched Agent', got %q", found.Name)
	}
	if found.Role != "fullstack" {
		t.Errorf("expected role 'fullstack', got %q", found.Role)
	}
	if found.Status != AgentStatusActive {
		t.Errorf("expected status Active, got %q", found.Status)
	}
}

func TestAgentStore_Patch_NotFound(t *testing.T) {
	store := NewAgentStore()
	ctxT := ctxWithTenant(ctx(), "tenant-1")
	err := store.Patch(ctxT, "non-existent-id", func(a *Agent) { a.Name = "x" })
	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}
}

func TestAgentStore_List(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	for i := 0; i < 5; i++ {
		agent := &Agent{ID: uid(), Name: "Agent-A" + string(rune('A'+i)), Role: "coder", TenantID: tenantID, Capabilities: []string{"coding"}}
		if err := store.Create(ctxT, agent); err != nil {
			t.Fatalf("Create %d failed: %v", i, err)
		}
	}

	agents, total, err := store.List(ctxT, "", "", "", 1, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(agents) != 5 {
		t.Errorf("expected 5 agents, got %d", len(agents))
	}

	// Page 1 with size 2
	agents2, total2, err := store.List(ctxT, "", "", "", 1, 2)
	if err != nil {
		t.Fatalf("List page failed: %v", err)
	}
	if len(agents2) != 2 {
		t.Errorf("expected 2 agents on page 1, got %d", len(agents2))
	}
	if total2 != 5 {
		t.Errorf("expected total 5, got %d", total2)
	}
}

func TestAgentStore_List_ByRole(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	store.Create(ctxT, &Agent{ID: uid(), Name: "A1", Role: "coder", TenantID: tenantID, Capabilities: []string{"code"}})
	store.Create(ctxT, &Agent{ID: uid(), Name: "A2", Role: "coder", TenantID: tenantID, Capabilities: []string{"code"}})
	store.Create(ctxT, &Agent{ID: uid(), Name: "A3", Role: "researcher", TenantID: tenantID, Capabilities: []string{"research"}})

	agents, _, err := store.List(ctxT, "coder", "", "", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 coder agents, got %d", len(agents))
	}

	agents2, _, err := store.List(ctxT, "researcher", "", "", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents2) != 1 {
		t.Errorf("expected 1 researcher agent, got %d", len(agents2))
	}
}

func TestAgentStore_List_ByStatus(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	store.Create(ctxT, &Agent{ID: uid(), Name: "Active", Role: "coder", TenantID: tenantID, Capabilities: []string{"code"}})
	store.Create(ctxT, &Agent{ID: uid(), Name: "Inactive", Role: "coder", TenantID: tenantID, Capabilities: []string{"code"}, Status: AgentStatusInactive})

	agents, _, err := store.List(ctxT, "", "active", "", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 active agent, got %d", len(agents))
	}

	agents2, _, err := store.List(ctxT, "", "inactive", "", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents2) != 1 {
		t.Errorf("expected 1 inactive agent, got %d", len(agents2))
	}
}

func TestAgentStore_List_ByCapability(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	store.Create(ctxT, &Agent{ID: uid(), Name: "A1", Role: "coder", TenantID: tenantID, Capabilities: []string{"code", "testing"}})
	store.Create(ctxT, &Agent{ID: uid(), Name: "A2", Role: "coder", TenantID: tenantID, Capabilities: []string{"research"}})

	agents, _, err := store.List(ctxT, "", "", "code", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent with capability 'code', got %d", len(agents))
	}

	agents2, _, err := store.List(ctxT, "", "", "research", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents2) != 1 {
		t.Errorf("expected 1 agent with capability 'research', got %d", len(agents2))
	}

	agents3, _, err := store.List(ctxT, "", "", "nonexistent", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents3) != 0 {
		t.Errorf("expected 0 agents with capability 'nonexistent', got %d", len(agents3))
	}
}

func TestAgentStore_Exists(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	agent := &Agent{ID: uid(), Name: "Exist", Role: "coder", TenantID: tenantID, Capabilities: []string{"code"}}
	if err := store.Create(ctxT, agent); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	exists, err := store.Exists(ctxT, agent.ID)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected agent to exist")
	}

	exists2, _ := store.Exists(ctxT, "non-existent")
	if exists2 {
		t.Error("expected agent to not exist")
	}
}

func TestAgentStore_List_TenantIsolation(t *testing.T) {
	store := NewAgentStore()

	ctx1 := ctxWithTenant(ctx(), "tenant-1")
	ctx2 := ctxWithTenant(ctx(), "tenant-2")

	store.Create(ctx1, &Agent{ID: uid(), Name: "A1", Role: "coder", TenantID: "tenant-1", Capabilities: []string{"code"}})
	store.Create(ctx2, &Agent{ID: uid(), Name: "A2", Role: "coder", TenantID: "tenant-2", Capabilities: []string{"code"}})

	agents1, _, err := store.List(ctx1, "", "", "", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents1) != 1 {
		t.Errorf("expected 1 agent for tenant-1, got %d", len(agents1))
	}
	if agents1[0].Name != "A1" {
		t.Errorf("expected agent A1 for tenant-1, got %q", agents1[0].Name)
	}

	agents2, _, err := store.List(ctx2, "", "", "", 1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(agents2) != 1 {
		t.Errorf("expected 1 agent for tenant-2, got %d", len(agents2))
	}
	if agents2[0].Name != "A2" {
		t.Errorf("expected agent A2 for tenant-2, got %q", agents2[0].Name)
	}
}

func TestAgentStore_Delete(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	agent := &Agent{
		ID:         uid(),
		Name:       "Test Agent",
		Role:       "coder",
		TenantID:   tenantID,
		Capabilities: []string{"code_generation"},
	}

	if err := store.Create(ctxT, agent); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify agent exists
	found, err := store.GetByID(ctxT, agent.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found.Name != "Test Agent" {
		t.Errorf("expected name 'Test Agent', got %q", found.Name)
	}

	// Delete the agent
	if err := store.Delete(ctxT, agent.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify agent no longer exists
	_, err = store.GetByID(ctxT, agent.ID)
	if err == nil {
		t.Error("expected error for deleted agent, got nil")
	}
}

func TestAgentStore_Delete_NotFound(t *testing.T) {
	store := NewAgentStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	err := store.Delete(ctxT, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent agent, got nil")
	}
}
