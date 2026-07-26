package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/capability"
	"github.com/operan/modules/03-agent-orchestration/internal/draft"
	"github.com/operan/modules/03-agent-orchestration/internal/execution/condition"
	"github.com/operan/modules/03-agent-orchestration/internal/llm"
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
	// AgentMaxTokens is the budget for one agent step. Zero uses
	// llm.DefaultMaxTokens, which is sized from what a real SOP step costs
	// against a reasoning model — see the constant.
	AgentMaxTokens int
	// Capabilities is the Module 08 client. Nil keeps capability-bearing
	// action nodes on the recorded pass-through — the honest state for a
	// deployment without the capability service.
	Capabilities *capability.Client
}

// NewNodeHandler builds the production NodeHandler executing each node type
// for real: agent → grounded LLM draft; human_gate → human task + M09
// approval, waiting for the (US-402) gate response; condition → the SOP's
// expression evaluated against the run's variables, recorded with its reason
// when undecidable; action → recorded pass-through until the capability layer
// binds it (stated, not faked).
func NewNodeHandler(deps NodeHandlerDeps) NodeHandler {
	if deps.GatePollEvery <= 0 {
		deps.GatePollEvery = 5 * time.Second
	}
	if deps.GateTimeout <= 0 {
		deps.GateTimeout = 24 * time.Hour
	}
	if deps.AgentMaxTokens <= 0 {
		deps.AgentMaxTokens = llm.DefaultMaxTokens
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
				MaxTokens:     deps.AgentMaxTokens,
			})
			if err != nil {
				return nil, fmt.Errorf("agent node %s: %w", node.ID, err)
			}
			result := map[string]interface{}{
				"text":   bound(out.Text, 8000),
				"model":  out.Model,
				"tokens": out.Tokens,
				"action": node.Action,
			}
			// A draft the model was cut off mid-way through is real work, but
			// it is not finished work. Recording that on the node is what lets
			// the approver see it rather than reading an unfinished assessment
			// as a complete one.
			if out.Truncated {
				result["truncated"] = true
				result["truncation_note"] = "the model reached its token budget before finishing — this draft is incomplete"
			}
			return result, nil

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
			// Who must sign this off. Module 05 resolves the SOP's
			// config.required_by (an agent-definition id) against the org chart
			// at compile time and passes the seat holder through here. With
			// nobody bound to that seat the gate goes to a role — a target
			// Module 09 models — and never to an invented user.
			assigneeType, assigneeID := "role", "department_head"
			var requiredApprovers []map[string]string
			if u := str(node.Parameters["required_approver_user_id"]); u != "" {
				assigneeType, assigneeID = "user", u
				requiredApprovers = []map[string]string{{"user_id": u}}
			} else {
				requiredApprovers = []map[string]string{{"role": "department_head"}}
			}

			task, err := deps.Tasks.Create(&store.HumanTask{
				TenantID:            auth.TenantID,
				PipelineExecutionID: workflowID, // run-scoped correlation
				StepID:              node.ID,
				AssigneeType:        store.HumanTaskAssigneeType(assigneeType),
				AssigneeID:          assigneeID,
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
				"request_id":         task.ID,
				"requester_id":       str(variables["department_id"]),
				"type":               "parallel",
				"title":              bound(title, 100),
				"required_approvers": requiredApprovers,
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

		case store.WorkflowNodeCondition:
			// The SOP's expression is evaluated against the run's variables and
			// the decision is recorded. It is never guessed: an expression the
			// grammar cannot parse, or one referencing data this run does not
			// carry, is reported as undecided with its reason so the timeline
			// shows why the branch was not taken.
			expr, _ := node.Parameters["condition"].(string)
			if expr == "" {
				return map[string]interface{}{
					"node_type": "condition",
					"decided":   false,
					"reason":    "step declares no condition expression",
				}, nil
			}
			res := condition.Evaluate(expr, variables)
			out := map[string]interface{}{
				"node_type": "condition",
				"condition": expr,
				"decided":   res.OK,
			}
			if res.OK {
				out["result"] = res.Value
				if p, ok := node.Parameters[map[bool]string{true: "true_path", false: "false_path"}[res.Value]].(string); ok && p != "" {
					// Recorded for visibility. Routing on it needs the template
					// to name a real step id — today these are prose labels.
					out["declared_path"] = p
				}
			} else {
				out["reason"] = res.Reason
			}
			return out, nil

		default: // action and anything future
			// A step that names a capability goes through Module 08's governed
			// funnel — binding, schema validation, policy, authority, audit —
			// and its refusals fail the node with the recorded reason, because
			// an unbound or denied action must stop the run, not pass silently.
			// A step that names no capability stays a recorded pass-through:
			// stated, never faked.
			if capID := str(node.Parameters["capability"]); capID != "" && deps.Capabilities != nil {
				return runCapability(ctx, deps, node, capID, workflowID, variables, auth)
			}
			return map[string]interface{}{
				"note":   "no executor bound for node type " + string(node.Type) + " — recorded as pass-through",
				"action": node.Action,
			}, nil
		}
	}
}

