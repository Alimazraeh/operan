package middleware

import (
	"context"
	"net/http"
	"strings"
)

// Keys for context values.
type contextKey string

const (
	TenantIDKey contextKey = "tenant_id"
	UserIDKey   contextKey = "user_id"
	TraceIDKey  contextKey = "trace_id"
)

// TenantInjector extracts the tenant ID from the X-Tenant-ID header and injects it into the context.
func TenantInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			// Default to a test tenant for development
			tenantID = "test-tenant-001"
		}

		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthValidator validates the Authorization header and extracts user ID.
func AuthValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// Skip auth for health check
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"invalid authorization scheme"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token == "" {
			http.Error(w, `{"error":"empty token"}`, http.StatusUnauthorized)
			return
		}

		// TODO: Actually validate the token and extract user ID
		// For now, use the token as the user ID for development
		ctx := context.WithValue(r.Context(), UserIDKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceInjector generates or extracts a trace ID for request tracing.
func TraceInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			// Generate a new trace ID
			traceID = generateTraceID()
		}

		ctx := context.WithValue(r.Context(), TraceIDKey, traceID)
		w.Header().Set("X-Trace-ID", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateTraceID creates a simple trace ID.
func generateTraceID() string {
	return "trace-" + generateID()
}

// generateID creates a simple random ID.
func generateID() string {
	// Use a simple counter-based approach for now
	// In production, use a proper UUID or ULID generator
	return "00000000-0000-0000-0000-000000000001"
}

// GetTenantID extracts the tenant ID from the context.
func GetTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

// GetUserID extracts the user ID from the context.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetTraceID extracts the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}
