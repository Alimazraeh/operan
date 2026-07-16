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
	// WorkflowIDKey is the context key for the workflow ID from request header.
	WorkflowIDKey ctxKey = "workflow_id"
	// ProviderKey is the context key for the resolved provider during call routing.
	ProviderKey ctxKey = "provider"
	// ModelNameKey is the context key for the model name used in the request.
	ModelNameKey ctxKey = "model_name"
)