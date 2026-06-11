// Package gates closes the human-supervision enforcement loop (US-402):
// Module 09 publishes gate lifecycle events; this consumer applies the
// human decision to the orchestrator's human tasks, so a pipeline blocked
// on a gate resumes on approval and fails on rejection.
//
// Correlation contract: when the orchestrator requests an approval from
// Module 09, it sends its human task ID as the approval's request_id.
// Module 09 echoes that as workflow_id on gate.raised; gate.responded
// carries only gate_id, so this consumer remembers the mapping.
package gates

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// TaskResponder is the slice of the human-task store the enforcer needs.
type TaskResponder interface {
	Respond(id string, action string, response map[string]interface{}, respondedBy string, comments string) (*store.HumanTask, error)
}

// Subscriber is the slice of the event broker the enforcer needs.
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage func(ctx context.Context, msg Message)) error
}

// Message mirrors events.Message without importing the package (avoids a
// dependency cycle; main adapts between the two).
type Message struct {
	Topic string
	Value []byte
}

// Enforcer maps Module 09 gate decisions onto orchestrator human tasks.
type Enforcer struct {
	mu    sync.Mutex
	gates map[string]gateRef // gate_id -> originating task
	tasks TaskResponder
}

type gateRef struct {
	TaskID   string
	TenantID string
}

// NewEnforcer creates an Enforcer over the given human-task store.
func NewEnforcer(tasks TaskResponder) *Enforcer {
	return &Enforcer{gates: make(map[string]gateRef), tasks: tasks}
}

// gateRaised is the slice of Module 09's GateRaised payload we need.
type gateRaised struct {
	GateID     string `json:"gate_id"`
	TenantID   string `json:"tenant_id"`
	WorkflowID string `json:"workflow_id"` // = the approval's request_id = our task ID
}

// gateResponded is the slice of Module 09's GateResponded payload we need.
type gateResponded struct {
	GateID     string `json:"gate_id"`
	TenantID   string `json:"tenant_id"`
	Response   string `json:"response"` // approve | reject | request_revision | escalate
	ResponseBy string `json:"response_by"`
	Comments   *string `json:"comments"`
}

// HandleRaised records the gate → task correlation.
func (e *Enforcer) HandleRaised(value []byte) {
	var p gateRaised
	if err := json.Unmarshal(value, &p); err != nil || p.GateID == "" || p.WorkflowID == "" {
		return
	}
	e.mu.Lock()
	e.gates[p.GateID] = gateRef{TaskID: p.WorkflowID, TenantID: p.TenantID}
	e.mu.Unlock()
}

// HandleResponded applies a human decision to the originating task.
// approve → task approved; reject / request_revision → task rejected;
// escalate leaves the task pending for the next approver.
func (e *Enforcer) HandleResponded(value []byte) {
	var p gateResponded
	if err := json.Unmarshal(value, &p); err != nil || p.GateID == "" {
		return
	}

	e.mu.Lock()
	ref, ok := e.gates[p.GateID]
	e.mu.Unlock()
	if !ok {
		return // gate not raised by this orchestrator instance
	}

	var action string
	switch p.Response {
	case "approve":
		action = "approve"
	case "reject", "request_revision":
		action = "reject"
	default: // escalate — still awaiting a decision
		return
	}

	comments := ""
	if p.Comments != nil {
		comments = *p.Comments
	}
	if _, err := e.tasks.Respond(ref.TaskID, action, map[string]interface{}{"gate_id": p.GateID}, p.ResponseBy, comments); err != nil {
		log.Printf("[GATES] gate %s: applying %s to task %s failed: %v", p.GateID, action, ref.TaskID, err)
		return
	}
	log.Printf("[GATES] gate %s %sd task %s (by %s)", p.GateID, action, ref.TaskID, p.ResponseBy)

	e.mu.Lock()
	delete(e.gates, p.GateID)
	e.mu.Unlock()
}

// Start subscribes the enforcer to the supervision gate topics.
func (e *Enforcer) Start(ctx context.Context, sub Subscriber, consumerGroup string) error {
	if err := sub.Subscribe(ctx, "operan.supervision.gate.raised", consumerGroup, func(_ context.Context, msg Message) {
		e.HandleRaised(msg.Value)
	}); err != nil {
		return err
	}
	if err := sub.Subscribe(ctx, "operan.supervision.gate.responded", consumerGroup, func(_ context.Context, msg Message) {
		e.HandleResponded(msg.Value)
	}); err != nil {
		return err
	}
	log.Printf("[GATES] supervision gate enforcement active (group %s)", consumerGroup)
	return nil
}
