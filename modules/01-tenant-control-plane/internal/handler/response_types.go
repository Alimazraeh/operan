// Package handler defines all response types used by tenant-control-plane handlers.
// These types are designed to match the OpenAPI contract schemas.
package handler

import (
	"net/http"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
)

// ─── Response DTOs ───────────────────────────────────────────────────────────

// TenantResponse represents a tenant in API responses.
type TenantResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	DisplayName      string                 `json:"display_name,omitempty"`
	Plan             string                 `json:"plan"`
	Region           string                 `json:"region"`
	IsolationLevel   string                 `json:"isolation_level"`
	Status           string                 `json:"status"`
	Quota            QuotaResponse          `json:"quota"`
	ContactEmail     string                 `json:"contact_email,omitempty"`
	AdminEmail       string                 `json:"admin_email,omitempty"`
	CustomMetadata   map[string]interface{} `json:"custom_metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// QuotaResponse represents resource quota information.
type QuotaResponse struct {
	MaxAgents              int `json:"max_agents"`
	MaxWorkflowsPerDay     int `json:"max_workflows_per_day"`
	MaxStorageGB           int `json:"max_storage_gb"`
	MaxMonthlyTokens       int `json:"max_monthly_tokens"`
	MaxConcurrentWorkflows int `json:"max_concurrent_workflows"`
}

// QuotaCheckResponse represents the result of a quota check operation.
type QuotaCheckResponse struct {
	TenantID        string             `json:"tenant_id"`
	QuotaType       string             `json:"quota_type"`
	Current         int                `json:"current_value"`
	Limit           int                `json:"limit"`
	Allowed         bool               `json:"allowed"`
	ActionTaken     string             `json:"action_taken,omitempty"`
	Reason          string             `json:"reason,omitempty"`
	CheckedAt       time.Time          `json:"checked_at"`
}

// TenantStatusResponse represents tenant status information.
type TenantStatusResponse struct {
	Status             string            `json:"status"`
	AllowedTransitions []string          `json:"allowed_transitions"`
	Transitions        []TenantTransition `json:"transitions"`
	UpdatedAt          time.Time         `json:"updated_at,omitempty"`
}

// TenantTransition represents a possible status change.
type TenantTransition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// AgentResponse represents an agent in API responses.
type AgentResponse struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id"`
	Name            string          `json:"name"`
	Model           string          `json:"model"`
	Role            string          `json:"role"`
	SystemPrompt    string          `json:"system_prompt"`
	Status          string          `json:"status"`
	CurrentWorkflow *string         `json:"current_workflow,omitempty"`
	CurrentTask     *string         `json:"current_task,omitempty"`
	ToolAccessJSON  []byte          `json:"tool_access_json,omitempty"`
	LastRunAt       *time.Time      `json:"last_run_at,omitempty"`
	SuccessCount    int             `json:"success_count"`
	FailureCount    int             `json:"failure_count"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ResourceResponse represents a cloud resource.
type ResourceResponse struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Region    string                 `json:"region"`
	Spec      ResourceSpecResponse   `json:"spec"`
	Status    string                 `json:"status"`
	Endpoint  string                 `json:"endpoint"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// ResourceSpecResponse represents resource configuration.
type ResourceSpecResponse struct {
	Engine      string                 `json:"engine,omitempty"`
	Size        string                 `json:"size,omitempty"`
	VCPUs       int                    `json:"vcpus,omitempty"`
	RAMGB       int                    `json:"ram_gb,omitempty"`
	StorageGB   int                    `json:"storage_gb,omitempty"`
	Replicas    int                    `json:"replicas,omitempty"`
	ExtraConfig map[string]interface{} `json:"extra_config,omitempty"`
}

