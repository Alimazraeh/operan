package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResourceStore_Create(t *testing.T) {
	store := NewResourceStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	t.Run("creates resource with auto-generated ID", func(t *testing.T) {
		res := &Resource{
			TenantID: "tenant-1",
			Name:     "test-db",
			Type:     ResourceTypeDatabase,
			Region:   RegionMEAST1,
		}

		created, err := store.Create(res)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
		if created.Status != ResourceStatusProvisioning {
			t.Errorf("expected provisioning status, got %q", created.Status)
		}
	})

	t.Run("creates resource with custom ID", func(t *testing.T) {
		res := &Resource{
			ID:       "custom-res-id",
			TenantID: "tenant-1",
			Name:     "custom-res",
			Type:     ResourceTypeCompute,
			Region:   RegionEUWest1,
		}

		created, err := store.Create(res)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID != "custom-res-id" {
			t.Errorf("expected ID 'custom-res-id', got %q", created.ID)
		}
	})
}

func TestResourceStore_GetByID(t *testing.T) {
	store := NewResourceStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	res := &Resource{
		TenantID: "tenant-1",
		Name:     "test-res",
		Type:     ResourceTypeStorage,
		Region:   RegionMEAST1,
	}

	created, err := store.Create(res)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves resource by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Name != "test-res" {
			t.Errorf("expected name 'test-res', got %q", got.Name)
		}
	})

	t.Run("returns error for non-existent resource", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returned copy is independent", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		got.Name = "modified"
		original, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if original.Name == "modified" {
			t.Fatal("modifying returned copy should not affect stored resource")
		}
	})
}

