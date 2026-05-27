// Package middleware provides HTTP middleware for the orchestration engine.
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

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── Context keys ────────────────────────────────────────────────────────────

type contextKey string

const (
	ctxKeyTenantID  contextKey = "tenant_id"
	ctxKeyTraceID   contextKey = "trace_id"
	ctxKeyRequestID contextKey = "request_id"
	ctxKeyUserID    contextKey = "user_id"
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

// ─── Handler struct ──────────────────────────────────────────────────────────

// Handler holds all stores and provides HTTP route registration.
type Handler struct {
	WorkflowStore    *store.WorkflowStore
	ScheduleStore    *store.ScheduleStore
	AgentStore       *store.AgentStore
	EventPublisher   *events.Publisher
}

// NewHandler creates a new Handler with all stores.
func NewHandler() *Handler {
	return &Handler{
		WorkflowStore:  store.NewWorkflowStore(),
		ScheduleStore:  store.NewScheduleStore(),
		AgentStore:     store.NewAgentStore(),
		EventPublisher: events.NewPublisher(),
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

// TenantIDFromContext extracts the tenant ID from the request context.
func TenantIDFromContext(ctx context.Context) string {
	v := ctx.Value(ctxKeyTenantID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// TraceIDFromContext extracts the trace ID from the request context.
func TraceIDFromContext(ctx context.Context) string {
	v := ctx.Value(ctxKeyTraceID)
	if v == nil {
		return ""
	}
	return v.(string)
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

// ─── ID generation ───────────────────────────────────────────────────────────

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
