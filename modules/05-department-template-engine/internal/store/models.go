// Package store provides in-memory storage for Module 05 with tenant isolation.
// All stores use a map keyed by ID with per-tenant filtering for List operations.
package store

import (
	"encoding/json"
	"fmt"
	"time"
)

var timeNow = time.Now

// ─── Models ──────────────────────────────────────────────────────────────────

// Template represents a department template.
type Template struct {
	ID                  string                 `json:"id"`
	TenantID            string                 `json:"tenant_id"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description,omitempty"`
	Category            string                 `json:"category"`
	Version             string                 `json:"version"`
	Agents              []AgentDefinition      `json:"agents,omitempty"`
	Workflows           []WorkflowDefinition   `json:"workflows,omitempty"`
	MemoryTopology      *MemoryTopology          `json:"memory_topology,omitempty"`
	GovernanceRules     []GovernanceRule       `json:"governance_rules,omitempty"`
	KPIS                []KPIDefinition        `json:"kpis,omitempty"`
	Integrations        []IntegrationDefinition `json:"integrations,omitempty"`
	OperationalPolicies []OperationalPolicy    `json:"operational_policies,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Status              string                 `json:"status"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	CreatedBy           string                 `json:"created_by"`
}

// AgentDefinition represents an agent within a template.
type AgentDefinition struct {
	ID               string                 `json:"id"`
	Role             string                 `json:"role"`
	Name             string                 `json:"name,omitempty"`
	Capabilities     []string               `json:"capabilities"`
	Model            string                 `json:"model,omitempty"`
	SystemPrompt     string                 `json:"system_prompt,omitempty"`
	MemoryProfile    string                 `json:"memory_profile,omitempty"`
	ToolRequirements []string               `json:"tool_requirements,omitempty"`
	Constraints      map[string]interface{} `json:"constraints,omitempty"`
	AccessControl    map[string]interface{} `json:"access_control,omitempty"`
	CreatedAt        *time.Time             `json:"created_at,omitempty"`
	UpdatedAt        *time.Time             `json:"updated_at,omitempty"`
}

// WorkflowDefinition represents a workflow within a template.
type WorkflowDefinition struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Steps         []WorkflowStep    `json:"steps"`
	Triggers      []string          `json:"triggers,omitempty"`
	ErrorHandling map[string]interface{} `json:"error_handling,omitempty"`
	CreatedAt     *time.Time        `json:"created_at,omitempty"`
	UpdatedAt     *time.Time        `json:"updated_at,omitempty"`
}

// WorkflowStep represents a step in a workflow.
type WorkflowStep struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"` // agent_call, api_call, data_fetch, transformation, approval, notification, tool_call, conditional
	Name           string                 `json:"name,omitempty"`
	Config         map[string]interface{} `json:"config,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	RetryPolicy    map[string]interface{} `json:"retry_policy,omitempty"`
}

// MemoryTopology represents the memory configuration for a template.
type MemoryTopology struct {
	SemanticEnabled    bool                   `json:"semantic_enabled,omitempty"`
	EpisodicEnabled    bool                   `json:"episodic_enabled,omitempty"`
	ProceduralEnabled  bool                   `json:"procedural_enabled,omitempty"`
	GraphEnabled       bool                   `json:"graph_enabled,omitempty"`
	MemoryProfiles     map[string]string      `json:"memory_profiles,omitempty"`
	RetentionPolicy    map[string]interface{} `json:"retention_policy,omitempty"`
	CompressionSettings map[string]interface{} `json:"compression_settings,omitempty"`
}

// GovernanceRule represents a governance rule in a template.
type GovernanceRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"` // access_control, data_usage, rate_limit, audit, compliance, custom
	Description string                 `json:"description,omitempty"`
	Enforcement string                 `json:"enforcement"` // enforce, warn, log
	Conditions  map[string]interface{} `json:"conditions,omitempty"`
	Actions     []string               `json:"actions,omitempty"`
	CreatedAt   *time.Time             `json:"created_at,omitempty"`
	UpdatedAt   *time.Time             `json:"updated_at,omitempty"`
}

// KPIDefinition represents a KPI in a template.
type KPIDefinition struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description,omitempty"`
	MetricType       string                 `json:"metric_type"` // counter, gauge, histogram, timer, boolean
	Unit             string                 `json:"unit,omitempty"`
	Thresholds       map[string]interface{} `json:"thresholds,omitempty"`
	AggregationPeriod string                `json:"aggregation_period,omitempty"`
	DashboardLink    string                 `json:"dashboard_link,omitempty"`
	CreatedAt        *time.Time             `json:"created_at,omitempty"`
	UpdatedAt        *time.Time             `json:"updated_at,omitempty"`
}

