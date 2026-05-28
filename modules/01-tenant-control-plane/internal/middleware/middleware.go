// Package middleware provides HTTP middleware for tenant-control-plane.
// It handles request IDs, trace propagation, tenant context injection,
// authentication, and request logging.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/events"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Context keys ────────────────────────────────────────────────────────────

type contextKey string

const (
	ctxKeyTenantID  contextKey = "tenant_id"
	ctxKeyTraceID   contextKey = "trace_id"
	ctxKeyRequestID contextKey = "request_id"
	ctxKeyUserID    contextKey = "user_id"
	ctxKeyAuthType  contextKey = "auth_type"
)

// ─── Middleware chain ────────────────────────────────────────────────────────

// RequestID generates a unique request ID and adds it to the context/response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := generateID()
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, reqID)
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceID generates a trace ID for OpenTelemetry correlation.
func TraceID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = generateID()
		}
		ctx := context.WithValue(r.Context(), ctxKeyTraceID, traceID)
		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TenantContext injects tenant ID from X-Tenant-ID header into context.
func TenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID != "" {
			ctx := context.WithValue(r.Context(), ctxKeyTenantID, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// ─── Handler struct ──────────────────────────────────────────────────────────

// Handler holds all stores and provides HTTP route registration.
type Handler struct {
	TenantStore        *store.TenantStore
	SecretStore        *store.SecretStore
	SubscriptionStore  *store.SubscriptionStore
	BillingStore       *store.BillingStore
	PaymentMethodStore *store.PaymentMethodStore
	EventPublisher     *events.Publisher
	AgentStore         *store.AgentStore
	ResourceStore      *store.ResourceStore
	NamespaceStore     *store.NamespaceStore
	DeploymentStore    *store.DeploymentStore
	PolicyStore        *store.PolicyStore
	EnvironmentStore   *store.EnvironmentStore
}

// NewHandler creates a new Handler with all stores.
func NewHandler(tenantStore *store.TenantStore, secretStore *store.SecretStore, subStore *store.SubscriptionStore, billingStore *store.BillingStore) *Handler {
	return &Handler{
		TenantStore:        tenantStore,
		SecretStore:        secretStore,
		SubscriptionStore:  subStore,
		BillingStore:       billingStore,
		PaymentMethodStore: store.NewPaymentMethodStore(),
		EventPublisher:     events.NewPublisher(),
		AgentStore:         store.NewAgentStore(),
		ResourceStore:      store.NewResourceStore(),
		NamespaceStore:     store.NewNamespaceStore(),
		DeploymentStore:    store.NewDeploymentStore(),
		PolicyStore:        store.NewPolicyStore(),
		EnvironmentStore:   store.NewEnvironmentStore(),
	}
}

// ─── Helper methods ──────────────────────────────────────────────────────────

// WriteJSON writes a JSON response with the given status code.
func (h *Handler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response matching the OpenAPI Error schema.
func (h *Handler) WriteError(w http.ResponseWriter, status int, code int, message string, details string) {
	resp := ErrorResponse{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// Logger wraps each request with timing and status logging.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lw.statusCode, duration)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

// ─── Response types ──────────────────────────────────────────────────────────

// ErrorResponse matches the OpenAPI Error schema.
type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details"`
	RequestID string `json:"request_id"`
}

// PaginatedResponse is the base wrapper for paginated lists.
type PaginatedResponse[T any] struct {
	Data    []*T `json:"data"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// TenantPatchRequest represents a partial update to a tenant.
type TenantPatchRequest struct {
	Name           string                 `json:"name,omitempty"`
	DisplayName    string                 `json:"display_name,omitempty"`
	Status         store.TenantStatus     `json:"status,omitempty"`
	Plan           store.Plan             `json:"plan,omitempty"`
	Region         store.Region           `json:"region,omitempty"`
	IsolationLevel store.IsolationLevel   `json:"isolation_level,omitempty"`
	ContactEmail   string                 `json:"contact_email,omitempty"`
	AdminEmail     string                 `json:"admin_email,omitempty"`
	CustomMetadata map[string]interface{} `json:"custom_metadata,omitempty"`
	Quota          *store.QuotaConfig     `json:"quota,omitempty"`
}

// AgentPatchRequest represents a partial update to an agent.
type AgentPatchRequest struct {
	Model          string            `json:"model,omitempty"`
	Role           string            `json:"role,omitempty"`
	SystemPrompt   string            `json:"system_prompt,omitempty"`
	Status         store.AgentStatus `json:"status,omitempty"`
	ToolAccessJSON []byte            `json:"tool_access_json,omitempty"`
}

// ResourcePatchRequest represents a partial update to a resource.
type ResourcePatchRequest struct {
	Name   string                 `json:"name,omitempty"`
	Spec   store.ResourceSpec     `json:"spec,omitempty"`
	Status store.ResourceStatus   `json:"status,omitempty"`
}

// InvoiceUpdateRequest for updating invoice status.
type InvoiceUpdateRequest struct {
	Status store.BillingStatus `json:"status"`
}

// SubscriptionUpdateRequest for PATCH /subscriptions/{id}.
type SubscriptionUpdateRequest struct {
	Plan         store.Plan         `json:"plan,omitempty"`
	BillingCycle store.BillingCycle `json:"billing_cycle,omitempty"`
	SeatCount    *int               `json:"seat_count,omitempty"`
	CustomQuotas *store.QuotaConfig `json:"custom_quotas,omitempty"`
}

// SubscriptionCancelRequest for cancel operations.
type SubscriptionCancelRequest struct {
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	Reason            string `json:"reason,omitempty"`
}

// SubscriptionUpgradeRequest for plan upgrades.
type SubscriptionUpgradeRequest struct {
	TargetPlan store.Plan `json:"target_plan"`
}

// ─── ID generation ───────────────────────────────────────────────────────────

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
