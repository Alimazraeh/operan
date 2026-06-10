// Package ctxkeys defines typed context keys for Module 07 (Tool Execution).
// Using typed keys prevents collisions and makes context value access explicit.
package ctxkeys

import "context"

// Key is a typed context key.
type Key string

const (
	// TenantID is the context key for the tenant ID (from JWT + X-Tenant-ID).
	TenantID Key = "tenant_id"
	// UserID is the context key for the authenticated user ID.
	UserID Key = "user_id"
	// UserRoles is the context key for the authenticated user's roles.
	UserRoles Key = "user_roles"
	// TraceID is the context key for the request trace ID.
	TraceID Key = "trace_id"
	// RequestID is the context key for the request ID.
	RequestID Key = "request_id"
)

// TenantIDFrom extracts the tenant ID from context.
func TenantIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(TenantID).(string); ok {
		return v
	}
	return ""
}

// UserIDFrom extracts the user ID from context.
func UserIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(UserID).(string); ok {
		return v
	}
	return ""
}

// UserRolesFrom extracts the user roles from context.
func UserRolesFrom(ctx context.Context) []string {
	if v, ok := ctx.Value(UserRoles).([]string); ok {
		return v
	}
	return nil
}

// TraceIDFrom extracts the trace ID from context.
func TraceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(TraceID).(string); ok {
		return v
	}
	return ""
}

// RequestIDFrom extracts the request ID from context.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(RequestID).(string); ok {
		return v
	}
	return ""
}

// WithTenantID returns a new context with the tenant ID set.
func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TenantID, id)
}

// WithUserID returns a new context with the user ID set.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserID, id)
}

// WithTraceID returns a new context with the trace ID set.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TraceID, id)
}

// WithRequestID returns a new context with the request ID set.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestID, id)
}
