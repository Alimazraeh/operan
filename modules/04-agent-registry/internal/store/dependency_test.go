package store

import (
	"testing"
)

func strPtr(s string) *string { return &s }

func TestCapabilityStore_UpdateAndList(t *testing.T) {
	store := NewCapabilityStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	// List initially returns error for non-existent agent in tenant
	_, err := store.ListAll(ctxT, agentID)
	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}

	// Upsert with capabilities
	entry := &CapabilityEntry{
		AgentID:    agentID,
		TenantID:   tenantID,
		Capability: "code_generation",
		Score:      0.95,
	}
	if err := store.Upsert(ctxT, entry); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// List now returns the capability
	caps, err := store.ListAll(ctxT, agentID)
	if err != nil {
		t.Fatalf("ListAll failed after upsert: %v", err)
	}
	if len(caps) != 1 {
		t.Errorf("expected 1 capability, got %d", len(caps))
	}

	// Different tenant should see nothing
	ctxOther := ctxWithTenant(ctx(), "tenant-2")
	_, err = store.Get(ctxOther, agentID)
	if err == nil {
		t.Fatal("expected error for other tenant, got nil")
	}
}

func TestCapabilityStore_Index(t *testing.T) {
	store := NewCapabilityStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	entry := &CapabilityEntry{
		AgentID:    agentID,
		TenantID:   tenantID,
		Capability: "code_generation",
		Score:      0.95,
	}

	if err := store.Upsert(ctxT, entry); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Index should update LastEvaluated
	if err := store.Index(ctxT, agentID); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	result, _ := store.ListAll(ctxT, agentID)
	if result[0].LastEvaluated.IsZero() {
		t.Error("LastEvaluated should be set after Index")
	}
}

func TestCapabilityStore_Index_NotFound(t *testing.T) {
	store := NewCapabilityStore()
	ctxT := ctxWithTenant(ctx(), "tenant-1")
	// Agent doesn't exist in this tenant — should fail (not found)
	err := store.Index(ctxT, "non-existent-agent")
	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}
}

func TestCapabilityStore_UpdateOverwrites(t *testing.T) {
	store := NewCapabilityStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	// First upsert
	entry1 := &CapabilityEntry{
		AgentID:    agentID,
		TenantID:   tenantID,
		Capability: "code_generation",
		Score:      0.95,
	}
	if err := store.Upsert(ctxT, entry1); err != nil {
		t.Fatalf("Upsert 1 failed: %v", err)
	}

	// Second upsert adds a new capability (multiple capabilities per agent)
	entry2 := &CapabilityEntry{
		AgentID:    agentID,
		TenantID:   tenantID,
		Capability: "research",
		Score:      0.90,
	}
	if err := store.Upsert(ctxT, entry2); err != nil {
		t.Fatalf("Upsert 2 failed: %v", err)
	}

	result, _ := store.ListAll(ctxT, agentID)
	if len(result) != 2 {
		t.Errorf("expected 2 capabilities after two upserts, got %d", len(result))
	}
	if result[0].Capability != "code_generation" {
		t.Errorf("expected first capability 'code_generation', got %q", result[0].Capability)
	}
	if result[1].Capability != "research" {
		t.Errorf("expected second capability 'research', got %q", result[1].Capability)
	}
}

func TestDependencyStore_AddAndList(t *testing.T) {
	store := NewDependencyStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	dep := &AgentDependency{
		ID:                uid(),
		TenantID:          tenantID,
		AgentID:           agentID,
		DependencyAgentID: "agent-2",
		DependencyType:    DependencyTypeHard,
		VersionConstraint: strPtr(">=1.0.0"),
	}

	if err := store.Add(ctxT, dep); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if dep.ID == "" {
		t.Fatal("created dependency should have an ID")
	}

	// List should return the dependency
	deps, _ := store.ListByAgent(ctxT, agentID)
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].DependencyAgentID != "agent-2" {
		t.Errorf("expected dependency agent 'agent-2', got %q", deps[0].DependencyAgentID)
	}
	if deps[0].DependencyType != DependencyTypeHard {
		t.Errorf("expected type Hard, got %q", deps[0].DependencyType)
	}
}

