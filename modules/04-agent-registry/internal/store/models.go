// Package store provides in-memory data stores for the Agent Registry module.
// All stores enforce tenant isolation via tenant-scoped indexes.
package store

import (
	"time"
)

// ─── Time control for tests ─────────────────────────────────────────────────

var timeNow = func() time.Time {
	return time.Now().UTC()
}

// ─── Enum constants ─────────────────────────────────────────────────────────

type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusInactive  AgentStatus = "inactive"
	AgentStatusDeprecated AgentStatus = "deprecated"
	AgentStatusArchived  AgentStatus = "archived"
)

type VersionStatus string

const (
	VersionStatusActive    VersionStatus = "active"
	VersionStatusBeta      VersionStatus = "beta"
	VersionStatusDeprecated VersionStatus = "deprecated"
	VersionStatusArchived  VersionStatus = "archived"
)

type DependencyType string

const (
	DependencyTypeHard    DependencyType = "hard"
	DependencyTypeSoft    DependencyType = "soft"
	DependencyTypeOptional DependencyType = "optional"
)

type Environment string

const (
	EnvironmentDev     Environment = "dev"
	EnvironmentStaging Environment = "staging"
	EnvironmentProduction Environment = "production"
)

type CapabilityTier string

const (
	CapabilityTierRecommend  CapabilityTier = "recommend"
	CapabilityTierAnalyze    CapabilityTier = "analyze"
	CapabilityTierCoordinate CapabilityTier = "coordinate"
	CapabilityTierDraft      CapabilityTier = "draft"
	CapabilityTierExecute    CapabilityTier = "execute"
)

// ─── Domain types ────────────────────────────────────────────────────────────

// Objective represents an agent objective with a metric and weight.
type Objective struct {
	Description string  `json:"description"`
	Metric      string  `json:"metric"`
	Weight      float64 `json:"weight"`
	Tier        string  `json:"tier"`
}

// MemoryAccess represents agent memory configuration.
type MemoryAccess struct {
	Scope          string   `json:"scope"`
	IsolatedStores []string `json:"isolated_stores"`
	AllowedTypes   []string `json:"allowed_types"`
	IsolationLevel string   `json:"isolation_level"`
}

// RuntimeConstraints represents agent runtime limits.
type RuntimeConstraints struct {
	MaxConcurrent     int            `json:"max_concurrent"`
	MaxDurationSeconds int           `json:"max_duration_seconds"`
	RateLimitPerMinute int           `json:"rate_limit_per_minute"`
	ResourceQuota      *ResourceQuota `json:"resource_quota,omitempty"`
}

// ResourceQuota defines resource limits for an agent.
type ResourceQuota struct {
	CPUMillicores int `json:"cpu_millicores"`
	MemoryMB      int `json:"memory_mb"`
	GPU           int `json:"gpu"`
}

// CostProfile defines cost parameters for an agent.
type CostProfile struct {
	CostPerExecution      float64 `json:"cost_per_execution"`
	CostPerToken          float64 `json:"cost_per_token"`
	EstimatedMonthlyCost  float64 `json:"estimated_monthly_cost"`
	BudgetLimit           int     `json:"budget_limit"`
	ThrottleThreshold     float64 `json:"throttle_threshold"`
	BillingTag            string  `json:"billing_tag"`
}

// ExecutionBudget defines execution budget constraints for an agent.
type ExecutionBudget struct {
	DailyTokenLimit     int     `json:"daily_token_limit"`
	MaxRunSeconds       int     `json:"max_run_seconds"`
	MonthlyExecutionCap int     `json:"monthly_execution_cap"`
	MonthlyBudgetUSD    float64 `json:"monthly_budget_usd"`
}

// AccessControl defines per-agent access control configuration.
type AccessControl struct {
	Scope        string   `json:"scope"`        // "tenant" | "department" | "global"
	AllowedRoles []string `json:"allowed_roles"`
	RestrictedTo []string `json:"restricted_to"`
}

