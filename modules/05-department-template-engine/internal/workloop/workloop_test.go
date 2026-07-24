package workloop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// fakeM03 scripts workflow create/execute/state.
func fakeM03(t *testing.T, states []clients.WorkflowState) (*httptest.Server, *int) {
	t.Helper()
	stateIdx := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/workflows"):
			json.NewEncoder(w).Encode(map[string]string{"id": "wf-run-1", "name": "run"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/execute"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/state"):
			i := stateIdx
			if i >= len(states) {
				i = len(states) - 1
			}
			json.NewEncoder(w).Encode(states[i])
			stateIdx++
		default:
			http.NotFound(w, r)
		}
	}))
	return srv, &stateIdx
}

func seedLoop(t *testing.T, orchURL string) (*Loop, *store.ServiceRequest, *store.Department) {
	t.Helper()
	requests := store.NewRequestStore()
	departments := store.NewDepartmentStore()
	templates := store.NewTemplateStore()

	tmpl, _ := templates.Create(&store.Template{
		TenantID: "t1", Name: "IT", Category: "it",
		Workflows: []store.WorkflowDefinition{{
			ID: "wf-desk", Name: "Service Desk SOP",
			Steps: []store.WorkflowStep{
				{ID: "s1", Type: "agent_call", Name: "Triage and draft response"},
				{ID: "s2", Type: "approval", Name: "Manager sign-off"},
			},
		}},
	})
	dept, _ := departments.Create(&store.Department{
		TenantID: "t1", Name: "IT Dept", Category: "it", Status: "operational",
		TemplateID: tmpl.ID,
		Services: []store.ServiceOffering{{
			ID: "svc-desk", Name: "Service Desk", DeliveryWorkflowID: "wf-desk",
		}},
		OrgChart: []store.Position{{ID: "p1", AgentDefID: "a1", AgentID: "live-agent-1"}},
	})
	req, _ := requests.Create(&store.ServiceRequest{
		TenantID: "t1", DepartmentID: dept.ID, ServiceID: "svc-desk",
		ServiceName: "Service Desk", Title: "VPN down",
	})
	loop := New(requests, departments, templates, &clients.OrchestrationClient{BaseURL: orchURL}, events.NewPublisher())
	return loop, req, dept
}

func TestDispatchAndPollToCompletion(t *testing.T) {
	states := []clients.WorkflowState{
		{Status: "running", Nodes: []clients.WorkflowNodeState{
			{NodeID: "s1", Status: "completed", Output: map[string]interface{}{
				"output": "Draft: restart the VPN concentrator", "tokens": float64(120), "node_type": "agent"}},
			{NodeID: "s2", Status: "running", Output: map[string]interface{}{"node_type": "human_gate"}},
		}},
		{Status: "completed", Nodes: []clients.WorkflowNodeState{
			{NodeID: "s1", Status: "completed", Output: map[string]interface{}{
				"output": "Draft: restart the VPN concentrator", "tokens": float64(120), "node_type": "agent"}},
			{NodeID: "s2", Status: "completed", Output: map[string]interface{}{
				"decision": "approved", "node_type": "human_gate"}},
		}},
	}
	srv, _ := fakeM03(t, states)
	defer srv.Close()

	loop, req, dept := seedLoop(t, srv.URL)
	loop.Dispatch("Bearer tok", "t1", req, dept)

	got, _ := loop.Requests.GetByIDAndTenant(req.ID, "t1")
	if got.Status != "in_progress" || got.WorkflowRunRef != "wf-run-1" {
		t.Fatalf("after dispatch: %+v", got)
	}

	// Poll 1: agent output + active gate → awaiting_approval.
	loop.pollOne(context.Background(), *got)
	got, _ = loop.Requests.GetByIDAndTenant(req.ID, "t1")
	if got.Status != "awaiting_approval" {
		t.Fatalf("after poll1 status = %s", got.Status)
	}
	if got.FirstResponseAt == nil || got.TokensUsed != 120 {
		t.Errorf("first response/tokens: %+v", got)
	}

	// Poll 2: run completed → request completed with output.
	loop.pollOne(context.Background(), *got)
	got, _ = loop.Requests.GetByIDAndTenant(req.ID, "t1")
	if got.Status != "completed" || got.CompletedAt == nil {
		t.Fatalf("after poll2: %+v", got)
	}
	if !strings.Contains(got.Output, "VPN concentrator") {
		t.Errorf("output = %q", got.Output)
	}
	kinds := map[string]bool{}
	for _, ev := range got.Timeline {
		kinds[ev.Kind] = true
	}
	for _, want := range []string{"created", "dispatched", "agent_output", "gate_raised", "gate_responded", "completed"} {
		if !kinds[want] {
			t.Errorf("timeline missing %q: %+v", want, got.Timeline)
		}
	}
}

func TestPollRunLost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/workflows") {
			json.NewEncoder(w).Encode(map[string]string{"id": "wf-run-1"})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/execute") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	loop, req, dept := seedLoop(t, srv.URL)
	loop.MissLimit = 2
	loop.Dispatch("Bearer tok", "t1", req, dept)

	got, _ := loop.Requests.GetByIDAndTenant(req.ID, "t1")
	loop.pollOne(context.Background(), *got)
	loop.pollOne(context.Background(), *got)
	got, _ = loop.Requests.GetByIDAndTenant(req.ID, "t1")
	if got.Status != "failed" {
		t.Fatalf("lost run should fail request, got %s", got.Status)
	}
}

func TestDispatchFallbackManualGate(t *testing.T) {
	states := []clients.WorkflowState{{Status: "running"}}
	srv, _ := fakeM03(t, states)
	defer srv.Close()

	loop, _, dept := seedLoop(t, srv.URL)
	// Request against a service with no resolvable SOP.
	req2, _ := loop.Requests.Create(&store.ServiceRequest{
		TenantID: "t1", DepartmentID: dept.ID, ServiceID: "svc-unknown",
		ServiceName: "Mystery", Title: "Handle this",
	})
	loop.Dispatch("Bearer tok", "t1", req2, dept)
	got, _ := loop.Requests.GetByIDAndTenant(req2.ID, "t1")
	if got.Status != "in_progress" {
		t.Fatalf("fallback dispatch should still run: %+v", got)
	}
	found := false
	for _, ev := range got.Timeline {
		if ev.Kind == "dispatched" && strings.Contains(ev.Detail, "Manual handling") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected manual-handling dispatch note: %+v", got.Timeline)
	}
}
