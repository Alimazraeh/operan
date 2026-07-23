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

// Template represents a department template (the blueprint).
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
	BusinessLogic       *BusinessLogic         `json:"business_logic,omitempty"`
	OrgChart            []Position             `json:"org_chart,omitempty"`
	Services            []ServiceOffering      `json:"services,omitempty"`
	ValueStreams        []ValueStream          `json:"value_streams,omitempty"`
	Risks               []RiskItem             `json:"risks,omitempty"`
	QualityStandards    []QualityStandard      `json:"quality_standards,omitempty"`
	ComplianceControls  []ComplianceControl    `json:"compliance_controls,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Status              string                 `json:"status"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	CreatedBy           string                 `json:"created_by"`
}

// AgentDefinition represents an agent within a template (canonical shape,
// superset of the historical formal and divergent template schemas).
type AgentDefinition struct {
	ID               string                 `json:"id"`
	Role             string                 `json:"role"`
	Name             string                 `json:"name,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Level            string                 `json:"level,omitempty"`          // junior, mid, senior, lead, director
	PositionTitle    string                 `json:"position_title,omitempty"`
	ReportsTo        string                 `json:"reports_to,omitempty"`     // AgentDefinition.ID of direct superior
	AutonomyTier     string                 `json:"autonomy_tier,omitempty"`  // recommend, analyze, coordinate, draft, execute
	Capabilities     []string               `json:"capabilities"`
	Model            string                 `json:"model,omitempty"`
	SystemPrompt     string                 `json:"system_prompt,omitempty"`
	MemoryProfile    string                 `json:"memory_profile,omitempty"`
	ToolRequirements []string               `json:"tool_requirements,omitempty"`
	Constraints      map[string]interface{} `json:"constraints,omitempty"`
	AccessControl    map[string]interface{} `json:"access_control,omitempty"`
	Services         []string               `json:"services,omitempty"`       // ServiceOffering.IDs owned
	DecisionRights   []DecisionRight        `json:"decision_rights,omitempty"`
	EscalationPath   []string               `json:"escalation_path,omitempty"` // ordered AgentDef IDs ending "human"
	Schedule         map[string]interface{} `json:"schedule,omitempty"`
	RiskRefs         []string               `json:"risk_refs,omitempty"`
	QualityRefs      []string               `json:"quality_refs,omitempty"`
	ComplianceRefs   []string               `json:"compliance_refs,omitempty"`
	CreatedAt        *time.Time             `json:"created_at,omitempty"`
	UpdatedAt        *time.Time             `json:"updated_at,omitempty"`
}

// ─── Operating-model sub-structs (shared by Template blueprint and Department instance) ───

// DecisionRight bounds a position's or agent's authority over one decision type.
type DecisionRight struct {
	Decision  string `json:"decision"`
	Authority string `json:"authority"` // decide, recommend, veto, must_be_informed
	Limit     string `json:"limit,omitempty"`
}

// CadenceEntry is one recurring operating ritual (standup, review, audit).
type CadenceEntry struct {
	Name         string   `json:"name"`
	Frequency    string   `json:"frequency"` // daily, weekly, monthly, quarterly, annually, on_demand
	Description  string   `json:"description,omitempty"`
	Participants []string `json:"participants,omitempty"` // Position IDs
}

// BusinessLogic captures why the department exists and how it operates.
type BusinessLogic struct {
	Purpose          string         `json:"purpose"`
	ValueProposition string         `json:"value_proposition,omitempty"`
	Activities       []string       `json:"activities,omitempty"`
	Stakeholders     []string       `json:"stakeholders,omitempty"`
	OperatingCadence []CadenceEntry `json:"operating_cadence,omitempty"`
}

// Position is one node of the org chart: a seat, its holder, and its authority.
type Position struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	RoleType         string          `json:"role_type"`   // director, manager, specialist, coordinator, analyst, support
	HolderType       string          `json:"holder_type"` // ai_agent, human, vacant
	AgentDefID       string          `json:"agent_def_id,omitempty"` // template agents[].id (blueprint link)
	AgentID          string          `json:"agent_id,omitempty"`     // provisioned M04 agent id (instance only)
	HumanRef         string          `json:"human_ref,omitempty"`
	ReportsTo        string          `json:"reports_to,omitempty"` // Position.ID; empty for the root
	Unit             string          `json:"unit,omitempty"`       // sub-team grouping
	AutonomyTier     string          `json:"autonomy_tier,omitempty"`
	DecisionRights   []DecisionRight `json:"decision_rights,omitempty"`
	EscalatesTo      string          `json:"escalates_to,omitempty"` // Position.ID; terminal → human via M09
	ApprovalGateRefs []string        `json:"approval_gate_refs,omitempty"`
}

// SLA is the service-level commitment attached to a ServiceOffering.
type SLA struct {
	Availability   string `json:"availability,omitempty"`
	ResponseTime   string `json:"response_time,omitempty"`
	ResolutionTime string `json:"resolution_time,omitempty"`
	Coverage       string `json:"coverage,omitempty"`
}

// ServiceOffering is one entry of the department's (or an agent's) service portfolio.
type ServiceOffering struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	OwnerPositionID    string   `json:"owner_position_id,omitempty"`
	OwnerAgentDefID    string   `json:"owner_agent_def_id,omitempty"`
	Consumers          []string `json:"consumers,omitempty"`
	SLA                *SLA     `json:"sla,omitempty"`
	DeliveryWorkflowID string   `json:"delivery_workflow_id,omitempty"` // WorkflowDefinition.ID
	KPIRefs            []string `json:"kpi_refs,omitempty"`             // KPIDefinition.IDs
	RequestChannel     string   `json:"request_channel,omitempty"`
	Status             string   `json:"status,omitempty"` // active, planned, retired
}

// ValueStage is one stage of a value stream: inputs → activities → outputs.
type ValueStage struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Inputs          []string `json:"inputs,omitempty"`
	Activities      []string `json:"activities,omitempty"`
	Outputs         []string `json:"outputs,omitempty"`
	OwnerPositionID string   `json:"owner_position_id,omitempty"`
	WorkflowRef     string   `json:"workflow_ref,omitempty"`
}

// ValueStream maps how the department turns inputs into business outcomes.
type ValueStream struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Description        string       `json:"description,omitempty"`
	Stages             []ValueStage `json:"stages"`
	Outcome            string       `json:"outcome,omitempty"`
	BusinessOutcome    string       `json:"business_outcome,omitempty"`
	ValueMetricKPIRefs []string     `json:"value_metric_kpi_refs,omitempty"`
}

// RiskItem is one entry of the department's risk register.
type RiskItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Category        string `json:"category,omitempty"` // operational, security, compliance, financial, model
	Severity        string `json:"severity"`           // low, medium, high, critical
	Likelihood      string `json:"likelihood"`         // rare, unlikely, possible, likely, almost_certain
	Mitigation      string `json:"mitigation,omitempty"`
	OwnerPositionID string `json:"owner_position_id,omitempty"`
	Scope           string `json:"scope,omitempty"` // department, agent, service
	AgentDefID      string `json:"agent_def_id,omitempty"`
	ServiceRef      string `json:"service_ref,omitempty"`
	Status          string `json:"status,omitempty"` // open, mitigating, accepted, closed
	ReviewCadence   string `json:"review_cadence,omitempty"`
}

// QualityStandard is a measurable quality bar (SLO, review gate, accuracy target).
type QualityStandard struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Type          string `json:"type,omitempty"` // slo, sla_internal, review_gate, accuracy_target
	Target        string `json:"target"`
	MeasureKPIRef string `json:"measure_kpi_ref,omitempty"`
	Scope         string `json:"scope,omitempty"` // department, agent, service
	ServiceRef    string `json:"service_ref,omitempty"`
	AgentDefID    string `json:"agent_def_id,omitempty"`
	EnforcedBy    string `json:"enforced_by,omitempty"` // GovernanceRule.ID or M09 gate ref
}

// ComplianceControl maps a framework control to the governance rules implementing it.
type ComplianceControl struct {
	ID                 string   `json:"id"`
	Framework          string   `json:"framework"` // ITIL-v4, ISO-27001, SOC2, NIST-CSF, GDPR, custom
	ControlID          string   `json:"control_id,omitempty"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	AppliesTo          string   `json:"applies_to,omitempty"` // department, agent, service
	AgentDefIDs        []string `json:"agent_def_ids,omitempty"`
	GovernanceRuleRefs []string `json:"governance_rule_refs,omitempty"`
	PolicyRef          string   `json:"policy_ref,omitempty"` // Module 10 policy id
	EvidenceKPIRefs    []string `json:"evidence_kpi_refs,omitempty"`
	Status             string   `json:"status,omitempty"` // implemented, planned, not_applicable
}

