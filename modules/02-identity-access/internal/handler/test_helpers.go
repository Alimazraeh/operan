package handler

import (
	"context"
	"net/http"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// setPrincipalInContext injects JWT principal info into the request context.
func setPrincipalInContext(r *http.Request, principal *middleware.JWTToken) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.UserIDKey, principal.Subject)
	ctx = context.WithValue(ctx, middleware.UserTypeKey, principal.UserType)
	ctx = context.WithValue(ctx, middleware.TenantIDKey, principal.TenantID)
	return r.WithContext(ctx)
}

// newTestRoleHandler creates a RoleHandler with nil Auth (uses in-memory store).
func newTestRoleHandler() *RoleHandler {
	return NewRoleHandler(nil, nil)
}

// newTestAuditHandler creates an AuditHandler with nil Auth.
func newTestAuditHandler() *AuditHandler {
	return NewAuditHandler(nil)
}

// newTestRBACHandler creates an RBACHandler with nil Auth.
func newTestRBACHandler() *RBACHandler {
	return NewRBACHandler(nil)
}

// newTestSSOHandler creates an SSOHandler with nil Auth.
func newTestSSOHandler() *SSOHandler {
	return NewSSOHandler(nil, nil)
}

// newTestSCIMHandler creates a SCIMHandler with nil Auth.
func newTestSCIMHandler() *SCIMHandler {
	return NewSCIMHandler(nil, nil)
}

// newTestMFAHandler creates an MFAHandler with nil Auth.
func newTestMFAHandler() *MFAHandler {
	return NewMFAHandler(nil, nil)
}

// newTestLDAPHandler creates an LDAPHandler with nil Auth.
func newTestLDAPHandler() *LDAPHandler {
	return NewLDAPHandler(nil, nil)
}

// newTestADHandler creates an ADHandler with nil Auth.
func newTestADHandler() *ADHandler {
	return NewADHandler(nil, nil)
}

// newTestDelegationHandler creates a DelegationHandler with nil Auth.
func newTestDelegationHandler() *DelegationHandler {
	return NewDelegationHandler(nil, nil)
}

// newTestABACHandler creates an ABACHandler with nil Auth.
func newTestABACHandler() *ABACHandler {
	return NewABACHandler(nil, nil, nil)
}

// NewTestUserHandler creates a UserHandler with nil Auth (uses in-memory store).
func NewTestUserHandler() *UserHandler {
	return NewUserHandler(nil, store.NewUserStore(), store.NewAuditStore(), events.NewPublisher(""))
}

// NewTestServiceIdentityHandler creates a ServiceIdentityHandler with nil Auth (uses in-memory store).
func NewTestServiceIdentityHandler() *ServiceIdentityHandler {
	return NewTestServiceIdentityHandlerRaw(nil, events.NewPublisher(""))
}

// NewTestServiceIdentityHandlerRaw creates a ServiceIdentityHandler for testing.
func NewTestServiceIdentityHandlerRaw(auth *authentik.Client, publisher *events.Publisher) *ServiceIdentityHandler {
	return &ServiceIdentityHandler{
		Auth:      auth,
		Store:     store.NewServiceIdentityStore(),
		Publisher: publisher,
	}
}

// NewTestAgentIdentityHandler creates an AgentIdentityHandler with nil Auth (uses in-memory store).
func NewTestAgentIdentityHandler() *AgentIdentityHandler {
	return &AgentIdentityHandler{
		Auth:      nil,
		Store:     store.NewAgentIdentityStore(),
		Publisher: events.NewPublisher(""),
	}
}