// InvoiceResponse represents a billing invoice.
type InvoiceResponse struct {
	ID             string                  `json:"id"`
	TenantID       string                  `json:"tenant_id"`
	SubscriptionID string                  `json:"subscription_id"`
	IssueDate      time.Time               `json:"issue_date"`
	DueDate        time.Time               `json:"due_date"`
	DueDateRaw     string                  `json:"due_date_raw"`
	Amount         float64                 `json:"amount"`
	Currency       string                  `json:"currency"`
	Status         string                  `json:"status"`
	LineItems      []InvoiceLineItemResponse `json:"line_items"`
	UsageSummary   map[string]interface{}  `json:"usage_summary,omitempty"`
	CreditApplied  float64                 `json:"credit_applied,omitempty"`
	PaidAt         *time.Time              `json:"paid_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// InvoiceLineItemResponse represents a single line item on an invoice.
type InvoiceLineItemResponse struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
}

// UsageResponse represents billing usage data.
type UsageResponse struct {
	ID             string                 `json:"id"`
	TenantID       string                 `json:"tenant_id"`
	PeriodStart    time.Time              `json:"period_start"`
	PeriodEnd      time.Time              `json:"period_end"`
	Metrics        map[string]interface{} `json:"metrics"`
	UsageGB        int                    `json:"usage_gb"`
	EstimatedCost  float64                `json:"estimated_cost"`
	Currency       string                 `json:"currency"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// PaymentMethodResponse represents a payment method.
type PaymentMethodResponse struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	Type            string     `json:"type"`
	LastFour        string     `json:"last_four"`
	ExpiryMonth     int        `json:"expiry_month,omitempty"`
	ExpiryYear      int        `json:"expiry_year,omitempty"`
	BillingAddress  string     `json:"billing_address,omitempty"`
	IsDefault       bool       `json:"is_default"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UpgradePlanResponse represents a plan upgrade operation result.
type UpgradePlanResponse struct {
	TenantID        string     `json:"tenant_id"`
	OldPlan         string     `json:"old_plan"`
	NewPlan         string     `json:"new_plan"`
	Status          string     `json:"status"`
	ProratedCharge  float64    `json:"prorated_charge,omitempty"`
	EffectiveDate   time.Time  `json:"effective_date"`
}

// SubscriptionResponse represents a tenant subscription.
type SubscriptionResponse struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	Plan               string         `json:"plan"`
	PlanName           string         `json:"plan_name"`
	Status             string         `json:"status"`
	BillingCycle       string         `json:"billing_cycle"`
	SeatCount          int            `json:"seat_count"`
	UnitPrice          float64        `json:"unit_price"`
	TotalAmount        float64        `json:"total_amount"`
	Currency           string         `json:"currency"`
	CurrentPeriodStart time.Time      `json:"current_period_start"`
	CurrentPeriodEnd   time.Time      `json:"current_period_end"`
	NextBillingDate    time.Time      `json:"next_billing_date"`
	CancelAtPeriodEnd  bool           `json:"cancel_at_period_end"`
	CancelledAt        *time.Time     `json:"cancelled_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	CustomQuotas       *QuotaResponse `json:"custom_quotas,omitempty"`
}

