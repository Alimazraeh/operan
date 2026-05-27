package models

import "time"

// User represents a human user within the Operan platform.
type User struct {
	ID                  string     `json:"id" db:"id"`
	TenantID            string     `json:"tenant_id" db:"tenant_id"`
	Email               string     `json:"email" db:"email"`
	DisplayName         string     `json:"display_name" db:"display_name"`
	Status              string     `json:"status" db:"status"`
	Roles               []string   `json:"roles" db:"-"`
	MFARolesJSON        string     `json:"-" db:"roles_json"`
	MFAEnabled          bool       `json:"mfa_enabled" db:"mfa_enabled"`
	LDAPDN              *string    `json:"ldap_dn,omitempty" db:"ldap_dn"`
	AuthenticationMethod string    `json:"-" db:"-"` // derived, not stored
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"-" db:"updated_at"`
	LastLoginAt         *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
}

// CreateUserRequest represents the request body for creating a user.
type CreateUserRequest struct {
	Email        string   `json:"email"`
	DisplayName  string   `json:"display_name"`
	Roles        []string `json:"roles,omitempty"`
	MFAEnabled   *bool    `json:"mfa_enabled,omitempty"`
	LDAPDN       *string  `json:"ldap_dn,omitempty"`
}

// UpdateUserRequest represents the request body for updating a user.
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	MFAEnabled  *bool   `json:"mfa_enabled,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// Validate checks that the update request has at least one field.
func (r *UpdateUserRequest) Validate() error {
	if r.DisplayName == nil && len(r.Roles) == 0 && r.MFAEnabled == nil && r.Status == nil {
		return &ValidationError{"no fields to update"}
	}
	return nil
}

// IsActive returns true if the user is active.
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// Role represents a named collection of permissions.
type Role struct {
	ID              string    `json:"id" db:"id"`
	TenantID        string    `json:"tenant_id" db:"tenant_id"`
	Name            string    `json:"name" db:"name"`
	Description     string    `json:"description" db:"description"`
	Permissions     []string  `json:"permissions" db:"-"`
	PermissionsJSON string    `json:"-" db:"permissions_json"`
	IsSystem        bool      `json:"is_system" db:"is_system"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// CreateRoleRequest represents the request body for creating a role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
	IsSystem    *bool    `json:"is_system,omitempty"`
}

// Validate checks that the create role request is valid.
func (r *CreateRoleRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{"role name is required"}
	}
	if len(r.Permissions) == 0 {
		return &ValidationError{"at least one permission is required"}
	}
	return nil
}

// ServiceIdentity represents a non-human identity for services.
type ServiceIdentity struct {
	ID          string    `json:"id" db:"id"`
	TenantID    string    `json:"tenant_id" db:"tenant_id"`
	Name        string    `json:"name" db:"name"`
	Roles       []string  `json:"roles" db:"-"`
	RolesJSON   string    `json:"-" db:"roles_json"`
	APIKeyID    string    `json:"api_key_id" db:"api_key_id"`
	Metadata    string    `json:"-" db:"metadata_json"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
}

// CreateServiceIdentityRequest represents the request body for creating a service identity.
type CreateServiceIdentityRequest struct {
	Name     string   `json:"name"`
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
	Metadata *string  `json:"metadata,omitempty"`
}

// Validate checks that the create service identity request is valid.
func (r *CreateServiceIdentityRequest) Validate() error {
	if r.Name == "" {
		return &ValidationError{"service identity name is required"}
	}
	if r.TenantID == "" {
		return &ValidationError{"tenant_id is required"}
	}
	if len(r.Roles) == 0 {
		return &ValidationError{"at least one role is required"}
	}
	return nil
}

// AgentIdentity represents an autonomous agent identity.
type AgentIdentity struct {
	ID                string    `json:"id" db:"id"`
	TenantID          string    `json:"tenant_id" db:"tenant_id"`
	AgentID           string    `json:"agent_id" db:"agent_id"`
	Capabilities      []string  `json:"capabilities" db:"-"`
	CapabilitiesJSON  string    `json:"-" db:"capabilities_json"`
	MemoryScope       []string  `json:"memory_scope" db:"-"`
	MemoryScopeJSON   string    `json:"-" db:"memory_scope_json"`
	AllowedTools      []string  `json:"allowed_tools" db:"-"`
	AllowedToolsJSON  string    `json:"-" db:"allowed_tools_json"`
	EscalationTargets []string  `json:"escalation_targets" db:"-"`
	EscalationTargetsJSON string `json:"-" db:"escalation_targets_json"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// RegisterAgentIdentityRequest represents the request body for registering an agent identity.
type RegisterAgentIdentityRequest struct {
	AgentID           string   `json:"agent_id"`
	TenantID          string   `json:"tenant_id"`
	Capabilities      []string `json:"capabilities"`
	MemoryScope       []string `json:"memory_scope,omitempty"`
	AllowedTools      []string `json:"allowed_tools,omitempty"`
	EscalationTargets []string `json:"escalation_targets,omitempty"`
}

// Validate checks that the register agent identity request is valid.
func (r *RegisterAgentIdentityRequest) Validate() error {
	if r.AgentID == "" {
		return &ValidationError{"agent_id is required"}
	}
	if r.TenantID == "" {
		return &ValidationError{"tenant_id is required"}
	}
	if len(r.Capabilities) == 0 {
		return &ValidationError{"at least one capability is required"}
	}
	return nil
}

// SSOConfig represents a single sign-on configuration.
type SSOConfig struct {
	ID            string                 `json:"id" db:"id"`
	TenantID      string                 `json:"tenant_id" db:"tenant_id"`
	Provider      string                 `json:"provider" db:"provider"`
	Type          string                 `json:"type" db:"type"`
	Configuration map[string]interface{} `json:"configuration" db:"-"`
	ConfigJSON    string                 `json:"-" db:"config_json"`
	Status        string                 `json:"status" db:"status"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`
}

// ConfigureSSORequest represents the request body for configuring SSO.
type ConfigureSSORequest struct {
	Provider      string                 `json:"provider"`
	Type          string                 `json:"type"`
	Configuration map[string]interface{} `json:"configuration"`
}

// Validate checks that the SSO configuration request is valid.
func (r *ConfigureSSORequest) Validate() error {
	if r.Provider == "" {
		return &ValidationError{"provider is required"}
	}
	if r.Type == "" {
		return &ValidationError{"type (saml/oidc) is required"}
	}
	if r.Configuration == nil {
		return &ValidationError{"configuration is required"}
	}
	return nil
}

// AuditEvent represents an auditable event in the platform.
type AuditEvent struct {
	ID           string                 `json:"id" db:"id"`
	TenantID     string                 `json:"tenant_id" db:"tenant_id"`
	ActorID      string                 `json:"actor_id" db:"actor_id"`
	ActorType    string                 `json:"actor_type" db:"actor_type"`
	Action       string                 `json:"action" db:"action"`
	ResourceType string                 `json:"resource_type" db:"resource_type"`
	ResourceID   string                 `json:"resource_id" db:"resource_id"`
	Result       string                 `json:"result" db:"result"`
	Details      map[string]interface{} `json:"details,omitempty" db:"-"`
	DetailsJSON  string                 `json:"-" db:"details_json"`
	IPAddress    string                 `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    string                 `json:"user_agent,omitempty" db:"user_agent"`
	Timestamp    time.Time              `json:"timestamp" db:"timestamp"`
}

// GetAuditTrailsRequest represents the query parameters for listing audit trails.
type GetAuditTrailsRequest struct {
	TenantID string
	ActorID  string
	Action   string
	From     *string
	To       *string
	Limit    int
	Offset   int
}

// SetRolesRequest represents the request body for setting user roles.
type SetRolesRequest struct {
	Roles []string `json:"roles"`
}

// Validate checks that the set roles request is valid.
func (r *SetRolesRequest) Validate() error {
	if len(r.Roles) == 0 {
		return &ValidationError{"at least one role is required"}
	}
	return nil
}

// PermissionCheckRequest represents the request body for RBAC/ABAC evaluation.
type PermissionCheckRequest struct {
	ActorID   string                 `json:"actor_id"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Validate checks that the permission check request is valid.
func (r *PermissionCheckRequest) Validate() error {
	if r.ActorID == "" {
		return &ValidationError{"actor_id is required"}
	}
	if r.Action == "" {
		return &ValidationError{"action is required"}
	}
	if r.Resource == "" {
		return &ValidationError{"resource is required"}
	}
	return nil
}

// PermissionCheckResult represents the result of a permission evaluation.
type PermissionCheckResult struct {
	Allowed      bool   `json:"allowed"`
	Reason       string `json:"reason"`
	PolicyMatch  *string `json:"policy_match,omitempty"`
	EvaluatedAt  string  `json:"evaluated_at"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
