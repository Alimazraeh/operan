// Package ctxkeys provides typed context keys for the Agent Registry module.
// This package is intentionally minimal (no external imports) to avoid
// circular dependencies between middleware, store, and events.
package ctxkeys

import "context"

// Key is a type alias for context keys.
type Key string

const (
	// TenantID is the context key for tenant ID.
	TenantID  Key = "tenant_id"
	// UserID is the context key for user ID.
	UserID Key = "user_id"
	// TraceID is the context key for trace ID.
	TraceID Key = "trace_id"
	// RequestID is the context key for request ID.
	RequestID Key = "request_id"
	// UserRole is the context key for user role.
	UserRole Key = "user_role"
)

// GetTenantID extracts tenant ID from context.
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(TenantID).(string); ok {
		return v
	}
	return ""
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserID).(string); ok {
		return v
	}
	return ""
}

// GetTraceID extracts trace ID from context.
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceID).(string); ok {
		return v
	}
	return ""
}

// GetRequestID extracts request ID from context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestID).(string); ok {
		return v
	}
	return ""
}

// GetUserRole extracts user role from context.
func GetUserRole(ctx context.Context) string {
	if v, ok := ctx.Value(UserRole).(string); ok {
		return v
	}
	return ""
}

// SetTenantID sets the tenant ID in context.
func SetTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantID, tenantID)
}

// SetUserID sets the user ID in context.
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserID, userID)
}

// SetUserRole sets the user role in context.
func SetUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, UserRole, role)
}

// SetTraceID sets the trace ID in context.
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceID, traceID)
}

// SetRequestID sets the request ID in context.
func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestID, requestID)
}
