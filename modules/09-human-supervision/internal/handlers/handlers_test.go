package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/operan/modules/09-human-supervision/internal/ctxkeys"
	"github.com/operan/modules/09-human-supervision/internal/events"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

const testTenant = "11111111-1111-1111-1111-111111111111"

type captureBroker struct {
	mu     sync.Mutex
	topics []string
}

func (c *captureBroker) Publish(_ context.Context, topic string, _, _ []byte, _ map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.topics = append(c.topics, topic)
	return nil
}

func (c *captureBroker) Close() error { return nil }

func (c *captureBroker) has(topic string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range c.topics {
		if t == topic {
			return true
		}
	}
	return false
}

func testHandlers() (*SupervisionHandlers, *http.ServeMux, *captureBroker) {
	broker := &captureBroker{}
	h := NewSupervisionHandlers(store.NewApprovalStore(), store.NewEscalationStore(), store.NewInterventionStore(), store.NewHitlStore(), events.NewPublisherWithBroker(broker), 100)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return h, mux, broker
}

func do(mux *http.ServeMux, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	ctx := ctxkeys.WithTenantID(context.Background(), testTenant)
	ctx = ctxkeys.WithUserID(ctx, "supervisor-1")
	ctx = ctxkeys.WithRequestID(ctx, "req-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func createApproval(t *testing.T, mux *http.ServeMux, requestID string) store.Approval {
	t.Helper()
	w := do(mux, "POST", "/approvals", map[string]interface{}{
		"request_id":   requestID,
		"requester_id": "22222222-2222-2222-2222-222222222222",
		"type":         "parallel",
		"title":        "Send contract to customer",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create approval: status %d: %s", w.Code, w.Body.String())
	}
	var a store.Approval
	json.Unmarshal(w.Body.Bytes(), &a)
	return a
}

func TestApprovalGateFlow(t *testing.T) {
	_, mux, broker := testHandlers()
	a := createApproval(t, mux, "33333333-3333-3333-3333-333333333333")

	if !broker.has("operan.supervision.gate.raised") {
		t.Errorf("gate.raised not published: %v", broker.topics)
	}

	// Approve it.
	w := do(mux, "POST", "/approvals/"+a.ID+"/approve", map[string]interface{}{
		"approver_id": "44444444-4444-4444-4444-444444444444",
		"comment":     "looks good",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("approve: status %d: %s", w.Code, w.Body.String())
	}
	var approved store.Approval
	json.Unmarshal(w.Body.Bytes(), &approved)
	if approved.Status != "approved" {
		t.Errorf("status = %s", approved.Status)
	}
	if !broker.has("operan.supervision.gate.responded") {
		t.Errorf("gate.responded not published: %v", broker.topics)
	}

	// Approving again conflicts (409).
	if w := do(mux, "POST", "/approvals/"+a.ID+"/approve", map[string]interface{}{"approver_id": "u"}); w.Code != http.StatusConflict {
		t.Errorf("double approve: status %d, want 409", w.Code)
	}
}

func TestApprovalRejectAndDelegate(t *testing.T) {
	_, mux, broker := testHandlers()

	a := createApproval(t, mux, "rej-1")
	w := do(mux, "POST", "/approvals/"+a.ID+"/reject", map[string]interface{}{
		"rejector_id": "u-1", "reason": "policy risk",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("reject: status %d: %s", w.Code, w.Body.String())
	}
	var rejected store.Approval
	json.Unmarshal(w.Body.Bytes(), &rejected)
	if rejected.Status != "rejected" {
		t.Errorf("status = %s", rejected.Status)
	}

	b := createApproval(t, mux, "del-1")
	w = do(mux, "POST", "/approvals/"+b.ID+"/delegate", map[string]interface{}{
		"delegator_id": "u-1", "new_approver_id": "u-2", "reason": "on leave",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("delegate: status %d: %s", w.Code, w.Body.String())
	}
	if !broker.has("operan.supervision.gate.escalated") {
		t.Errorf("gate.escalated not published: %v", broker.topics)
	}

	// Validation errors.
	if w := do(mux, "POST", "/approvals/"+b.ID+"/reject", map[string]interface{}{"rejector_id": "u"}); w.Code != http.StatusBadRequest {
		t.Errorf("reject without reason: status %d", w.Code)
	}
	if w := do(mux, "POST", "/approvals", map[string]interface{}{"request_id": "x", "requester_id": "y", "type": "bogus"}); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad type: status %d", w.Code)
	}
}

func TestApprovalExpiryPublishesTimeout(t *testing.T) {
	_, mux, broker := testHandlers()
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	w := do(mux, "POST", "/approvals", map[string]interface{}{
		"request_id": "exp-1", "requester_id": "u", "type": "parallel", "expires_at": past,
	})
	var a store.Approval
	json.Unmarshal(w.Body.Bytes(), &a)

	gw := do(mux, "GET", "/approvals/"+a.ID, nil)
	var got store.Approval
	json.Unmarshal(gw.Body.Bytes(), &got)
	if got.Status != "expired" {
		t.Errorf("status = %s, want expired", got.Status)
	}
	if !broker.has("operan.supervision.gate.timeout") {
		t.Errorf("gate.timeout not published: %v", broker.topics)
	}
}

func TestEscalationFlow(t *testing.T) {
	_, mux, broker := testHandlers()

	w := do(mux, "POST", "/escalations", map[string]interface{}{
		"severity": "critical", "category": "security", "title": "prompt injection detected",
		"source_agent_id": "55555555-5555-5555-5555-555555555555",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create escalation: status %d: %s", w.Code, w.Body.String())
	}
	var e store.Escalation
	json.Unmarshal(w.Body.Bytes(), &e)
	if e.Status != "open" {
		t.Errorf("status = %s", e.Status)
	}
	if !broker.has("operan.supervision.policy.violation_detected") {
		t.Errorf("policy.violation_detected not published for security escalation: %v", broker.topics)
	}

	rw := do(mux, "POST", "/escalations/"+e.ID+"/resolve", map[string]interface{}{
		"resolver_id": "u-1", "resolution_notes": "mitigated", "resolution_type": "manual_resolution",
	})
	if rw.Code != http.StatusOK {
		t.Fatalf("resolve: status %d: %s", rw.Code, rw.Body.String())
	}
	if w := do(mux, "POST", "/escalations/"+e.ID+"/resolve", map[string]interface{}{"resolver_id": "u-1"}); w.Code != http.StatusConflict {
		t.Errorf("double resolve: status %d, want 409", w.Code)
	}

	// Operational escalation (no violation event expected) and bad enum.
	broker.topics = nil
	do(mux, "POST", "/escalations", map[string]interface{}{"severity": "low", "category": "operational", "title": "slow queue"})
	if broker.has("operan.supervision.policy.violation_detected") {
		t.Error("operational escalation should not publish violation")
	}
	if w := do(mux, "POST", "/escalations", map[string]interface{}{"severity": "bogus", "category": "security", "title": "x"}); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad severity: status %d", w.Code)
	}
}

func TestInterventionFlow(t *testing.T) {
	_, mux, _ := testHandlers()

	w := do(mux, "POST", "/interventions", map[string]interface{}{
		"action": "pause", "target_agent_id": "agent-7", "reason": "runaway tool loop",
		"duration_minutes": 30, "scope": map[string]string{"type": "agent", "resource_id": "agent-7"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create intervention: status %d: %s", w.Code, w.Body.String())
	}
	var iv store.Intervention
	json.Unmarshal(w.Body.Bytes(), &iv)
	if iv.Status != "active" || iv.ExpiresAt == nil || iv.IssuedBy != "supervisor-1" {
		t.Errorf("intervention = %+v", iv)
	}

	rw := do(mux, "POST", "/interventions/"+iv.ID+"/revoke", nil)
	if rw.Code != http.StatusOK {
		t.Fatalf("revoke: status %d: %s", rw.Code, rw.Body.String())
	}
	var revoked store.Intervention
	json.Unmarshal(rw.Body.Bytes(), &revoked)
	if revoked.Status != "revoked" || revoked.RevokedBy != "supervisor-1" {
		t.Errorf("revoked = %+v", revoked)
	}
	if w := do(mux, "POST", "/interventions/"+iv.ID+"/revoke", nil); w.Code != http.StatusConflict {
		t.Errorf("double revoke: status %d, want 409", w.Code)
	}

	if w := do(mux, "POST", "/interventions", map[string]interface{}{"action": "bogus", "target_agent_id": "a", "reason": "r"}); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad action: status %d", w.Code)
	}
}

func TestQueueMergesAllTypes(t *testing.T) {
	_, mux, _ := testHandlers()
	createApproval(t, mux, "q-1")
	do(mux, "POST", "/escalations", map[string]interface{}{"severity": "p0", "category": "system", "title": "outage"})
	do(mux, "POST", "/interventions", map[string]interface{}{"action": "stop", "target_agent_id": "a", "reason": "r"})

	w := do(mux, "GET", "/queue", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("queue: status %d", w.Code)
	}
	var q struct {
		Items []store.QueueItem `json:"items"`
		Total int               `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &q)
	if q.Total != 3 {
		t.Fatalf("queue total = %d, want 3", q.Total)
	}
	types := map[string]bool{}
	for _, it := range q.Items {
		types[it.ItemType] = true
		if it.ItemType == "escalation" && it.Priority != "critical" {
			t.Errorf("p0 escalation priority = %s, want critical", it.Priority)
		}
	}
	if !types["approval"] || !types["escalation"] || !types["intervention"] {
		t.Errorf("queue types = %v", types)
	}

	fw := do(mux, "GET", "/queue?type=escalation", nil)
	json.Unmarshal(fw.Body.Bytes(), &q)
	if q.Total != 1 || q.Items[0].ItemType != "escalation" {
		t.Errorf("filtered queue = %+v", q)
	}
	if w := do(mux, "GET", "/queue?type=bogus", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad type filter: status %d", w.Code)
	}
}

func TestRiskDashboard(t *testing.T) {
	_, mux, _ := testHandlers()
	createApproval(t, mux, "d-1")
	do(mux, "POST", "/escalations", map[string]interface{}{"severity": "critical", "category": "security", "title": "incident"})
	do(mux, "POST", "/interventions", map[string]interface{}{"action": "suspend", "target_agent_id": "a", "reason": "r"})

	w := do(mux, "GET", "/risk-dashboard?tenant_id="+testTenant, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard: status %d", w.Code)
	}
	var d map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &d)
	if d["active_approvals_count"].(float64) != 1 || d["pending_escalations_count"].(float64) != 1 || d["active_interventions_count"].(float64) != 1 {
		t.Errorf("counts = %v", d)
	}
	// 20 (critical) + 3 (approval) + 15 (intervention) = 38
	if d["overall_risk_score"].(float64) != 38 {
		t.Errorf("risk score = %v, want 38", d["overall_risk_score"])
	}

	if w := do(mux, "GET", "/risk-dashboard?tenant_id=other", nil); w.Code != http.StatusForbidden {
		t.Errorf("tenant mismatch: status %d", w.Code)
	}
}

func TestHitlAnswerAppliesGateDecision(t *testing.T) {
	_, mux, broker := testHandlers()
	a := createApproval(t, mux, "66666666-6666-6666-6666-666666666666")

	w := do(mux, "POST", "/hitl/66666666-6666-6666-6666-666666666666/answer", map[string]interface{}{
		"answer": "Approved — terms verified", "confidence": "high", "action_taken": "approve",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("hitl answer: status %d: %s", w.Code, w.Body.String())
	}

	gw := do(mux, "GET", "/approvals/"+a.ID, nil)
	var got store.Approval
	json.Unmarshal(gw.Body.Bytes(), &got)
	if got.Status != "approved" {
		t.Errorf("approval after hitl = %s, want approved", got.Status)
	}
	if !broker.has("operan.supervision.gate.responded") {
		t.Errorf("gate.responded not published: %v", broker.topics)
	}

	// Duplicate answer conflicts.
	if w := do(mux, "POST", "/hitl/66666666-6666-6666-6666-666666666666/answer", map[string]interface{}{"answer": "again"}); w.Code != http.StatusConflict {
		t.Errorf("duplicate answer: status %d, want 409", w.Code)
	}
	// Answer without a gate still records (no approval lookup required).
	if w := do(mux, "POST", "/hitl/no-gate-request/answer", map[string]interface{}{"answer": "noted", "action_taken": "ignore"}); w.Code != http.StatusCreated {
		t.Errorf("gateless answer: status %d", w.Code)
	}
}

func TestErrorSchemaShape(t *testing.T) {
	_, mux, _ := testHandlers()
	w := do(mux, "GET", "/approvals/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatalf("missing error wrapper: %s", w.Body.String())
	}
	if resp.Error["code"] != "NOT_FOUND" {
		t.Errorf("code = %v, want NOT_FOUND", resp.Error["code"])
	}
	for _, f := range []string{"code", "message", "request_id"} {
		if _, ok := resp.Error[f]; !ok {
			t.Errorf("error missing field %q", f)
		}
	}
}

func TestUpdateAndDeleteEndpoints(t *testing.T) {
	_, mux, _ := testHandlers()

	// Approval update + delete.
	a := createApproval(t, mux, "ud-1")
	w := do(mux, "PATCH", "/approvals/"+a.ID, map[string]interface{}{"title": "renamed", "description": "new desc"})
	if w.Code != http.StatusOK {
		t.Fatalf("patch approval: status %d: %s", w.Code, w.Body.String())
	}
	var ua store.Approval
	json.Unmarshal(w.Body.Bytes(), &ua)
	if ua.Title != "renamed" {
		t.Errorf("title = %q", ua.Title)
	}
	if w := do(mux, "DELETE", "/approvals/"+a.ID, nil); w.Code != http.StatusNoContent {
		t.Errorf("delete approval: status %d", w.Code)
	}
	if w := do(mux, "DELETE", "/approvals/"+a.ID, nil); w.Code != http.StatusNotFound {
		t.Errorf("double delete approval: status %d", w.Code)
	}

	// Escalation update + delete + get.
	ew := do(mux, "POST", "/escalations", map[string]interface{}{"severity": "medium", "category": "operational", "title": "drift"})
	var e store.Escalation
	json.Unmarshal(ew.Body.Bytes(), &e)
	w = do(mux, "PATCH", "/escalations/"+e.ID, map[string]interface{}{"status": "in_progress", "assigned_to": "u-9"})
	if w.Code != http.StatusOK {
		t.Fatalf("patch escalation: status %d: %s", w.Code, w.Body.String())
	}
	if w := do(mux, "PATCH", "/escalations/"+e.ID, map[string]interface{}{"status": "bogus"}); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad status patch: %d", w.Code)
	}
	if w := do(mux, "GET", "/escalations/"+e.ID, nil); w.Code != http.StatusOK {
		t.Errorf("get escalation: status %d", w.Code)
	}
	if w := do(mux, "DELETE", "/escalations/"+e.ID, nil); w.Code != http.StatusNoContent {
		t.Errorf("delete escalation: status %d", w.Code)
	}

	// Intervention update + get + delete.
	iw := do(mux, "POST", "/interventions", map[string]interface{}{"action": "restrict", "target_agent_id": "a-1", "reason": "scope creep"})
	var iv store.Intervention
	json.Unmarshal(iw.Body.Bytes(), &iv)
	w = do(mux, "PATCH", "/interventions/"+iv.ID, map[string]interface{}{"reason": "tightened", "duration_minutes": 15})
	if w.Code != http.StatusOK {
		t.Fatalf("patch intervention: status %d: %s", w.Code, w.Body.String())
	}
	var uiv store.Intervention
	json.Unmarshal(w.Body.Bytes(), &uiv)
	if uiv.Reason != "tightened" || uiv.ExpiresAt == nil {
		t.Errorf("patched intervention = %+v", uiv)
	}
	if w := do(mux, "PATCH", "/interventions/"+iv.ID, map[string]interface{}{"duration_minutes": 99999}); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad duration patch: %d", w.Code)
	}
	if w := do(mux, "GET", "/interventions/"+iv.ID, nil); w.Code != http.StatusOK {
		t.Errorf("get intervention: status %d", w.Code)
	}
	if w := do(mux, "DELETE", "/interventions/"+iv.ID, nil); w.Code != http.StatusNoContent {
		t.Errorf("delete intervention: status %d", w.Code)
	}
}

func TestQueueUserFilterAndPagination(t *testing.T) {
	_, mux, _ := testHandlers()
	// Approval assigned to u-7 via required_approvers.
	do(mux, "POST", "/approvals", map[string]interface{}{
		"request_id": "qf-1", "requester_id": "u", "type": "parallel", "title": "assigned",
		"required_approvers": []map[string]string{{"user_id": "u-7"}},
	})
	createApproval(t, mux, "qf-2") // unassigned

	w := do(mux, "GET", "/queue?user_id=u-7", nil)
	var q struct {
		Items []store.QueueItem `json:"items"`
		Total int               `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &q)
	if q.Total != 1 || q.Items[0].AssignedTo != "u-7" {
		t.Errorf("user filter: %+v", q)
	}

	pw := do(mux, "GET", "/queue?page=1&page_size=1", nil)
	json.Unmarshal(pw.Body.Bytes(), &q)
	if q.Total != 2 || len(q.Items) != 1 {
		t.Errorf("pagination: total=%d len=%d", q.Total, len(q.Items))
	}
}
