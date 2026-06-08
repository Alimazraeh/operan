package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

func delegPrincipalReq(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, http.NoBody)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	return setPrincipalInContext(r, &middleware.JWTToken{
		Subject: "user-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"},
	})
}

func TestDelegationHandler_GrantRevokeDelete(t *testing.T) {
	mock := newDelegMockAuthClient()
	// Seed a user so findUserUUID resolves.
	mock.UsersAPI.users["u-9"] = &authentik.User{UUID: "u-9", Email: "u9@example.com"}
	h := NewDelegationHandler(mock, newNoopPublisher())

	// Create a delegation role first.
	rr := httptest.NewRecorder()
	h.Create(rr, delegPrincipalReq(http.MethodPost, "/api/v1/iam/admin/delegations",
		`{"name":"role-x","description":"d","scope":"tenant","permissions":["read"]}`))
	if rr.Code != http.StatusCreated {
		t.Fatalf("Create() = %d. Body: %s", rr.Code, rr.Body.String())
	}
	var role models.DelegationRole
	_ = json.Unmarshal(rr.Body.Bytes(), &role)
	roleID := role.ID
	if roleID == "" {
		t.Fatal("Create() returned empty role ID")
	}

	// Grant the role to the seeded user.
	rr = httptest.NewRecorder()
	h.Grant(rr, delegPrincipalReq(http.MethodPost, "/api/v1/iam/admin/delegations/"+roleID+"/grant", `{"user_id":"u-9"}`))
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated && rr.Code != http.StatusNotFound {
		t.Errorf("Grant() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Grant validation error: missing user_id.
	rr = httptest.NewRecorder()
	h.Grant(rr, delegPrincipalReq(http.MethodPost, "/api/v1/iam/admin/delegations/"+roleID+"/grant", `{}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Grant(no user) = %d, want 400", rr.Code)
	}

	// Revoke a specific user.
	rr = httptest.NewRecorder()
	h.Revoke(rr, delegPrincipalReq(http.MethodDelete, "/api/v1/iam/admin/delegations/"+roleID+"/grant?user_id=u-9", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent && rr.Code != http.StatusNotFound {
		t.Errorf("Revoke() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// ListDelegations for the role.
	rr = httptest.NewRecorder()
	h.ListDelegations(rr, delegPrincipalReq(http.MethodGet, "/api/v1/iam/admin/delegations/"+roleID+"/delegations", ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNotFound {
		t.Errorf("ListDelegations() = %d", rr.Code)
	}

	// Delete the role.
	rr = httptest.NewRecorder()
	h.Delete(rr, delegPrincipalReq(http.MethodDelete, "/api/v1/iam/admin/delegations/"+roleID, ""))
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent && rr.Code != http.StatusNotFound {
		t.Errorf("Delete() = %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Missing role ID on Grant/Revoke -> 400.
	rr = httptest.NewRecorder()
	h.Grant(rr, delegPrincipalReq(http.MethodPost, "/api/v1/iam/admin/delegations", `{"user_id":"u-9"}`))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Grant(no role) = %d, want 400", rr.Code)
	}
}
