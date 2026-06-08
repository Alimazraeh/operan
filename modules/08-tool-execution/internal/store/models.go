package store

import (
	"errors"
	"time"
)

// Sentinel errors.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrTenantMismatch = errors.New("tenant_id is required or does not match")
	ErrValidation     = errors.New("validation failed")
)

// timeNow returns the current UTC time (indirection eases testing).
var timeNow = func() time.Time { return time.Now().UTC() }

// Tool is a registered, executable capability available to agents.
type Tool struct {
	ID                   string                 `json:"id"`
	TenantID             string                 `json:"tenant_id"`
	Name                 string                 `json:"name"`
	Version              string                 `json:"version"`
	Description          string                 `json:"description,omitempty"`
	Category             string                 `json:"category,omitempty"`
	InputSchema          map[string]interface{} `json:"input_schema,omitempty"`
	OutputSchema         map[string]interface{} `json:"output_schema,omitempty"`
	AuthRequirements     []string               `json:"auth_requirements,omitempty"`
	RateLimit            *RateLimit             `json:"rate_limit,omitempty"`
	Status               string                 `json:"status"`
	CostPerCall          *CostPerCall           `json:"cost_per_call,omitempty"`
	SecurityRequirements []string               `json:"security_requirements,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	CreatedBy            string                 `json:"created_by,omitempty"`
}

// RateLimit captures per-tool throttling.
type RateLimit struct {
	MaxRequestsPerMinute int `json:"max_requests_per_minute,omitempty"`
	MaxConcurrent        int `json:"max_concurrent,omitempty"`
}

// CostPerCall captures monetary cost.
type CostPerCall struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// ToolVersion records an immutable version of a tool's schema.
type ToolVersion struct {
	ID            string                 `json:"id"`
	ToolID        string                 `json:"tool_id"`
	TenantID      string                 `json:"tenant_id"`
	Version       string                 `json:"version"`
	ChangeSummary string                 `json:"change_summary,omitempty"`
	InputSchema   map[string]interface{} `json:"input_schema,omitempty"`
	OutputSchema  map[string]interface{} `json:"output_schema,omitempty"`
	Status        string                 `json:"status"`
	CreatedBy     string                 `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// ExecutionStatus enumerates tool execution states.
type ExecutionStatus string

const (
	ExecQueued    ExecutionStatus = "queued"
	ExecRunning   ExecutionStatus = "running"
	ExecCompleted ExecutionStatus = "completed"
	ExecFailed    ExecutionStatus = "failed"
	ExecCancelled ExecutionStatus = "cancelled"
)

// ToolExecution records a single tool invocation.
type ToolExecution struct {
	ID              string                 `json:"id"`
	TenantID        string                 `json:"tenant_id"`
	AgentID         string                 `json:"agent_id"`
	Tool            string                 `json:"tool"`
	ToolVersion     string                 `json:"tool_version,omitempty"`
	Status          ExecutionStatus        `json:"status"`
	Input           map[string]interface{} `json:"input,omitempty"`
	Output          map[string]interface{} `json:"output,omitempty"`
	ExecutionTimeMS int                    `json:"execution_time_ms,omitempty"`
	Cost            *CostPerCall           `json:"cost,omitempty"`
	ErrorCode       string                 `json:"error_code,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	RetryCount      int                    `json:"retry_count"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}
