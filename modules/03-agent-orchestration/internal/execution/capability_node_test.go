package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/03-agent-orchestration/internal/capability"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// The step's declared inputs resolve {{variable}} references from the run.
func TestSubstituteInputsResolvesRunVariables(t *testing.T) {
	got := substituteInputs(map[string]interface{}{
		"team":       "helpdesk",
		"ticket_ref": "{{request_id}}",
		"reason":     "{{request_title}} ({{priority}})",
		"count":      3,
	}, map[string]interface{}{
		"request_id": "req-42", "request_title": "VPN down", "priority": "high",
	})
	if got["ticket_ref"] != "req-42" {
		t.Fatalf("ticket_ref = %v", got["ticket_ref"])
	}
	if got["reason"] != "VPN down (high)" {
		t.Fatalf("reason = %v", got["reason"])
	}
	if got["team"] != "helpdesk" || got["count"] != 3 {
		t.Fatalf("literals mangled: %v", got)
	}
}

// A long variable (an agent draft) must not balloon the capability payload.
func TestSubstituteInputsBoundsLongValues(t *testing.T) {
	long := strings.Repeat("é", 5000) // multibyte: bounding must be rune-safe
	got := substituteInputs(map[string]interface{}{"resolution": "{{last_agent_output}}"},
		map[string]interface{}{"last_agent_output": long})
	s, _ := got["resolution"].(string)
	if len([]rune(s)) > 2001 {
		t.Fatalf("value not bounded: %d runes", len([]rune(s)))
	}
	if !strings.HasPrefix(s, "é") {
		t.Fatalf("rune boundary broken: %q", s[:8])
	}
}

func capabilityStub(t *testing.T, status string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoke" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var req capability.InvokeRequest
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if status == "completed" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "inv-1", "capability_id": req.CapabilityID, "status": "completed",
				"simulated": true, "provider_kind": "simulated",
				"policy_decision": "allowed",
				"output":          map[string]interface{}{"assigned_team": "helpdesk"},
				"external_ref": map[string]interface{}{
					"system": "simulated-itsm", "kind": "ticket", "id": "SIM-1", "url": "https://x",
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "inv-2", "capability_id": req.CapabilityID, "status": status,
			"error": "no enabled binding for " + req.CapabilityID,
		})
	}))
}

func capabilityNode() store.WorkflowNode {
	return store.WorkflowNode{
		ID: "ticket-02", Type: store.WorkflowNodeAction, Action: "Route Ticket",
		Parameters: map[string]interface{}{
			"capability":          "itsm.ticket.assign",
			"inputs":              map[string]interface{}{"team": "helpdesk", "ticket_ref": "{{request_id}}"},
			"actor_agent_id":      "agent-9",
			"actor_position_id":   "pos-sys",
			"actor_autonomy_tier": "execute",
		},
	}
}

// A completed invocation lands on the node with its audit identity — and
// deliberately without "text", which would chain into last_agent_output and
// overwrite the draft a later gate shows its approver.
func TestActionNodeDispatchesThroughTheFunnel(t *testing.T) {
	srv := capabilityStub(t, "completed")
	defer srv.Close()
	handler := NewNodeHandler(NodeHandlerDeps{Capabilities: capability.New(srv.URL)})

	out, err := handler(context.Background(), capabilityNode(), "wf-1",
		map[string]interface{}{"request_id": "req-7", "department_id": "dept-1"})
	if err != nil {
		t.Fatal(err)
	}
	if out["execution_id"] != "inv-1" || out["simulated"] != true {
		t.Fatalf("audit identity missing: %v", out)
	}
	if _, hasText := out["text"]; hasText {
		t.Fatal(`capability output must not emit "text" — it would clobber last_agent_output`)
	}
	sum, _ := out["summary"].(string)
	if !strings.Contains(sum, "SIMULATED") {
		t.Fatalf("a simulated action must say so where people read it: %q", sum)
	}
}

// A refusal fails the node with the funnel's stage and reason — the run stops
// honestly instead of passing an unperformed action.
func TestRefusedInvocationFailsTheNode(t *testing.T) {
	srv := capabilityStub(t, "blocked_no_binding")
	defer srv.Close()
	handler := NewNodeHandler(NodeHandlerDeps{Capabilities: capability.New(srv.URL)})

	_, err := handler(context.Background(), capabilityNode(), "wf-1", map[string]interface{}{})
	if err == nil {
		t.Fatal("refused invocation completed the node")
	}
	if !strings.Contains(err.Error(), "blocked_no_binding") {
		t.Fatalf("the stage must be in the failure: %v", err)
	}
}

// Without a capability service configured, capability-bearing nodes stay on
// the recorded pass-through — stated, never faked.
func TestNoCapabilityServiceKeepsThePassThrough(t *testing.T) {
	handler := NewNodeHandler(NodeHandlerDeps{})
	out, err := handler(context.Background(), capabilityNode(), "wf-1", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	note, _ := out["note"].(string)
	if !strings.Contains(note, "pass-through") {
		t.Fatalf("expected the honest pass-through, got %v", out)
	}
}
