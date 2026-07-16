package store

import (
	"errors"
	"time"
)

// ErrNoRows is returned when a query expected a row but found none.
var ErrNoRows = errors.New("no rows in result set")

// ModelProvider represents a registered LLM backend.
type ModelProvider struct {
	ID                string            `db:"id" json:"id"`
	TenantID          string            `db:"tenant_id" json:"tenant_id"`
	Name              string            `db:"name" json:"name"`
	Description       string            `db:"description" json:"description,omitempty"`
	Type              string            `db:"type" json:"type"`
	BaseURL           string            `db:"base_url" json:"base_url"`
	APIKeySecretName  string            `db:"api_key_secret_name" json:"api_key_secret_name,omitempty"`
	IsActive          bool              `db:"is_active" json:"is_active"`
	Priority          int               `db:"priority" json:"priority"`
	MaxRetries        int               `db:"max_retries" json:"max_retries"`
	TimeoutMs         int               `db:"timeout_ms" json:"timeout_ms"`
	Metadata          map[string]any    `db:"metadata" json:"metadata"`
	CreatedAt         time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time         `db:"updated_at" json:"updated_at"`
}

// ModelRegistry represents a model → provider mapping.
type ModelRegistry struct {
	ID                string            `db:"id" json:"id"`
	TenantID          string            `db:"tenant_id" json:"tenant_id"`
	ModelName         string            `db:"model_name" json:"model_name"`
	ProviderID        string            `db:"provider_id" json:"provider_id"`
	ProviderModelName string            `db:"provider_model_name" json:"provider_model_name"`
	SupportsChat      bool              `db:"supports_chat" json:"supports_chat"`
	SupportsEmbed     bool              `db:"supports_embed" json:"supports_embed"`
	MaxTokens         int               `db:"max_tokens" json:"max_tokens"`
	CostPerToken      map[string]any    `db:"cost_per_token" json:"cost_per_token"`
	IsDefault         bool              `db:"is_default" json:"is_default"`
	IsActive          bool              `db:"is_active" json:"is_active"`
	CreatedAt         time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time         `db:"updated_at" json:"updated_at"`
}

// ModelCall records an LLM inference invocation.
type ModelCall struct {
	ID              string    `db:"id" json:"id"`
	TenantID        string    `db:"tenant_id" json:"tenant_id"`
	AgentID         string    `db:"agent_id" json:"agent_id,omitempty"`
	WorkflowID      string    `db:"workflow_id" json:"workflow_id,omitempty"`
	ModelName       string    `db:"model_name" json:"model_name"`
	ProviderID      string    `db:"provider_id" json:"provider_id"`
	PromptTokens    int       `db:"prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int      `db:"completion_tokens" json:"completion_tokens"`
	TotalTokens     int       `db:"total_tokens" json:"total_tokens"`
	CostUSD         float64   `db:"cost_usd" json:"cost_usd"`
	Status          string    `db:"status" json:"status"`
	ErrorMessage    string    `db:"error_message" json:"error_message,omitempty"`
	LatencyMs       int       `db:"latency_ms" json:"latency_ms"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}