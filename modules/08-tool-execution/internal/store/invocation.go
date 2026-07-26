package store

import (
	"sort"
	"sync"
	"time"
)

// Actor is who performed a capability — the agent or user, and the seat whose
// authority they acted under. Recorded on every invocation because "an AI did
// it, within its authority" is only worth anything if the record says whose
// authority that was.
//
// AutonomyTier is the caller's claim, taken verbatim from the request body —
// it is transport input, not a fact. It is never compared against a
// capability's minimum; Invocation.ResolvedAutonomyTier is. Keeping the claim
// on the record is what makes a caller lying about their seat's authority a
// legible, auditable event instead of a silently corrected one.
type Actor struct {
	Type         string `json:"type"` // agent | user | system
	ID           string `json:"id,omitempty"`
	PositionID   string `json:"position_id,omitempty"`
	AutonomyTier string `json:"autonomy_tier,omitempty"`
}

// Correlation ties an invocation back to the work that caused it.
type Correlation struct {
	RequestID    string `json:"request_id,omitempty"`
	WorkflowID   string `json:"workflow_id,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	DepartmentID string `json:"department_id,omitempty"`
}

// ExternalRef is the canonical pointer to whatever the provider created or
// touched — the only vendor data Operan canonicalizes. Payloads stay opaque;
// references stay tiny and stable.
type ExternalRef struct {
	System string `json:"system"`
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	URL    string `json:"url,omitempty"`
}

// Invocation statuses. Everything that is not completed states exactly which
// funnel stage refused and why.
const (
	InvocationCompleted       = "completed"
	InvocationBlockedNoBind   = "blocked_no_binding"
	InvocationInvalidInput    = "invalid_input"
	InvocationDeniedPolicy    = "denied_policy"
	InvocationDeniedAuthority = "denied_authority"
	InvocationFailed          = "failed"
)

// Invocation is the immutable record of one governed capability execution —
// the audit trail the whole layer exists to produce. It is written for every
// attempt, refusals included: a denied action is as much a fact as a
// completed one.
type Invocation struct {
	ID             string                 `json:"id"`
	TenantID       string                 `json:"tenant_id"`
	CapabilityID   string                 `json:"capability_id"`
	SideEffect     string                 `json:"side_effect,omitempty"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	ProviderKind   string                 `json:"provider_kind,omitempty"`
	ProviderTool   string                 `json:"provider_tool,omitempty"`
	Actor          Actor                  `json:"actor"`
	Correlation    Correlation            `json:"correlation"`
	Input          map[string]interface{} `json:"input,omitempty"`
	Output         map[string]interface{} `json:"output,omitempty"`
	ExternalRef    *ExternalRef           `json:"external_ref,omitempty"`
	Simulated      bool                   `json:"simulated"`
	PolicyDecision string                 `json:"policy_decision,omitempty"`
	// ResolvedAutonomyTier is the acting seat's real tier, resolved
	// server-side from Module 05's org chart at invoke time — never taken
	// from the request. Empty means the seat could not be resolved, which
	// ranks below every real tier (store.AutonomyRank) and is therefore no
	// authority at all. Compare against Actor.AutonomyTier (the claim) to see
	// a caller misrepresenting its own authority.
	ResolvedAutonomyTier string    `json:"resolved_autonomy_tier,omitempty"`
	Status               string    `json:"status"`
	Error                string    `json:"error,omitempty"`
	DurationMS           int64     `json:"duration_ms"`
	CreatedAt            time.Time `json:"created_at"`
}

// InvocationStore is append-only: invocations are never updated or deleted.
type InvocationStore struct {
	mu   sync.RWMutex
	rows []*Invocation
	sink *durabilitySink
}

func NewInvocationStore() *InvocationStore { return &InvocationStore{} }

func (s *InvocationStore) Append(inv *Invocation) {
	s.mu.Lock()
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = timeNow()
	}
	s.rows = append(s.rows, inv)
	s.mu.Unlock()
	s.saveInvocation(inv)
}

// List returns a tenant's invocations, newest first, with optional filters.
func (s *InvocationStore) List(tenantID, capabilityID, status, requestID string, limit int) []*Invocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []*Invocation
	for _, r := range s.rows {
		if r.TenantID != tenantID {
			continue
		}
		if capabilityID != "" && r.CapabilityID != capabilityID {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		if requestID != "" && r.Correlation.RequestID != requestID {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
