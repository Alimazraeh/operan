package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// errAuthClient returns an Authentik client whose every call fails with 500,
// exercising the error-handling branches of the handlers.
func errAuthClient(t *testing.T) *authentik.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"boom"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return authentik.NewClient(srv.URL, "tok")
}

func TestHandlers_ErrorBranches(t *testing.T) {
	pub := events.NewPublisher("")

	// SSO Configure (OAuth2 + SAML) -> provider creation fails -> 500.
	sso := NewSSOHandler(errAuthClient(t), pub)
	for _, typ := range []string{"oauth2", "saml"} {
		rr := httptest.NewRecorder()
		body := `{"provider":"p","type":"` + typ + `","configuration":{"client_id":"a"}}`
		sso.Configure(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/configure", body))
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("SSO Configure(%s) error = %d, want 500", typ, rr.Code)
		}
	}

	// SCIM Provision -> user creation fails.
	scim := NewSCIMHandler(errAuthClient(t), pub)
	rr := httptest.NewRecorder()
	scim.Provision(rr, tenantReq(http.MethodPost, "/api/v1/iam/scim/users",
		`{"userName":"a@x.io","name":{"formatted":"A"},"emails":[{"value":"a@x.io"}],"active":true}`))
	if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
		t.Errorf("SCIM Provision error = %d, want non-2xx", rr.Code)
	}

	// Service identity Create -> application creation fails.
	svc := &ServiceIdentityHandler{Auth: errAuthClient(t), Store: store.NewServiceIdentityStore(), Publisher: pub}
	rr = httptest.NewRecorder()
	svc.Create(rr, userTenantReq(http.MethodPost, "/api/v1/iam/service-identities",
		`{"name":"svc","tenant_id":"t1","role_ids":["r"]}`))
	if rr.Code == http.StatusCreated {
		t.Errorf("ServiceIdentity Create error = %d, want non-201", rr.Code)
	}

	// Agent identity Register -> group/user creation fails.
	agent := &AgentIdentityHandler{Auth: errAuthClient(t), Store: store.NewAgentIdentityStore(), Publisher: pub}
	rr = httptest.NewRecorder()
	agent.Register(rr, userTenantReq(http.MethodPost, "/api/v1/iam/agent-identities",
		`{"agent_id":"a9","tenant_id":"t1","capabilities":["chat"]}`))
	if rr.Code == http.StatusCreated {
		t.Errorf("AgentIdentity Register error = %d, want non-201", rr.Code)
	}

	// MFA Enroll/RegenerateRecoveryCodes -> user lookup fails.
	mfa := NewMFAHandler(errAuthClient(t), pub)
	rr = httptest.NewRecorder()
	mfa.Enroll(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/enroll", `{"method":"totp"}`))
	if rr.Code == http.StatusOK {
		t.Errorf("MFA Enroll error = %d, want non-200", rr.Code)
	}
	rr = httptest.NewRecorder()
	mfa.RegenerateRecoveryCodes(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/recovery-codes", ""))
	if rr.Code == http.StatusOK {
		t.Errorf("MFA RegenerateRecoveryCodes error = %d, want non-200", rr.Code)
	}

	// AD/LDAP config handlers -> source list fails -> 500.
	ad := NewADHandler(errAuthClient(t), pub)
	rr = httptest.NewRecorder()
	ad.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/ad/config", ""))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("AD GetConfig error = %d, want 500", rr.Code)
	}
	ldap := NewLDAPHandler(errAuthClient(t), pub)
	rr = httptest.NewRecorder()
	ldap.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/ldap/config", ""))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("LDAP GetConfig error = %d, want 500", rr.Code)
	}

	// RBAC Evaluate -> user lookup fails -> denied (200 with denial).
	rbac := NewRBACHandler(errAuthClient(t))
	rr = httptest.NewRecorder()
	rbac.Evaluate(rr, userTenantReq(http.MethodPost, "/api/v1/iam/rbac/evaluate",
		`{"actor_id":"u1","resource":"user","action":"read"}`))
	if rr.Code != http.StatusOK {
		t.Errorf("RBAC Evaluate(user not found) = %d, want 200", rr.Code)
	}

	// Audit GetSessionReplay -> events list fails -> 500.
	audit := NewAuditHandler(errAuthClient(t))
	rr = httptest.NewRecorder()
	audit.GetSessionReplay(rr, tenantReq(http.MethodGet, "/api/v1/iam/audit/session-replay/s1", ""))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Audit GetSessionReplay error = %d, want 500", rr.Code)
	}
}
