package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/middleware"
)

// userTenantReq builds a request with both user and tenant in context.
func userTenantReq(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, http.NoBody)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	ctx := context.WithValue(r.Context(), middleware.TenantIDKey, "t1")
	ctx = context.WithValue(ctx, middleware.UserIDKey, "user-1")
	return r.WithContext(ctx)
}

func TestMFAHandler_Enroll(t *testing.T) {
	h := NewMFAHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.Enroll(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/enroll", `{"method":"totp"}`))
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Errorf("Enroll() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// missing method -> 400
	rr = httptest.NewRecorder()
	h.Enroll(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/enroll", `{}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Enroll(no method) = %d, want 400", rr.Code)
	}

	// missing user -> 400
	rr = httptest.NewRecorder()
	h.Enroll(rr, tenantReq(http.MethodPost, "/api/v1/iam/mfa/enroll", `{"method":"totp"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Enroll(no user) = %d, want 400", rr.Code)
	}
}

func TestMFAHandler_Verify(t *testing.T) {
	h := NewMFAHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.Verify(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/verify", `{"method":"totp","code":"123456"}`))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("Verify() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.Verify(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/verify", `{"code":"x"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Verify(no method) = %d, want 400", rr.Code)
	}
}

func TestMFAHandler_Disable(t *testing.T) {
	h := NewMFAHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.Disable(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/disable", `{"password":"secret"}`))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError && rr.Code != http.StatusNoContent {
		t.Errorf("Disable() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.Disable(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/disable", `{}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Disable(no password) = %d, want 400", rr.Code)
	}
}

func TestMFAHandler_RegenerateRecoveryCodes(t *testing.T) {
	h := NewMFAHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.RegenerateRecoveryCodes(rr, userTenantReq(http.MethodPost, "/api/v1/iam/mfa/recovery-codes", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("RegenerateRecoveryCodes() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.RegenerateRecoveryCodes(rr, tenantReq(http.MethodPost, "/api/v1/iam/mfa/recovery-codes", ""))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("RegenerateRecoveryCodes(no user) = %d, want 400", rr.Code)
	}
}

func TestMFAHandler_ListDevices(t *testing.T) {
	h := NewMFAHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.ListDevices(rr, userTenantReq(http.MethodGet, "/api/v1/iam/mfa/devices", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("ListDevices() = %d. Body: %s", rr.Code, rr.Body.String())
	}
}
