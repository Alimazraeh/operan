package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/draft"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// Run-scoped caller credentials travel in the execution context (memory only —
// never persisted with the workflow).
type runAuthKey struct{}

// RunAuth carries the identity a workflow run executes under.
type RunAuth struct {
	Authorization string
	TenantID      string
	UserID        string
}

// WithRunAuth returns ctx carrying the run identity.
func WithRunAuth(ctx context.Context, a RunAuth) context.Context {
	return context.WithValue(ctx, runAuthKey{}, a)
}

// RunAuthFrom extracts the run identity (zero value when absent).
func RunAuthFrom(ctx context.Context) RunAuth {
	if a, ok := ctx.Value(runAuthKey{}).(RunAuth); ok {
		return a
	}
	return RunAuth{}
}

// HumanTaskCreator is the slice of the human-task store the gate handler needs.
type HumanTaskCreator interface {
	Create(t *store.HumanTask) (*store.HumanTask, error)
	GetByID(id string) (*store.HumanTask, error)
}

// NodeHandlerDeps wires the real node executor.
type NodeHandlerDeps struct {
	Draft         *draft.Engine    // nil → agent nodes fail honestly (LLM not configured)
	Tasks         HumanTaskCreator // human-gate tasks
	M09BaseURL    string           // human-supervision base URL; empty → gates fail honestly
	GatePollEvery time.Duration    // default 5s
	GateTimeout   time.Duration    // default 24h
}

// NewNodeHandler builds the production NodeHandler executing each node type
// for real: agent → grounded LLM draft; human_gate → human task + M09
// approval, waiting for the (US-402) gate response; action/condition →
// recorded pass-through (no executor bound yet — stated, not faked).
func NewNodeHandler(deps NodeHandlerDeps) NodeHandler {
	if deps.GatePollEvery <= 0 {
		deps.GatePollEvery = 5 * time.Second
	}
	if deps.GateTimeout <= 0 {
		deps.GateTimeout = 24 * time.Hour
	}
	httpc := &http.Client{Timeout: 15 * time.Second}

	return func(ctx context.Context, node store.WorkflowNode, workflowID string, variables map[string]interface{}) (map[string]interface{}, error) {
		auth := RunAuthFrom(ctx)

		switch node.Type {
		case store.WorkflowNodeAgent:
			if deps.Draft == nil || deps.Draft.LLM == nil {
				return nil, fmt.Errorf("agent node %s: LLM gateway not configured", node.ID)
			}
			role := str(variables["role"])
			if r, ok := node.Parameters["role"].(string); ok && r != "" {
				role = r
			}
			instruction := buildInstruction(node, variables)
			out, err := deps.Draft.Draft(ctx, draft.Input{
				Role:          role,
				Instruction:   instruction,
				DepartmentID:  str(variables["department_id"]),
				Authorization: auth.Authorization,
				TenantID:      auth.TenantID,
				MaxTokens:     2000,
			})
			if err != nil {
				return nil, fmt.Errorf("agent node %s: %w", node.ID, err)
			}
			return map[string]interface{}{
				"text":   bound(out.Text, 8000),
				"model":  out.Model,
				"tokens": out.Tokens,
				"action": node.Action,
			}, nil

		case store.WorkflowNodeHumanGate:
			if deps.Tasks == nil || deps.M09BaseURL == "" {
				return nil, fmt.Errorf("human_gate node %s: supervision not configured", node.ID)
			}
			title := str(variables["request_title"])
			if title == "" {
				title = node.Action
			}
			instructions := "Sign-off required: " + node.Action +
				"\n\nRequest: " + title
			if body := str(variables["request_body"]); body != "" {
				instructions += "\n\n" + bound(body, 2000)
			}
			if prior := str(variables["last_agent_output"]); prior != "" {
				instructions += "\n\nAgent work product:\n" + bound(prior, 4000)
			}
			task, err := deps.Tasks.Create(&store.HumanTask{
				TenantID:            auth.TenantID,
				PipelineExecutionID: workflowID, // run-scoped correlation
				StepID:              node.ID,
				AssigneeType:        "role",
				AssigneeID:          "manager",
				TaskType:            "approval",
				Instructions:        instructions,
				Label:               bound(title, 120),
				Status:              "pending",
			})
			if err != nil {
				return nil, fmt.Errorf("human_gate node %s: create task: %w", node.ID, err)
			}

			// Raise the M09 approval — request_id = task id is exactly the
			// correlation the US-402 gates consumer resolves back onto the task.
			approval := map[string]interface{}{
				"request_id":   task.ID,
				"requester_id": str(variables["department_id"]),
				"type":         "parallel",
				"title":        bound(title, 100),
			}
			ab, _ := json.Marshal(approval)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				strings.TrimRight(deps.M09BaseURL, "/")+"/approvals", bytes.NewReader(ab))
			if err != nil {
				return nil, fmt.Errorf("human_gate node %s: %w", node.ID, err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", auth.Authorization)
			req.Header.Set("X-Tenant-ID", auth.TenantID)
			resp, err := httpc.Do(req)
			if err != nil {
				return nil, fmt.Errorf("human_gate node %s: raise approval: %w", node.ID, err)
			}
			body, _ := json.Marshal(map[string]string{})
			_ = body
			respBody := make([]byte, 0)
			if resp.Body != nil {
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				respBody = buf.Bytes()
				resp.Body.Close()
			}
			if resp.StatusCode >= 300 {
				return nil, fmt.Errorf("human_gate node %s: approval upstream %d: %s",
					node.ID, resp.StatusCode, bound(string(respBody), 160))
			}

			// Wait for the human decision (gates consumer flips the task).
			deadline := time.Now().Add(deps.GateTimeout)
			ticker := time.NewTicker(deps.GatePollEvery)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-ticker.C:
					t, err := deps.Tasks.GetByID(task.ID)
					if err != nil {
						continue
					}
					switch string(t.Status) {
					case "approved":
						return map[string]interface{}{
							"decision": "approved", "task_id": task.ID,
							"responded_by": t.RespondedBy, "action": node.Action,
						}, nil
					case "rejected":
						return nil, fmt.Errorf("human_gate node %s: rejected by %s", node.ID, t.RespondedBy)
					}
					if time.Now().After(deadline) {
						return nil, fmt.Errorf("human_gate node %s: approval timed out", node.ID)
					}
				}
			}

		default: // action, condition, and anything future
			return map[string]interface{}{
				"note":   "no executor bound for node type " + string(node.Type) + " — recorded as pass-through",
				"action": node.Action,
			}, nil
		}
	}
}

func buildInstruction(node store.WorkflowNode, variables map[string]interface{}) string {
	var b strings.Builder
	if t := str(variables["request_title"]); t != "" {
		b.WriteString("Service request: " + t + "\n")
	}
	if body := str(variables["request_body"]); body != "" {
		b.WriteString(bound(body, 4000) + "\n\n")
	}
	if prior := str(variables["last_agent_output"]); prior != "" {
		b.WriteString("Work so far:\n" + bound(prior, 4000) + "\n\n")
	}
	step := node.Action
	if step == "" {
		step = node.ID
	}
	b.WriteString("Your step in the SOP: " + step + ". Produce this step's work product.")
	return b.String()
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func bound(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
