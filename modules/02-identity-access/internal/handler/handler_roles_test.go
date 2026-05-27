package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

func NewTestRoleHandler() *RoleHandler {
	return &RoleHandler{
		Roles:     store.NewRoleStore(),
		Audit:     store.NewAuditStore(),
		Publisher: events.NewPublisher(""),
	}
}

func setPrincipalInContext(req *http.Request, principal *middleware.JWTToken) *http.Request {
	ctx := req.Context()
	ctx = context.WithValue(ctx, "jwt_token", principal)
	ctx = context.WithValue(ctx, middleware.UserIDKey, principal.Subject)
	ctx = context.WithValue(ctx, middleware.UserTypeKey, principal.UserType)
	ctx = context.WithValue(ctx, middleware.TenantIDKey, principal.TenantID)
	return req.WithContext(ctx)
}

func TestRoleHandlerCreateSuccess(t *testing.T) {
	h := NewTestRoleHandler()

	payload := `{"name":"admin","description":"Administrator role","permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")

	// Set principal in context
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
		t.Errorf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.Role
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON response: %v", err)
	}
	if resp.Name != "admin" {
		t.Errorf("Create() name = %v, want admin", resp.Name)
	}
	if resp.TenantID != "tenant-1" {
		t.Errorf("Create() tenant_id = %v, want tenant-1", resp.TenantID)
	}
}

func TestRoleHandlerCreateMissingName(t *testing.T) {
	h := NewTestRoleHandler()

	payload := `{"description":"No name role"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() missing name status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestRoleHandlerCreateDuplicate(t *testing.T) {
	h := NewTestRoleHandler()

	payload := `{"name":"unique-role","description":"First role","permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
		t.Fatalf("Create() first role status = %v, want %v", w.Code, http.StatusCreated)
	}

	// Try to create duplicate
	payload2 := `{"name":"unique-role","description":"Duplicate role","permissions":["read","write","delete"]}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Tenant-ID", "tenant-1")
	req2 = setPrincipalInContext(req2, principal)

	w2 := httptest.NewRecorder()
	h.Create(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Create() duplicate status = %v, want %v", w2.Code, http.StatusConflict)
	}
}

func TestRoleHandlerList(t *testing.T) {
	h := NewTestRoleHandler()

	// Create a few roles first
	for i := 0; i < 3; i++ {
		payload := `{"name":"role-` + string(rune('a'+i)) + `","description":"Test role","permissions":["read","write","delete"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
	}

	// Now list
	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=1&page_size=10", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON response: %v", err)
	}
	if resp["page"].(float64) != 1 {
		t.Errorf("List() page = %v, want 1", resp["page"])
	}
	if resp["total"].(float64) != 3 {
		t.Errorf("List() total = %v, want 3", resp["total"])
	}
}

func TestRoleHandlerListDefaultPageSize(t *testing.T) {
	h := NewTestRoleHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON: %v", err)
	}
	if resp["page_size"].(float64) != 50 {
		t.Errorf("List() default page_size = %v, want 50", resp["page_size"])
	}
}

func TestRoleHandlerGetByID(t *testing.T) {
	h := NewTestRoleHandler()

	// Create a role first
	payload := `{"name":"get-role","description":"Get Role","permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var created models.Role
	json.Unmarshal(w.Body.Bytes(), &created)

	// Get by ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer token")

	w = httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetByID() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.Role
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetByID() invalid JSON: %v", err)
	}
	if resp.Name != "get-role" {
		t.Errorf("GetByID() name = %v, want get-role", resp.Name)
	}
}

func TestRoleHandlerGetByIDNotFound(t *testing.T) {
	h := NewTestRoleHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer token")

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GetByID() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestRoleHandlerDelete(t *testing.T) {
	h := NewTestRoleHandler()

	// Create a role first
	payload := `{"name":"delete-role","description":"Delete Role","permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var created models.Role
	json.Unmarshal(w.Body.Bytes(), &created)

	// Delete the role
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/iam/roles/"+created.ID, nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req = setPrincipalInContext(req, principal)

	w = httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Delete() status = %v, want %v", w.Code, http.StatusNoContent)
	}
}

func TestRoleHandlerDeleteNotFound(t *testing.T) {
	h := NewTestRoleHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/roles/nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Delete() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestRoleHandlerCreateInvalidJSON(t *testing.T) {
	h := NewTestRoleHandler()

	payload := `{"name":"invalid-role"`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestRoleHandlerCreateWithPermissions(t *testing.T) {
	h := NewTestRoleHandler()

	payload := `{"name":"perm-role","description":"Permissions role","permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.Role
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON: %v", err)
	}
	if len(resp.Permissions) != 3 {
		t.Errorf("Create() permissions count = %v, want 3", len(resp.Permissions))
	}
}

func TestRoleHandlerCreateWithIsSystem(t *testing.T) {
	h := NewTestRoleHandler()

	isSystem := true
	payload := `{"name":"system-role","description":"System role","is_system":true,"permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.Role
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON: %v", err)
	}
	if !resp.IsSystem {
		t.Error("Create() should set IsSystem to true")
	}
	_ = isSystem // avoid unused variable error
}

func TestExtractRoleID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/iam/roles/role-123", "role-123"},
		{"/api/v1/iam/roles/role-456/", "role-456"},
		{"/api/v1/iam/roles/", ""},
		{"/api/v1/iam/roles", ""},
	}

	for _, tt := range tests {
		got := extractRoleID(tt.path)
		if got != tt.want {
			t.Errorf("extractRoleID(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestRoleHandlerListPagination(t *testing.T) {
	h := NewTestRoleHandler()

	// Create 7 roles for tenant-1
	for i := 0; i < 7; i++ {
		payload := fmt.Sprintf(`{"name":"page-role-%d","description":"Page role","permissions":["read","write","delete"]}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
	}

	// Test page 1, pageSize 3
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=1&page_size=3", nil)
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
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON: %v", err)
	}
	if resp["total"].(float64) != 7 {
		t.Errorf("List() total = %v, want 7", resp["total"])
	}
	roles := resp["roles"].([]interface{})
	if len(roles) != 3 {
		t.Errorf("List() len(roles) = %v, want 3", len(roles))
	}

	// Test page 3, pageSize 3 (last page has 1)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=3&page_size=3", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")

	principal = &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w = httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	roles = resp["roles"].([]interface{})
	if len(roles) != 1 {
		t.Errorf("List() page3 len = %v, want 1", len(roles))
	}

	// Test page 4 (beyond total)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=4&page_size=3", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req.Header.Set("Authorization", "Bearer token")

	principal = &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w = httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	roles = resp["roles"].([]interface{})
	if len(roles) != 0 {
		t.Errorf("List() page4 len = %v, want 0", len(roles))
	}
}

func TestRoleHandlerListInvalidPageSize(t *testing.T) {
	h := NewTestRoleHandler()

	// pageSize > 100 should be clamped to 50
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=1&page_size=200", nil)
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
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON: %v", err)
	}
	if resp["page_size"].(float64) != 50 {
		t.Errorf("List() page_size = %v, want 50 (clamped)", resp["page_size"])
	}
}

func TestRoleHandlerListInvalidPage(t *testing.T) {
	h := NewTestRoleHandler()

	// page < 1 should default to 1
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=0&page_size=10", nil)
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
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON: %v", err)
	}
	if resp["page"].(float64) != 1 {
		t.Errorf("List() page = %v, want 1", resp["page"])
	}
}

func TestRoleHandlerDeleteMissingRoleID(t *testing.T) {
	h := NewTestRoleHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/roles", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Delete() missing role ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestRoleHandlerGetByIDMissingRoleID(t *testing.T) {
	h := NewTestRoleHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles", nil)
	req.Header.Set("Authorization", "Bearer token")

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetByID() missing role ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestRoleHandlerListTenantIsolation(t *testing.T) {
	h := NewTestRoleHandler()

	// Create 2 roles for tenant-1
	for i := 0; i < 2; i++ {
		payload := fmt.Sprintf(`{"name":"t1-role-%d","description":"Tenant 1 role","permissions":["read","write","delete"]}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
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
	}

	// Create 3 roles for tenant-2
	for i := 0; i < 3; i++ {
		payload := fmt.Sprintf(`{"name":"t2-role-%d","description":"Tenant 2 role","permissions":["read","write","delete"]}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/roles", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "tenant-2")

		principal := &middleware.JWTToken{
			Subject:  "user-1",
			UserType: "user",
			TenantID: "tenant-2",
			Roles:    []string{"admin"},
		}
		req = setPrincipalInContext(req, principal)

		w := httptest.NewRecorder()
		h.Create(w, req)
	}

	// List tenant-1 roles
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=1&page_size=10", nil)
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
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 2 {
		t.Errorf("List() tenant-1 total = %v, want 2", resp["total"])
	}

	// List tenant-2 roles
	req = httptest.NewRequest(http.MethodGet, "/api/v1/iam/roles?page=1&page_size=10", nil)
	req.Header.Set("X-Tenant-ID", "tenant-2")
	req.Header.Set("Authorization", "Bearer token")

	principal = &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-2",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w = httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 3 {
		t.Errorf("List() tenant-2 total = %v, want 3", resp["total"])
	}
}