// runCapability dispatches one action node through the capability funnel.
func runCapability(ctx context.Context, deps NodeHandlerDeps, node store.WorkflowNode, capID, workflowID string, variables map[string]interface{}, auth RunAuth) (map[string]interface{}, error) {
	input := substituteInputs(node.Parameters["inputs"], variables)
	req := capability.InvokeRequest{
		CapabilityID: capID,
		Input:        input,
		Actor: capability.Actor{
			Type:         "agent",
			ID:           str(node.Parameters["actor_agent_id"]),
			PositionID:   str(node.Parameters["actor_position_id"]),
			AutonomyTier: str(node.Parameters["actor_autonomy_tier"]),
		},
		Correlation: capability.Correlation{
			RequestID:    str(variables["request_id"]),
			WorkflowID:   workflowID,
			NodeID:       node.ID,
			DepartmentID: str(variables["department_id"]),
		},
	}
	inv, err := deps.Capabilities.Invoke(ctx, auth.Authorization, auth.TenantID, req)
	if err != nil {
		return nil, fmt.Errorf("capability node %s (%s): %w", node.ID, capID, err)
	}
	if inv.Status != "completed" {
		// The funnel already recorded the refusal; the node carries it to the
		// run so the request fails with the stage and reason on the record.
		return nil, fmt.Errorf("capability node %s: %s refused — %s: %s", node.ID, capID, inv.Status, inv.Error)
	}

	result := map[string]interface{}{
		"capability":      inv.CapabilityID,
		"execution_id":    inv.ID,
		"status":          inv.Status,
		"simulated":       inv.Simulated,
		"provider_kind":   inv.ProviderKind,
		"policy_decision": inv.PolicyDecision,
		"action":          node.Action,
		// summary, not "text": text chains into last_agent_output and would
		// overwrite the agent draft a later gate needs to show its approver.
		"summary": capabilitySummary(inv),
	}
	if inv.ExternalRef != nil {
		result["external_ref"] = map[string]interface{}{
			"system": inv.ExternalRef.System, "kind": inv.ExternalRef.Kind,
			"id": inv.ExternalRef.ID, "url": inv.ExternalRef.URL,
		}
	}
	if inv.Output != nil {
		result["output"] = inv.Output
	}
	return result, nil
}

// substituteInputs resolves {{variable}} references in the step's declared
// inputs against the run's variables. Values are bounded so a long agent
// draft cannot balloon a capability payload.
func substituteInputs(raw interface{}, variables map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return out
	}
	for k, v := range m {
		sv, isStr := v.(string)
		if !isStr {
			out[k] = v
			continue
		}
		resolved := sv
		for name, val := range variables {
			needle := "{{" + name + "}}"
			if strings.Contains(resolved, needle) {
				resolved = strings.ReplaceAll(resolved, needle, str(val))
			}
		}
		out[k] = bound(resolved, 2000)
	}
	return out
}

func capabilitySummary(inv *capability.Invocation) string {
	label := inv.CapabilityID
	if inv.Simulated {
		label += " (SIMULATED)"
	}
	if inv.ExternalRef != nil {
		return fmt.Sprintf("%s → %s/%s %s", label, inv.ExternalRef.System, inv.ExternalRef.Kind, inv.ExternalRef.ID)
	}
	return label
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
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
