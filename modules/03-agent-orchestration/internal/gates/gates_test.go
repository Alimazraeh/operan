package gates

import (
	"fmt"
	"testing"

	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// fakeResponder records Respond calls and can simulate store errors.
type fakeResponder struct {
	calls []respondCall
	err   error
}

type respondCall struct {
	ID, Action, By, Comments string
}

func (f *fakeResponder) Respond(id, action string, _ map[string]interface{}, by, comments string) (*store.HumanTask, error) {
	f.calls = append(f.calls, respondCall{ID: id, Action: action, By: by, Comments: comments})
	if f.err != nil {
		return nil, f.err
	}
	return &store.HumanTask{ID: id}, nil
}

func raised(gateID, taskID string) []byte {
	return []byte(`{"gate_id":"` + gateID + `","tenant_id":"t1","workflow_id":"` + taskID + `"}`)
}

func responded(gateID, response string) []byte {
	return []byte(`{"gate_id":"` + gateID + `","tenant_id":"t1","response":"` + response + `","response_by":"supervisor-1","comments":"checked"}`)
}

func TestApproveResumesTask(t *testing.T) {
	f := &fakeResponder{}
	e := NewEnforcer(f)

	e.HandleRaised(raised("g1", "task-1"))
	e.HandleResponded(responded("g1", "approve"))

	if len(f.calls) != 1 {
		t.Fatalf("Respond calls = %d, want 1", len(f.calls))
	}
	c := f.calls[0]
	if c.ID != "task-1" || c.Action != "approve" || c.By != "supervisor-1" || c.Comments != "checked" {
		t.Errorf("call = %+v", c)
	}

	// Mapping is consumed: a second response for the same gate is a no-op.
	e.HandleResponded(responded("g1", "approve"))
	if len(f.calls) != 1 {
		t.Errorf("consumed gate replayed: %d calls", len(f.calls))
	}
}

func TestRejectAndRevisionFailTask(t *testing.T) {
	for _, response := range []string{"reject", "request_revision"} {
		f := &fakeResponder{}
		e := NewEnforcer(f)
		e.HandleRaised(raised("g1", "task-1"))
		e.HandleResponded(responded("g1", response))
		if len(f.calls) != 1 || f.calls[0].Action != "reject" {
			t.Errorf("%s: calls = %+v", response, f.calls)
		}
	}
}

func TestEscalateLeavesTaskPending(t *testing.T) {
	f := &fakeResponder{}
	e := NewEnforcer(f)
	e.HandleRaised(raised("g1", "task-1"))
	e.HandleResponded(responded("g1", "escalate"))
	if len(f.calls) != 0 {
		t.Errorf("escalate should not respond to task: %+v", f.calls)
	}
	// The gate stays mapped; a later approve still lands.
	e.HandleResponded(responded("g1", "approve"))
	if len(f.calls) != 1 {
		t.Errorf("approve after escalate: %d calls", len(f.calls))
	}
}

func TestForeignAndMalformedEventsAreIgnored(t *testing.T) {
	f := &fakeResponder{}
	e := NewEnforcer(f)

	e.HandleResponded(responded("never-raised", "approve")) // unknown gate
	e.HandleRaised([]byte(`{{{not json`))
	e.HandleResponded([]byte(`{{{not json`))
	e.HandleRaised([]byte(`{"gate_id":"","workflow_id":""}`)) // missing ids

	if len(f.calls) != 0 {
		t.Errorf("no Respond expected, got %+v", f.calls)
	}
}

func TestStoreErrorKeepsMapping(t *testing.T) {
	f := &fakeResponder{err: fmt.Errorf("task not pending")}
	e := NewEnforcer(f)
	e.HandleRaised(raised("g1", "task-1"))
	e.HandleResponded(responded("g1", "approve"))
	if len(f.calls) != 1 {
		t.Fatalf("Respond calls = %d", len(f.calls))
	}
	// Mapping survives the failure so a retry can land.
	f.err = nil
	e.HandleResponded(responded("g1", "approve"))
	if len(f.calls) != 2 {
		t.Errorf("retry after store error: %d calls", len(f.calls))
	}
}

func TestEndToEndWithRealStore(t *testing.T) {
	tasks := store.NewHumanTaskStore()
	created, err := tasks.Create(&store.HumanTask{TenantID: "t1", PipelineExecutionID: "pe-1", AssigneeID: "u-1", Instructions: "review"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	e := NewEnforcer(tasks)
	e.HandleRaised(raised("g1", created.ID))
	e.HandleResponded(responded("g1", "approve"))

	got, _ := tasks.GetByID(created.ID)
	if got.Status != store.HumanTaskStatusApproved {
		t.Errorf("task status = %s, want approved", got.Status)
	}
	if got.RespondedBy != "supervisor-1" {
		t.Errorf("responded_by = %s", got.RespondedBy)
	}
}