// IntegrationDefinition represents an integration in a template.
type IntegrationDefinition struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // erp, crm, email, calendar, document, messaging, custom
	Name       string                 `json:"name,omitempty"`
	Provider   string                 `json:"provider,omitempty"`
	Config     map[string]interface{} `json:"config"`
	AuthMethod string                 `json:"auth_method,omitempty"` // oauth2, api_key, basic, jwt, custom
	Status     string                 `json:"status,omitempty"`      // active, inactive, error, pending
	CreatedAt  *time.Time             `json:"created_at,omitempty"`
	UpdatedAt  *time.Time             `json:"updated_at,omitempty"`
}

// OperationalPolicy represents an operational policy in a template.
type OperationalPolicy struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Scope         string                 `json:"scope"` // agent, workflow, department, system
	Rules         []map[string]interface{} `json:"rules,omitempty"`
	EffectiveFrom *time.Time             `json:"effective_from,omitempty"`
	EffectiveTo   *time.Time             `json:"effective_to,omitempty"`
	Version       string                 `json:"version,omitempty"`
	CreatedAt     *time.Time             `json:"created_at,omitempty"`
	UpdatedAt     *time.Time             `json:"updated_at,omitempty"`
}

// TemplateVersion represents an immutable snapshot of a template at a version.
type TemplateVersion struct {
	ID         string                 `json:"id"`
	TenantID   string                 `json:"tenant_id"`
	TemplateID string                 `json:"template_id"`
	Version    string                 `json:"version"`
	Snapshot   map[string]interface{} `json:"snapshot"`
	CreatedAt  time.Time              `json:"created_at"`
}

// TemplateDeployment represents a deployment of a template.
type TemplateDeployment struct {
	ID                  string                 `json:"id"`
	TenantID            string                 `json:"tenant_id"`
	TemplateID          string                 `json:"template_id"`
	Version             string                 `json:"version"`
	Status              string                 `json:"status"` // select, configure, connect_data, provision_memory, deploy_swarm, operational, failed, rolled_back
	Environment         string                 `json:"environment"` // dev, staging, production
	Configuration       map[string]interface{} `json:"configuration,omitempty"`
	ProvisionedEntities map[string]interface{} `json:"provisioned_entities,omitempty"`
	StartedAt           *time.Time             `json:"started_at,omitempty"`
	CompletedAt         *time.Time             `json:"completed_at,omitempty"`
	ErrorMessage        string                 `json:"error_message,omitempty"`
	DeployedBy          string                 `json:"deployed_by,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// CustomTemplate represents a user-created custom template.
type CustomTemplate struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Content     map[string]interface{} `json:"content"`
	OwnerID     string                 `json:"owner_id"`
	SharedWith  []string               `json:"shared_with,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Status      string                 `json:"status"` // draft, published, archived
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedBy   string                 `json:"created_by"`
}

// ─── Helper: generate JSON bytes for stored arrays ───────────────────────────

func toJSONArray(v interface{}) json.RawMessage {
	if v == nil {
		return json.RawMessage("[]")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("[]")
	}
	return json.RawMessage(b)
}

func toJSON(v interface{}) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}

// ─── Common store errors ─────────────────────────────────────────────────────

var ErrNotFound = fmt.Errorf("not found")
var ErrTenantMismatch = fmt.Errorf("tenant mismatch")

// ─── Request DTOs ────────────────────────────────────────────────────────────

// TemplateCreate is the request body for creating a standard template.
type TemplateCreate struct {
	Name                string                 `json:"name"`
	Description         string                 `json:"description,omitempty"`
	Category            string                 `json:"category"`
	Agents              []AgentDefinition      `json:"agents,omitempty"`
	Workflows           []WorkflowDefinition   `json:"workflows,omitempty"`
	MemoryTopology      *MemoryTopology        `json:"memory_topology,omitempty"`
	GovernanceRules     []GovernanceRule       `json:"governance_rules,omitempty"`
	KPIS                []KPIDefinition        `json:"kpis,omitempty"`
	Integrations        []IntegrationDefinition `json:"integrations,omitempty"`
	OperationalPolicies []OperationalPolicy    `json:"operational_policies,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
}

// CustomTemplateCreate is the request body for creating a custom template.
type CustomTemplateCreate struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Content     map[string]interface{} `json:"content"`
	SharedWith  []string               `json:"shared_with,omitempty"`
}

// DeployRequest is the request body for deploying a template.
type DeployRequest struct {
	Environment    string                 `json:"environment"`
	Version        string                 `json:"version,omitempty"`
	Configuration  map[string]interface{} `json:"configuration,omitempty"`
}

