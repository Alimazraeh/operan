// Package store provides in-memory, tenant-isolated storage for Module 11
// (Observability): metrics, trace spans, alerts, and component health.
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
)

// timeNow returns the current UTC time (indirection eases testing).
var timeNow = func() time.Time { return time.Now().UTC() }

// MetricType per the Metric schema.
type MetricType string

const (
	MetricCounter   MetricType = "counter"
	MetricGauge     MetricType = "gauge"
	MetricHistogram MetricType = "histogram"
	MetricTimer     MetricType = "timer"
)

// ValidMetricType reports whether s is a known metric type.
func ValidMetricType(s string) bool {
	switch MetricType(s) {
	case MetricCounter, MetricGauge, MetricHistogram, MetricTimer:
		return true
	}
	return false
}

// SpanType per the TraceSpan schema.
type SpanType string

const (
	SpanOrchestration SpanType = "orchestration"
	SpanTool          SpanType = "tool"
	SpanMemory        SpanType = "memory"
	SpanPolicy        SpanType = "policy"
	SpanHumanGate     SpanType = "human_gate"
)

// ValidSpanType reports whether s is a known span type.
func ValidSpanType(s string) bool {
	switch SpanType(s) {
	case SpanOrchestration, SpanTool, SpanMemory, SpanPolicy, SpanHumanGate:
		return true
	}
	return false
}

// SpanStatus per the TraceSpan schema.
type SpanStatus string

const (
	SpanOK        SpanStatus = "ok"
	SpanError     SpanStatus = "error"
	SpanCancelled SpanStatus = "cancelled"
)

// ValidSpanStatus reports whether s is a known span status.
func ValidSpanStatus(s string) bool {
	switch SpanStatus(s) {
	case SpanOK, SpanError, SpanCancelled:
		return true
	}
	return false
}

// AlertSeverity per the Alert schema.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// ValidAlertSeverity reports whether s is a known severity.
func ValidAlertSeverity(s string) bool {
	switch AlertSeverity(s) {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

// HealthState per the HealthStatus schema.
type HealthState string

const (
	Healthy   HealthState = "healthy"
	Degraded  HealthState = "degraded"
	Unhealthy HealthState = "unhealthy"
)

// Metric matches the OpenAPI Metric schema.
type Metric struct {
	ID          string                 `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	MetricName  string                 `json:"metric_name"`
	MetricValue float64                `json:"metric_value"`
	MetricType  MetricType             `json:"metric_type"`
	Labels      map[string]interface{} `json:"labels,omitempty"`
	SourceID    string                 `json:"source_id,omitempty"`
	RecordedAt  time.Time              `json:"recorded_at"`
}

// TraceSpan matches the OpenAPI TraceSpan schema.
type TraceSpan struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	TenantID   string                 `json:"tenant_id"`
	SpanName   string                 `json:"span_name"`
	SpanType   SpanType               `json:"span_type"`
	StartTime  time.Time              `json:"start_time"`
	DurationMs int                    `json:"duration_ms"`
	WorkflowID string                 `json:"workflow_id,omitempty"`
	AgentID    string                 `json:"agent_id,omitempty"`
	EndTime    *time.Time             `json:"end_time,omitempty"`
	Status     SpanStatus             `json:"status,omitempty"`
	Tags       map[string]interface{} `json:"tags,omitempty"`
}

// Alert matches the OpenAPI Alert schema.
type Alert struct {
	ID                   string        `json:"id"`
	TenantID             string        `json:"tenant_id"`
	AlertName            string        `json:"alert_name"`
	Severity             AlertSeverity `json:"severity"`
	TriggeredAt          time.Time     `json:"triggered_at"`
	ConditionDescription string        `json:"condition_description,omitempty"`
	CurrentValue         float64       `json:"current_value,omitempty"`
	Threshold            float64       `json:"threshold,omitempty"`
	ResolvedAt           *time.Time    `json:"resolved_at"`
	ResolvedBy           string        `json:"resolved_by,omitempty"`
}

// HealthStatus matches the OpenAPI HealthStatus schema.
type HealthStatus struct {
	TenantID       string      `json:"tenant_id"`
	ComponentID    string      `json:"component_id"`
	ComponentType  string      `json:"component_type"` // agent | workflow | tool | memory | policy | gateway
	NewStatus      HealthState `json:"new_status"`
	ChangedAt      time.Time   `json:"changed_at"`
	PreviousStatus HealthState `json:"previous_status,omitempty"`
	Reason         string      `json:"reason,omitempty"`
}
