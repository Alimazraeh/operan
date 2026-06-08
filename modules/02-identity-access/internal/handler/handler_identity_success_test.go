package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// identitySuccessClient serves Authentik responses that let the identity
// handlers complete their full success paths for tenant "t1".
func identitySuccessClient(t *testing.T) *authentik.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		p := r.URL.Path
		switch {
		case p == "/api/v3/core/applications/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"count":1,"next":"","results":[{"uuid":"app1","name":"operan-service-t1-svc"}]}`))
		case strings.HasPrefix(p, "/api/v3/core/applications/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"uuid":"app1","name":"operan-service-t1-svc"}`))
		case p == "/api/v3/core/users/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"count":1,"next":"","results":[{"uuid":"usr1","username":"agent-t1-agent-9"}]}`))
		case p == "/api/v3/core/groups/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"count":1,"next":"","results":[{"uuid":"grp1","name":"operan-agents-t1","users":["usr1"]}]}`))
		case strings.HasPrefix(p, "/api/v3/core/groups/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"uuid":"grp1","name":"operan-agents-t1","users":["usr1"]}`))
		default:
			_, _ = w.Write([]byte(`{"uuid":"id1","name":"n","pk":"pk1","count":0,"next":"","results":[]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return authentik.NewClient(srv.URL, "tok")
}

func TestServiceIdentityHandler_ListAndGet_Success(t *testing.T) {
	h := &ServiceIdentityHandler{Auth: identitySuccessClient(t), Store: store.NewServiceIdentityStore(), Publisher: events.NewPublisher("")}

	rr := httptest.NewRecorder()
	h.List(rr, userTenantReq(http.MethodGet, "/api/v1/iam/service-identities?page=1&page_size=10", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("List() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "operan-service-t1-svc") {
		t.Errorf("List() should include the matching identity: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.GetByID(rr, userTenantReq(http.MethodGet, "/api/v1/iam/service-identities/app1", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("GetByID() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestAgentIdentityHandler_GetByAgent_Success(t *testing.T) {
	h := &AgentIdentityHandler{Auth: identitySuccessClient(t), Store: store.NewAgentIdentityStore(), Publisher: events.NewPublisher("")}

	rr := httptest.NewRecorder()
	h.GetByAgent(rr, userTenantReq(http.MethodGet, "/api/v1/iam/agent-identities/agent/agent-9", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("GetByAgent() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.List(rr, userTenantReq(http.MethodGet, "/api/v1/iam/agent-identities", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("List() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}
