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

func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(TenantIDKey).(string); ok {
		return v
	}
	return ""
}

func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

func GetRoles(ctx context.Context) []string {
	if v, ok := ctx.Value(RolesKey).([]string); ok {
		return v
	}
	return nil
}

func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(TraceIDKey).(string); ok {
		return v
	}
	return ""
}

func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}