// Department is a deployed, living department instance materialized from a Template.
type Department struct {
	ID                  string                 `json:"id"`
	TenantID            string                 `json:"tenant_id"`
	Name                string                 `json:"name"`
	Slug                string                 `json:"slug,omitempty"`
	Category            string                 `json:"category"`
	Description         string                 `json:"description,omitempty"`
	TemplateID          string                 `json:"template_id,omitempty"`
	TemplateVersion     string                 `json:"template_version,omitempty"`
	DeploymentID        string                 `json:"deployment_id,omitempty"`
	Status              string                 `json:"status"` // provisioning, operational, degraded, suspended, archived
	Mission             string                 `json:"mission,omitempty"`
	BusinessLogic       *BusinessLogic         `json:"business_logic,omitempty"`
	OrgChart            []Position             `json:"org_chart,omitempty"`
	Services            []ServiceOffering      `json:"services,omitempty"`
	ValueStreams        []ValueStream          `json:"value_streams,omitempty"`
	Risks               []RiskItem             `json:"risks,omitempty"`
	QualityStandards    []QualityStandard      `json:"quality_standards,omitempty"`
	ComplianceControls  []ComplianceControl    `json:"compliance_controls,omitempty"`
	GovernanceRules     []GovernanceRule       `json:"governance_rules,omitempty"`
	KPIS                []KPIDefinition        `json:"kpis,omitempty"`
	OperationalPolicies []OperationalPolicy    `json:"operational_policies,omitempty"`
	AgentIDs            []string               `json:"agent_ids,omitempty"`    // provisioned M04 agent ids
	WorkflowIDs         []string               `json:"workflow_ids,omitempty"`
	MemoryRefs          []string               `json:"memory_refs,omitempty"`  // M07 document ids
	Environment         string                 `json:"environment,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	CreatedBy           string                 `json:"created_by,omitempty"`
}

// StageRecord is one entry of a deployment's stage history.
type StageRecord struct {
	Stage       string     `json:"stage"`  // select, configure, connect_data, provision_memory, deploy_swarm, operational
	Status      string     `json:"status"` // running, completed, failed
	Detail      string     `json:"detail,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
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
	DepartmentID        string                 `json:"department_id,omitempty"` // the Department instance this deploy materialized
	Stages              []StageRecord          `json:"stages,omitempty"`        // server-orchestrated stage history
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
	BusinessLogic       *BusinessLogic         `json:"business_logic,omitempty"`
	OrgChart            []Position             `json:"org_chart,omitempty"`
	Services            []ServiceOffering      `json:"services,omitempty"`
	ValueStreams        []ValueStream          `json:"value_streams,omitempty"`
	Risks               []RiskItem             `json:"risks,omitempty"`
	QualityStandards    []QualityStandard      `json:"quality_standards,omitempty"`
	ComplianceControls  []ComplianceControl    `json:"compliance_controls,omitempty"`
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
	DepartmentName string                 `json:"department_name,omitempty"` // override the instance name (defaults to template name)
}

