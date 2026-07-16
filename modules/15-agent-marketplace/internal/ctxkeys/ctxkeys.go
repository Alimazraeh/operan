package ctxkeys

import "context"

type ctxKey string

const (
	TenantIDKey  ctxKey = "tenant_id"
	UserIDKey    ctxKey = "user_id"
	RolesKey     ctxKey = "roles"
	TraceIDKey   ctxKey = "trace_id"
	RequestIDKey ctxKey = "request_id"
)

// GetTenantID returns the tenant ID from the context.
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(TenantIDKey).(string); ok {
		return v
	}
	return ""
}

// GetUserID returns the user ID from the context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// GetRoles returns the roles from the context.
func GetRoles(ctx context.Context) []string {
	if v, ok := ctx.Value(RolesKey).([]string); ok {
		return v
	}
	return nil
}

// GetTraceID returns the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// GetRequestID returns the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}