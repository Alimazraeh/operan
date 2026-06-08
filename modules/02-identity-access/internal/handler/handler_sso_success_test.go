package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
)

// ssoScimSuccessClient serves an OAuth2 provider and real users for tenant t1,
// enabling the SSO/SCIM success paths to complete.
func ssoScimSuccessClient(t *testing.T) *authentik.Client {
	t.Helper()
	usersList := `{"count":2,"next":"","results":[` +
		`{"uuid":"u1","username":"alice","email":"alice@x.io","is_active":true,"attributes":{"external_id":"ext-1"}},` +
		`{"uuid":"u2","username":"bob","email":"bob@x.io","is_active":true}` +
		`]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		p := r.URL.Path
		switch {
		case p == "/api/v3/providers/oauth2/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"count":1,"next":"","results":[{"uuid":"o1","name":"operan-t1-oauth","client_id":"cid","client_secret":"sec"}]}`))
		case p == "/api/v3/core/users/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(usersList))
		case strings.HasPrefix(p, "/api/v3/core/users/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"uuid":"u1","username":"alice","email":"alice@x.io","is_active":true}`))
		case strings.Contains(p, "setup_urls"):
			_, _ = w.Write([]byte(`{"authorization_url":"http://x/a","token_url":"http://x/t"}`))
		default:
			_, _ = w.Write([]byte(`{"uuid":"u1","username":"alice","email":"alice@x.io","is_active":true,"count":0,"next":"","results":[]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return authentik.NewClient(srv.URL, "tok")
}

func TestSSOHandler_GetConfigAndTest_Success(t *testing.T) {
	h := NewSSOHandler(ssoScimSuccessClient(t), nil)

	rr := httptest.NewRecorder()
	h.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/sso/config", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("GetConfig() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "oauth2") {
		t.Errorf("GetConfig() should return the matched OAuth2 provider: %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.Test(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/test", `{"provider":"operan-t1-oauth"}`))
	if rr.Code != http.StatusOK {
		t.Errorf("Test() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSCIMHandler_ListUsers_BuildsResources(t *testing.T) {
	h := NewSCIMHandler(ssoScimSuccessClient(t), nil)

	// Plain list builds SCIMUser resources (scimUserFromAuthentik, emails).
	rr := httptest.NewRecorder()
	h.ListUsers(rr, tenantReq(http.MethodGet, "/api/v1/iam/scim/users", ""))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "alice@x.io") {
		t.Errorf("ListUsers() = %d, body: %s", rr.Code, rr.Body.String())
	}

	// Filtered + sorted list exercises matchesScimFilter and compareSCIMSort.
	rr = httptest.NewRecorder()
	h.ListUsers(rr, tenantReq(http.MethodGet, `/api/v1/iam/scim/users?filter=userName+co+"a"&sortBy=userName&sortOrder=ascending`, ""))
	if rr.Code != http.StatusOK {
		t.Errorf("ListUsers(filter) = %d", rr.Code)
	}
}

func TestSCIMHandler_UpdateDelete_ResolveSuccess(t *testing.T) {
	h := NewSCIMHandler(ssoScimSuccessClient(t), nil)

	// resolveScimUser via GetByID success, then applyReplaceAdd updates fields.
	rr := httptest.NewRecorder()
	h.UpdateUser(rr, tenantReq(http.MethodPut, "/api/v1/iam/scim/users/u1",
		`{"op":"Replace","value":{"userName":"alice2","active":false,"name":{"formatted":"Alice Two"}}}`))
	if rr.Code != http.StatusNoContent {
		t.Errorf("UpdateUser() = %d, want 204. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.DeleteUser(rr, tenantReq(http.MethodDelete, "/api/v1/iam/scim/users/u1", ""))
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
		t.Errorf("DeleteUser() = %d. Body: %s", rr.Code, rr.Body.String())
	}
}
