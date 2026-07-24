// Package workloop is the department work loop: it dispatches a service
// request into a per-request M03 workflow run (compiled from the service's
// SOP) and polls run state back onto the request — timeline, status flips,
// SLA stamps, final output. This is the runtime that turns a deployed
// operating model into an operating department.
package workloop

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/deploy"
	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// Loop dispatches and tracks request runs.
type Loop struct {
	Requests    *store.RequestStore
	Departments *store.DepartmentStore
	Templates   *store.TemplateStore
	Orch        *clients.OrchestrationClient
	Publisher   *events.Publisher

	PollEvery time.Duration // default 15s
	MissLimit int           // consecutive state-read failures before failing the request (default 8)

	mu     sync.Mutex
	misses map[string]int    // request id → consecutive misses
	auths  map[string]string // request id → caller Authorization (memory only)
}

func New(requests *store.RequestStore, departments *store.DepartmentStore, templates *store.TemplateStore, orch *clients.OrchestrationClient, pub *events.Publisher) *Loop {
	return &Loop{
		Requests: requests, Departments: departments, Templates: templates,
		Orch: orch, Publisher: pub,
		PollEvery: 15 * time.Second, MissLimit: 8,
		misses: map[string]int{}, auths: map[string]string{},
	}
}

// Dispatch compiles the service's SOP into a per-request M03 workflow and
// starts it. Any failure is recorded honestly on the request.
func (l *Loop) Dispatch(auth, tenantID string, req *store.ServiceRequest, dept *store.Department) {
	failReq := func(detail string) {
		l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) {
			sr.Status = "failed"
			sr.Timeline = append(sr.Timeline, store.RequestEvent{
				At: time.Now().UTC(), Kind: "failed", Detail: detail})
		})
		l.Publisher.PublishRequestFailed(events.RequestLifecyclePayload{
			RequestID: req.ID, TenantID: tenantID, DepartmentID: dept.ID,
			Status: "failed", Detail: detail, Timestamp: time.Now(),
		})
	}

	wfDef := l.resolveWorkflowDef(tenantID, req, dept)
	if wfDef == nil {
		// Intake never dead-ends: unresolvable SOP → single-gate manual run.
		wfDef = &store.WorkflowDefinition{
			ID:          "manual-" + req.ID,
			Name:        "Manual handling — " + req.ServiceName,
			Description: "No SOP resolved for this service; routed to human handling.",
			Steps: []store.WorkflowStep{{
				ID: "gate-manual", Type: "approval", Name: "Handle and sign off: " + req.Title,
			}},
		}
	}

	agentByDef := map[string]string{}
	for _, p := range dept.OrgChart {
		if p.AgentDefID != "" && p.AgentID != "" {
			agentByDef[p.AgentDefID] = p.AgentID
		}
	}

	wcr := deploy.CompileWorkflow(wfDef, dept.ID, agentByDef)
	wcr.Name = fmt.Sprintf("run: %s [%s]", strings.TrimSpace(req.Title), req.ID[:8])
	wcr.Description = "Service request run — " + req.ServiceName
	wcr.Variables = map[string]interface{}{
		"request_id":    req.ID,
		"request_title": req.Title,
		"request_body":  req.Body,
		"department_id": dept.ID,
		"service_id":    req.ServiceID,
		"priority":      req.Priority,
	}
	// Rejection at a gate must stop the run immediately.
	wcr.Graph["error_strategy"] = "abort"

	caller := clients.Caller{Authorization: auth, TenantID: tenantID}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	created, err := l.Orch.CreateWorkflow(ctx, caller, wcr)
	if err != nil {
		failReq("dispatch: workflow create failed: " + boundStr(err.Error(), 200))
		return
	}
	if err := l.Orch.ExecuteWorkflow(ctx, caller, created.ID); err != nil {
		failReq("dispatch: workflow execute failed: " + boundStr(err.Error(), 200))
		return
	}

	l.mu.Lock()
	l.auths[req.ID] = auth
	l.mu.Unlock()

	var gateNodes []string
	for _, st := range wfDef.Steps {
		if st.Type == "approval" || st.Type == "human_gate" {
			gateNodes = append(gateNodes, st.ID)
		}
	}
	l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) {
		sr.Status = "in_progress"
		sr.WorkflowRunRef = created.ID
		sr.GateNodeIDs = gateNodes
		sr.Timeline = append(sr.Timeline, store.RequestEvent{
			At: time.Now().UTC(), Kind: "dispatched",
			Detail: "SOP run started (" + wfDef.Name + ")"})
	})
	l.Publisher.PublishRequestDispatched(events.RequestLifecyclePayload{
		RequestID: req.ID, TenantID: tenantID, DepartmentID: dept.ID,
		ServiceID: req.ServiceID, Title: req.Title, Status: "in_progress",
		Timestamp: time.Now(),
	})
}

