package store

import "testing"

func TestPolicyCreateAndList(t *testing.T) {
	s := NewPolicyStore()
	p, err := s.Create(&RetentionPolicy{TenantID: "t1", MemoryType: MemoryEphemeral, TTLSeconds: 3600})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" || p.CreationDate.IsZero() {
		t.Error("Create should assign ID and creation date")
	}

	items, total, hasMore := s.List("t1", 1, 10)
	if total != 1 || len(items) != 1 || hasMore {
		t.Errorf("List = %d/%d/%v, want 1/1/false", total, len(items), hasMore)
	}
}

func TestPolicyValidation(t *testing.T) {
	s := NewPolicyStore()
	if _, err := s.Create(&RetentionPolicy{MemoryType: MemoryPersonal}); err != ErrTenantMismatch {
		t.Errorf("missing tenant: got %v", err)
	}
	if _, err := s.Create(&RetentionPolicy{TenantID: "t1", MemoryType: MemoryType("bogus")}); err != ErrValidation {
		t.Errorf("bad memory type: got %v", err)
	}
}

func TestPolicyTenantIsolation(t *testing.T) {
	s := NewPolicyStore()
	s.Create(&RetentionPolicy{TenantID: "t1", MemoryType: MemoryPlatform})
	if _, total, _ := s.List("t2", 1, 10); total != 0 {
		t.Errorf("cross-tenant List total = %d, want 0", total)
	}
}

func TestOperationLifecycle(t *testing.T) {
	s := NewOperationStore()
	op := s.Start("t1")
	if op.Status != "processing" || op.StartedAt == nil {
		t.Errorf("Start: status=%s", op.Status)
	}

	s.Complete(op.ID, 42)
	done, ok := s.Get(op.ID)
	if !ok || done.Status != "completed" || done.BatchSize != 42 || done.CompletedAt == nil {
		t.Errorf("Complete: %+v", done)
	}

	op2 := s.Start("t1")
	s.Fail(op2.ID, "boom")
	failed, _ := s.Get(op2.ID)
	if failed.Status != "failed" || failed.ErrorMessage != "boom" {
		t.Errorf("Fail: %+v", failed)
	}

	if _, ok := s.Get("nope"); ok {
		t.Error("unknown op should not be found")
	}
}
