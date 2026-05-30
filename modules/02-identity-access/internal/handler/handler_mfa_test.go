package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/middleware"
)

func TestMFAHandlerEnrollMissingMethod(t *testing.T) {
	h := newTestMFAHandler()

	// Empty body — fails JSON decode before method check
	payload := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Enroll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Enroll() with missing method body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerEnrollEmptyMethod(t *testing.T) {
	h := newTestMFAHandler()

	payload := `{"method":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Enroll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Enroll() with empty method status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerEnrollAuthNil(t *testing.T) {
	h := newTestMFAHandler()

	payload := `{"method":"totp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Enroll(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Enroll() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestMFAHandlerEnrollMissingUserID(t *testing.T) {
	h := newTestMFAHandler()

	payload := `{"method":"totp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	// No principal set — user_id will be empty
	w := httptest.NewRecorder()
	h.Enroll(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Enroll() missing user_id status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerVerifyMissingCode(t *testing.T) {
	h := newTestMFAHandler()

	// Empty body — fails JSON decode
	payload := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/verify", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	w := httptest.NewRecorder()
	h.Verify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Verify() with missing code body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerVerifyMissingMethod(t *testing.T) {
	h := newTestMFAHandler()

	// Valid JSON but missing method
	payload := `{"code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/verify", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	w := httptest.NewRecorder()
	h.Verify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Verify() with missing method status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerVerifyAuthNil(t *testing.T) {
	h := newTestMFAHandler()

	payload := `{"method":"totp","code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/verify", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	w := httptest.NewRecorder()
	h.Verify(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Verify() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestMFAHandlerDisableMissingMethod(t *testing.T) {
	h := newTestMFAHandler()

	// Missing password in body
	payload := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/disable", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Disable(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Disable() with missing password status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerDisableAuthNil(t *testing.T) {
	h := newTestMFAHandler()

	payload := `{"password":"test-password"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/disable", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Disable(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Disable() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestMFAHandlerListDevicesAuthNil(t *testing.T) {
	h := newTestMFAHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/mfa/enrolled", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.ListDevices(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("ListDevices() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestMFAHandlerRegenerateRecoveryCodesAuthNil(t *testing.T) {
	h := newTestMFAHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/recovery-codes", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.RegenerateRecoveryCodes(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("RegenerateRecoveryCodes() with nil Auth status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestExtractMFAUserID(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "enroll path no user ID",
			path: "/api/v1/iam/mfa/enroll",
			want: "",
		},
		{
			name: "verify path no user ID",
			path: "/api/v1/iam/mfa/verify",
			want: "",
		},
		{
			name: "disabled path with UUID",
			path: "/api/v1/iam/mfa/device-abc-123",
			want: "", // not a UUID format (no curly braces)
		},
		{
			name: "disabled path with trailing slash",
			path: "/api/v1/iam/mfa/device-abc-123/",
			want: "", // not a UUID format (no curly braces)
		},
		{
			name: "recovery codes path",
			path: "/api/v1/iam/mfa/recovery-codes",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMFAUserID(tt.path)
			if got != tt.want {
				t.Errorf("extractMFAUserID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractQueryInt(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string][]string
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "valid value",
			params:     map[string][]string{"count": {"5"}},
			key:        "count",
			defaultVal: 10,
			want:       5,
		},
		{
			name:       "missing key",
			params:     map[string][]string{"other": {"3"}},
			key:        "count",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "invalid value",
			params:     map[string][]string{"count": {"abc"}},
			key:        "count",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "zero value",
			params:     map[string][]string{"count": {"0"}},
			key:        "count",
			defaultVal: 10,
			want:       0,
		},
		{
			name:       "empty string value",
			params:     map[string][]string{"count": {""}},
			key:        "count",
			defaultVal: 10,
			want:       10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractQueryInt(tt.params, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("extractQueryInt(%v, %q, %d) = %d, want %d",
					tt.params, tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

// Ensure context is used to prevent unused import.
var _ = context.Background