// resolveWorkflowDef finds the SOP behind the request's service on the
// department's source template.
func (l *Loop) resolveWorkflowDef(tenantID string, req *store.ServiceRequest, dept *store.Department) *store.WorkflowDefinition {
	var wfID string
	for i := range dept.Services {
		if dept.Services[i].ID == req.ServiceID {
			wfID = dept.Services[i].DeliveryWorkflowID
			break
		}
	}
	if wfID == "" || dept.TemplateID == "" {
		return nil
	}
	tmpl, err := l.Templates.GetByIDAndTenant(dept.TemplateID, tenantID)
	if err != nil {
		return nil
	}
	for i := range tmpl.Workflows {
		if tmpl.Workflows[i].ID == wfID {
			cp := tmpl.Workflows[i]
			return &cp
		}
	}
	return nil
}

// Run is the request poller: mirrors M03 run state onto requests until each
// reaches a terminal status. Survives restarts (requests are persisted; the
// poller resumes from store state — auth tokens do not survive restarts, so
// orphaned runs are failed honestly after MissLimit reads without auth).
func (l *Loop) Run(ctx context.Context) {
	t := time.NewTicker(l.PollEvery)
	defer t.Stop()
	log.Printf("[WORKLOOP] poller started (every %s)", l.PollEvery)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, req := range l.Requests.ListNonTerminal() {
				l.pollOne(ctx, req)
			}
		}
	}
}

