package store

import (
	"testing"
	"time"
)

// ─── Approvals ───────────────────────────────────────────────────────────────

func newApproval(tenant, typ string) *Approval {
	return &Approval{
		TenantID:    tenant,
		RequestID:   "req-" + typ,
		RequesterID: "agent-1",
		Type:        typ,
		Title:       "test approval",
	}
}

func TestApprovalCreateAndGet(t *testing.T) {
	s := NewApprovalStore()
	a, err := s.Create(newApproval("t1", "parallel"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == "" || a.Status != "pending" {
		t.Errorf("approval = %+v", a)
	}

	got, _, err := s.Get(a.ID, "t1")
	if err != nil || got.RequestID != "req-parallel" {
		t.Errorf("Get: %+v err=%v", got, err)
	}
	if _, _, err := s.Get(a.ID, "t2"); err != ErrNotFound {
		t.Errorf("cross-tenant Get: %v", err)
	}

	byReq, err := s.GetByRequestID("req-parallel", "t1")
	if err != nil || byReq.ID != a.ID {
		t.Errorf("GetByRequestID: %+v err=%v", byReq, err)
	}
}

func TestApprovalValidation(t *testing.T) {
	s := NewApprovalStore()
	if _, err := s.Create(&Approval{RequestID: "r", RequesterID: "u", Type: "parallel"}); err != ErrTenantMismatch {
		t.Errorf("missing tenant: %v", err)
	}
	if _, err := s.Create(&Approval{TenantID: "t1", RequesterID: "u", Type: "parallel"}); err != ErrValidation {
		t.Errorf("missing request id: %v", err)
	}
	if _, err := s.Create(&Approval{TenantID: "t1", RequestID: "r", RequesterID: "u", Type: "bogus"}); err != ErrValidation {
		t.Errorf("bad type: %v", err)
	}
}

func TestApprovalSingleApprovalApproves(t *testing.T) {
	s := NewApprovalStore()
	a, _ := s.Create(newApproval("t1", "parallel"))

	out, err := s.Approve(a.ID, "t1", ApprovalAction{ActorID: "user-1", Comment: "lgtm"})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if out.Status != "approved" || out.ApprovedAt == nil || len(out.Approvals) != 1 {
		t.Errorf("approved = %+v", out)
	}

	// Terminal state conflicts.
	if _, err := s.Approve(a.ID, "t1", ApprovalAction{ActorID: "user-2"}); err != ErrConflict {
		t.Errorf("approve after terminal: %v", err)
	}
	if _, err := s.Reject(a.ID, "t1", ApprovalAction{ActorID: "user-2"}); err != ErrConflict {
		t.Errorf("reject after terminal: %v", err)
	}
}

func TestApprovalThresholdRules(t *testing.T) {
	s := NewApprovalStore()
	a := newApproval("t1", "threshold")
	a.ThresholdConfig = &ThresholdConfig{MinApprovals: 2, MaxRejections: 1}
	created, _ := s.Create(a)

	first, _ := s.Approve(created.ID, "t1", ApprovalAction{ActorID: "u1"})
	if first.Status != "in_progress" {
		t.Errorf("after 1 of 2 approvals: %s", first.Status)
	}
	second, _ := s.Approve(created.ID, "t1", ApprovalAction{ActorID: "u2"})
	if second.Status != "approved" {
		t.Errorf("after 2 of 2 approvals: %s", second.Status)
	}

	// Rejection threshold.
	b := newApproval("t1", "threshold")
	b.RequestID = "req-b"
	b.ThresholdConfig = &ThresholdConfig{MinApprovals: 3, MaxRejections: 1}
	createdB, _ := s.Create(b)
	r1, _ := s.Reject(createdB.ID, "t1", ApprovalAction{ActorID: "u1"})
	if r1.Status != "in_progress" {
		t.Errorf("1 rejection of allowed 1: %s", r1.Status)
	}
	r2, _ := s.Reject(createdB.ID, "t1", ApprovalAction{ActorID: "u2"})
	if r2.Status != "rejected" || r2.RejectedAt == nil {
		t.Errorf("2nd rejection should reject: %+v", r2)
	}
}

func TestApprovalDelegate(t *testing.T) {
	s := NewApprovalStore()
	a, _ := s.Create(newApproval("t1", "sequential"))

	out, err := s.Delegate(a.ID, "t1", ApprovalDelegate{FromUserID: "u1", ToUserID: "u2", Reason: "OOO"})
	if err != nil {
		t.Fatalf("Delegate: %v", err)
	}
	if out.Status != "delegated" || len(out.Delegates) != 1 {
		t.Errorf("delegated = %+v", out)
	}
	// Delegated approval can still be approved.
	final, err := s.Approve(a.ID, "t1", ApprovalAction{ActorID: "u2"})
	if err != nil || final.Status != "approved" {
		t.Errorf("approve after delegate: %+v err=%v", final, err)
	}
}

func TestApprovalExpiry(t *testing.T) {
	s := NewApprovalStore()
	past := time.Now().UTC().Add(-time.Minute)
	a := newApproval("t1", "parallel")
	a.ExpiresAt = &past
	created, _ := s.Create(a)

	got, justExpired, err := s.Get(created.ID, "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "expired" || !justExpired {
		t.Errorf("expired lazily: status=%s justExpired=%v", got.Status, justExpired)
	}
	// Second get: already expired, no re-transition.
	_, again, _ := s.Get(created.ID, "t1")
	if again {
		t.Error("second Get should not report expiry again")
	}
	if _, err := s.Approve(created.ID, "t1", ApprovalAction{ActorID: "u"}); err != ErrConflict {
		t.Errorf("approve expired: %v", err)
	}
}

func TestApprovalUpdateAndDelete(t *testing.T) {
	s := NewApprovalStore()
	a, _ := s.Create(newApproval("t1", "parallel"))

	upd, err := s.Update(a.ID, "t1", func(x *Approval) { x.Title = "renamed" })
	if err != nil || upd.Title != "renamed" {
		t.Errorf("Update: %+v err=%v", upd, err)
	}

	s.Approve(a.ID, "t1", ApprovalAction{ActorID: "u"})
	if _, err := s.Update(a.ID, "t1", func(x *Approval) {}); err != ErrConflict {
		t.Errorf("update terminal: %v", err)
	}

	if err := s.Delete(a.ID, "t1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, _, err := s.Get(a.ID, "t1"); err != ErrNotFound {
		t.Errorf("get after delete: %v", err)
	}
	if _, err := s.GetByRequestID("req-parallel", "t1"); err != ErrNotFound {
		t.Errorf("request index after delete: %v", err)
	}
}

// ─── Escalations ─────────────────────────────────────────────────────────────

func TestEscalationLifecycle(t *testing.T) {
	s := NewEscalationStore()
	e, err := s.Create(&Escalation{TenantID: "t1", Severity: "high", Category: "security", Title: "prompt injection"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.Status != "open" {
		t.Errorf("status = %s", e.Status)
	}

	upd, _ := s.Update(e.ID, "t1", func(x *Escalation) { x.Status = "acknowledged"; x.AssignedTo = "u1" })
	if upd.Status != "acknowledged" {
		t.Errorf("update: %+v", upd)
	}

	res, err := s.Resolve(e.ID, "t1", "u1", "patched", "manual_resolution")
	if err != nil || res.Status != "resolved" || res.ResolvedAt == nil {
		t.Errorf("resolve: %+v err=%v", res, err)
	}
	if _, err := s.Resolve(e.ID, "t1", "u1", "", ""); err != ErrConflict {
		t.Errorf("double resolve: %v", err)
	}
	if _, err := s.Update(e.ID, "t1", func(x *Escalation) {}); err != ErrConflict {
		t.Errorf("update resolved: %v", err)
	}

	if len(s.Open("t1")) != 0 {
		t.Error("resolved escalation still in Open")
	}
	if len(s.All("t1")) != 1 {
		t.Error("All should include resolved")
	}
	if _, err := s.Get(e.ID, "t2"); err != ErrNotFound {
		t.Errorf("cross-tenant: %v", err)
	}
}

func TestEscalationValidation(t *testing.T) {
	s := NewEscalationStore()
	if _, err := s.Create(&Escalation{TenantID: "t1", Severity: "bogus", Category: "security", Title: "x"}); err != ErrValidation {
		t.Errorf("bad severity: %v", err)
	}
	if _, err := s.Create(&Escalation{TenantID: "t1", Severity: "low", Category: "bogus", Title: "x"}); err != ErrValidation {
		t.Errorf("bad category: %v", err)
	}
}

// ─── Interventions ───────────────────────────────────────────────────────────

func TestInterventionLifecycle(t *testing.T) {
	s := NewInterventionStore()
	iv, err := s.Create(&Intervention{TenantID: "t1", Action: "pause", TargetAgentID: "agent-1", Reason: "runaway loop", DurationMinutes: 60})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if iv.Status != "active" || iv.ExpiresAt == nil {
		t.Errorf("created = %+v", iv)
	}

	upd, _ := s.Update(iv.ID, "t1", func(x *Intervention) { x.Reason = "updated" })
	if upd.Reason != "updated" {
		t.Errorf("update: %+v", upd)
	}

	rev, err := s.Revoke(iv.ID, "t1", "supervisor-1")
	if err != nil || rev.Status != "revoked" || rev.RevokedBy != "supervisor-1" {
		t.Errorf("revoke: %+v err=%v", rev, err)
	}
	if _, err := s.Revoke(iv.ID, "t1", "x"); err != ErrConflict {
		t.Errorf("double revoke: %v", err)
	}
	if len(s.Active("t1")) != 0 {
		t.Error("revoked intervention still active")
	}
}

func TestInterventionExpiry(t *testing.T) {
	s := NewInterventionStore()
	iv, _ := s.Create(&Intervention{TenantID: "t1", Action: "stop", TargetAgentID: "a", Reason: "r", DurationMinutes: 1})
	// Force the deadline into the past.
	past := time.Now().UTC().Add(-time.Minute)
	s.mu.Lock()
	s.interventions[iv.ID].ExpiresAt = &past
	s.mu.Unlock()

	got, _ := s.Get(iv.ID, "t1")
	if got.Status != "expired" {
		t.Errorf("status = %s, want expired", got.Status)
	}
	if _, err := s.Update(iv.ID, "t1", func(x *Intervention) {}); err != ErrConflict {
		t.Errorf("update expired: %v", err)
	}
}

func TestInterventionValidation(t *testing.T) {
	s := NewInterventionStore()
	if _, err := s.Create(&Intervention{TenantID: "t1", Action: "bogus", TargetAgentID: "a", Reason: "r"}); err != ErrValidation {
		t.Errorf("bad action: %v", err)
	}
	if _, err := s.Create(&Intervention{TenantID: "t1", Action: "pause", Reason: "r"}); err != ErrValidation {
		t.Errorf("missing target: %v", err)
	}
	if _, err := s.Create(&Intervention{TenantID: "t1", Action: "pause", TargetAgentID: "a", Reason: "r", Scope: &InterventionScope{Type: "bogus"}}); err != ErrValidation {
		t.Errorf("bad scope: %v", err)
	}
}

// ─── HITL ────────────────────────────────────────────────────────────────────

func TestHitlSubmitOnceAndGet(t *testing.T) {
	s := NewHitlStore()
	a, err := s.Submit(&HitlAnswer{TenantID: "t1", RequestID: "req-1", Answer: "yes, proceed"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if a.ID == "" {
		t.Error("Submit should assign ID")
	}

	if _, err := s.Submit(&HitlAnswer{TenantID: "t1", RequestID: "req-1", Answer: "duplicate"}); err != ErrConflict {
		t.Errorf("duplicate submit: %v", err)
	}
	// Same request id under another tenant is fine.
	if _, err := s.Submit(&HitlAnswer{TenantID: "t2", RequestID: "req-1", Answer: "other tenant"}); err != nil {
		t.Errorf("other tenant submit: %v", err)
	}

	got, err := s.Get("req-1", "t1")
	if err != nil || got.Answer != "yes, proceed" {
		t.Errorf("Get: %+v err=%v", got, err)
	}
	if _, err := s.Get("req-x", "t1"); err != ErrNotFound {
		t.Errorf("missing: %v", err)
	}
}
