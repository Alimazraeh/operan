package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// fakeTasks records what the gate handler asked for and immediately approves,
// so the handler returns without waiting on the poll loop.
type fakeTasks struct{ created *store.HumanTask }

func (f *fakeTasks) Create(t *store.HumanTask) (*store.HumanTask, error) {
	cp := *t
	cp.ID = "task-1"
	cp.Status = "approved"
	f.created = &cp
	return &cp, nil
}
func (f *fakeTasks) GetByID(id string) (*store.HumanTask, error) { return f.created, nil }

// runGate executes a human_gate node and returns what M09 was sent plus the
// task that was created.
func runGate(t *testing.T, params map[string]interface{}) (map[string]interface{}, *store.HumanTask) {
	t.Helper()
	var sent map[string]interface{}
	m09 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "apr-1"})
	}))
	defer m09.Close()

	tasks := &fakeTasks{}
	h := NewNodeHandler(NodeHandlerDeps{
		Tasks: tasks, M09BaseURL: m09.URL,
		GatePollEvery: time.Millisecond, GateTimeout: 2 * time.Second,
	})
	node := store.WorkflowNode{ID: "s2", Type: store.WorkflowNodeHumanGate,
		Action: "CAB Review", Parameters: params}
	if _, err := h(context.Background(), node, "wf-1",
		map[string]interface{}{"request_title": "Change X", "department_id": "d1"}); err != nil {
		t.Fatalf("gate node: %v", err)
	}
	return sent, tasks.created
}

// When Module 05 resolved a real holder for the seat that owns the gate, the
// approval must be routed to that person — this is what makes a personal
// approval inbox possible at all.
func TestGateRoutesToTheResolvedApprover(t *testing.T) {
	sent, task := runGate(t, map[string]interface{}{
		"required_approver_user_id": "user-dana",
	})
	if task.AssigneeType != "user" || task.AssigneeID != "user-dana" {
		t.Errorf("task assignee = %s/%s, want user/user-dana", task.AssigneeType, task.AssigneeID)
	}
	ra, _ := sent["required_approvers"].([]interface{})
	if len(ra) != 1 {
		t.Fatalf("required_approvers = %v", sent["required_approvers"])
	}
	if got := ra[0].(map[string]interface{})["user_id"]; got != "user-dana" {
		t.Errorf("required approver = %v, want user-dana", got)
	}
}

// With nobody bound to the seat the gate falls back to a role — a target M09
// models — and must never name a fabricated user.
func TestGateFallsBackToARoleNotAFabricatedUser(t *testing.T) {
	sent, task := runGate(t, nil)
	if task.AssigneeType != "role" || task.AssigneeID != "department_head" {
		t.Errorf("task assignee = %s/%s, want role/department_head", task.AssigneeType, task.AssigneeID)
	}
	ra, _ := sent["required_approvers"].([]interface{})
	if len(ra) != 1 {
		t.Fatalf("required_approvers = %v", sent["required_approvers"])
	}
	target := ra[0].(map[string]interface{})
	if target["role"] != "department_head" {
		t.Errorf("fallback target = %v, want a role", target)
	}
	if _, hasUser := target["user_id"]; hasUser {
		t.Error("the fallback names a user — an approver was invented")
	}
	// The old behaviour hardcoded "manager", which matched no role the
	// platform defines.
	if task.AssigneeID == "manager" {
		t.Error("still routing to the hardcoded 'manager'")
	}
}