func (l *Loop) pollOne(ctx context.Context, req store.ServiceRequest) {
	if req.WorkflowRunRef == "" {
		return // never dispatched (dispatcher off) — nothing to poll
	}
	l.mu.Lock()
	auth := l.auths[req.ID]
	l.mu.Unlock()

	caller := clients.Caller{Authorization: auth, TenantID: req.TenantID}
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	st, err := l.Orch.GetWorkflowState(rctx, caller, req.WorkflowRunRef)
	if err != nil {
		l.mu.Lock()
		l.misses[req.ID]++
		miss := l.misses[req.ID]
		l.mu.Unlock()
		if miss >= l.MissLimit {
			l.finish(req, "failed", "run lost (orchestrator unreachable or restarted): "+boundStr(err.Error(), 140), "")
		}
		return
	}
	l.mu.Lock()
	l.misses[req.ID] = 0
	l.mu.Unlock()

	// Mirror node completions onto the timeline (dedupe against it).
	seen := map[string]bool{}
	for _, ev := range req.Timeline {
		if ev.Node != "" {
			seen[ev.Node+":"+ev.Kind] = true
		}
	}
	var lastOutput string
	tokens := 0
	gateActive := false
	for _, n := range st.Nodes {
		out := n.Output
		text, _ := out["output"].(string)
		if text == "" {
			text, _ = out["text"].(string)
		}
		nodeType, _ := out["node_type"].(string)
		if n.Status == "completed" && !seen[n.NodeID+":agent_output"] && text != "" {
			l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) {
				now := time.Now().UTC()
				if sr.FirstResponseAt == nil {
					sr.FirstResponseAt = &now
				}
				sr.Timeline = append(sr.Timeline, store.RequestEvent{
					At: now, Kind: "agent_output", Node: n.NodeID,
					Detail: boundStr(text, 4000)})
			})
		}
		if text != "" {
			lastOutput = text
		}
		if tk, ok := out["tokens"].(float64); ok {
			tokens += int(tk)
		}
		if decision, ok := out["decision"].(string); ok && decision != "" && !seen[n.NodeID+":gate_responded"] {
			l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) {
				sr.Timeline = append(sr.Timeline, store.RequestEvent{
					At: time.Now().UTC(), Kind: "gate_responded", Node: n.NodeID,
					Detail: "sign-off: " + decision})
			})
		}
		isGate := nodeType == "human_gate" || strings.HasPrefix(n.NodeID, "gate")
		for _, g := range req.GateNodeIDs {
			if n.NodeID == g {
				isGate = true
			}
		}
		if n.Status == "running" && isGate {
			gateActive = true
		}
	}
	if tokens > 0 {
		l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) { sr.TokensUsed = tokens })
	}

	switch st.Status {
	case "completed":
		l.finish(req, "completed", "SOP run completed", lastOutput)
	case "failed":
		// A gate rejection is a business decision, not a system failure.
		if runErrorMentionsRejection(st) {
			l.finish(req, "rejected", "rejected at sign-off", lastOutput)
		} else {
			l.finish(req, "failed", "SOP run failed", lastOutput)
		}
	case "cancelled":
		l.finish(req, "failed", "SOP run cancelled", lastOutput)
	default: // pending/running
		if gateActive && req.Status != "awaiting_approval" {
			l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) {
				sr.Status = "awaiting_approval"
				sr.Timeline = append(sr.Timeline, store.RequestEvent{
					At: time.Now().UTC(), Kind: "gate_raised",
					Detail: "awaiting human sign-off in Supervision"})
			})
			l.Publisher.PublishRequestAwaitingApproval(events.RequestLifecyclePayload{
				RequestID: req.ID, TenantID: req.TenantID, DepartmentID: req.DepartmentID,
				Title: req.Title, Status: "awaiting_approval", Timestamp: time.Now(),
			})
		}
	}

	// SLA breach detection (resolution clock) — once.
	if req.SLAResolutionDue != nil && time.Now().After(*req.SLAResolutionDue) && !hasEvent(req.Timeline, "sla_breached") && !store.TerminalRequestStatus(req.Status) {
		l.Requests.AppendEvent(req.ID, store.RequestEvent{
			Kind: "sla_breached", Detail: "resolution SLA exceeded"})
		l.Publisher.PublishRequestSLABreached(events.RequestLifecyclePayload{
			RequestID: req.ID, TenantID: req.TenantID, DepartmentID: req.DepartmentID,
			Title: req.Title, Status: req.Status, Timestamp: time.Now(),
		})
	}
}

func (l *Loop) finish(req store.ServiceRequest, status, detail, output string) {
	now := time.Now().UTC()
	l.Requests.Mutate(req.ID, func(sr *store.ServiceRequest) {
		if store.TerminalRequestStatus(sr.Status) {
			return
		}
		sr.Status = status
		sr.CompletedAt = &now
		if output != "" {
			sr.Output = output
		}
		sr.Timeline = append(sr.Timeline, store.RequestEvent{
			At: now, Kind: status, Detail: detail})
	})
	l.mu.Lock()
	delete(l.auths, req.ID)
	delete(l.misses, req.ID)
	l.mu.Unlock()

	payload := events.RequestLifecyclePayload{
		RequestID: req.ID, TenantID: req.TenantID, DepartmentID: req.DepartmentID,
		ServiceID: req.ServiceID, Title: req.Title, Status: status,
		Detail: detail, TokensUsed: req.TokensUsed, Timestamp: now,
	}
	switch status {
	case "completed":
		l.Publisher.PublishRequestCompleted(payload)
	case "rejected":
		l.Publisher.PublishRequestRejected(payload)
	default:
		l.Publisher.PublishRequestFailed(payload)
	}
}

func runErrorMentionsRejection(st *clients.WorkflowState) bool {
	for _, n := range st.Nodes {
		if n.Status == "failed" {
			if msg, ok := n.Output["error"].(string); ok && strings.Contains(msg, "rejected") {
				return true
			}
		}
	}
	// Node errors aren't always surfaced in state output; treat a failed run
	// whose last active node was a gate as a rejection.
	for _, n := range st.Nodes {
		if n.Status == "failed" && strings.HasPrefix(n.NodeID, "gate") {
			return true
		}
	}
	return false
}

func hasEvent(timeline []store.RequestEvent, kind string) bool {
	for _, ev := range timeline {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func boundStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Truncate on a rune boundary — byte slicing can split UTF-8 mid-rune.
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r) + "…"
}
