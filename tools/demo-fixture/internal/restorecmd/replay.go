package restorecmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/operan/tools/demo-fixture/internal/apiclient"
	"github.com/operan/tools/demo-fixture/internal/fixture"
)

// ReplayOptions tunes the polling loop. Zero values fall back to sane
// defaults in Replay; tests override Sleep with a no-op so the control flow
// (poll N times, then act) runs without waiting on a real clock.
type ReplayOptions struct {
	PollInterval time.Duration
	MaxAttempts  int
	Sleep        func(time.Duration)
}

func (o ReplayOptions) withDefaults() ReplayOptions {
	if o.PollInterval <= 0 {
		o.PollInterval = 3 * time.Second
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 60 // 3s * 60 = 3 minutes, matching draft+gate timings seen live per the handoff notes
	}
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	return o
}

// ReplayResult is what actually happened, for reporting and for tests.
type ReplayResult struct {
	RequestID      string
	FinalStatus    string
	Approved       bool
	ApprovalItemID string
	Attempts       int
}

// terminalRequestStatus mirrors store.TerminalRequestStatus
// (modules/05-department-template-engine/internal/store/requests.go:62-69) —
// duplicated rather than imported because this tool deliberately does not
// depend on any module's internal packages (see the package doc comment).
func terminalRequestStatus(s string) bool {
	switch s {
	case "completed", "rejected", "failed", "cancelled":
		return true
	}
	return false
}

// boundTitle mirrors the rune-safe bound() helper in M03's node_handler.go
// (modules/03-agent-orchestration/internal/execution/node_handler.go, fixed
// under WO-1) — the truncation Module 09's approval Title actually carries,
// per Module 05's workloop.go:87 seeding request_title from the raised
// request's own Title. Replicated here (not imported, same reason as
// above) so the queue-matching heuristic in findApprovalForRequest compares
// against the exact string Module 09 will hold.
func boundTitle(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Replay raises one fresh demonstration request from f.Replay and drives it
// to completion: create → poll → (if a gate appears) find and approve the
// matching Module 09 approval → poll to a terminal state. It requires
// pr (the Result from a prior Provision call, possibly in the same
// process) so it knows the department id and the real id behind
// f.Replay.ApproverRef.
//
// This is, unavoidably, the least verifiable part of this tool without a
// live cluster: it depends on Module 03's human_gate node actually raising
// a Module 09 approval titled from the same request_title Module 05 seeds
// (traced through node_handler.go and workloop.go — see boundTitle's doc
// comment — but never observed running). See this tool's README for the
// full list of what remains unverified.
func Replay(ctx context.Context, cfg Config, f *fixture.Fixture, c *Clients, pr *Result, opts ReplayOptions) (*ReplayResult, error) {
	if f.Replay == nil {
		return nil, fmt.Errorf("replay: fixture has no replay section")
	}
	opts = opts.withDefaults()

	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: POST %s/api/v1/iam/auth/login  body={email:<approver email>, tenant:%q}", cfg.IdentityAccessURL, f.Tenant.Name)
		cfg.logf("[dry-run] PLAN: POST %s/departments/{department-id}/requests  body={service_id:%q, title:%q, priority:%q}",
			cfg.DepartmentsURL, f.Replay.ServiceID, f.Replay.Title, f.Replay.Priority)
		cfg.logf("[dry-run] PLAN: poll GET %s/requests/{request-id} up to %d times (every %s) for a terminal status",
			cfg.DepartmentsURL, opts.MaxAttempts, opts.PollInterval)
		if f.Replay.ApproverRef != "" {
			cfg.logf("[dry-run] PLAN: IF status becomes awaiting_approval: GET %s/queue?type=approval&user_id={approver-id}, match title %q, then POST .../approvals/{id}/approve",
				cfg.HumanSupervisionURL, boundTitle(f.Replay.Title, 100))
		}
		return &ReplayResult{RequestID: dryRunPlaceholder}, nil
	}

	if pr == nil || pr.Department.ID == "" || pr.Department.ID == dryRunPlaceholder {
		return nil, fmt.Errorf("replay: no provisioned department id available — run Provision (non-dry-run) first")
	}

	// Resolve who raises (and, if named, approves) the request. Using the
	// same fixture user for both mirrors the one documented live example
	// (a department head raising a change against their own department and
	// then approving it via their own decision rights) — see
	// fixture.ReplaySpec's doc comment.
	var actorToken, actorUserID string
	if f.Replay.ApproverRef != "" {
		email, ok := emailForRef(f, f.Replay.ApproverRef)
		if !ok {
			return nil, fmt.Errorf("replay: approver_ref %q does not match any fixture user", f.Replay.ApproverRef)
		}
		if cfg.UserPassword == "" {
			return nil, fmt.Errorf("replay: approver_ref %q is set but no --user-password was provided, so this tool has no credential to log in as that user — provide one, or clear approver_ref to raise the request as admin and skip the approval step (which will only succeed if the SOP raises no gate)", f.Replay.ApproverRef)
		}
		login, err := c.IAM.Login(ctx, email, cfg.UserPassword, f.Tenant.Name)
		if err != nil {
			return nil, fmt.Errorf("replay: login as %s: %w", email, err)
		}
		actorToken, actorUserID = login.Token, login.UserID
		cfg.logf("replay: logged in as %s (id=%s)", email, actorUserID)
	} else {
		cfg.logf("replay: no approver_ref set — raising the request as admin; if the SOP raises a gate, this replay will report an incomplete run rather than guess who should approve it")
		actorToken = pr.AdminToken
	}

	req, err := c.Departments.CreateRequest(ctx, actorToken, f.Tenant.Name, pr.Department.ID, apiclient.CreateRequestBody{
		ServiceID: f.Replay.ServiceID, Title: f.Replay.Title, Body: f.Replay.Body, Priority: f.Replay.Priority,
	})
	if err != nil {
		return nil, fmt.Errorf("replay: create request: %w", err)
	}
	cfg.logf("replay: request raised (id=%s, status=%s)", req.ID, req.Status)

	result := &ReplayResult{RequestID: req.ID}
	approvalAttempted := false

	err = pollUntil(ctx, opts.PollInterval, opts.MaxAttempts, opts.Sleep, func() (bool, error) {
		result.Attempts++
		current, err := c.Departments.GetRequest(ctx, actorToken, f.Tenant.Name, req.ID)
		if err != nil {
			return false, fmt.Errorf("poll request %s: %w", req.ID, err)
		}
		result.FinalStatus = current.Status
		cfg.logf("replay: poll %d — request %s status=%s", result.Attempts, req.ID, current.Status)

		if terminalRequestStatus(current.Status) {
			return true, nil
		}

		if current.Status == "awaiting_approval" && !approvalAttempted {
			if actorUserID == "" {
				return false, fmt.Errorf("request %s is awaiting_approval but no named approver was logged in (approver_ref was empty) — cannot proceed", req.ID)
			}
			approvalAttempted = true
			itemID, err := findApprovalForRequest(ctx, c, actorToken, f.Tenant.Name, actorUserID, current.Title)
			if err != nil {
				return false, err
			}
			if _, err := c.Supervision.Approve(ctx, actorToken, f.Tenant.Name, itemID, "approved by demo-fixture replay"); err != nil {
				return false, fmt.Errorf("approve %s: %w", itemID, err)
			}
			result.Approved = true
			result.ApprovalItemID = itemID
			cfg.logf("replay: approved queue item %s", itemID)
		}
		return false, nil
	})
	if err != nil {
		return result, fmt.Errorf("replay: %w", err)
	}

	if result.FinalStatus != "completed" {
		return result, fmt.Errorf("replay: request %s reached terminal status %q, not completed", req.ID, result.FinalStatus)
	}
	return result, nil
}

