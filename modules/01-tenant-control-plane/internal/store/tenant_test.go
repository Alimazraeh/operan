package store

import (
	"testing"
	"time"
)

func TestTenantStore_Create(t *testing.T) {
	store := NewTenantStore()

	t.Run("creates tenant with auto-generated ID", func(t *testing.T) {
		now := time.Now()
		timeNow = func() time.Time { return now }
		defer func() { timeNow = time.Now }()

		tenant := &Tenant{
			Name:           "test-tenant",
			Plan:           PlanSaaS,
			Region:         RegionMEAST1,
			IsolationLevel: IsolationNamespace,
			Status:         TenantStatusProvisioning,
		}

		created, err := store.Create(tenant)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
		if created.Name != "test-tenant" {
			t.Errorf("expected name 'test-tenant', got %q", created.Name)
		}
		if created.Status != TenantStatusProvisioning {
			t.Errorf("expected status provisioning, got %q", created.Status)
		}
	})

	t.Run("creates tenant with custom ID", func(t *testing.T) {
		now := time.Now()
		timeNow = func() time.Time { return now }
		defer func() { timeNow = time.Now }()

		tenant := &Tenant{
			ID:   "custom-id-123",
			Name: "custom-tenant",
			Plan: PlanEnterprise,
			Region: RegionEUWest1,
		}

		created, err := store.Create(tenant)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID != "custom-id-123" {
			t.Errorf("expected ID 'custom-id-123', got %q", created.ID)
		}
	})

	t.Run("duplicate tenant ID returns error", func(t *testing.T) {
		now := time.Now()
		timeNow = func() time.Time { return now }
		defer func() { timeNow = time.Now }()

		tenant := &Tenant{
			ID:   "dup-id",
			Name: "first-tenant",
			Plan: PlanSaaS,
			Region: RegionMEAST1,
		}

		_, err := store.Create(tenant)
		if err != nil {
			t.Fatalf("expected no error on first create, got %v", err)
		}

		tenant2 := &Tenant{
			ID:   "dup-id",
			Name: "second-tenant",
			Plan: PlanSaaS,
			Region: RegionMEAST1,
		}

		_, err = store.Create(tenant2)
		if err == nil {
			t.Fatal("expected error on duplicate ID, got nil")
		}
	})

	t.Run("duplicate tenant name returns error", func(t *testing.T) {
		now := time.Now()
		timeNow = func() time.Time { return now }
		defer func() { timeNow = time.Now }()

		tenant := &Tenant{
			Name: "same-name",
			Plan: PlanSaaS,
			Region: RegionMEAST1,
		}

		_, err := store.Create(tenant)
		if err != nil {
			t.Fatalf("expected no error on first create, got %v", err)
		}

		tenant2 := &Tenant{
			Name: "same-name",
			Plan: PlanEnterprise,
			Region: RegionEUWest1,
		}

		_, err = store.Create(tenant2)
		if err == nil {
			t.Fatal("expected error on duplicate name, got nil")
		}
	})
}

func TestTenantStore_GetByID(t *testing.T) {
	store := NewTenantStore()

	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	tenant := &Tenant{
		Name: "get-tenant",
		Plan: PlanSaaS,
		Region: RegionMEAST1,
	}

	created, err := store.Create(tenant)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves tenant by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("expected ID %q, got %q", created.ID, got.ID)
		}
		if got.Name != "get-tenant" {
			t.Errorf("expected name 'get-tenant', got %q", got.Name)
		}
	})

	t.Run("returns error for non-existent tenant", func(t *testing.T) {
		_, err := store.GetByID("non-existent-id")
		if err == nil {
			t.Fatal("expected error for non-existent tenant, got nil")
		}
	})

	t.Run("returned copy does not affect stored tenant", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		got.Name = "modified-name"
		
		original, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if original.Name == "modified-name" {
			t.Fatal("modifying returned copy should not affect stored tenant")
		}
	})
}

