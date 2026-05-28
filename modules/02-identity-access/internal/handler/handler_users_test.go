package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

func NewTestUserHandler() *UserHandler {
	return NewUserHandler(nil, store.NewUserStore(), store.NewAuditStore(), events.NewPublisher(""))
}

func TestUserHandlerCreateSuccess(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","display_name":"Test User","roles":["viewer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON response: %v", err)
	}
	if resp.Email != "test@example.com" {
		t.Errorf("Create() email = %v, want test@example.com", resp.Email)
	}
	if resp.Status != "pending" {
		t.Errorf("Create() status = %v, want pending", resp.Status)
	}
}

func TestUserHandlerCreateMissingEmail(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"display_name":"Test User","roles":["viewer"]}`
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() missing email status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerCreateMissingDisplayName(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","roles":["viewer"]}`
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() missing display_name status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerCreateMissingRoles(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","display_name":"Test User"}`
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() missing roles status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerList(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users?page=1&page_size=10", nil)
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
		t.Errorf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON response: %v", err)
	}
	if resp["page"] != float64(1) {
		t.Errorf("List() page = %v, want 1", resp["page"])
	}
	if resp["page_size"] != float64(10) {
		t.Errorf("List() page_size = %v, want 10", resp["page_size"])
	}
}

func TestUserHandlerListDefaultPageSize(t *testing.T) {
	h := NewTestUserHandler()

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
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON response: %v", err)
	}
	if resp["page_size"] != float64(50) {
		t.Errorf("List() default page_size = %v, want 50", resp["page_size"])
	}
}

func TestUserHandlerUpdate(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	payload := `{"display_name":"Updated Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Update() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Update() invalid JSON response: %v", err)
	}
	if resp.DisplayName != "Updated Name" {
		t.Errorf("Update() display_name = %v, want Updated Name", resp.DisplayName)
	}
}

func TestUserHandlerUpdateMissingBody(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader(""))
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
		t.Errorf("Update() missing body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerUpdateInvalidBody(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader("invalid json"))
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
		t.Errorf("Update() invalid body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerGetByID(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users/user-123", nil)
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
		t.Errorf("GetByID() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetByID() invalid JSON response: %v", err)
	}
	if resp.ID != "user-123" {
		t.Errorf("GetByID() id = %v, want user-123", resp.ID)
	}
}

func TestUserHandlerGetByIDNotFound(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users/nonexistent", nil)
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

	if w.Code != http.StatusNotFound {
		t.Errorf("GetByID() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestUserHandlerDeactivate(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/users/user-123", nil)
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
	h.Deactivate(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Deactivate() status = %v, want %v", w.Code, http.StatusNoContent)
	}
}

func TestUserHandlerSetRoles(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	payload := `{"roles":["admin","editor"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.SetRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("SetRoles() invalid JSON response: %v", err)
	}
	if resp["user_id"] != "user-123" {
		t.Errorf("SetRoles() user_id = %v, want user-123", resp["user_id"])
	}
}

func TestUserHandlerSetRolesInvalidBody(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"roles":"not-an-array"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader(payload))
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("SetRoles() invalid body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerCreateInvalidJSON(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","display_name":"Test`
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerCreateEmptyBody(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(""))
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
		t.Errorf("Create() empty body status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerUpdateNoop(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	// Send empty JSON object - should return 400 because all fields are nil
	payload := `{}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Update() noop status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerCreateSetsCreatedAt(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","display_name":"Test User","roles":["viewer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON: %v", err)
	}
	if resp.CreatedAt.IsZero() {
		t.Error("Create() should set CreatedAt")
	}
}

func TestUserHandlerCreateSetsAuthMethod(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"test@example.com","display_name":"Test User","roles":["viewer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON: %v", err)
	}
	if resp.AuthenticationMethod != "password" {
		t.Errorf("Create() auth method = %v, want password", resp.AuthenticationMethod)
	}
}

func TestUserHandlerUpdateMFA(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	payload := `{"mfa_enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Update() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Update() invalid JSON: %v", err)
	}
	if !resp.MFAEnabled {
		t.Error("Update() should enable MFA")
	}
	if resp.AuthenticationMethod != "mfa" {
		t.Errorf("Update() auth method = %v, want mfa", resp.AuthenticationMethod)
	}
}

func TestUserHandlerGetByIDReturnsTenant(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users/user-123", nil)
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
		t.Fatalf("GetByID() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetByID() invalid JSON: %v", err)
	}
	if resp.TenantID == "" {
		t.Error("GetByID() should return TenantID")
	}
}

func TestUserHandlerListWithInvalidPageSize(t *testing.T) {
	h := NewTestUserHandler()

	// page_size > 100 should be clamped to 100
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users?page=1&page_size=200", nil)
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
	if ps := resp["page_size"].(float64); ps > 100 {
		t.Errorf("List() page_size = %v, want clamped to 100", ps)
	}
}

func TestUserHandlerListWithInvalidPage(t *testing.T) {
	h := NewTestUserHandler()

	// page < 1 should default to 1
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/users?page=0&page_size=10", nil)
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
	if page := resp["page"].(float64); page < 1 {
		t.Errorf("List() page = %v, want at least 1", page)
	}
}

func TestUserHandlerSetRolesEmptyArray(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	payload := `{"roles":[]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.SetRoles(w, req)

	// Handler rejects empty roles array - validation requires at least one role
	if w.Code != http.StatusBadRequest {
		t.Errorf("SetRoles() empty array status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlerUpdateStatus(t *testing.T) {
	h := NewTestUserHandler()

	// First, create a user
	h.CreateUserForTest("user-123", "test@example.com", "Test User")

	payload := `{"status":"suspended"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Update() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Update() invalid JSON: %v", err)
	}
	if resp.Status != "suspended" {
		t.Errorf("Update() status = %v, want suspended", resp.Status)
	}
}

func TestUserHandlerCreateMFAUser(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"email":"mfa@example.com","display_name":"MFA User","roles":["viewer"],"mfa_enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Create() MFA status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() MFA invalid JSON: %v", err)
	}
	if !resp.MFAEnabled {
		t.Error("Create() MFA should enable MFA")
	}
	if resp.AuthenticationMethod != "mfa" {
		t.Errorf("Create() MFA auth method = %v, want mfa", resp.AuthenticationMethod)
	}
}

// CreateUserForTest is a test helper that directly creates a user in the store
func (h *UserHandler) CreateUserForTest(id, email, displayName string) {
	user := &models.User{
		ID:                   id,
		Email:                email,
		DisplayName:          displayName,
		Status:               "active",
		Roles:                []string{"viewer"},
		TenantID:             "tenant-test",
		MFAEnabled:           false,
		AuthenticationMethod: "password",
	}
	h.Users.Create(user)
}
