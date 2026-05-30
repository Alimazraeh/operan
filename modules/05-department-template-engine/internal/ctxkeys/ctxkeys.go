// Package ctxkeys defines typed context keys for Module 05.
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
	v := ctx.Value(TenantID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// UserIDFrom extracts the user ID from context.
func UserIDFrom(ctx context.Context) string {
	v := ctx.Value(UserID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// UserRolesFrom extracts the user roles from context.
func UserRolesFrom(ctx context.Context) []string {
	v := ctx.Value(UserRoles)
	if v == nil {
		return nil
	}
	roles, ok := v.([]string)
	if !ok {
		return nil
	}
	return roles
}

// TraceIDFrom extracts the trace ID from context.
func TraceIDFrom(ctx context.Context) string {
	v := ctx.Value(TraceID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// RequestIDFrom extracts the request ID from context.
func RequestIDFrom(ctx context.Context) string {
	v := ctx.Value(RequestID)
	if v == nil {
		return ""
	}
	return v.(string)
}

// WithTenantID returns a new context with the tenant ID set.
func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TenantID, id)
}

// WithUserID returns a new context with the user ID set.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserID, id)
}

// WithUserRoles returns a new context with the user roles set.
func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, UserRoles, roles)
}

// WithTraceID returns a new context with the trace ID set.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, TraceID, id)
}

// WithRequestID returns a new context with the request ID set.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestID, id)
}
