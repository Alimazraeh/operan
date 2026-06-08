package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
)

// ldapSourceServer returns an Authentik client whose LDAP-sources list endpoint
// yields a single source with the given name; all other endpoints return 200.
func ldapSourceServer(t *testing.T, sourceName string) *authentik.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v3/sources/ldap/" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(fmt.Sprintf(
				`{"count":1,"next":"","results":[{"uuid":"src1","name":%q,"connected":true,"authentication":{"bind_dn":"cn=admin"},"ingestion":{}}]}`,
				sourceName)))
			return
		}
		_, _ = w.Write([]byte(`{"uuid":"src1","name":"x","connected":true,"authentication":{},"ingestion":{},"count":0,"next":"","results":[]}`))
	}))
	t.Cleanup(srv.Close)
	return authentik.NewClient(srv.URL, "tok")
}

// ─── AD handler ──────────────────────────────────────────────────────────────

func TestADHandler_AuthBacked(t *testing.T) {
	h := NewADHandler(ldapSourceServer(t, "operan-t1-ad-source"), nil)

	rr := httptest.NewRecorder()
	h.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/ad/config", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("GetConfig() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.Test(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/ad/test", `{"domain_controller":"dc1","bind_dn":"cn=a","bind_password":"p"}`))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Test() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.UpdateConfig(rr, tenantReq(http.MethodPatch, "/api/v1/iam/auth/ad/config", `{"display_name":"New AD"}`))
	if rr.Code != http.StatusOK {
		t.Errorf("UpdateConfig() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.DeleteConfig(rr, tenantReq(http.MethodDelete, "/api/v1/iam/auth/ad/config", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("DeleteConfig() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestADHandler_NotFound(t *testing.T) {
	// Source named for a different tenant -> handlers report not found.
	h := NewADHandler(ldapSourceServer(t, "operan-other-ad"), nil)

	rr := httptest.NewRecorder()
	h.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/ad/config", ""))
	if rr.Code != http.StatusNotFound {
		t.Errorf("GetConfig(no match) = %d, want 404", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.DeleteConfig(rr, tenantReq(http.MethodDelete, "/api/v1/iam/auth/ad/config", ""))
	if rr.Code != http.StatusNotFound {
		t.Errorf("DeleteConfig(no match) = %d, want 404", rr.Code)
	}
}

func TestADHandler_UpdateValidationError(t *testing.T) {
	h := NewADHandler(ldapSourceServer(t, "operan-t1-ad-source"), nil)
	rr := httptest.NewRecorder()
	// empty body -> Validate fails (no fields to update)
	h.UpdateConfig(rr, tenantReq(http.MethodPatch, "/api/v1/iam/auth/ad/config", `{}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("UpdateConfig(empty) = %d, want 400", rr.Code)
	}
}

// ─── LDAP handler ────────────────────────────────────────────────────────────

func TestLDAPHandler_AuthBacked(t *testing.T) {
	h := NewLDAPHandler(ldapSourceServer(t, "operan-t1-ldap-source"), nil)

	rr := httptest.NewRecorder()
	h.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/ldap/config", ""))
	if rr.Code != http.StatusOK {
		t.Errorf("GetConfig() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.Test(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/ldap/test", `{"url":"ldap://h","bind_dn":"cn=a","bind_password":"p"}`))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Test() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.UpdateConfig(rr, tenantReq(http.MethodPatch, "/api/v1/iam/auth/ldap/config", `{"display_name":"New LDAP"}`))
	if rr.Code != http.StatusOK {
		t.Errorf("UpdateConfig() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.DeleteConfig(rr, tenantReq(http.MethodDelete, "/api/v1/iam/auth/ldap/config", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("DeleteConfig() = %d, want 200/204. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestLDAPHandler_NotFound(t *testing.T) {
	h := NewLDAPHandler(ldapSourceServer(t, "operan-other-ldap"), nil)
	rr := httptest.NewRecorder()
	h.GetConfig(rr, tenantReq(http.MethodGet, "/api/v1/iam/auth/ldap/config", ""))
	if rr.Code != http.StatusNotFound {
		t.Errorf("GetConfig(no match) = %d, want 404", rr.Code)
	}
}
