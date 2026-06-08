package store

import (
	"testing"
)

func TestVersionStore_CreateAndGetByID(t *testing.T) {
	store := NewVersionStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	version := &AgentVersion{
		ID:        uid(),
		TenantID:  tenantID,
		AgentID:   "agent-1",
		Version:   "1.0.0",
	}

	if err := store.Create(ctxT, version); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if version.ID == "" {
		t.Fatal("created version should have an ID")
	}

	found, err := store.GetByID(ctxT, version.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if found.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", found.Version)
	}
}

func TestVersionStore_ListByAgent(t *testing.T) {
	store := NewVersionStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	store.Create(ctxT, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "1.0.0"})
	store.Create(ctxT, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "1.1.0"})
	store.Create(ctxT, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "2.0.0"})
	store.Create(ctxT, &AgentVersion{ID: uid(), AgentID: "agent-2", TenantID: tenantID, Version: "1.0.0"})

	versions, _ := store.ListByAgent(ctxT, "agent-1")
	if len(versions) != 3 {
		t.Errorf("expected 3 versions for agent-1, got %d", len(versions))
	}

	versions2, _ := store.ListByAgent(ctxT, "agent-2")
	if len(versions2) != 1 {
		t.Errorf("expected 1 version for agent-2, got %d", len(versions2))
	}
}

func TestVersionStore_ListByAgentAndStatus(t *testing.T) {
	store := NewVersionStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	store.Create(ctxT, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "1.0.0", Status: VersionStatusActive})
	store.Create(ctxT, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "0.9.0", Status: VersionStatusBeta})

	versions, _ := store.ListByAgentAndStatus(ctxT, "agent-1", "active")
	if len(versions) != 1 {
		t.Errorf("expected 1 active version, got %d", len(versions))
	}
}

func TestVersionStore_Patch(t *testing.T) {
	store := NewVersionStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	version := &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "1.0.0"}
	if err := store.Create(ctxT, version); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err := store.Patch(ctxT, version.ID, func(v *AgentVersion) {
		v.Status = VersionStatusDeprecated
	})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}

	found, _ := store.GetByID(ctxT, version.ID)
	if found.Status != VersionStatusDeprecated {
		t.Errorf("expected status deprecated, got %q", found.Status)
	}
}

func TestVersionStore_SetPromoted(t *testing.T) {
	store := NewVersionStore()
	tenantID := "tenant-1"
	ctxT := ctxWithTenant(ctx(), tenantID)

	version := &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: tenantID, Version: "1.0.0"}
	if err := store.Create(ctxT, version); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := store.SetPromoted(ctxT, version.ID, "production", "v1-prod-1"); err != nil {
		t.Fatalf("SetPromoted failed: %v", err)
	}

	found, _ := store.GetByID(ctxT, version.ID)
	if found.PromotedTo == nil {
		t.Fatal("PromotedTo should not be nil")
	}

	if found.PromotedTo["production"] != "v1-prod-1" {
		t.Errorf("expected production=v1-prod-1, got %q", found.PromotedTo["production"])
	}
}

func TestVersionStore_GetByID_NotFound(t *testing.T) {
	store := NewVersionStore()
	ctxT := ctxWithTenant(ctx(), "tenant-1")
	_, err := store.GetByID(ctxT, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent version, got nil")
	}
}

func TestVersionStore_TenantIsolation(t *testing.T) {
	store := NewVersionStore()

	ctx1 := ctxWithTenant(ctx(), "tenant-1")
	ctx2 := ctxWithTenant(ctx(), "tenant-2")

	store.Create(ctx1, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: "tenant-1", Version: "1.0.0"})
	store.Create(ctx2, &AgentVersion{ID: uid(), AgentID: "agent-1", TenantID: "tenant-2", Version: "2.0.0"})

	versions1, _ := store.ListByAgent(ctx1, "agent-1")
	versions2, _ := store.ListByAgent(ctx2, "agent-1")

	if len(versions1) != 1 || versions1[0].Version != "1.0.0" {
		t.Errorf("tenant-1: expected version 1.0.0, got %d", len(versions1))
	}
	if len(versions2) != 1 || versions2[0].Version != "2.0.0" {
		t.Errorf("tenant-2: expected version 2.0.0, got %d", len(versions2))
	}
}