func TestTenantStore_Patch(t *testing.T) {
	store := NewTenantStore()

	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	tenant := &Tenant{
		Name: "patch-tenant",
		Plan: PlanSaaS,
		Region: RegionMEAST1,
		Status: TenantStatusProvisioning,
	}

	created, err := store.Create(tenant)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("patches tenant name", func(t *testing.T) {
		updated, err := store.Patch(created.ID, TenantPatchRequest{
			Name: "updated-name",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Name != "updated-name" {
			t.Errorf("expected name 'updated-name', got %q", updated.Name)
		}
	})

	t.Run("patches tenant status with valid transition", func(t *testing.T) {
		updated, err := store.Patch(created.ID, TenantPatchRequest{
			Status: TenantStatusActive,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Status != TenantStatusActive {
			t.Errorf("expected status active, got %q", updated.Status)
		}
	})

	t.Run("rejects invalid status transition", func(t *testing.T) {
		_, err := store.Patch(created.ID, TenantPatchRequest{
			Status: TenantStatusDeprovisioned,
		})
		if err == nil {
			t.Fatal("expected error on invalid transition, got nil")
		}
	})

	t.Run("returns error for non-existent tenant", func(t *testing.T) {
		_, err := store.Patch("non-existent-id", TenantPatchRequest{
			Name: "test",
		})
		if err == nil {
			t.Fatal("expected error for non-existent tenant, got nil")
		}
	})
}

func TestTenantStore_Delete(t *testing.T) {
	store := NewTenantStore()

	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	tenant := &Tenant{
		Name: "delete-tenant",
		Plan: PlanSaaS,
		Region: RegionMEAST1,
	}

	created, err := store.Create(tenant)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("deletes tenant successfully", func(t *testing.T) {
		err := store.Delete(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = store.GetByID(created.ID)
		if err == nil {
			t.Fatal("expected error after deletion, got nil")
		}
	})

	t.Run("returns error for non-existent tenant", func(t *testing.T) {
		err := store.Delete("non-existent-id")
		if err == nil {
			t.Fatal("expected error for non-existent tenant, got nil")
		}
	})
}

func TestTenantStore_List(t *testing.T) {
	store := NewTenantStore()

	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	// Create multiple tenants
	for i := 0; i < 5; i++ {
		tenant := &Tenant{
			Name: "tenant-" + string(rune('a'+i)),
			Plan: PlanSaaS,
			Region: RegionMEAST1,
			Status: TenantStatusActive,
		}
		_, err := store.Create(tenant)
		if err != nil {
			t.Fatalf("expected no error on create, got %v", err)
		}
	}

	t.Run("returns all tenants without filter", func(t *testing.T) {
		items, total, hasMore := store.List(1, 20, nil)
		if total != 5 {
			t.Errorf("expected total 5, got %d", total)
		}
		if len(items) != 5 {
			t.Errorf("expected 5 items, got %d", len(items))
		}
		if hasMore {
			t.Error("expected hasMore false")
		}
	})

	t.Run("paginates results correctly", func(t *testing.T) {
		items, total, hasMore := store.List(1, 2, nil)
		if total != 5 {
			t.Errorf("expected total 5, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items on page 1, got %d", len(items))
		}
		if !hasMore {
			t.Error("expected hasMore true")
		}

		items2, _, hasMore2 := store.List(2, 2, nil)
		if len(items2) != 2 {
			t.Errorf("expected 2 items on page 2, got %d", len(items2))
		}
		if !hasMore2 {
			t.Error("expected hasMore true on page 2")
		}

		items3, _, hasMore3 := store.List(3, 2, nil)
		if len(items3) != 1 {
			t.Errorf("expected 1 item on page 3, got %d", len(items3))
		}
		if hasMore3 {
			t.Error("expected hasMore false on last page")
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		// Create a provisioning tenant
		_, err := store.Create(&Tenant{
			Name: "provisioning-tenant",
			Plan: PlanSaaS,
			Region: RegionMEAST1,
			Status: TenantStatusProvisioning,
		})
		if err != nil {
			t.Fatalf("expected no error on create, got %v", err)
		}

		status := "provisioning"
		items, total, _ := store.List(1, 20, &status)
		if total != 1 {
			t.Errorf("expected 1 provisioning tenant, got %d", total)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item, got %d", len(items))
		}
		if items[0].Status != TenantStatusProvisioning {
			t.Errorf("expected provisioning status, got %q", items[0].Status)
		}
	})
}

func TestTenantStore_CountTotal(t *testing.T) {
	store := NewTenantStore()

	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	if store.CountTotal() != 0 {
		t.Error("expected 0 tenants initially")
	}

	for i := 0; i < 3; i++ {
		tenant := &Tenant{
			Name: "count-tenant-" + string(rune('a'+i)),
			Plan: PlanSaaS,
			Region: RegionMEAST1,
		}
		_, err := store.Create(tenant)
		if err != nil {
			t.Fatalf("expected no error on create, got %v", err)
		}
	}

	if store.CountTotal() != 3 {
		t.Errorf("expected 3 tenants, got %d", store.CountTotal())
	}
}

func TestAllowedTransitions(t *testing.T) {
	tests := []struct {
		name       string
		from       TenantStatus
		expectLen  int
		contains   TenantStatus
	}{
		{"provisioning can go to active", TenantStatusProvisioning, 2, TenantStatusActive},
		{"active can go to suspended", TenantStatusActive, 2, TenantStatusSuspended},
		{"suspended can go to active", TenantStatusSuspended, 2, TenantStatusActive},
		{"deprovisioning can go to deprovisioned", TenantStatusDeprovisioning, 1, TenantStatusDeprovisioned},
		{"deprovisioned has no transitions", TenantStatusDeprovisioned, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transitions := AllowedTransitions(tt.from)
			if len(transitions) != tt.expectLen {
				t.Errorf("expected %d transitions, got %d", tt.expectLen, len(transitions))
			}
			if tt.contains != "" {
				found := false
				for _, t := range transitions {
					if t == tt.contains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected transitions to contain %q", tt.contains)
				}
			}
		})
	}
}

func TestPlanDefaults(t *testing.T) {
	tests := []struct {
		name           Plan
		expectAgents   int
		expectStorage  int
	}{
		{"saas", 5, 10},
		{"enterprise", 50, 500},
		{"sovereign", 200, 2000},
		{"unknown", 5, 10},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			defaults := PlanDefaults(tt.name)
			if defaults.MaxAgents != tt.expectAgents {
				t.Errorf("expected %d max agents, got %d", tt.expectAgents, defaults.MaxAgents)
			}
			if defaults.MaxStorageGB != tt.expectStorage {
				t.Errorf("expected %d storage GB, got %d", tt.expectStorage, defaults.MaxStorageGB)
			}
		})
	}
}

func TestTenant_NameCompare(t *testing.T) {
	a := &Tenant{Name: "alpha"}
	b := &Tenant{Name: "beta"}
	c := &Tenant{Name: "alpha"}

	if a.NameCompare(b) >= 0 {
		t.Error("expected alpha < beta")
	}
	if b.NameCompare(a) <= 0 {
		t.Error("expected beta > alpha")
	}
	if a.NameCompare(c) != 0 {
		t.Error("expected alpha == alpha")
	}
}
