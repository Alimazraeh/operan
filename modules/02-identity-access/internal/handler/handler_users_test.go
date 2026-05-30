package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// ----- users handler — missing tenant / auth errors -----

func TestUserHandlerCreateMissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","display_name":"Test User","role_ids":["viewer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Create() missing tenant status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerCreateNoContentLength(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() nil body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerListMissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users", nil)

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("List() missing tenant status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerGetByIDMissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users/user-123", nil)

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetByID() missing tenant status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerUpdateMissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader(`{"display_name":"Updated"}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Update() missing tenant status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerDeactivateMissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/users/user-123", nil)

	w := httptest.NewRecorder()
	h.Deactivate(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Deactivate() missing tenant status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerSetRolesMissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("SetRoles() missing tenant status = %v, want %v", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlerGetByIDMissingUserID(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetByID() missing user ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerUpdateMissingUserID(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/", strings.NewReader(`{"display_name":"Updated"}`))
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
	h.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Update() missing user ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerDeactivateMissingUserID(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/users/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Deactivate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Deactivate() missing user ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// ----- users handler — update non-existent user -----

func TestUserHandlerUpdateNonExistent(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"display_name":"Updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/non-existent", strings.NewReader(payload))
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
	h.Update(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Update() non-existent user status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

func TestUserHandlerDeactivateNonExistent(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/users/non-existent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Deactivate(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Deactivate() non-existent user status = %v, want %v", w.Code, http.StatusInternalServerError)
	}
}

// ----- users handler — set roles non-existent user -----

func TestUserHandlerSetRolesNonExistent(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/non-existent/roles", strings.NewReader(payload))
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
	h.SetRoles(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("SetRoles() non-existent user status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

// ----- users handler — authentik path (when Auth is set) -----

func TestUserHandlerCreateWithAuthentik(t *testing.T) {
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI: &mockAuthentikUsers{},
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"email":"test@example.com","display_name":"Test User","role_ids":["viewer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(payload))
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
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Create() with authentik status = %v, want %v", w.Code, http.StatusCreated)
	}
}

func TestUserHandlerListWithAuthentik(t *testing.T) {
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI: &mockAuthentikUsers{},
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("List() with authentik status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestUserHandlerGetByIDWithAuthentik(t *testing.T) {
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI: &mockAuthentikUsers{},
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users/test-user-uuid", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetByID() with authentik status = %v, want %v", w.Code, http.StatusOK)
	}
}

// ----- users handler — update with authentik path -----

func TestUserHandlerUpdateWithAuthentik(t *testing.T) {
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI: &mockAuthentikUsers{},
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"display_name":"Updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/test-user-uuid", strings.NewReader(payload))
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
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Update() with authentik status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestUserHandlerDeactivateWithAuthentik(t *testing.T) {
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI: &mockAuthentikUsers{},
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/users/test-user-uuid", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Deactivate(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Deactivate() with authentik status = %v, want %v", w.Code, http.StatusNoContent)
	}
}

// ----- extractUserID tests -----

func TestExtractUserIDVariousPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"standard path", "/api/v1/iam/users/user-123", "user-123"},
		{"with trailing slash", "/api/v1/iam/users/user-123/", "user-123"},
		{"roles path", "/api/v1/iam/users/user-456/roles", "user-456"},
		{"empty id", "/api/v1/iam/users/", ""},
		{"no id segment", "/api/v1/iam/users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserID(tt.path)
			if got != tt.want {
				t.Errorf("extractUserID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ----- mock authentik users API for testing -----

type mockAuthentikUsers struct{}

func (m *mockAuthentikUsers) Create(ctx context.Context, req authentik.CreateUserRequest) (*authentik.User, error) {
	return &authentik.User{
		UUID:       "auth-user-uuid",
		Username:   req.Username,
		Email:      req.Email,
		Name:       req.Name,
		IsActive:   req.IsActive,
		Tenant:     req.Tenant,
		DateJoined: &nowTime,
	}, nil
}

func (m *mockAuthentikUsers) Update(ctx context.Context, uuid string, req authentik.UpdateUserRequest) (*authentik.User, error) {
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	return &authentik.User{
		UUID:       uuid,
		Email:      "updated@example.com",
		Name:       name,
		IsActive:   true,
		Tenant:     "tenant-1",
		DateJoined: &nowTime,
	}, nil
}

func (m *mockAuthentikUsers) GetByID(ctx context.Context, uuid string) (*authentik.User, error) {
	return &authentik.User{
		UUID:       uuid,
		Username:   "testuser",
		Email:      "test@example.com",
		Name:       "Test User",
		IsActive:   true,
		Tenant:     "tenant-1",
		DateJoined: &nowTime,
	}, nil
}

func (m *mockAuthentikUsers) Delete(ctx context.Context, uuid string) error {
	return nil
}

func (m *mockAuthentikUsers) List(ctx context.Context) ([]*authentik.User, error) {
	return []*authentik.User{
		{UUID: "u1", Email: "user1@example.com", Name: "User 1", IsActive: true, Tenant: "tenant-1"},
		{UUID: "u2", Email: "user2@example.com", Name: "User 2", IsActive: true, Tenant: "tenant-1"},
	}, nil
}

var nowTime = time.Now().UTC()
