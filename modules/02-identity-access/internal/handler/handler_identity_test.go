package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
	"github.com/operan/modules/02-identity-access/internal/store"
)

// ==========================================================================
// ServiceIdentityHandler tests (Auth=nil = in-memory fallback)
// ==========================================================================

func TestServiceIdentityHandlerCreateValid(t *testing.T) {
	h := NewTestServiceIdentityHandler()

	payload := `{"name":"my-service","tenant_id":"tenant-1","role_ids":["viewer","editor"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/service-identities", strings.NewReader(payload))
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
		t.Errorf("Create() valid status = %v, want %v", w.Code, http.StatusCreated)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Create() failed to unmarshal response: %v", err)
	}
	if result["name"] != "my-service" {
		t.Errorf("Create() name = %v, want %v", result["name"], "my-service")
	}
	if result["api_key_id"] == nil || result["api_key_id"].(string) == "" {
		t.Error("Create() missing api_key_id")
	}
}

func TestServiceIdentityHandlerCreateMissingName(t *testing.T) {
	h := NewTestServiceIdentityHandler()

	payload := `{"tenant_id":"tenant-1","role_ids":["viewer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/service-identities", strings.NewReader(payload))
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

func TestServiceIdentityHandlerCreateInvalidJSON(t *testing.T) {
	h := NewTestServiceIdentityHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/service-identities", strings.NewReader("not json"))
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

func TestServiceIdentityHandlerListEmpty(t *testing.T) {
	h := NewTestServiceIdentityHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/service-identities", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

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

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("List() failed to unmarshal response: %v", err)
	}
	identities, ok := result["service_identities"].([]interface{})
	if !ok || len(identities) != 0 {
		t.Errorf("List() identities = %v, want empty list", result["service_identities"])
	}
	if result["total"].(float64) != 0 {
		t.Errorf("List() total = %v, want 0", result["total"])
	}
}

func TestServiceIdentityHandlerCreateAndGetByID(t *testing.T) {
	h := NewTestServiceIdentityHandler()

	store := store.NewServiceIdentityStore()
	h.Store = store

	// Create identity directly in store for GetByID test
	identity := &models.ServiceIdentity{
		ID:       "test-id-123",
		Name:     "test-svc",
		TenantID: "tenant-1",
	}
	store.Create(identity)

	// Now test GetByID through the handler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/service-identities/test-id-123", nil)
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

	if w.Code != http.StatusOK {
		t.Errorf("GetByID() status = %v, want %v", w.Code, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("GetByID() failed to unmarshal response: %v", err)
	}
	if result["id"] != "test-id-123" {
		t.Errorf("GetByID() id = %v, want %v", result["id"], "test-id-123")
	}
}

func TestServiceIdentityHandlerGetByIDMissing(t *testing.T) {
	h := NewTestServiceIdentityHandler()

	store := store.NewServiceIdentityStore()
	h.Store = store

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/service-identities/non-existent-id", nil)
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

	if w.Code != http.StatusNotFound {
		t.Errorf("GetByID() missing status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestServiceIdentityHandlerAuthenticErrorStatus(t *testing.T) {
	h := &ServiceIdentityHandler{Auth: &authentik.Client{}, Store: store.NewServiceIdentityStore(), Publisher: events.NewPublisher("")}

	// nil error should return 500
	if got := h.authenticErrorStatus(nil); got != http.StatusInternalServerError {
		t.Errorf("authenticErrorStatus(nil) = %v, want %v", got, http.StatusInternalServerError)
	}

	// 403 API error should return 403
	apiErr := &authentik.APIError{StatusCode: 403}
	if got := h.authenticErrorStatus(apiErr); got != 403 {
		t.Errorf("authenticErrorStatus(403) = %v, want %v", got, 403)
	}

	// 409 API error should return 409
	apiErr409 := &authentik.APIError{StatusCode: 409}
	if got := h.authenticErrorStatus(apiErr409); got != 409 {
		t.Errorf("authenticErrorStatus(409) = %v, want %v", got, 409)
	}

	// 500 API error should return 500
	apiErr500 := &authentik.APIError{StatusCode: 500}
	if got := h.authenticErrorStatus(apiErr500); got != 500 {
		t.Errorf("authenticErrorStatus(500) = %v, want %v", got, 500)
	}
}

// ==========================================================================
// AgentIdentityHandler tests (Auth=nil = in-memory fallback)
// ==========================================================================

func TestAgentIdentityHandlerRegisterValid(t *testing.T) {
	h := NewTestAgentIdentityHandler()

	payload := `{"agent_id":"my-agent","tenant_id":"tenant-1","capabilities":["read","write"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/agent-identities", strings.NewReader(payload))
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
	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Register() valid status = %v, want %v", w.Code, http.StatusCreated)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Register() failed to unmarshal response: %v", err)
	}
	if result["agent_id"] != "my-agent" {
		t.Errorf("Register() agent_id = %v, want %v", result["agent_id"], "my-agent")
	}
	if result["id"] == nil || result["id"].(string) == "" {
		t.Error("Register() missing identity id")
	}
}

func TestAgentIdentityHandlerRegisterMissingAgentID(t *testing.T) {
	h := NewTestAgentIdentityHandler()

	payload := `{"tenant_id":"tenant-1","capabilities":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/agent-identities", strings.NewReader(payload))
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
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Register() missing agent_id status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestAgentIdentityHandlerRegisterInvalidJSON(t *testing.T) {
	h := NewTestAgentIdentityHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/agent-identities", strings.NewReader("{bad json"))
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
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Register() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestAgentIdentityHandlerListEmpty(t *testing.T) {
	h := NewTestAgentIdentityHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/agent-identities", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

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

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("List() failed to unmarshal response: %v", err)
	}
	if result["total"].(float64) != 0 {
		t.Errorf("List() total = %v, want 0", result["total"])
	}
}

func TestAgentIdentityHandlerRegisterAndGetByAgent(t *testing.T) {
	h := NewTestAgentIdentityHandler()

	// Register first
	payload := `{"agent_id":"agent-123","tenant_id":"tenant-1","capabilities":["execute"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/agent-identities", strings.NewReader(payload))
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
	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Register() status = %v, want %v", w.Code, http.StatusCreated)
	}

	// GetByAgent through handler
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/iam/agent-identities/agent/agent-123", nil)
	req2.Header.Set("X-Tenant-ID", "tenant-1")
	req2 = setPrincipalInContext(req2, principal)

	w2 := httptest.NewRecorder()
	h.GetByAgent(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("GetByAgent() status = %v, want %v", w2.Code, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &result); err != nil {
		t.Fatalf("GetByAgent() failed to unmarshal response: %v", err)
	}
	if result["agent_id"] != "agent-123" {
		t.Errorf("GetByAgent() agent_id = %v, want %v", result["agent_id"], "agent-123")
	}
}

func TestAgentIdentityHandlerGetByAgentMissing(t *testing.T) {
	h := NewTestAgentIdentityHandler()

	store := store.NewAgentIdentityStore()
	h.Store = store

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/agent-identities/agent/non-existent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetByAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GetByAgent() missing status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestAgentIdentityHandlerGetAgentGroupID(t *testing.T) {
	// Test with nil Auth - should fail because getAgentGroupID requires authentik
	h := &AgentIdentityHandler{Auth: nil, Store: store.NewAgentIdentityStore(), Publisher: events.NewPublisher("")}
	ctx := context.Background()

	// With nil Auth, the underlying API calls in getAgentGroupID will panic.
	// This tests the expected behavior: getAgentGroupID should fail when Auth is nil.
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("getAgentGroupID() with nil Auth should panic, but did not")
		}
	}()
	_, _ = h.getAgentGroupID(ctx, "tenant-1")
}

func TestAgentIdentityHandlerAuthenticErrorStatus(t *testing.T) {
	h := &AgentIdentityHandler{Auth: &authentik.Client{}, Store: store.NewAgentIdentityStore(), Publisher: events.NewPublisher("")}

	// nil error should return 500
	if got := h.authenticErrorStatus(nil); got != http.StatusInternalServerError {
		t.Errorf("authenticErrorStatus(nil) = %v, want %v", got, http.StatusInternalServerError)
	}

	// 403 API error should return 403
	apiErr := &authentik.APIError{StatusCode: 403}
	if got := h.authenticErrorStatus(apiErr); got != 403 {
		t.Errorf("authenticErrorStatus(403) = %v, want %v", got, 403)
	}

	// 401 API error should return 401
	apiErr401 := &authentik.APIError{StatusCode: 401}
	if got := h.authenticErrorStatus(apiErr401); got != 401 {
		t.Errorf("authenticErrorStatus(401) = %v, want %v", got, 401)
	}

	// 502 API error should return 502
	apiErr502 := &authentik.APIError{StatusCode: 502}
	if got := h.authenticErrorStatus(apiErr502); got != 502 {
		t.Errorf("authenticErrorStatus(502) = %v, want %v", got, 502)
	}
}

// ==========================================================================
// Helper function tests
// ==========================================================================

func TestExtractIdentityID(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"standard path", "/api/v1/iam/service-identities/abc-123", "abc-123"},
		{"with trailing slash", "/api/v1/iam/service-identities/abc-123/", "abc-123"},
		{"empty string", "", ""},
		{"just prefix", "/api/v1/iam/service-identities/", ""},
		{"only prefix", "/api/v1/iam/service-identities", ""},
		{"uuid format", "/api/v1/iam/service-identities/550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIdentityID(tt.path)
			if got != tt.want {
				t.Errorf("extractIdentityID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractAgentID(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"valid path", "/api/v1/iam/agent-identities/agent/agent-123", "agent-123"},
		{"with trailing slash", "/api/v1/iam/agent-identities/agent/agent-123/", "agent-123"},
		{"different agent id", "/api/v1/iam/agent-identities/agent/svc-001", "svc-001"},
		{"empty path", "", ""},
		{"no agent prefix", "/api/v1/iam/agent-identities/", ""},
		{"just prefix", "/api/v1/iam/agent-identities/agent/", ""},
		{"wrong path", "/api/v1/iam/users/user-1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAgentID(tt.path)
			if got != tt.want {
				t.Errorf("extractAgentID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestContainsUUID(t *testing.T) {
	tests := []struct {
			name   string
			ids    []string
			uuid   string
			wantOK bool
		}{
			{
				name:   "valid uuid in list",
				ids:    []string{"550e8400-e29b-41d4-a716-446655440000", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
				uuid:   "550e8400-e29b-41d4-a716-446655440000",
				wantOK: true,
			},
			{
				name:   "uuid not in list",
				ids:    []string{"550e8400-e29b-41d4-a716-446655440000", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
				uuid:   "00000000-0000-0000-0000-000000000000",
				wantOK: false,
			},
			{
				name:   "empty list",
				ids:    []string{},
				uuid:   "550e8400-e29b-41d4-a716-446655440000",
				wantOK: false,
			},
			{
				name:   "nil list",
				ids:    nil,
				uuid:   "550e8400-e29b-41d4-a716-446655440000",
				wantOK: false,
			},
		}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsUUID(tt.ids, tt.uuid)
			if got != tt.wantOK {
				t.Errorf("containsUUID(%v, %q) = %v, want %v", tt.ids, tt.uuid, got, tt.wantOK)
			}
		})
	}
}