func TestResourceStore_Patch(t *testing.T) {
	store := NewResourceStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	res := &Resource{
		TenantID: "tenant-1",
		Name:     "test-res",
		Type:     ResourceTypeCompute,
		Region:   RegionMEAST1,
	}

	created, err := store.Create(res)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("patches name and status", func(t *testing.T) {
		updated, err := store.Patch(created.ID, ResourcePatchRequest{
			Name:   "updated-name",
			Status: ResourceStatusProvisioned,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Name != "updated-name" {
			t.Errorf("expected name 'updated-name', got %q", updated.Name)
		}
		if updated.Status != ResourceStatusProvisioned {
			t.Errorf("expected provisioned status, got %q", updated.Status)
		}
	})

	t.Run("patches spec fields", func(t *testing.T) {
		updated, err := store.Patch(created.ID, ResourcePatchRequest{
			Spec: ResourceSpec{
				VCPUs:   8,
				RAMGB:   32,
				StorageGB: 500,
			},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Spec.VCPUs != 8 {
			t.Errorf("expected 8 VCPUs, got %d", updated.Spec.VCPUs)
		}
		if updated.Spec.RAMGB != 32 {
			t.Errorf("expected 32 RAM GB, got %d", updated.Spec.RAMGB)
		}
	})

	t.Run("returns error for non-existent resource", func(t *testing.T) {
		_, err := store.Patch("non-existent", ResourcePatchRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestResourceStore_Delete(t *testing.T) {
	store := NewResourceStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	res := &Resource{
		TenantID: "tenant-1",
		Name:     "test-res",
		Type:     ResourceTypeStorage,
		Region:   RegionMEAST1,
	}

	created, err := store.Create(res)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	err = store.Delete(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = store.GetByID(created.ID)
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}

	// Also removed from byTenant
	items, total, _ := store.ListByTenant("tenant-1", 1, 20)
	if total != 0 {
		t.Errorf("expected 0 items after deletion, got %d", total)
	}
	if len(items) != 0 {
		t.Error("expected empty items after deletion")
	}

	t.Run("returns error for non-existent resource", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestResourceStore_ListByTenant(t *testing.T) {
	store := NewResourceStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	store.Create(&Resource{TenantID: "tenant-1", Name: "resource-b", Type: ResourceTypeDatabase, Region: RegionMEAST1})
	store.Create(&Resource{TenantID: "tenant-1", Name: "resource-a", Type: ResourceTypeStorage, Region: RegionEUWest1})
	store.Create(&Resource{TenantID: "tenant-2", Name: "resource-x", Type: ResourceTypeCompute, Region: RegionUSEAST1})

	t.Run("lists resources for tenant", func(t *testing.T) {
		items, total, _ := store.ListByTenant("tenant-1", 1, 20)
		if total != 2 {
			t.Errorf("expected 2 resources, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("returns empty for tenant without resources", func(t *testing.T) {
		items, total, hasMore := store.ListByTenant("non-existent", 1, 20)
		if total != 0 {
			t.Errorf("expected 0 resources, got %d", total)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
		if hasMore {
			t.Error("expected hasMore false")
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		items, total, hasMore := store.ListByTenant("tenant-1", 1, 1)
		if total != 2 {
			t.Errorf("expected total 2, got %d", total)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item, got %d", len(items))
		}
		if !hasMore {
			t.Error("expected hasMore true")
		}
	})
}

func TestResourceStore_CountTotal(t *testing.T) {
	store := NewResourceStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	if store.CountTotal() != 0 {
		t.Error("expected 0 resources initially")
	}

	store.Create(&Resource{TenantID: "tenant-1", Name: "res-1", Type: ResourceTypeDatabase, Region: RegionMEAST1})
	store.Create(&Resource{TenantID: "tenant-1", Name: "res-2", Type: ResourceTypeStorage, Region: RegionEUWest1})

	if store.CountTotal() != 2 {
		t.Errorf("expected 2 resources, got %d", store.CountTotal())
	}
}

// AgentStore tests
func TestAgentStore_Create(t *testing.T) {
	store := NewAgentStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	t.Run("creates agent with auto-generated ID and ready status", func(t *testing.T) {
		agent := &Agent{
			TenantID: "tenant-1",
			Name:     "test-agent",
			Model:    "gpt-4",
			Role:     "assistant",
		}

		created, err := store.Create(agent)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
		if created.Status != AgentStatusReady {
			t.Errorf("expected ready status, got %q", created.Status)
		}
	})

	t.Run("creates agent with custom ID", func(t *testing.T) {
		agent := &Agent{
			ID:       "custom-agent-id",
			TenantID: "tenant-1",
			Name:     "custom-agent",
			Model:    "claude",
			Role:     "executor",
		}

		created, err := store.Create(agent)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID != "custom-agent-id" {
			t.Errorf("expected ID 'custom-agent-id', got %q", created.ID)
		}
	})
}

func TestAgentStore_GetByID(t *testing.T) {
	store := NewAgentStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	agent := &Agent{
		TenantID: "tenant-1",
		Name:     "test-agent",
		Model:    "gpt-4",
	}

	created, err := store.Create(agent)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves agent by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Name != "test-agent" {
			t.Errorf("expected name 'test-agent', got %q", got.Name)
		}
	})

	t.Run("returns error for non-existent agent", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returned copy is independent", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		got.Name = "modified"
		original, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if original.Name == "modified" {
			t.Fatal("modifying returned copy should not affect stored agent")
		}
	})
}

func TestAgentStore_Patch(t *testing.T) {
	store := NewAgentStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	agent := &Agent{
		TenantID: "tenant-1",
		Name:     "test-agent",
		Model:    "gpt-4",
		Role:     "assistant",
	}

	created, err := store.Create(agent)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("patches model and role", func(t *testing.T) {
		updated, err := store.Patch(created.ID, AgentPatchRequest{
			Model: "claude-3",
			Role:  "executor",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Model != "claude-3" {
			t.Errorf("expected model 'claude-3', got %q", updated.Model)
		}
		if updated.Role != "executor" {
			t.Errorf("expected role 'executor', got %q", updated.Role)
		}
	})

	t.Run("patches status", func(t *testing.T) {
		updated, err := store.Patch(created.ID, AgentPatchRequest{
			Status: AgentStatusRunning,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Status != AgentStatusRunning {
			t.Errorf("expected running status, got %q", updated.Status)
		}
	})

	t.Run("patches tool access JSON", func(t *testing.T) {
		jsonData := json.RawMessage(`{"tools": ["search", "execute"]}`)
		updated, err := store.Patch(created.ID, AgentPatchRequest{
			ToolAccessJSON: jsonData,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if string(updated.ToolAccessJSON) != `{"tools": ["search", "execute"]}` {
			t.Errorf("expected tool access JSON to be set")
		}
	})

	t.Run("returns error for non-existent agent", func(t *testing.T) {
		_, err := store.Patch("non-existent", AgentPatchRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestAgentStore_Delete(t *testing.T) {
	store := NewAgentStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	agent := &Agent{
		TenantID: "tenant-1",
		Name:     "test-agent",
		Model:    "gpt-4",
	}

	created, err := store.Create(agent)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	err = store.Delete(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = store.GetByID(created.ID)
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}

	// Also removed from byTenant
	items, total, _ := store.ListByTenant("tenant-1", 1, 20)
	if total != 0 {
		t.Errorf("expected 0 items after deletion, got %d", total)
	}
	if len(items) != 0 {
		t.Error("expected empty items after deletion")
	}

	t.Run("returns error for non-existent agent", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestAgentStore_ListByTenant(t *testing.T) {
	store := NewAgentStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	store.Create(&Agent{TenantID: "tenant-1", Name: "agent-b", Model: "gpt-4"})
	store.Create(&Agent{TenantID: "tenant-1", Name: "agent-a", Model: "claude"})
	store.Create(&Agent{TenantID: "tenant-2", Name: "agent-x", Model: "gpt-3"})

	t.Run("lists agents for tenant", func(t *testing.T) {
		items, total, _ := store.ListByTenant("tenant-1", 1, 20)
		if total != 2 {
			t.Errorf("expected 2 agents, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("returns empty for tenant without agents", func(t *testing.T) {
		items, total, hasMore := store.ListByTenant("non-existent", 1, 20)
		if total != 0 {
			t.Errorf("expected 0 agents, got %d", total)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
		if hasMore {
			t.Error("expected hasMore false")
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		items, total, hasMore := store.ListByTenant("tenant-1", 1, 1)
		if total != 2 {
			t.Errorf("expected total 2, got %d", total)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item, got %d", len(items))
		}
		if !hasMore {
			t.Error("expected hasMore true")
		}
	})
}

func TestAgentStore_CountTotal(t *testing.T) {
	store := NewAgentStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	if store.CountTotal() != 0 {
		t.Error("expected 0 agents initially")
	}

	store.Create(&Agent{TenantID: "tenant-1", Name: "agent-1", Model: "gpt-4"})
	store.Create(&Agent{TenantID: "tenant-1", Name: "agent-2", Model: "claude"})

	if store.CountTotal() != 2 {
		t.Errorf("expected 2 agents, got %d", store.CountTotal())
	}
}
