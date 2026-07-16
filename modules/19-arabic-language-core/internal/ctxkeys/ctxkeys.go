package ctxkeys

// Context key types for the request pipeline.
type ctxKey string

const (
	// TenantIDKey is the context key for the parsed tenant identifier.
	TenantIDKey ctxKey = "tenant_id"
	// UserIDKey is the context key for the authenticated user/service ID.
	UserIDKey ctxKey = "user_id"
	// RolesKey is the context key for the authenticated principal's roles.
	RolesKey ctxKey = "roles"
)