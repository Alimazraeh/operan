// Package store provides in-memory, tenant-isolated storage for Module 09
// (Human Supervision): approvals, escalations, interventions, and HITL answers.
package store

import (
	"errors"
	"time"
)

// Sentinel errors shared by all stores.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrTenantMismatch = errors.New("tenant_id is required or does not match")
	ErrValidation     = errors.New("validation failed")
	ErrConflict       = errors.New("resource state prevents this operation")
)

// timeNow returns the current UTC time (indirection eases testing).
var timeNow = func() time.Time { return time.Now().UTC() }

// ─── Enums ───────────────────────────────────────────────────────────────────

func oneOf(s string, allowed ...string) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}

// ValidApprovalType per CreateApprovalRequest.type.
func ValidApprovalType(s string) bool {
	return oneOf(s, "sequential", "parallel", "conditional", "threshold")
}

// ValidEscalationSeverity per Escalation.severity.
func ValidEscalationSeverity(s string) bool {
	return oneOf(s, "low", "medium", "high", "critical", "p0")
}

// ValidEscalationCategory per Escalation.category.
func ValidEscalationCategory(s string) bool {
	return oneOf(s, "hallucination", "security", "financial", "operational", "compliance", "system")
}

// ValidEscalationStatus per Escalation.status.
func ValidEscalationStatus(s string) bool {
	return oneOf(s, "open", "acknowledged", "in_progress", "resolved", "closed")
}

// ValidInterventionAction per Intervention.action.
func ValidInterventionAction(s string) bool {
	return oneOf(s, "pause", "stop", "restrict", "override", "redirect", "suspend")
}

// ValidResolutionType per ResolveEscalationRequest.resolution_type.
func ValidResolutionType(s string) bool {
	return oneOf(s, "manual_resolution", "automated_fix", "escalation_to_human", "no_action_needed", "configuration_change")
}

// ValidHitlConfidence per HitlAnswerRequest.confidence.
func ValidHitlConfidence(s string) bool {
	return oneOf(s, "low", "medium", "high")
}

// ValidHitlAction per HitlAnswerRequest.action_taken.
func ValidHitlAction(s string) bool {
	return oneOf(s, "approve", "reject", "modify", "escalate", "ignore")
}

// ValidScopeType per Intervention scope.type.
func ValidScopeType(s string) bool {
	return oneOf(s, "agent", "workflow", "model", "tenant", "region")
}

// ─── Models (match openapi-09-human-supervision.yaml) ────────────────────────

// ApprovalTarget identifies a required approver (user, role, or group).
type ApprovalTarget struct {
	UserID      string `json:"user_id,omitempty"`
	Role        string `json:"role,omitempty"`
	UserGroupID string `json:"user_group_id,omitempty"`
}

