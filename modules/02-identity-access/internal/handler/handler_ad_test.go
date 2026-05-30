package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/middleware"
)

func TestADHandlerConfigureMissingDomainController(t *testing.T) {
	h := newTestADHandler()

	// Missing domain_controller — fails validation
	payload := `{"display_name":"AD Config","domain_name":"example.com","bind_dn":"CN=admin,DC=example,DC=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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
		t.Errorf("Configure() with missing domain_controller status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestADHandlerConfigureMissingBindDN(t *testing.T) {
	h := newTestADHandler()

	// Missing bind_dn — fails validation
	payload := `{"display_name":"AD Config","domain_name":"example.com","domain_controller":"dc.example.com","bind_password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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

func TestADHandlerConfigureInvalidJSON(t *testing.T) {
	h := newTestADHandler()

	payload := `{"display_name":"AD Config"`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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

func TestADHandlerConfigureAuthNil(t *testing.T) {
	h := newTestADHandler()

	// Valid config but nil Auth
	payload := `{"display_name":"AD Config","domain_name":"example.com","domain_controller":"dc.example.com","bind_dn":"CN=admin,DC=example,DC=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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
		t.Errorf("Configure() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestADHandlerConfigureValidRequest(t *testing.T) {
	h := newTestADHandler()

	// Valid config with all required fields
	payload := `{"display_name":"AD Config","domain_name":"example.com","domain_controller":"dc.example.com","bind_dn":"CN=admin,DC=example,DC=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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
		t.Errorf("Configure() with valid request status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestADHandlerTestAuthNil(t *testing.T) {
	h := newTestADHandler()

	payload := `{"domain_controller":"dc.example.com","bind_dn":"CN=admin,DC=example,DC=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/test", strings.NewReader(payload))
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
		t.Errorf("Test() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestADHandlerGetConfigAuthNil(t *testing.T) {
	h := newTestADHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/ad/config", nil)
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

func TestADHandlerUpdateConfigMissingFields(t *testing.T) {
	h := newTestADHandler()

	// Empty body — fails validation (at least one field to update is required)
	payload := `{}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/auth/ad/config", strings.NewReader(payload))
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

func TestADHandlerUpdateConfigAuthNil(t *testing.T) {
	h := newTestADHandler()

	payload := `{"domain_controller":"dc2.example.com"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/auth/ad/config", strings.NewReader(payload))
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

	if w.Code != http.StatusInternalServerError {
		t.Errorf("UpdateConfig() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestADHandlerDeleteConfigAuthNil(t *testing.T) {
	h := newTestADHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/auth/ad/config", nil)
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

func TestExtractADSubRoute(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "configure sub-route",
			path: "/api/v1/iam/auth/ad/configure",
			want: "/configure",
		},
		{
			name: "config sub-route",
			path: "/api/v1/iam/auth/ad/config",
			want: "/config",
		},
		{
			name: "test sub-route",
			path: "/api/v1/iam/auth/ad/test",
			want: "/test",
		},
		{
			name: "root sub-route",
			path: "/api/v1/iam/auth/ad/",
			want: "", // trailing slash trimmed, equals prefix length
		},
		{
			name: "empty path",
			path: "/api/v1/iam/auth/ad",
			want: "",
		},
		{
			name: "short path",
			path: "/api/v1/iam/auth/",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractADSubRoute(tt.path)
			if got != tt.want {
				t.Errorf("extractADSubRoute(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestADHandlerConfigureMissingDisplayName(t *testing.T) {
	h := newTestADHandler()

	// Missing display_name — fails validation
	payload := `{"domain_name":"example.com","domain_controller":"dc.example.com","bind_dn":"CN=admin,DC=example,DC=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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

func TestADHandlerConfigureMissingDomainName(t *testing.T) {
	h := newTestADHandler()

	// Missing domain_name — fails validation
	payload := `{"display_name":"AD Config","domain_controller":"dc.example.com","bind_dn":"CN=admin,DC=example,DC=com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/configure", strings.NewReader(payload))
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
		t.Errorf("Configure() with missing domain_name status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestADHandlerTestInvalidJSON(t *testing.T) {
	h := newTestADHandler()

	payload := `{"domain_controller":"dc.example.com"`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/auth/ad/test", strings.NewReader(payload))
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
