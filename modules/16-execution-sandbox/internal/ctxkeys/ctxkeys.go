package ctxkeys

import (
	"context"
)

// Context key types for values stored in request context.
type ctxKey string

const (
	// TenantIDKey is the context key for the resolved tenant ID.
	TenantIDKey ctxKey = "tenant_id"
	// UserIDKey is the context key for the resolved user ID / subject.
	UserIDKey ctxKey = "user_id"
	// RolesKey is the context key for the resolved user roles.
	RolesKey ctxKey = "roles"
	// TraceIDKey is the context key for the trace ID.
	TraceIDKey ctxKey = "trace_id"
	// RequestIDKey is the context key for the request ID.
	RequestIDKey ctxKey = "request_id"
)

// GetTenantID returns the tenant ID from context.
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(TenantIDKey).(string); ok {
		return v
	}
	return ""
}

// GetUserID returns the user ID from context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// GetRoles returns the roles from context.
func GetRoles(ctx context.Context) []string {
	if v, ok := ctx.Value(RolesKey).([]string); ok {
		return v
	}
	return nil
}

// GetTraceID returns the trace ID from context.
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// GetRequestID returns the request ID from context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}