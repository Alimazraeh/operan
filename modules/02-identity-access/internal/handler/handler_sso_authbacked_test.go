package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/middleware"
)

// ssoUniversalServer returns an httptest server whose responses satisfy every
// Authentik unmarshal target used by the SSO/SCIM handlers, plus a real client
// pointed at it.
func ssoAuthClient(t *testing.T) *authentik.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "setup_urls") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"authorization_url":"http://x/auth","token_url":"http://x/token","jwks_url":"http://x/jwks","issuer":"http://x"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"uuid":"id1","name":"n","pk":"pk1","client_id":"cid","client_secret":"sec","count":0,"next":"","results":[]}`))
	}))
	t.Cleanup(srv.Close)
	return authentik.NewClient(srv.URL, "tok")
}

func tenantReq(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, http.NoBody)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.Header.Set("X-Tenant-ID", "t1")
	return r.WithContext(context.WithValue(r.Context(), middleware.TenantIDKey, "t1"))
}

func TestSSOHandler_Configure_OAuth2(t *testing.T) {
	h := NewSSOHandler(ssoAuthClient(t), nil)
	body := `{"provider":"authentik","type":"oauth2","configuration":{"client_id":"abc","client_secret":"xyz"}}`
	rr := httptest.NewRecorder()
	h.Configure(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/configure", body))
	if rr.Code != http.StatusCreated {
		t.Errorf("Configure(oauth2) = %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSSOHandler_Configure_SAML(t *testing.T) {
	h := NewSSOHandler(ssoAuthClient(t), nil)
	body := `{"provider":"okta","type":"saml","configuration":{"metadata_url":"http://idp/meta"}}`
	rr := httptest.NewRecorder()
	h.Configure(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/configure", body))
	if rr.Code != http.StatusCreated {
		t.Errorf("Configure(saml) = %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSSOHandler_Configure_UnsupportedType(t *testing.T) {
	h := NewSSOHandler(ssoAuthClient(t), nil)
	body := `{"provider":"x","type":"ldap","configuration":{"a":"b"}}`
	rr := httptest.NewRecorder()
	h.Configure(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/configure", body))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Configure(unsupported) = %d, want 400", rr.Code)
	}
}

func TestSSOHandler_Configure_ValidationError(t *testing.T) {
	h := NewSSOHandler(ssoAuthClient(t), nil)
	// missing configuration
	body := `{"provider":"x","type":"oauth2"}`
	rr := httptest.NewRecorder()
	h.Configure(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/configure", body))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Configure(missing config) = %d, want 400", rr.Code)
	}
}

func TestSSOHandler_GetConfig_WithAuth(t *testing.T) {
	h := NewSSOHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	// No provider matches the operan-t1 prefix, so fetchSSOConfig returns nil -> 404.
	h.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/sso/config", ""))
	if rr.Code != http.StatusNotFound {
		t.Errorf("GetConfig(auth, no match) = %d, want 404", rr.Code)
	}
}

func TestSCIMHandler_Provision(t *testing.T) {
	h := NewSCIMHandler(ssoAuthClient(t), nil)
	body := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"alice@x.io","name":{"formatted":"Alice"},"emails":[{"value":"alice@x.io","primary":true}],"active":true}`
	rr := httptest.NewRecorder()
	h.Provision(rr, tenantReq(http.MethodPost, "/api/v1/iam/scim/users", body))
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Errorf("Provision() = %d, want 2xx. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSCIMHandler_ListUsers_WithAuth(t *testing.T) {
	h := NewSCIMHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.ListUsers(rr, tenantReq(http.MethodGet, "/api/v1/iam/scim/users", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("ListUsers() = %d, want 200", rr.Code)
	}
}

func TestSCIMHandler_UpdateAndDeleteUser(t *testing.T) {
	h := NewSCIMHandler(ssoAuthClient(t), nil)

	rr := httptest.NewRecorder()
	body := `{"op":"Replace","value":{"userName":"alice2@x.io","active":true}}`
	h.UpdateUser(rr, tenantReq(http.MethodPut, "/api/v1/iam/scim/users/u1", body))
	if rr.Code != http.StatusNoContent {
		t.Errorf("UpdateUser() = %d, want 204. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.DeleteUser(rr, tenantReq(http.MethodDelete, "/api/v1/iam/scim/users/u1", ""))
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("DeleteUser() = %d", rr.Code)
	}
}

func TestSCIMHandler_BulkProvision(t *testing.T) {
	h := NewSCIMHandler(ssoAuthClient(t), nil)
	body := `{
	  "Operations": [
	    {"method":"POST","path":"/api/v1/iam/scim/users","bulkId":"b1","data":{"userName":"new@x.io","name":{"formatted":"New"},"active":true}},
	    {"method":"PATCH","path":"/api/v1/iam/scim/users/u1","bulkId":"b2","data":{"active":false}},
	    {"method":"DELETE","path":"/api/v1/iam/scim/users/u1","bulkId":"b3"},
	    {"method":"POST","path":"/api/v1/iam/scim/users","bulkId":"b4"}
	  ],
	  "failOnError": false
	}`
	rr := httptest.NewRecorder()
	h.BulkProvision(rr, tenantReq(http.MethodPost, "/api/v1/iam/scim/Bulk", body))
	// Mixed success/failure across operations yields 200 (all ok) or 422 (some
	// failed); either way every bulk-op branch is exercised.
	if rr.Code != http.StatusOK && rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("BulkProvision() = %d, want 200 or 422. Body: %s", rr.Code, rr.Body.String())
	}
}
