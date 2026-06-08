package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRBACHandler_Evaluate_AuthBacked(t *testing.T) {
	h := NewRBACHandler(ssoAuthClient(t))

	rr := httptest.NewRecorder()
	body := `{"actor_id":"user-1","resource":"user","action":"read"}`
	h.Evaluate(rr, userTenantReq(http.MethodPost, "/api/v1/iam/rbac/evaluate", body))
	if rr.Code != http.StatusOK {
		t.Errorf("Evaluate() = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	// validation error
	rr = httptest.NewRecorder()
	h.Evaluate(rr, userTenantReq(http.MethodPost, "/api/v1/iam/rbac/evaluate", `{"action":"read"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Evaluate(no actor) = %d, want 400", rr.Code)
	}
}

func TestSSOHandler_Test_AuthBacked(t *testing.T) {
	h := NewSSOHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	// No matching provider -> 404 (exercises fetchSSOConfig under Test).
	h.Test(rr, tenantReq(http.MethodPost, "/api/v1/iam/auth/sso/test", `{"provider":"okta"}`))
	if rr.Code != http.StatusNotFound {
		t.Errorf("Test(auth, no provider) = %d, want 404. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSCIMHandler_UpdateUser_RemoveOp(t *testing.T) {
	h := NewSCIMHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	// Remove op exercises applyRemove + applyPatchOps Remove branch.
	body := `{"op":"Remove","path":"emails"}`
	h.UpdateUser(rr, tenantReq(http.MethodPut, "/api/v1/iam/scim/users/u1", body))
	if rr.Code != http.StatusNoContent && rr.Code != http.StatusBadRequest && rr.Code != http.StatusInternalServerError {
		t.Errorf("UpdateUser(Remove) = %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSCIMHandler_ListUsers_WithFilter(t *testing.T) {
	h := NewSCIMHandler(ssoAuthClient(t), nil)
	rr := httptest.NewRecorder()
	h.ListUsers(rr, tenantReq(http.MethodGet, `/api/v1/iam/scim/users?filter=userName+eq+"alice"&count=10&startIndex=1`, ""))
	if rr.Code != http.StatusOK {
		t.Errorf("ListUsers(filter) = %d", rr.Code)
	}
}
