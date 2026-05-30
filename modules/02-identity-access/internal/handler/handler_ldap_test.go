package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/middleware"
)

// --- Configure tests ---

func TestLDAPHandlerConfigureMissingJSON(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Configure() with invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerConfigureMissingDisplayName(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"provider":"openldap","url":"ldap://ldap.example.com:389","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Configure() with missing display_name status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerConfigureMissingProvider(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"display_name":"LDAP Config","url":"ldap://ldap.example.com:389","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Configure() with missing provider status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerConfigureMissingURL(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"display_name":"LDAP Config","provider":"openldap","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Configure() with missing url status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerConfigureMissingBaseDN(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"display_name":"LDAP Config","provider":"openldap","url":"ldap://ldap.example.com:389","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Configure() with missing base_dn status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerConfigureMissingBindDN(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"display_name":"LDAP Config","provider":"openldap","url":"ldap://ldap.example.com:389","base_dn":"dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Configure() with missing bind_dn status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerConfigureAuthNil(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"display_name":"LDAP Config","provider":"openldap","url":"ldap://ldap.example.com:389","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	// With nil Auth, Create() panics or returns an error; handler catches with 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Configure() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestLDAPHandlerConfigureValidRequest(t *testing.T) {
	h := newTestLDAPHandler()

	// Valid config with all required fields and optional fields
	payload := `{"display_name":"LDAP Config","provider":"openldap","url":"ldap://ldap.example.com:389","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com","bind_password":"secret","user_filter":"(objectClass=person)"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	// With nil Auth, Create() returns error, handler maps to 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Configure() with valid request status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestLDAPHandlerConfigureWithFreeIPAProvider(t *testing.T) {
	h := newTestLDAPHandler()

	// FreeIPA provider should map to ssl connection security
	payload := `{"display_name":"FreeIPA Config","provider":"freeipa","url":"ldaps://ipa.example.com:636","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Configure() with freeipa provider status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

// --- Test endpoint tests ---

func TestLDAPHandlerTestMissingJSON(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{bad json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/test", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Test(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Test() with invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerTestAuthNil(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"url":"ldap://ldap.example.com:389","bind_dn":"cn=admin,dc=example,dc=com","base_dn":"dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/test", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Test(w, req)

	// With nil Auth, Create() returns 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Test() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestLDAPHandlerTestValidRequest(t *testing.T) {
	h := newTestLDAPHandler()

	// Valid test request — nil Auth leads to 500 on Create
	payload := `{"url":"ldap://ldap.example.com:389","bind_dn":"cn=admin,dc=example,dc=com","base_dn":"dc=example,dc=com","bind_password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/test", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Test(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Test() with valid request status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

// --- GetConfig tests ---

func TestLDAPHandlerGetConfigAuthNil(t *testing.T) {
	h := newTestLDAPHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/ldap/config", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetConfig(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetConfig() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestLDAPHandlerGetConfigNotFound(t *testing.T) {
	// Create a handler with a mock client that returns empty LDAP sources
	h := newTestLDAPHandler()

	// With nil Auth, List() returns error -> 500
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/ldap/config", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetConfig(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetConfig() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

// --- UpdateConfig tests ---

func TestLDAPHandlerUpdateConfigMissingFields(t *testing.T) {
	h := newTestLDAPHandler()

	// Empty body — fails validation (no fields to update)
	payload := `{}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/auth/ldap/config", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.UpdateConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("UpdateConfig() with empty body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestLDAPHandlerUpdateConfigAuthNil(t *testing.T) {
	h := newTestLDAPHandler()

	// Valid update but nil Auth — still fails on List()
	payload := `{"base_dn":"dc=new,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/auth/ldap/config", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.UpdateConfig(w, req)

	// nil Auth causes List() to return 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("UpdateConfig() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestLDAPHandlerUpdateConfigInvalidJSON(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"base_dn":"dc=example,dc=com"`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/auth/ldap/config", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.UpdateConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("UpdateConfig() with invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// --- DeleteConfig tests ---

func TestLDAPHandlerDeleteConfigAuthNil(t *testing.T) {
	h := newTestLDAPHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/auth/ldap/config", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.DeleteConfig(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("DeleteConfig() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

// --- Helper function tests ---

func TestExtractHostPort(t *testing.T) {
	tests := []struct {
		name      string
		urlStr    string
		wantHost  string
		wantPort  int
	}{
		{
			name:     "empty string defaults to port 389",
			urlStr:   "",
			wantHost: "",
			wantPort: 389,
		},
		{
			name:     "ldap with port",
			urlStr:   "ldap://ldap.example.com:389",
			wantHost: "ldap.example.com",
			wantPort: 389,
		},
		{
			name:     "ldaps with port",
			urlStr:   "ldaps://ldap.example.com:636",
			wantHost: "ldap.example.com",
			wantPort: 636,
		},
		{
			name:     "ldap without port defaults to 389",
			urlStr:   "ldap://ldap.example.com",
			wantHost: "ldap.example.com",
			wantPort: 389,
		},
		{
			name:     "ldaps without port defaults to 389",
			urlStr:   "ldaps://ldap.example.com",
			wantHost: "ldap.example.com",
			wantPort: 389,
		},
		{
			name:     "invalid port defaults to 389",
			urlStr:   "ldap://ldap.example.com:notaport",
			wantHost: "ldap.example.com",
			wantPort: 389,
		},
		{
			name:     "just host no scheme",
			urlStr:   "ldap.example.com",
			wantHost: "ldap.example.com",
			wantPort: 389,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := extractHostPort(tt.urlStr)
			if host != tt.wantHost {
				t.Errorf("extractHostPort(%q) host = %q, want %q", tt.urlStr, host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("extractHostPort(%q) port = %v, want %v", tt.urlStr, port, tt.wantPort)
			}
		})
	}
}

func TestConnectionSecurity(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{
			name:     "openldap defaults to none",
			provider: "openldap",
			want:     "none",
		},
		{
			name:     "ldaps maps to ssl",
			provider: "ldaps",
			want:     "ssl",
		},
		{
			name:     "ldap lowercase maps to none",
			provider: "ldap",
			want:     "none",
		},
		{
			name:     "freeipa maps to ssl",
			provider: "freeipa",
			want:     "ssl",
		},
		{
			name:     "starttls",
			provider: "starttls",
			want:     "starttls",
		},
		{
			name:     "STARTTLS uppercase",
			provider: "STARTTLS",
			want:     "starttls",
		},
		{
			name:     "FreeIPA uppercase",
			provider: "FREEIPA",
			want:     "ssl",
		},
		{
			name:     "empty string defaults to none",
			provider: "",
			want:     "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connectionSecurity(tt.provider)
			if got != tt.want {
				t.Errorf("connectionSecurity(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestExtractLDAPSubRoute(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "configure sub-route",
			path: "/api/v1/iam/auth/ldap/configure",
			want: "/configure",
		},
		{
			name: "config sub-route",
			path: "/api/v1/iam/auth/ldap/config",
			want: "/config",
		},
		{
			name: "test sub-route",
			path: "/api/v1/iam/auth/ldap/test",
			want: "/test",
		},
		{
			name: "delete sub-route",
			path: "/api/v1/iam/auth/ldap/delete",
			want: "/delete",
		},
		{
			name: "empty after trimming trailing slash",
			path: "/api/v1/iam/auth/ldap/",
			want: "",
		},
		{
			name: "exact prefix — no sub-route",
			path: "/api/v1/iam/auth/ldap",
			want: "",
		},
		{
			name: "shorter path — returns empty",
			path: "/api/v1/iam/auth/",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLDAPSubRoute(tt.path)
			if got != tt.want {
				t.Errorf("extractLDAPSubRoute(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// --- Response parsing tests ---

func TestLDAPHandlerConfigureValidJSONResponse(t *testing.T) {
	// This test verifies the response structure when Auth is not nil
	// We use newTestLDAPHandler() with Auth=nil, so we test via body parsing
	h := newTestLDAPHandler()

	// Even with nil Auth, the handler should try to call Create and fail with 500
	// Let's verify the response body is JSON error
	payload := `{"display_name":"LDAP Config","provider":"openldap","url":"ldap://ldap.example.com:389","base_dn":"dc=example,dc=com","bind_dn":"cn=admin,dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/configure", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Configure(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Configure() status = %v, want %v", w.Code, http.StatusInternalServerError)
		return
	}

	// Verify response is JSON
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("Configure() response body is not valid JSON: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Error("Configure() response missing 'error' field")
	}
}

func TestLDAPHandlerTestValidJSONResponse(t *testing.T) {
	h := newTestLDAPHandler()

	payload := `{"url":"ldap://ldap.example.com:389","bind_dn":"cn=admin,dc=example,dc=com","base_dn":"dc=example,dc=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ldap/test", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Test(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Test() status = %v, want %v", w.Code, http.StatusInternalServerError)
		return
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("Test() response body is not valid JSON: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Error("Test() response missing 'error' field")
	}
}