// ApprovalAction records one approve/reject act.
type ApprovalAction struct {
	ActorID    string    `json:"actor_id"`
	Action     string    `json:"action"` // approve | reject
	Comment    string    `json:"comment,omitempty"`
	Conditions []string  `json:"conditions,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ApprovalDelegate records a delegation hand-off.
type ApprovalDelegate struct {
	FromUserID string    `json:"from_user_id"`
	ToUserID   string    `json:"to_user_id"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ThresholdConfig governs threshold-type approvals.
type ThresholdConfig struct {
	MinApprovals  int `json:"min_approvals,omitempty"`
	MaxRejections int `json:"max_rejections,omitempty"`
}

// ConditionalConfig governs conditional-type approvals.
type ConditionalConfig struct {
	Condition        string `json:"condition,omitempty"`
	FallbackApproval bool   `json:"fallback_approval,omitempty"`
}

// Approval matches the OpenAPI Approval schema (tenant_id is internal).
type Approval struct {
	ID                string                 `json:"id"`
	TenantID          string                 `json:"-"`
	RequestID         string                 `json:"request_id"`
	RequesterID       string                 `json:"requester_id"`
	Type              string                 `json:"type"`
	Title             string                 `json:"title,omitempty"`
	Description       string                 `json:"description,omitempty"`
	Status            string                 `json:"status"` // pending | in_progress | approved | rejected | delegated | expired | cancelled
	CurrentStep       int                    `json:"current_step,omitempty"`
	Approvals         []ApprovalAction       `json:"approvals,omitempty"`
	Rejections        []ApprovalAction       `json:"rejections,omitempty"`
	Delegates         []ApprovalDelegate     `json:"delegates,omitempty"`
	RequiredApprovers []ApprovalTarget       `json:"-"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ConditionalConfig *ConditionalConfig     `json:"-"`
	ThresholdConfig   *ThresholdConfig       `json:"-"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	ApprovedAt        *time.Time             `json:"approved_at,omitempty"`
	RejectedAt        *time.Time             `json:"rejected_at,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// Terminal reports whether the approval is in a final state.
func (a *Approval) Terminal() bool {
	return oneOf(a.Status, "approved", "rejected", "expired", "cancelled")
}

// InterventionScope bounds an intervention's blast radius.
type InterventionScope struct {
	Type       string `json:"type"`
	ResourceID string `json:"resource_id,omitempty"`
}

// Intervention matches the OpenAPI Intervention schema.
type Intervention struct {
	ID               string                 `json:"id"`
	TenantID         string                 `json:"-"`
	Action           string                 `json:"action"`
	TargetAgentID    string                 `json:"target_agent_id"`
	TargetWorkflowID string                 `json:"target_workflow_id,omitempty"`
	Reason           string                 `json:"reason,omitempty"`
	Status           string                 `json:"status"` // active | expired | revoked | completed
	Scope            *InterventionScope     `json:"scope,omitempty"`
	DurationMinutes  int                    `json:"duration_minutes,omitempty"`
	ExpiresAt        *time.Time             `json:"expires_at,omitempty"`
	IssuedBy         string                 `json:"issued_by,omitempty"`
	RevokedAt        *time.Time             `json:"revoked_at,omitempty"`
	RevokedBy        string                 `json:"revoked_by,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	IssuedAt         time.Time              `json:"issued_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Escalation matches the OpenAPI Escalation schema.
type Escalation struct {
	ID                string                 `json:"id"`
	TenantID          string                 `json:"-"`
	Severity          string                 `json:"severity"`
	Category          string                 `json:"category"`
	Title             string                 `json:"title"`
	Description       string                 `json:"description,omitempty"`
	Status            string                 `json:"status"`
	RelatedApprovalID string                 `json:"related_approval_id,omitempty"`
	SourceAgentID     string                 `json:"source_agent_id,omitempty"`
	ImpactScope       string                 `json:"impact_scope,omitempty"`
	RequestedAction   string                 `json:"requested_action,omitempty"`
	AssignedTo        string                 `json:"assigned_to,omitempty"`
	ResolvedAt        *time.Time             `json:"resolved_at,omitempty"`
	ResolverID        string                 `json:"resolver_id,omitempty"`
	ResolutionNotes   string                 `json:"resolution_notes,omitempty"`
	ResolutionType    string                 `json:"resolution_type,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// HitlAnswer matches the OpenAPI HitlAnswer schema.
type HitlAnswer struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"-"`
	RequestID   string                 `json:"request_id"`
	Answer      string                 `json:"answer"`
	Confidence  string                 `json:"confidence,omitempty"`
	ActionTaken string                 `json:"action_taken,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// QueueItem matches the OpenAPI QueueItem schema.
type QueueItem struct {
	ItemID     string     `json:"item_id"`
	ItemType   string     `json:"item_type"` // approval | escalation | intervention
	Title      string     `json:"title,omitempty"`
	Priority   string     `json:"priority,omitempty"` // low | medium | high | critical
	Status     string     `json:"status,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	DueAt      *time.Time `json:"due_at,omitempty"`
	AssignedTo string     `json:"assigned_to,omitempty"`
}