// Agent represents a registered agent.
type Agent struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Role               string              `json:"role"`
	Description        string              `json:"description"`
	TenantID           string              `json:"tenant_id"`
	DepartmentID       *string             `json:"department_id,omitempty"`
	Status             AgentStatus         `json:"status"`
	Objectives         []Objective         `json:"objectives,omitempty"`
	Capabilities       []string            `json:"capabilities"`
	Tools              []string            `json:"tools"`
	MemoryAccess       *MemoryAccess       `json:"memory_access,omitempty"`
	EscalationRules    []string            `json:"escalation_rules"`
	GovernancePolicies []string            `json:"governance_policies"`
	SupportedLanguages []string            `json:"supported_languages,omitempty"`
	RuntimeConstraints *RuntimeConstraints `json:"runtime_constraints,omitempty"`
	CostProfile        *CostProfile        `json:"cost_profile,omitempty"`
	ExecutionBudget    *ExecutionBudget    `json:"execution_budget,omitempty"`
	AccessControl      *AccessControl      `json:"access_control,omitempty"`
	CurrentVersionID   *string             `json:"current_version_id,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// CreateAgentRequest represents the request to create an agent.
type CreateAgentRequest struct {
	Name               string              `json:"name"`
	Role               string              `json:"role"`
	Description        string              `json:"description,omitempty"`
	TenantID           string              `json:"tenant_id"`
	DepartmentID       *string             `json:"department_id,omitempty"`
	Objectives         []Objective         `json:"objectives,omitempty"`
	Capabilities       []string            `json:"capabilities,omitempty"`
	Tools              []string            `json:"tools,omitempty"`
	MemoryAccess       *MemoryAccess       `json:"memory_access,omitempty"`
	EscalationRules    []string            `json:"escalation_rules,omitempty"`
	GovernancePolicies []string            `json:"governance_policies,omitempty"`
	SupportedLanguages []string            `json:"supported_languages,omitempty"`
	RuntimeConstraints *RuntimeConstraints `json:"runtime_constraints,omitempty"`
	CostProfile        *CostProfile        `json:"cost_profile,omitempty"`
	ExecutionBudget    *ExecutionBudget    `json:"execution_budget,omitempty"`
}

// UpdateAgentRequest represents the request to update an agent.
type UpdateAgentRequest struct {
	Name               *string             `json:"name,omitempty"`
	Role               *string             `json:"role,omitempty"`
	Description        *string             `json:"description,omitempty"`
	DepartmentID       *string             `json:"department_id,omitempty"`
	Objectives         *[]Objective        `json:"objectives,omitempty"`
	Capabilities       *[]string           `json:"capabilities,omitempty"`
	Tools              *[]string           `json:"tools,omitempty"`
	MemoryAccess       *MemoryAccess       `json:"memory_access,omitempty"`
	EscalationRules    *[]string           `json:"escalation_rules,omitempty"`
	GovernancePolicies *[]string           `json:"governance_policies,omitempty"`
	SupportedLanguages *[]string           `json:"supported_languages,omitempty"`
	Status             *AgentStatus        `json:"status,omitempty"`
	RuntimeConstraints *RuntimeConstraints `json:"runtime_constraints,omitempty"`
	CostProfile        *CostProfile        `json:"cost_profile,omitempty"`
	ExecutionBudget    *ExecutionBudget    `json:"execution_budget,omitempty"`
	AccessControl      *AccessControl      `json:"access_control,omitempty"`
}

// AgentVersion represents a version of an agent.
type AgentVersion struct {
	ID                string             `json:"id"`
	AgentID           string             `json:"agent_id"`
	TenantID          string             `json:"tenant_id"`
	Version           string             `json:"version"`
	Status            VersionStatus      `json:"status"`
	ModelConfig       map[string]any     `json:"model_config,omitempty"`
	PromptTemplateRef *string            `json:"prompt_template_ref,omitempty"`
	Description       string             `json:"description"`
	ChangeSummary     string             `json:"change_summary,omitempty"`
	DiffFromPrevious  *string            `json:"diff_from_previous,omitempty"`
	CreatedBy         string             `json:"created_by"`
	PromotedTo        map[string]string  `json:"promoted_to,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// CreateVersionRequest represents the request to create an agent version.
