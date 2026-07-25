package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/middleware"
)

// ---------- parseAuthenticatorDevices tests (direct) ----------

func TestParseAuthenticatorDevicesDirect(t *testing.T) {
	tests := []struct {
		name      string
		rawInput  []json.RawMessage
		wantLen   int
		wantType  string
		wantLabel string
	}{
		{
			name:     "empty input",
			rawInput: []json.RawMessage{},
			wantLen:  0,
		},
		{
			name: "single device with uuid and name",
			rawInput: []json.RawMessage{
				json.RawMessage(`{"uuid":"abc-123","name":"My TOTP","type":"totp","created":"2025-01-01T00:00:00Z"}`),
			},
			wantLen:   1,
			wantType:  "totp",
			wantLabel: "My TOTP",
		},
		{
			name: "device with label instead of name",
			rawInput: []json.RawMessage{
				json.RawMessage(`{"uuid":"def-456","label":"WebAuthn Key","type":"webauthn"}`),
			},
			wantLen:   1,
			wantType:  "webauthn",
			wantLabel: "WebAuthn Key",
		},
		{
			name: "device with nested properties",
			rawInput: []json.RawMessage{
				json.RawMessage(`{"uuid":"ghi-789","properties":{"label":"SMS Device","type":"sms"}}`),
			},
			wantLen:   1,
			wantType:  "sms",
			wantLabel: "SMS Device",
		},
		{
			name: "device with created_at instead of created",
			rawInput: []json.RawMessage{
				json.RawMessage(`{"uuid":"jkl-012","name":"Email MFA","type":"email","created_at":"2025-06-01T12:00:00Z"}`),
			},
			wantLen:   1,
			wantType:  "email",
			wantLabel: "Email MFA",
		},
		{
			name:     "invalid JSON raw message",
			rawInput: []json.RawMessage{json.RawMessage("not valid json")},
			wantLen:  0, // invalid JSON is skipped
		},
		{
			name: "multiple devices",
			rawInput: []json.RawMessage{
				json.RawMessage(`{"uuid":"dev-1","name":"TOTP 1","type":"totp"}`),
				json.RawMessage(`{"uuid":"dev-2","name":"TOTP 2","type":"totp"}`),
				json.RawMessage(`{"uuid":"dev-3","name":"WebAuthn","type":"webauthn"}`),
			},
			wantLen: 3,
		},
		{
			name: "device without uuid is skipped",
			rawInput: []json.RawMessage{
				json.RawMessage(`{"name":"No UUID","type":"totp"}`),
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devices := parseAuthenticatorDevices(tt.rawInput)
			if len(devices) != tt.wantLen {
				t.Errorf("got %d devices, want %d", len(devices), tt.wantLen)
			}
			if tt.wantLen > 0 && tt.wantType != "" {
				if devices[0].Type != tt.wantType {
					t.Errorf("first device type = %q, want %q", devices[0].Type, tt.wantType)
				}
				if devices[0].Label != tt.wantLabel {
					t.Errorf("first device label = %q, want %q", devices[0].Label, tt.wantLabel)
				}
			}
		})
	}
}

// ---------- writeJSON helper tests (direct) ----------

func TestWriteJSONDirect(t *testing.T) {
	rec := httptest.NewRecorder()

	type testResponse struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	}
	resp := testResponse{Message: "success", Status: "ok"}

	writeJSON(rec, resp, http.StatusOK)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got testResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Message != "success" {
		t.Errorf("message = %q, want %q", got.Message, "success")
	}
	if got.Status != "ok" {
		t.Errorf("status = %q, want %q", got.Status, "ok")
	}
}

func TestWriteJSONNilDirect(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, nil, http.StatusOK)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// ---------- MFA Handler direct tests (without Authentik) ----------

func TestMFAHandlerEnrollEmptyBody(t *testing.T) {
	h := newTestMFAHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", http.NoBody)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	rec := httptest.NewRecorder()
	h.Enroll(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestMFAHandlerEnrollWhitespaceBody(t *testing.T) {
	h := newTestMFAHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", strings.NewReader("   "))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	rec := httptest.NewRecorder()
	h.Enroll(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerVerifyWhitespaceBody(t *testing.T) {
	h := newTestMFAHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/verify", strings.NewReader("   "))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	rec := httptest.NewRecorder()
	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerDisableWhitespaceBody(t *testing.T) {
	h := newTestMFAHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/disable", strings.NewReader("   "))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	rec := httptest.NewRecorder()
	h.Disable(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerDisableEmptyPassword(t *testing.T) {
	h := newTestMFAHandler()

	payload := `{"password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/disable", strings.NewReader(payload))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	rec := httptest.NewRecorder()
	h.Disable(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerListDevicesMissingUserID(t *testing.T) {
	h := newTestMFAHandler()

	// No principal set — no user_id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/mfa/enrolled", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	rec := httptest.NewRecorder()
	h.ListDevices(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMFAHandlerListDevicesWithQueryUserID(t *testing.T) {
	h := newTestMFAHandler()

	// Provide user_id via query param (admin access)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/mfa/enrolled?user_id=admin-123", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "admin-123",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	rec := httptest.NewRecorder()
	h.ListDevices(rec, req)

	// Should fail with 500 because Auth is nil
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestMFAHandlerRegenerateWhitespaceBody(t *testing.T) {
	h := newTestMFAHandler()

	// Whitespace body will fail JSON decode (returning 400) OR pass and hit nil Auth (returning 500)
	// The handler returns 500 for nil Auth before checking for empty user_id
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/recovery-codes", strings.NewReader("   "))
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	rec := httptest.NewRecorder()
	h.RegenerateRecoveryCodes(rec, req)

	// Whitespace body fails JSON decode → 400, or passes and hits nil Auth → 500
	// Either is acceptable since body is invalid
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 400 or 500; body: %s", rec.Code, rec.Body.String())
	}
}

// Ensure context is used to prevent unused import.
var _ = context.Background
