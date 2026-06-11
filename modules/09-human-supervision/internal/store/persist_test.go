package store

import "testing"

func TestApprovalPersistRoundTrip(t *testing.T) {
	s := NewApprovalStore()
	a, _ := s.Create(newApproval("t1", "threshold"))
	s.Approve(a.ID, "t1", ApprovalAction{ActorID: "u1", Comment: "ok"})

	data, err := s.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	restored := NewApprovalStore()
	if err := restored.Import(data); err != nil {
		t.Fatalf("Import: %v", err)
	}
	got, _, err := restored.Get(a.ID, "t1")
	if err != nil || got.Status != "approved" || len(got.Approvals) != 1 {
		t.Errorf("restored = %+v err=%v", got, err)
	}
	// Request index rebuilt (tenant survives despite json:"-" on the model).
	if _, err := restored.GetByRequestID(a.RequestID, "t1"); err != nil {
		t.Errorf("byRequest index: %v", err)
	}
	if _, _, err := restored.Get(a.ID, "t2"); err != ErrNotFound {
		t.Errorf("tenant isolation after restore: %v", err)
	}
}

func TestEscalationInterventionHitlPersistRoundTrip(t *testing.T) {
	e := NewEscalationStore()
	esc, _ := e.Create(&Escalation{TenantID: "t1", Severity: "high", Category: "security", Title: "x"})
	data, _ := e.Export()
	e2 := NewEscalationStore()
	e2.Import(data)
	if got, err := e2.Get(esc.ID, "t1"); err != nil || got.Severity != "high" {
		t.Errorf("escalation: %+v err=%v", got, err)
	}

	iv := NewInterventionStore()
	in, _ := iv.Create(&Intervention{TenantID: "t1", Action: "pause", TargetAgentID: "a", Reason: "r"})
	data, _ = iv.Export()
	iv2 := NewInterventionStore()
	iv2.Import(data)
	if got, err := iv2.Get(in.ID, "t1"); err != nil || got.Status != "active" {
		t.Errorf("intervention: %+v err=%v", got, err)
	}

	h := NewHitlStore()
	h.Submit(&HitlAnswer{TenantID: "t1", RequestID: "r1", Answer: "yes"})
	data, _ = h.Export()
	h2 := NewHitlStore()
	h2.Import(data)
	if got, err := h2.Get("r1", "t1"); err != nil || got.Answer != "yes" {
		t.Errorf("hitl: %+v err=%v", got, err)
	}
	// Duplicate detection survives restore.
	if _, err := h2.Submit(&HitlAnswer{TenantID: "t1", RequestID: "r1", Answer: "again"}); err != ErrConflict {
		t.Errorf("dup after restore: %v", err)
	}
}