type CreateVersionRequest struct {
	AgentID           string             `json:"agent_id"`
	Version           string             `json:"version"`
	ModelConfig       map[string]any     `json:"model_config,omitempty"`
	PromptTemplateRef *string            `json:"prompt_template_ref,omitempty"`
	Description       string             `json:"description,omitempty"`
	ChangeSummary     string             `json:"change_summary,omitempty"`
}

// UpdateVersionRequest represents the request to update an agent version.
type UpdateVersionRequest struct {
	Status        *VersionStatus `json:"status,omitempty"`
	Description   *string        `json:"description,omitempty"`
	ChangeSummary *string        `json:"change_summary,omitempty"`
}

// PromoteVersionRequest represents the request to promote an agent version.
type PromoteVersionRequest struct {
	Environment string `json:"environment"`
}

// CapabilityEntry represents a capability scored for an agent.
type CapabilityEntry struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agent_id"`
	TenantID      string    `json:"tenant_id"`
	Capability    string    `json:"capability"`
	Score         float64   `json:"score"`
	LastEvaluated time.Time `json:"last_evaluated"`
	Tier          string    `json:"tier"`
}

// CapabilityUpdateRequest represents a request to update agent capabilities.
type CapabilityUpdateRequest struct {
	Capabilities []CapabilityUpdate `json:"capabilities"`
}

// CapabilityUpdate represents a single capability update.
type CapabilityUpdate struct {
	Capability string    `json:"capability"`
	Score      float64   `json:"score"`
	Tier       string    `json:"tier"`
}

// AgentDependency represents a dependency between agents.
type AgentDependency struct {
	ID                string        `json:"id"`
	AgentID           string        `json:"agent_id"`
	TenantID          string        `json:"tenant_id"`
	DependencyAgentID string        `json:"dependency_id"`
	DependencyType    DependencyType `json:"dependency_type"`
	VersionConstraint *string       `json:"version_constraint,omitempty"`
	Description       string        `json:"description,omitempty"`
	Active            bool          `json:"active"`
	CreatedAt         time.Time     `json:"created_at"`
}

// AddDependencyRequest represents the request to add a dependency.
type AddDependencyRequest struct {
	DependencyID      string         `json:"dependency_id"`
	DependencyType    DependencyType `json:"dependency_type"`
	VersionConstraint *string        `json:"version_constraint,omitempty"`
	Description       string         `json:"description,omitempty"`
}

// AgentSearchRequest represents the request to search agents.
type AgentSearchRequest struct {
	TenantID           string              `json:"tenant_id"`
	Capabilities       []string            `json:"capabilities,omitempty"`
	Tools              []string            `json:"tools,omitempty"`
	MinCapabilityScore *float64            `json:"min_capability_score,omitempty"`
	Constraints        *RuntimeConstraints `json:"constraints,omitempty"`
	CostRange          *CostRange          `json:"cost_range,omitempty"`
	DepartmentID       *string             `json:"department_id,omitempty"`
	Status             *AgentStatus        `json:"status,omitempty"`
	SupportedLanguages []string            `json:"supported_languages,omitempty"`
}

// CostRange represents a cost filter range.
type CostRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// AgentSearchResponse represents the response to a search request.
type AgentSearchResponse struct {
	Results []*Agent `json:"results"`
	Total   int      `json:"total"`
}

// CapabilityList represents the response to a capabilities list request.
type CapabilityList struct {
	AgentID      string            `json:"agent_id"`
	Capabilities []*CapabilityEntry `json:"capabilities"`
}

// DependencyList represents the response to a dependencies list request.
type DependencyList struct {
	Dependencies []*AgentDependency `json:"dependencies"`
}
