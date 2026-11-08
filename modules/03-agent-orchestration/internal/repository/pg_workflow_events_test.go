package repository

import (
	"testing"

	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// workflow_events.workflow_id is UUID NOT NULL REFERENCES workflows(id), and
// AddEvent used to bind evt.NodeID into it as well as into node_id. Every write
// therefore failed 22P02 on a node name like "access-02", and both callers
// threw the error away — so workflow_events never held a row and a failed run's
// reason died with its goroutine. Three blockers in the department work loop had
// to be reverse-engineered from timings and side effects because of it.
func TestAddEvent_BindsWorkflowIDNotNodeID(t *testing.T) {
	db, drv := newFakeDB(1, nil)
	defer db.Close()
	r := &WorkflowPostgres{db: db}

	const wfID = "d31d3ee3-a78e-4560-818a-7ea447e61592"
	const nodeID = "access-02"

	if err := r.AddEvent(wfID, &store.ExecutionEvent{
		EventID:   "11111111-1111-4111-8111-111111111111",
		NodeID:    nodeID,
		EventType: "node_failed",
		Details:   map[string]interface{}{"error": "raise approval: boom"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// INSERT ... (event_id, workflow_id, node_id, event_type, timestamp, details)
	if len(drv.lastArgs) < 3 {
		t.Fatalf("expected the insert to bind at least 3 args, got %d", len(drv.lastArgs))
	}
	if got := drv.lastArgs[1]; got != wfID {
		t.Errorf("workflow_id bound to %v — want the workflow id %q; a node name here is the 22P02", got, wfID)
	}
	if got := drv.lastArgs[2]; got != nodeID {
		t.Errorf("node_id bound to %v — want %q", got, nodeID)
	}
}

// Without an id there is no row worth writing: reject it here rather than
// sending an empty string for the column to refuse.
func TestAddEvent_RefusesAnEmptyWorkflowID(t *testing.T) {
	db, _ := newFakeDB(1, nil)
	defer db.Close()
	r := &WorkflowPostgres{db: db}

	if err := r.AddEvent("", &store.ExecutionEvent{EventType: "node_failed"}); err == nil {
		t.Error("expected an error when the workflow id is empty")
	}
}
