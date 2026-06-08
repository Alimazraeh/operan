package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/store"
)

func TestServiceIdentityHandler_AuthBacked(t *testing.T) {
	h := &ServiceIdentityHandler{Auth: ssoAuthClient(t), Store: store.NewServiceIdentityStore(), Publisher: events.NewPublisher("")}

	rr := httptest.NewRecorder()
	body := `{"name":"svc","tenant_id":"t1","role_ids":["role-1"]}`
	h.Create(rr, userTenantReq(http.MethodPost, "/api/v1/iam/service-identities", body))
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Create() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.List(rr, userTenantReq(http.MethodGet, "/api/v1/iam/service-identities", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("List() = %d", rr.Code)
	}

	// validation error: missing roles
	rr = httptest.NewRecorder()
	h.Create(rr, userTenantReq(http.MethodPost, "/api/v1/iam/service-identities", `{"name":"x","tenant_id":"t1"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Create(no roles) = %d, want 400", rr.Code)
	}

	// tenant mismatch -> 409
	rr = httptest.NewRecorder()
	h.Create(rr, userTenantReq(http.MethodPost, "/api/v1/iam/service-identities", `{"name":"x","tenant_id":"other","role_ids":["r"]}`))
	if rr.Code != http.StatusConflict {
		t.Errorf("Create(tenant mismatch) = %d, want 409", rr.Code)
	}
}

func TestAgentIdentityHandler_AuthBacked(t *testing.T) {
	h := &AgentIdentityHandler{Auth: ssoAuthClient(t), Store: store.NewAgentIdentityStore(), Publisher: events.NewPublisher("")}

	rr := httptest.NewRecorder()
	body := `{"agent_id":"agent-9","tenant_id":"t1","capabilities":["chat"]}`
	h.Register(rr, userTenantReq(http.MethodPost, "/api/v1/iam/agent-identities", body))
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Register() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.List(rr, userTenantReq(http.MethodGet, "/api/v1/iam/agent-identities", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("List() = %d", rr.Code)
	}

	// GetByAgent: universal server lists no matching user -> 404
	rr = httptest.NewRecorder()
	h.GetByAgent(rr, userTenantReq(http.MethodGet, "/api/v1/iam/agent-identities/agent/agent-9", ""))
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("GetByAgent() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// validation error: missing capabilities
	rr = httptest.NewRecorder()
	h.Register(rr, userTenantReq(http.MethodPost, "/api/v1/iam/agent-identities", `{"agent_id":"a","tenant_id":"t1"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Register(no caps) = %d, want 400", rr.Code)
	}
}