// SecretMetadataResponse represents a secret for listing purposes.
type SecretMetadataResponse struct {
	ID           string    `json:"id"`
	Key          string    `json:"key"`
	Description  string    `json:"description"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Version      int       `json:"version"`
	VersionCount int       `json:"version_count"`
}

// SecretResponse represents a secret with its value.
type SecretResponse struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Value          string    `json:"value,omitempty"`
	EncryptedValue string    `json:"encrypted_value"`
	Description    string    `json:"description"`
	Tags           []string  `json:"tags"`
	Version        int       `json:"version"`
	VersionCount   int       `json:"version_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ModuleStatusResponse represents the module health/status endpoint.
type ModuleStatusResponse struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`
	Health    string `json:"health"`
}

// ─── Paginated Response Wrappers (OpenAPI spec) ─────────────────────────────

// TenantListResponse wraps paginated tenant data per OpenAPI contract.
// Fields: items, page, page_size, total, has_more
type TenantListResponse struct {
	Items   []*TenantResponse `json:"items"`
	Page    int               `json:"page"`
	PageSize int              `json:"page_size"`
	Total   int               `json:"total"`
	HasMore bool              `json:"has_more"`
}

// AgentListResponse wraps paginated agent data per OpenAPI contract.
type AgentListResponse struct {
	Items   []*AgentResponse `json:"items"`
	Page    int              `json:"page"`
	PageSize int             `json:"page_size"`
	Total   int              `json:"total"`
	HasMore bool             `json:"has_more"`
}

// ResourceListResponse wraps paginated resource data per OpenAPI contract.
type ResourceListResponse struct {
	Items   []*ResourceResponse `json:"items"`
	Page    int                 `json:"page"`
	PageSize int                `json:"page_size"`
	Total   int                 `json:"total"`
	HasMore bool                `json:"has_more"`
}

// InvoiceListResponse wraps paginated invoice data per OpenAPI contract.
// Fields: items, page, page_size, total, has_more
type InvoiceListResponse struct {
	Items   []*InvoiceResponse `json:"items"`
	Page    int                `json:"page"`
	PageSize int               `json:"page_size"`
	Total   int                `json:"total"`
	HasMore bool               `json:"has_more"`
}

// SecretListResponse wraps paginated secret metadata per OpenAPI contract.
type SecretListResponse struct {
	Items   []*SecretMetadataResponse `json:"items"`
	Page    int                       `json:"page"`
	PageSize int                      `json:"page_size"`
	Total   int                       `json:"total"`
	HasMore bool                      `json:"has_more"`
}

// UsageListResponse wraps paginated usage data per OpenAPI contract.
type UsageListResponse struct {
	Items   []*UsageResponse `json:"items"`
	Page    int              `json:"page"`
	PageSize int             `json:"page_size"`
	Total   int              `json:"total"`
	HasMore bool             `json:"has_more"`
}

// PaymentMethodListResponse wraps paginated payment method data per OpenAPI contract.
type PaymentMethodListResponse struct {
	Items   []*PaymentMethodResponse `json:"items"`
	Page    int                      `json:"page"`
	PageSize int                     `json:"page_size"`
	Total   int                      `json:"total"`
	HasMore bool                     `json:"has_more"`
}

// SubscriptionListResponse wraps subscription data per OpenAPI contract.
// Fields: items, total
type SubscriptionListResponse struct {
	Items []*SubscriptionResponse `json:"items"`
	Total int                     `json:"total"`
}

// ─── Namespace/ListResponse types ────────────────────────────────────────────

// NamespaceListResponse wraps paginated namespace data per OpenAPI contract.
// Fields: items, page, page_size, total, has_more
type NamespaceListResponse struct {
	Items    []*NamespaceResponse `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int                  `json:"total"`
	HasMore  bool                 `json:"has_more"`
}

// ─── DeploymentListResponse types ────────────────────────────────────────────

// DeploymentListResponse wraps paginated deployment data per OpenAPI contract.
// Fields: items, page, page_size, total, has_more
type DeploymentListResponse struct {
	Items    []*DeploymentResponse `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Total    int                   `json:"total"`
	HasMore  bool                  `json:"has_more"`
}

// ─── PolicyListResponse types ────────────────────────────────────────────────

// PolicyListResponse wraps paginated policy data per OpenAPI contract.
// Fields: items, page, page_size, total, has_more
type PolicyListResponse struct {
	Items    []*PolicyResponse `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int               `json:"total"`
	HasMore  bool              `json:"has_more"`
}

// ─── EnvironmentListResponse types ───────────────────────────────────────────

// EnvironmentListResponse wraps paginated environment data per OpenAPI contract.
// Fields: items, page, page_size, total, has_more
type EnvironmentListResponse struct {
	Items    []*EnvironmentResponse `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"`
	HasMore  bool                   `json:"has_more"`
}

// ─── Route Registration ──────────────────────────────────────────────────────

