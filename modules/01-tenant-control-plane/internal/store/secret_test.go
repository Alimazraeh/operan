package store

import (
	"testing"
	"time"
)

func TestSecretStore_Create(t *testing.T) {
	store := NewSecretStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	t.Run("creates secret with tags", func(t *testing.T) {
		sec, err := store.Create("tenant-1", "db-password", "secret-value", "Database password", []string{"db", "prod"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if sec.ID == "" {
			t.Error("expected non-empty auto-generated ID")
		}
		if sec.TenantID != "tenant-1" {
			t.Errorf("expected tenant_id 'tenant-1', got %q", sec.TenantID)
		}
		if sec.Key != "db-password" {
			t.Errorf("expected key 'db-password', got %q", sec.Key)
		}
		if sec.EncryptedValue == "secret-value" {
			t.Error("expected encrypted value, got plaintext")
		}
		if len(sec.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(sec.Tags))
		}
	})

	t.Run("rejects empty key", func(t *testing.T) {
		_, err := store.Create("tenant-1", "", "value", "desc", nil)
		if err == nil {
			t.Fatal("expected error for empty key, got nil")
		}
	})

	t.Run("rejects duplicate key for same tenant", func(t *testing.T) {
		_, err := store.Create("tenant-1", "db-password", "value", "desc", nil)
		if err == nil {
			t.Fatal("expected error for duplicate key, got nil")
		}
	})

	t.Run("allows same key for different tenants", func(t *testing.T) {
		_, err := store.Create("tenant-2", "db-password", "value", "desc", nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestSecretStore_GetByID(t *testing.T) {
	store := NewSecretStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sec, err := store.Create("tenant-1", "api-key", "my-secret", "API key", nil)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves secret by ID", func(t *testing.T) {
		got, err := store.GetByID(sec.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Key != "api-key" {
			t.Errorf("expected key 'api-key', got %q", got.Key)
		}
	})

	t.Run("returns error for non-existent secret", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Fatal("expected error for non-existent secret, got nil")
		}
	})

	t.Run("returned copy is independent", func(t *testing.T) {
		created, err := store.Create("tenant-1", "indep-key", "val", "desc", nil)
		if err != nil {
			t.Fatalf("expected no error on create, got %v", err)
		}
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		got.Description = "modified"
		original, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if original.Description == "modified" {
			t.Fatal("modifying returned copy should not affect stored secret")
		}
	})
}

func TestSecretStore_List(t *testing.T) {
	store := NewSecretStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	// Create secrets for tenant-1
	store.Create("tenant-1", "key-a", "val-a", "A", nil)
	store.Create("tenant-1", "key-b", "val-b", "B", nil)
	store.Create("tenant-1", "key-c", "val-c", "C", nil)

	// Create secret for tenant-2
	store.Create("tenant-2", "key-x", "val-x", "X", nil)

	t.Run("lists secrets for specific tenant", func(t *testing.T) {
		items, total, _ := store.List("tenant-1", 1, 20)
		if total != 3 {
			t.Errorf("expected 3 secrets, got %d", total)
		}
		if len(items) != 3 {
			t.Errorf("expected 3 items, got %d", len(items))
		}
	})

	t.Run("filters by tenant", func(t *testing.T) {
		items, total, _ := store.List("tenant-2", 1, 20)
		if total != 1 {
			t.Errorf("expected 1 secret, got %d", total)
		}
		if items[0].Key != "key-x" {
			t.Errorf("expected key 'key-x', got %q", items[0].Key)
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		items, total, hasMore := store.List("tenant-1", 1, 2)
		if total != 3 {
			t.Errorf("expected total 3, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if !hasMore {
			t.Error("expected hasMore true")
		}
	})
}

func TestSecretStore_Update(t *testing.T) {
	store := NewSecretStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sec, err := store.Create("tenant-1", "api-key", "val", "old desc", []string{"old"})
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	updated, err := store.Update(sec.ID, "new desc", []string{"new", "updated"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Description != "new desc" {
		t.Errorf("expected 'new desc', got %q", updated.Description)
	}
	if len(updated.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(updated.Tags))
	}

	t.Run("returns error for non-existent secret", func(t *testing.T) {
		_, err := store.Update("non-existent", "desc", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSecretStore_Rotate(t *testing.T) {
	store := NewSecretStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sec, err := store.Create("tenant-1", "api-key", "v1", "key", nil)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	rotated, err := store.Rotate(sec.ID, "v2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rotated.Version != 2 {
		t.Errorf("expected version 2, got %d", rotated.Version)
	}

	// Rotate again
	rotated2, err := store.Rotate(sec.ID, "v3")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rotated2.Version != 3 {
		t.Errorf("expected version 3, got %d", rotated2.Version)
	}
}

func TestSecretStore_Delete(t *testing.T) {
	store := NewSecretStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sec, err := store.Create("tenant-1", "api-key", "val", "desc", nil)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	err = store.Delete(sec.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = store.GetByID(sec.ID)
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}

	t.Run("returns error for non-existent secret", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
