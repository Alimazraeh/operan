package store

import "time"

// RoutingRule represents a routing rule for a tenant.
type RoutingRule struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	RuleName         string    `json:"rule_name"`
	Description      string    `json:"description,omitempty"`
	TaskType         string    `json:"task_type"`
	Priority         int       `json:"priority"`
	MinCostThreshold float64   `json:"min_cost_threshold"`
	MaxLatencyMs     int       `json:"max_latency_ms"`
	MaxTokens        int       `json:"max_tokens"`
	FailoverEnabled  bool      `json:"failover_enabled"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RoutingRuleModel associates a model with a routing rule.
type RoutingRuleModel struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	RuleID         string    `json:"rule_id"`
	ModelID        string    `json:"model_id"`
	CapabilityScore float64  `json:"capability_score"`
	CostWeight     float64   `json:"cost_weight"`
	LatencyWeight  float64   `json:"latency_weight"`
	ReliabilityWeight float64 `json:"reliability_weight"`
	CreatedAt      time.Time `json:"created_at"`
}

// RoutingPerformance tracks per-model, per-task performance metrics.
type RoutingPerformance struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ModelID      string    `json:"model_id"`
	TaskType     string    `json:"task_type"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	P99LatencyMs float64   `json:"p99_latency_ms"`
	ErrorRate    float64   `json:"error_rate"`
	CallsCount   int       `json:"calls_count"`
	AvgCostUSD   float64   `json:"avg_cost_usd"`
	QualityScore float64   `json:"quality_score"`
	LastCallAt   *time.Time `json:"last_call_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}