// RegisterRoutes attaches all API endpoints to the given mux.
func RegisterRoutes(h *middleware.Handler, mux *http.ServeMux) {
	// Tenant CRUD
	mux.HandleFunc("GET /v1/tenants", ListTenants(h))
	mux.HandleFunc("POST /v1/tenants", CreateTenant(h))
	mux.HandleFunc("GET /v1/tenants/{id}", GetTenant(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}", PatchTenant(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}", DeleteTenant(h))

	// Tenant quota
	mux.HandleFunc("GET /v1/tenants/{id}/quota", GetTenantQuota(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/quota", PatchTenantQuota(h))

	// Tenant status
	mux.HandleFunc("GET /v1/tenants/{id}/status", GetTenantStatus(h))
	mux.HandleFunc("POST /v1/tenants/{id}/status/transition", TransitionTenantStatus(h))

	// Agent management
	mux.HandleFunc("GET /v1/tenants/{id}/agents", ListAgents(h))
	mux.HandleFunc("POST /v1/tenants/{id}/agents", CreateAgent(h))
	mux.HandleFunc("GET /v1/tenants/{id}/agents/{agent_id}", GetAgent(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/agents/{agent_id}", PatchAgent(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/agents/{agent_id}", DeleteAgent(h))

	// Resource management
	mux.HandleFunc("GET /v1/tenants/{id}/resources", ListResources(h))
	mux.HandleFunc("POST /v1/tenants/{id}/resources", CreateResource(h))
	mux.HandleFunc("GET /v1/tenants/{id}/resources/{resource_id}", GetResource(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/resources/{resource_id}", PatchResource(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/resources/{resource_id}", DeleteResource(h))

	// Billing: invoices
	mux.HandleFunc("GET /v1/tenants/{id}/billing/invoices", ListInvoices(h))
	mux.HandleFunc("GET /v1/tenants/{id}/billing/invoices/{invoice_id}", GetInvoice(h))
	mux.HandleFunc("GET /v1/tenants/{id}/billing/invoices/{invoice_id}/download", DownloadInvoice(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/billing/invoices/{invoice_id}", UpdateInvoice(h))

	// Billing: usage
	mux.HandleFunc("GET /v1/tenants/{id}/billing/usage", GetBillingUsage(h))

	// Billing: payment methods
	mux.HandleFunc("GET /v1/tenants/{id}/billing/payment-methods", ListPaymentMethods(h))
	mux.HandleFunc("POST /v1/tenants/{id}/billing/payment-methods", CreatePaymentMethod(h))
	mux.HandleFunc("GET /v1/tenants/{id}/billing/payment-methods/{method_id}", GetBillingMethod(h))
	mux.HandleFunc("POST /v1/tenants/{id}/billing/payment-methods/{pm_id}/set-default", SetDefaultPaymentMethod(h))

	// Billing: subscription
	mux.HandleFunc("GET /v1/tenants/{id}/subscriptions", ListSubscriptions(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/subscriptions", PatchSubscription(h))
	mux.HandleFunc("POST /v1/tenants/{id}/subscriptions/cancel", CancelSubscription(h))

	// Subscription detail endpoints
	mux.HandleFunc("GET /v1/tenants/{id}/subscriptions/{subscription_id}", GetSubscriptionByID(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/subscriptions/{subscription_id}", UpdateSubscriptionByID(h))
	mux.HandleFunc("POST /v1/tenants/{id}/subscriptions/{subscription_id}/upgrade", UpgradeSubscription(h))

	// Billing: upgrade plan
	mux.HandleFunc("POST /v1/tenants/{id}/billing/upgrade-plan", UpgradePlan(h))

	// Secrets
	mux.HandleFunc("GET /v1/tenants/{id}/secrets", ListSecrets(h))
	mux.HandleFunc("POST /v1/tenants/{id}/secrets", CreateSecret(h))
	mux.HandleFunc("GET /v1/tenants/{id}/secrets/{secret_id}", GetSecret(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/secrets/{secret_id}", UpdateSecret(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/secrets/{secret_id}", DeleteSecret(h))
	mux.HandleFunc("POST /v1/tenants/{id}/secrets/{secret_id}/rotate", RotateSecret(h))

	// Namespaces
	mux.HandleFunc("GET /v1/tenants/{id}/namespaces", ListNamespaces(h))
	mux.HandleFunc("POST /v1/tenants/{id}/namespaces", CreateNamespace(h))
	mux.HandleFunc("GET /v1/tenants/{id}/namespaces/{ns_id}", GetNamespace(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/namespaces/{ns_id}", PatchNamespace(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/namespaces/{ns_id}", DeleteNamespace(h))
	mux.HandleFunc("GET /v1/tenants/{id}/namespaces/{ns_id}/quota", GetNamespaceQuota(h))
	mux.HandleFunc("GET /v1/tenants/{id}/namespaces/{ns_id}/quota/check", CheckNamespaceQuota(h))

	// Deployments
	mux.HandleFunc("GET /v1/tenants/{id}/deployments", ListDeployments(h))
	mux.HandleFunc("POST /v1/tenants/{id}/deployments", CreateDeployment(h))
	mux.HandleFunc("GET /v1/tenants/{id}/deployments/{deployment_id}", GetDeployment(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/deployments/{deployment_id}", PatchDeployment(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/deployments/{deployment_id}", DeleteDeployment(h))
	mux.HandleFunc("POST /v1/tenants/{id}/deployments/{deployment_id}/rollout", RolloutDeployment(h))
	mux.HandleFunc("POST /v1/tenants/{id}/deployments/{deployment_id}/scale", ScaleDeployment(h))
	mux.HandleFunc("GET /v1/tenants/{id}/deployments/{deployment_id}/status", GetDeploymentStatus(h))
	mux.HandleFunc("POST /v1/tenants/{id}/deployments/{deployment_id}/pause", PauseDeployment(h))
	mux.HandleFunc("POST /v1/tenants/{id}/deployments/{deployment_id}/resume", ResumeDeployment(h))

	// Policies
	mux.HandleFunc("GET /v1/tenants/{id}/policies", ListPolicies(h))
	mux.HandleFunc("POST /v1/tenants/{id}/policies", CreatePolicy(h))
	mux.HandleFunc("GET /v1/tenants/{id}/policies/{policy_id}", GetPolicy(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/policies/{policy_id}", PatchPolicy(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/policies/{policy_id}", DeletePolicy(h))
	mux.HandleFunc("POST /v1/tenants/{id}/policies/evaluate", EvaluatePolicies(h))
	mux.HandleFunc("GET /v1/tenants/{id}/policies/check-compliance", CheckPolicyCompliance(h))
	mux.HandleFunc("GET /v1/tenants/{id}/policies/stats", GetPolicyStats(h))

	// Environments
	mux.HandleFunc("GET /v1/tenants/{id}/environments", ListEnvironments(h))
	mux.HandleFunc("POST /v1/tenants/{id}/environments", CreateEnvironment(h))
	mux.HandleFunc("GET /v1/tenants/{id}/environments/{env_id}", GetEnvironment(h))
	mux.HandleFunc("PATCH /v1/tenants/{id}/environments/{env_id}", PatchEnvironment(h))
	mux.HandleFunc("DELETE /v1/tenants/{id}/environments/{env_id}", DeleteEnvironment(h))
	mux.HandleFunc("POST /v1/tenants/{id}/environments/{env_id}/activate", ActivateEnvironment(h))
	mux.HandleFunc("POST /v1/tenants/{id}/environments/{env_id}/deactivate", DeactivateEnvironment(h))
	mux.HandleFunc("GET /v1/tenants/{id}/environments/{env_id}/isolation-config", GetEnvironmentIsolationConfig(h))
	mux.HandleFunc("PUT /v1/tenants/{id}/environments/{env_id}/isolation-config", UpdateEnvironmentIsolationConfig(h))

	// Health/status
	mux.HandleFunc("GET /v1/status", GetModuleStatus(h))
}
