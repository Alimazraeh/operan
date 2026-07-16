package store

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a query expected a row but found none.
var ErrNotFound = errors.New("record not found")

// CostBudget represents a per-tenant or per-agent cost budget.
type CostBudget struct {
	ID            string         `db:"id" json:"id"`
	TenantID      string         `db:"tenant_id" json:"tenant_id"`
	AgentID       *string        `db:"agent_id" json:"agent_id,omitempty"`
	Description   *string        `db:"description" json:"description,omitempty"`
	BudgetAmount  float64        `db:"budget_amount" json:"budget_amount"`
	Currency      string         `db:"currency" json:"currency"`
	Period        string         `db:"period" json:"period"`
	SoftLimitPct  int            `db:"soft_limit_pct" json:"soft_limit_pct"`
	HardLimitPct  int            `db:"hard_limit_pct" json:"hard_limit_pct"`
	IsActive      bool           `db:"is_active" json:"is_active"`
	CreatedAt     time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at" json:"updated_at"`
	StartedAt     time.Time      `db:"started_at" json:"started_at"`
	EndedAt       *time.Time     `db:"ended_at" json:"ended_at,omitempty"`
	SpentThisPeriod *float64     `db:"-" json:"spent_this_period,omitempty"`
	PercentageUsed  *float64     `db:"-" json:"percentage_used,omitempty"`
}

// CostEvent represents a recorded cost event from M12, M08, or manual entry.
type CostEvent struct {
	ID              string    `db:"id" json:"id"`
	TenantID        string    `db:"tenant_id" json:"tenant_id"`
	AgentID         *string   `db:"agent_id" json:"agent_id,omitempty"`
	SourceModule    string    `db:"source_module" json:"source_module"`
	SourceID        *string   `db:"source_id" json:"source_id,omitempty"`
	ModelName       *string   `db:"model_name" json:"model_name,omitempty"`
	CostUSD         float64   `db:"cost_usd" json:"cost_usd"`
	PromptTokens    int       `db:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int      `db:"completion_tokens" json:"completion_tokens"`
	EventType       string    `db:"event_type" json:"event_type"`
	BillingTag      *string   `db:"billing_tag" json:"billing_tag,omitempty"`
	EventTimestamp  time.Time `db:"event_timestamp" json:"event_timestamp"`
	RecordedAt      time.Time `db:"recorded_at" json:"recorded_at"`
}

// CostAlert represents an alert generated when a budget threshold is crossed.
type CostAlert struct {
	ID            string    `db:"id" json:"id"`
	TenantID      string    `db:"tenant_id" json:"tenant_id"`
	BudgetID      *string   `db:"budget_id" json:"budget_id,omitempty"`
	AgentID       *string   `db:"agent_id" json:"agent_id,omitempty"`
	AlertType     string    `db:"alert_type" json:"alert_type"`
	CurrentSpend  float64   `db:"current_spend" json:"current_spend"`
	BudgetAmount  float64   `db:"budget_amount" json:"budget_amount"`
	PercentageUsed float64  `db:"percentage_used" json:"percentage_used"`
	Severity      string    `db:"severity" json:"severity"`
	IsResolved    bool      `db:"is_resolved" json:"is_resolved"`
	ResolvedAt    *time.Time `db:"resolved_at" json:"resolved_at,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// ThrottleState tracks the manual throttle override.
type ThrottleState struct {
	State     string    `db:"state" json:"state"`    // "none", "soft", "hard"
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}