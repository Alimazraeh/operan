package store

import "testing"

func TestVectorStorePersistRoundTrip(t *testing.T) {
	s := NewVectorStore()
	v, _ := s.Create(newVector("t1", "d1", "remember me", EmbeddingAgentPersonal))
	s.Create(newVector("t2", "d2", "other tenant", EmbeddingDepartment))

	data, err := s.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	restored := NewVectorStore()
	if err := restored.Import(data); err != nil {
		t.Fatalf("Import: %v", err)
	}
	got, err := restored.GetByIDAndTenant(v.ID, "t1")
	if err != nil || got.SemanticContent != "remember me" {
		t.Errorf("restored vector: %+v err=%v", got, err)
	}
	// Tenant index rebuilt.
	if _, total, _ := restored.List("t2", 1, 10, nil, nil, nil); total != 1 {
		t.Errorf("t2 total = %d", total)
	}
	if _, total, _ := restored.List("t1", 1, 10, nil, nil, nil); total != 1 {
		t.Errorf("t1 total = %d", total)
	}
}

func TestPolicyAndOperationPersistRoundTrip(t *testing.T) {
	p := NewPolicyStore()
	p.Create(&RetentionPolicy{TenantID: "t1", MemoryType: MemoryEphemeral, TTLSeconds: 60})
	data, _ := p.Export()
	p2 := NewPolicyStore()
	if err := p2.Import(data); err != nil {
		t.Fatalf("policy import: %v", err)
	}
	if _, total, _ := p2.List("t1", 1, 10); total != 1 {
		t.Errorf("policies = %d", total)
	}

	o := NewOperationStore()
	op := o.Start("t1")
	o.Complete(op.ID, 7)
	data, _ = o.Export()
	o2 := NewOperationStore()
	if err := o2.Import(data); err != nil {
		t.Fatalf("ops import: %v", err)
	}
	got, ok := o2.Get(op.ID)
	if !ok || got.Status != "completed" || got.BatchSize != 7 {
		t.Errorf("restored op = %+v ok=%v", got, ok)
	}
}

func TestImportRejectsGarbage(t *testing.T) {
	if err := NewVectorStore().Import([]byte("{{{")); err == nil {
		t.Error("expected error for garbage snapshot")
	}
}