// findApprovalForRequest locates the single Module 09 queue item whose
// title matches the request's own title (after the same truncation Module
// 03 applies — see boundTitle). Module 09 has no endpoint that returns an
// approval's originating Module 05 request id directly (its RequestID
// field is Module 03's internal human-task id, not this one — traced in
// node_handler.go); title matching against the caller's own queue is the
// best correlation available over the public API today. An ambiguous or
// absent match is a hard error: guessing which gate to approve in a shared
// tenant is worse than failing loudly.
func findApprovalForRequest(ctx context.Context, c *Clients, token, tenantID, userID, requestTitle string) (string, error) {
	items, err := c.Supervision.GetQueue(ctx, token, tenantID, userID)
	if err != nil {
		return "", fmt.Errorf("list approval queue: %w", err)
	}
	want := boundTitle(requestTitle, 100)
	var matches []apiclient.QueueItem
	for _, it := range items {
		if it.ItemType != "approval" {
			continue
		}
		if it.Title == want || strings.HasPrefix(want, it.Title) || strings.HasPrefix(it.Title, want) {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no pending approval in %s's queue matches title %q (queue had %d approval item(s))", userID, want, countApprovals(items))
	case 1:
		return matches[0].ItemID, nil
	default:
		return "", fmt.Errorf("%d pending approvals in %s's queue match title %q — cannot disambiguate", len(matches), userID, want)
	}
}

func countApprovals(items []apiclient.QueueItem) int {
	n := 0
	for _, it := range items {
		if it.ItemType == "approval" {
			n++
		}
	}
	return n
}

func emailForRef(f *fixture.Fixture, ref string) (string, bool) {
	for _, u := range f.Users {
		if u.Ref == ref {
			return u.Email, true
		}
	}
	return "", false
}