func TestDependencyStore_Remove(t *testing.T) {
	store := NewDependencyStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	dep := &AgentDependency{
		ID:                uid(),
		TenantID:          tenantID,
		AgentID:           agentID,
		DependencyAgentID: "agent-2",
		DependencyType:    DependencyTypeHard,
	}

	if err := store.Add(ctxT, dep); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Should be able to remove (uses dep ID)
	if err := store.Remove(ctxT, dep.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// List should be empty
	deps, _ := store.ListByAgent(ctxT, agentID)
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies after remove, got %d", len(deps))
	}
}

func TestDependencyStore_Remove_NotFound(t *testing.T) {
	store := NewDependencyStore()
	ctxT := ctxWithTenant(ctx(), "tenant-1")
	err := store.Remove(ctxT, "non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent dependency, got nil")
	}
}

func TestDependencyStore_GetByID(t *testing.T) {
	store := NewDependencyStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	dep := &AgentDependency{
		ID:                uid(),
		TenantID:          tenantID,
		AgentID:           "agent-1",
		DependencyAgentID: "agent-2",
		DependencyType:    DependencyTypeSoft,
	}

	if err := store.Add(ctxT, dep); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	found, err := store.GetByID(ctxT, dep.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found.ID != dep.ID {
		t.Errorf("expected ID %q, got %q", dep.ID, found.ID)
	}
}

func TestDependencyStore_GetByID_NotFound(t *testing.T) {
	store := NewDependencyStore()
	ctxT := ctxWithTenant(ctx(), "tenant-1")
	_, err := store.GetByID(ctxT, "non-existent-id")
	if err == nil {
		t.Fatal("expected error for non-existent dependency, got nil")
	}
}

func TestDependencyStore_List_Pagination(t *testing.T) {
	store := NewDependencyStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	for i := 0; i < 5; i++ {
		store.Add(ctxT, &AgentDependency{
			ID:                uid(),
			TenantID:          tenantID,
			AgentID:           agentID,
			DependencyAgentID: "agent-" + string(rune('A'+i)),
		})
	}

	deps, _ := store.ListByAgent(ctxT, agentID)
	if len(deps) != 5 {
		t.Errorf("expected 5 dependencies, got %d", len(deps))
	}
}

func TestDependencyStore_TenantIsolation(t *testing.T) {
	store := NewDependencyStore()

	ctx1 := ctxWithTenant(ctx(), "tenant-1")
	ctx2 := ctxWithTenant(ctx(), "tenant-2")

	dep1 := &AgentDependency{
		ID:                uid(),
		TenantID:          "tenant-1",
		AgentID:           "agent-1",
		DependencyAgentID: "agent-A",
	}
	dep2 := &AgentDependency{
		ID:                uid(),
		TenantID:          "tenant-2",
		AgentID:           "agent-1",
		DependencyAgentID: "agent-B",
	}

	store.Add(ctx1, dep1)
	store.Add(ctx2, dep2)

	deps1, _ := store.ListByAgent(ctx1, "agent-1")
	deps2, _ := store.ListByAgent(ctx2, "agent-1")

	if len(deps1) != 1 || deps1[0].DependencyAgentID != "agent-A" {
		t.Errorf("tenant-1: expected 1 dep (agent-A), got %d", len(deps1))
	}
	if len(deps2) != 1 || deps2[0].DependencyAgentID != "agent-B" {
		t.Errorf("tenant-2: expected 1 dep (agent-B), got %d", len(deps2))
	}
}

func TestCapabilityStore_Exists(t *testing.T) {
	store := NewCapabilityStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	// Non-existent agent should return false
	exists, err := store.Exists(ctxT, agentID)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected agent to not exist")
	}

	// Create agent with capability
	entry := &CapabilityEntry{
		AgentID:    agentID,
		TenantID:   tenantID,
		Capability: "code_generation",
		Score:      0.95,
	}
	if err := store.Upsert(ctxT, entry); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Now should exist
	exists, err = store.Exists(ctxT, agentID)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected agent to exist")
	}
}

func TestDependencyStore_Exists(t *testing.T) {
	store := NewDependencyStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)
	agentID := "agent-1"

	// Non-existent dependency ID should return false
	exists, err := store.Exists(ctxT, "non-existent-id")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected dependency to not exist")
	}

	// Add dependency
	depID := uid()
	store.Add(ctxT, &AgentDependency{
		ID:                depID,
		TenantID:          tenantID,
		AgentID:           agentID,
		DependencyAgentID: "agent-A",
	})

	// Now should exist for that dependency ID
	exists, err = store.Exists(ctxT, depID)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected dependency to exist")
	}
}
