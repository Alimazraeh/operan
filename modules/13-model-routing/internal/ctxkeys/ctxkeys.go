package ctxkeys

// Context keys for the request context.
type ctxKey string

const (
	// TenantIDKey is the context key for the resolved tenant identifier.
	TenantIDKey ctxKey = "tenant_id"
	// PrincipalKey is the context key for the JWT principal claim.
	PrincipalKey ctxKey = "principal"
)