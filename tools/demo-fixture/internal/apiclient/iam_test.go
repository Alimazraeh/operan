package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminLoginBypassesTenantHeaderAndAuth(t *testing.T) {
	var gotPath string
	var gotAuthHeader, gotTenantHeader string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthHeader = r.Header.Get("Authorization")
		gotTenantHeader = r.Header.Get("X-Tenant-ID")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(AdminLoginResponse{Token: "admin-jwt", UserID: "admin-001", Email: "admin@operan"})
	}))
	defer srv.Close()

	c := &IAMClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.AdminLogin(context.Background(), "operan-admin-2026", "smoke-tenant")
	if err != nil {
		t.Fatalf("AdminLogin: %v", err)
	}
	if gotPath != "/api/v1/iam/admin/login" {
		t.Errorf("path = %q, want /api/v1/iam/admin/login", gotPath)
	}
	if gotAuthHeader != "" {
		t.Errorf("expected no Authorization header on the bootstrap login, got %q", gotAuthHeader)
	}
	if gotTenantHeader != "" {
		t.Errorf("expected no X-Tenant-ID header on the bootstrap login, got %q", gotTenantHeader)
	}
	if gotBody["password"] != "operan-admin-2026" || gotBody["tenant"] != "smoke-tenant" {
		t.Errorf("request body = %+v", gotBody)
	}
	if got.Token != "admin-jwt" {
		t.Errorf("got.Token = %q", got.Token)
	}
}

func TestLoginPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(LoginResponse{Token: "dana-jwt", UserID: "u-1", Roles: []string{"department_head"}})
	}))
	defer srv.Close()

	c := &IAMClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.Login(context.Background(), "dana@adri.nz", "pw", "smoke-tenant")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if gotPath != "/api/v1/iam/auth/login" {
		t.Errorf("path = %q, want /api/v1/iam/auth/login", gotPath)
	}
	if got.Token != "dana-jwt" {
		t.Errorf("got.Token = %q", got.Token)
	}
}

func TestCreateUserSendsAuthAndTenantHeaders(t *testing.T) {
	var gotAuth, gotTenant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(User{ID: "u-2", Email: "dana@adri.nz", Status: "pending"})
	}))
	defer srv.Close()

	c := &IAMClient{BaseURL: srv.URL, Doer: NewDoer()}
	got, err := c.CreateUser(context.Background(), "admin-jwt", "smoke-tenant", CreateUserRequest{
		Email: "dana@adri.nz", DisplayName: "Dana Q", RoleIDs: []string{"department_head"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if gotAuth != "Bearer admin-jwt" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotTenant != "smoke-tenant" {
		t.Errorf("X-Tenant-ID = %q", gotTenant)
	}
	if got.ID != "u-2" {
		t.Errorf("got.ID = %q", got.ID)
	}
}

func TestFindUserByEmailPaginatesAndMatches(t *testing.T) {
	const total = 75
	all := make([]*User, total)
	for i := range all {
		all[i] = &User{ID: fmt.Sprintf("u-%d", i), Email: fmt.Sprintf("user%d@example.com", i)}
	}
	all[60].Email = "dana@adri.nz"

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page, pageSize := 1, 50
		fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page)
		fmt.Sscanf(r.URL.Query().Get("page_size"), "%d", &pageSize)
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		_ = json.NewEncoder(w).Encode(userListResponse{Users: all[start:end], Total: total})
	}))
	defer srv.Close()

	c := &IAMClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindUserByEmail(context.Background(), "tok", "smoke-tenant", "dana@adri.nz")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	if found == nil || found.Email != "dana@adri.nz" {
		t.Fatalf("expected to find dana, got %+v", found)
	}
	if calls != 2 {
		t.Errorf("expected 2 page requests for 75 items at page_size 50, got %d", calls)
	}
}

func TestFindUserByEmailNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(userListResponse{Users: []*User{{ID: "1", Email: "someone-else@example.com"}}, Total: 1})
	}))
	defer srv.Close()

	c := &IAMClient{BaseURL: srv.URL, Doer: NewDoer()}
	found, err := c.FindUserByEmail(context.Background(), "tok", "smoke-tenant", "dana@adri.nz")
	if err != nil {
		t.Fatalf("FindUserByEmail: unexpected error: %v", err)
	}
	if found != nil {
		t.Fatalf("expected nil, got %+v", found)
	}
}

func TestSetPasswordPath(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &IAMClient{BaseURL: srv.URL, Doer: NewDoer()}
	err := c.SetPassword(context.Background(), "admin-jwt", "smoke-tenant", "u-2", "dana-operan-2026!")
	if err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/iam/users/u-2/password" {
		t.Errorf("method/path = %s %s, want POST /api/v1/iam/users/u-2/password", gotMethod, gotPath)
	}
	if gotBody["password"] != "dana-operan-2026!" {
		t.Errorf("request body password = %q", gotBody["password"])
	}
}
