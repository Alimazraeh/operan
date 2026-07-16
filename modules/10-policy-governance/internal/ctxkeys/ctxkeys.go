package ctxkeys

import "context"

// Context key types to avoid collisions.
type ctxKey string

const (
	// TenantIDKey is the context key for the tenant ID extracted from JWT.
	TenantIDKey ctxKey = "tenant_id"
	// UserIDKey is the context key for the user/agent principal.
	UserIDKey ctxKey = "user_id"
	// RolesKey is the context key for the user's roles.
	RolesKey ctxKey = "roles"
	// TraceIDKey is the context key for the request trace ID.
	TraceIDKey ctxKey = "trace_id"
	// RequestIDKey is the context key for the request correlation ID.
	RequestIDKey ctxKey = "request_id"
	// AgentIDKey is the context key for the agent executing a tool.
	AgentIDKey ctxKey = "agent_id"
	// DepartmentIDKey is the context key for the department ID.
	DepartmentIDKey ctxKey = "department_id"
)

// GetTenantID returns the tenant ID from context, or empty string.
func GetTenantID(ctx context.Context) string {
	if v := ctx.Value(TenantIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetUserID returns the user ID from context, or empty string.
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetRoles returns the roles slice from context, or nil.
func GetRoles(ctx context.Context) []string {
	if v := ctx.Value(RolesKey); v != nil {
		if r, ok := v.([]string); ok {
			return r
		}
	}
	return nil
}

// GetTraceID returns the trace ID from context, or empty string.
func GetTraceID(ctx context.Context) string {
	if v := ctx.Value(TraceIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetRequestID returns the request correlation ID from context, or empty string.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetAgentID returns the agent ID from context, or empty string.
func GetAgentID(ctx context.Context) string {
	if v := ctx.Value(AgentIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetDepartmentID returns the department ID from context, or empty string.
func GetDepartmentID(ctx context.Context) string {
	if v := ctx.Value(DepartmentIDKey); v != nil {
		return v.(string)
	}
	return ""
}