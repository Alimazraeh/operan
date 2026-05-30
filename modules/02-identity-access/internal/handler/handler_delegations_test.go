package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ---------------------------------------------------------------------------
// Mock group API (unique name to avoid conflict with handler_users_extra_test.go)
// ---------------------------------------------------------------------------

type delegMockGroups struct {
	mu     sync.Mutex
	groups map[string]*authentik.Group
	byName map[string]string // fullName -> UUID
}

func newDelegMockGroups() *delegMockGroups {
	return &delegMockGroups{
		groups: make(map[string]*authentik.Group),
		byName: make(map[string]string),
	}
}

func (m *delegMockGroups) Create(_ context.Context, req authentik.CreateGroupRequest) (*authentik.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byName[req.Name]; exists {
		return nil, &authentik.APIError{StatusCode: 409}
	}

	g := &authentik.Group{
		UUID:       "uuid-" + req.Name,
		Name:       req.Name,
		IsStaff:    false,
		Tenant:     req.Tenant,
		Properties: delegMockProps(req.Name),
	}
	m.groups[g.UUID] = g
	m.byName[g.Name] = g.UUID
	return g, nil
}

func delegMockProps(name string) map[string]interface{} {
	return map[string]interface{}{
		"permissions":          []interface{}{"read", "write"},
		"scope":                "tenant",
		"max_delegation_depth": float64(1),
		"role_name":            name,
	}
}

func (m *delegMockGroups) List(_ context.Context) ([]*authentik.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*authentik.Group, 0, len(m.groups))
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result, nil
}

func (m *delegMockGroups) GetByID(_ context.Context, uuid string) (*authentik.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, exists := m.groups[uuid]
	if !exists {
		return nil, &authentik.APIError{StatusCode: 404}
	}
	return g, nil
}

func (m *delegMockGroups) Update(_ context.Context, uuid string, name string) (*authentik.Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, exists := m.groups[uuid]
	if !exists {
		return nil, &authentik.APIError{StatusCode: 404}
	}
	oldName := g.Name
	delete(m.byName, oldName)
	g.Name = name
	m.byName[name] = uuid
	return g, nil
}

func (m *delegMockGroups) Delete(_ context.Context, uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, exists := m.groups[uuid]
	if !exists {
		return &authentik.APIError{StatusCode: 404}
	}
	delete(m.byName, g.Name)
	delete(m.groups, uuid)
	return nil
}

func (m *delegMockGroups) AddUser(_ context.Context, groupUUID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.groups[groupUUID]
	if !exists {
		return &authentik.APIError{StatusCode: 404}
	}
	return nil
}

func (m *delegMockGroups) RemoveUser(_ context.Context, groupUUID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.groups[groupUUID]
	if !exists {
		return &authentik.APIError{StatusCode: 404}
	}
	return nil
}

func (m *delegMockGroups) GetMembers(_ context.Context, groupUUID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, exists := m.groups[groupUUID]
	if !exists {
		return nil, &authentik.APIError{StatusCode: 404}
	}
	return g.Users, nil
}

// ---------------------------------------------------------------------------
// Mock user API implementation
// ---------------------------------------------------------------------------

type delegMockUsers struct {
	mu      sync.Mutex
	users   map[string]*authentik.User
	byEmail map[string]string
}

func newDelegMockUsers() *delegMockUsers {
	return &delegMockUsers{
		users:   make(map[string]*authentik.User),
		byEmail: make(map[string]string),
	}
}

func (m *delegMockUsers) Create(_ context.Context, _ authentik.CreateUserRequest) (*authentik.User, error) {
	return nil, nil
}

func (m *delegMockUsers) GetByID(_ context.Context, uuid string) (*authentik.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, exists := m.users[uuid]
	if !exists {
		return nil, &authentik.APIError{StatusCode: 404}
	}
	return u, nil
}

func (m *delegMockUsers) List(_ context.Context) ([]*authentik.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*authentik.User, 0, len(m.users))
	for _, u := range m.users {
		result = append(result, u)
	}
	return result, nil
}

func (m *delegMockUsers) Update(_ context.Context, uuid string, _ authentik.UpdateUserRequest) (*authentik.User, error) {
	return nil, nil
}

func (m *delegMockUsers) Delete(_ context.Context, uuid string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Mock authentik client for delegation tests
// ---------------------------------------------------------------------------

type delegMockAuthClient struct {
	GroupsAPI *delegMockGroups
	UsersAPI  *delegMockUsers
}

func newDelegMockAuthClient() *delegMockAuthClient {
	return &delegMockAuthClient{
		GroupsAPI: newDelegMockGroups(),
		UsersAPI:  newDelegMockUsers(),
	}
}

func (m *delegMockAuthClient) Groups() authentik.GroupsAPIOps {
	return m.GroupsAPI
}

func (m *delegMockAuthClient) Users() authentik.UsersAPIOps {
	return m.UsersAPI
}

// ---------------------------------------------------------------------------
// Helper: build a DelegationHandler with mock Authentik client
// ---------------------------------------------------------------------------

func newMockDelegationHandler() *DelegationHandler {
	mock := newDelegMockAuthClient()
	// Use a real publisher to avoid nil pointer panics during tests.
	// The publisher will fail to connect to the broker, but the handler
	// should publish asynchronously and not block test execution.
	return NewDelegationHandler(mock, newNoopPublisher())
}

func newNoopPublisher() *noOpPublisher {
	return &noOpPublisher{}
}

// noOpPublisher implements a no-op Publisher to prevent nil panics
// in handler tests where AMQP connectivity is not needed.
type noOpPublisher struct{}

func (p *noOpPublisher) Publish(ctx context.Context, eventType, correlationID, tenantID, timestamp string, payload map[string]interface{}) error {
	return nil
}

// ---------------------------------------------------------------------------
// Helper: set tenant context on request
// ---------------------------------------------------------------------------

func setDelegationTestContext(req *http.Request, tenantID string) *http.Request {
	return setPrincipalInContext(req, &middleware.JWTToken{
		Subject:  "admin-1",
		UserType: "user",
		TenantID: tenantID,
		Roles:    []string{"admin"},
	})
}

// ---------------------------------------------------------------------------
// Test: Create
// ---------------------------------------------------------------------------

func TestDelegationHandlerCreateSuccess(t *testing.T) {
	h := newMockDelegationHandler()

	payload := `{"name":"admin-role","description":"Admin role","scope":"tenant","permissions":["read","write","delete"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, &middleware.JWTToken{Subject:"user-1", UserType:"user", TenantID:"tenant-1", Roles:[]string{"admin"}})

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp models.DelegationRole
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Create() invalid JSON: %v", err)
	}
	if resp.Name != "admin-role" {
		t.Errorf("Create() name = %v, want admin-role", resp.Name)
	}
	if resp.TenantID != "tenant-1" {
		t.Errorf("Create() tenant_id = %v, want tenant-1", resp.TenantID)
	}
	if resp.Scope != "tenant" {
		t.Errorf("Create() scope = %v, want tenant", resp.Scope)
	}
}

func TestDelegationHandlerCreateMissingFields(t *testing.T) {
	h := newMockDelegationHandler()

	// Missing scope and permissions
	payload := `{"name":"partial-role","description":"Missing fields"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() missing fields status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestDelegationHandlerCreateEmptyName(t *testing.T) {
	h := newMockDelegationHandler()

	payload := `{"description":"no name","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() empty name status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestDelegationHandlerCreateInvalidJSON(t *testing.T) {
	h := newMockDelegationHandler()

	payload := `{"name":"bad-role"`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Create() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Test: List
// ---------------------------------------------------------------------------

func TestDelegationHandlerListEmpty(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON: %v", err)
	}
	roles, ok := resp["delegation_roles"].([]interface{})
	if !ok {
		t.Fatalf("List() delegation_roles not an array")
	}
	if len(roles) != 0 {
		t.Errorf("List() empty list roles count = %v, want 0", len(roles))
	}
	if resp["total"].(float64) != 0 {
		t.Errorf("List() total = %v, want 0", resp["total"])
	}
}

func TestDelegationHandlerListWithRoles(t *testing.T) {
	h := newMockDelegationHandler()

	// Create two roles for tenant-1 via the Create handler
	for _, name := range []string{"role-alpha", "role-beta"} {
		payload := `{"name":"` + name + `","scope":"tenant","permissions":["read","write"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		setDelegationTestContext(req, "tenant-1")

		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("Create(%s) status = %v, want 201", name, w.Code)
		}
	}

	// List roles
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations?page=1&page_size=10", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("List() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("List() invalid JSON: %v", err)
	}
	if resp["total"].(float64) != 2 {
		t.Errorf("List() total = %v, want 2", resp["total"])
	}
}

// ---------------------------------------------------------------------------
// Test: GetByID
// ---------------------------------------------------------------------------

func TestDelegationHandlerGetByIDSuccess(t *testing.T) {
	h := newMockDelegationHandler()

	// Create a role first
	payload := `{"name":"get-role","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var created models.DelegationRole
	json.Unmarshal(w.Body.Bytes(), &created)

	// Get by the group UUID (Authentik group UUID)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations/"+created.ID, nil)
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetByID() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.DelegationRole
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetByID() invalid JSON: %v", err)
	}
	if resp.Name != "get-role" {
		t.Errorf("GetByID() name = %v, want get-role", resp.Name)
	}
}

func TestDelegationHandlerGetByIDNotFound(t *testing.T) {
	h := newMockDelegationHandler()

	// No roles created - should return 404
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations/nonexistent", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GetByID() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestDelegationHandlerGetByIDMissingID(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetByID() missing ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Test: Update
// ---------------------------------------------------------------------------

func TestDelegationHandlerUpdateSuccess(t *testing.T) {
	h := newMockDelegationHandler()

	// Create a role first
	payload := `{"name":"update-role","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var created models.DelegationRole
	json.Unmarshal(w.Body.Bytes(), &created)

	// Update via name (the handler looks up by name when UUID lookup fails)
	payload = `{"description":"Updated","scope":"tenant","permissions":["read","write"]}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/iam/admin/delegations/"+created.Name, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Update() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp models.DelegationRole
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Update() invalid JSON: %v", err)
	}
	if resp.Name != "update-role" {
		t.Errorf("Update() name = %v, want update-role", resp.Name)
	}
}

func TestDelegationHandlerUpdateNotFound(t *testing.T) {
	h := newMockDelegationHandler()

	payload := `{"description":"New desc","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/admin/delegations/nonexistent", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Update() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestDelegationHandlerUpdateInvalidJSON(t *testing.T) {
	h := newMockDelegationHandler()

	// First create a role to have something to update
	payload := `{"name":"upd-invalid","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	// Send invalid JSON for update
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/iam/admin/delegations/upd-invalid", strings.NewReader(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Update() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Test: Delete
// ---------------------------------------------------------------------------

func TestDelegationHandlerDeleteSuccess(t *testing.T) {
	h := newMockDelegationHandler()

	// Create a role first
	payload := `{"name":"del-role","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var created models.DelegationRole
	json.Unmarshal(w.Body.Bytes(), &created)

	// Delete by name (the handler falls back to name-based lookup)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/iam/admin/delegations/"+created.Name, nil)
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Delete() success status = %v, want %v", w.Code, http.StatusNoContent)
	}
}

func TestDelegationHandlerDeleteNotFound(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/admin/delegations/nonexistent", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Delete() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestDelegationHandlerDeleteMissingID(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/admin/delegations", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Delete() missing ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Test: Grant
// ---------------------------------------------------------------------------

func TestDelegationHandlerGrantSuccess(t *testing.T) {
	h := newMockDelegationHandler()
	mock := h.Auth.(*delegMockAuthClient)

	// Create a role first
	payload := `{"name":"grant-role","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create() status = %v, want %v", w.Code, http.StatusCreated)
	}

	// Pre-register a user in the mock so findUserUUID can resolve it
	mock.UsersAPI.mu.Lock()
	mock.UsersAPI.users["user-123"] = &authentik.User{UUID: "user-123", Email: "user@example.com"}
	mock.UsersAPI.mu.Unlock()

	// Grant the role
	payload = `{"user_id":"user-123","scope":"tenant"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations/grant-role/grant", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.Grant(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Grant() status = %v, want %v", w.Code, http.StatusCreated)
	}

	var resp delegationGrantResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Grant() invalid JSON: %v", err)
	}
	if resp.UserID != "user-123" {
		t.Errorf("Grant() user_id = %v, want user-123", resp.UserID)
	}
	if !resp.IsActive {
		t.Error("Grant() should have IsActive = true")
	}
}

func TestDelegationHandlerGrantInvalidUserID(t *testing.T) {
	h := newMockDelegationHandler()

	// Create a role first
	payload := `{"name":"grant-invalid-role","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	// Grant with missing user_id
	payload = `{"scope":"tenant"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations/grant-invalid-role/grant", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.Grant(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Grant() missing user_id status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestDelegationHandlerGrantNotFoundRole(t *testing.T) {
	h := newMockDelegationHandler()

	payload := `{"user_id":"user-123","scope":"tenant"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations/nonexistent-role/grant", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Grant(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Grant() role not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Test: Revoke
// ---------------------------------------------------------------------------

func TestDelegationHandlerRevokeSuccess(t *testing.T) {
	h := newMockDelegationHandler()

	// Create a role first
	payload := `{"name":"revoke-role","scope":"tenant","permissions":["read"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Create(w, req)

	// Revoke
	req = httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations/revoke-role/revoke", nil)
	setDelegationTestContext(req, "tenant-1")

	w = httptest.NewRecorder()
	h.Revoke(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Revoke() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Revoke() invalid JSON: %v", err)
	}
	if resp["message"] != "delegation revoked" {
		t.Errorf("Revoke() message = %v, want delegation revoked", resp["message"])
	}
}

func TestDelegationHandlerRevokeNotFoundRole(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations/nonexistent/revoke", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.Revoke(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Revoke() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestDelegationHandlerRevokeMissingRoleID(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/admin/delegations/revoke", nil)
	req = setPrincipalInContext(req, &middleware.JWTToken{Subject:"user-1", UserType:"user", TenantID:"tenant-1", Roles:[]string{"admin"}})

	w := httptest.NewRecorder()
	h.Revoke(w, req)

	// Missing role ID in path returns 404 (path must match /delegations/{role_id}/revoke)
	if w.Code != http.StatusNotFound {
		t.Errorf("Revoke() missing role ID status = %v, want 404", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Test: ListDelegations
// ---------------------------------------------------------------------------

func TestDelegationHandlerListDelegationsSuccess(t *testing.T) {
	h := newMockDelegationHandler()
	mock := h.Auth.(*delegMockAuthClient)

	// Manually add members to the mock group so ListDelegations finds them
	groupName := "operan-delegation-tenant-1-list-deleg-role"
	g := "uuid-" + groupName
	mock.GroupsAPI.mu.Lock()
	mock.GroupsAPI.byName[groupName] = g
	mock.GroupsAPI.groups[g] = &authentik.Group{UUID: g, Name: groupName}
	mock.GroupsAPI.groups[g].Users = []string{"member-1", "member-2"}
	mock.GroupsAPI.mu.Unlock()

	// List delegations
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations/list-deleg-role/delegations", nil)
	req = setPrincipalInContext(req, &middleware.JWTToken{Subject:"user-1", UserType:"user", TenantID:"tenant-1", Roles:[]string{"admin"}})

	w := httptest.NewRecorder()
	h.ListDelegations(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListDelegations() status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ListDelegations() invalid JSON: %v", err)
	}
	if resp["total"].(float64) != 2 {
		t.Errorf("ListDelegations() total = %v, want 2", resp["total"])
	}
}

func TestDelegationHandlerListDelegationsNotFound(t *testing.T) {
	h := newMockDelegationHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/admin/delegations/nonexistent/delegations", nil)
	setDelegationTestContext(req, "tenant-1")

	w := httptest.NewRecorder()
	h.ListDelegations(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("ListDelegations() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// Test: extractDelegationRoleID
// ---------------------------------------------------------------------------

func TestExtractDelegationRoleID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/iam/admin/delegations/role-123", "role-123"},
		{"/api/v1/iam/admin/delegations/role-123/", "role-123"},
		{"/api/v1/iam/admin/delegations/", ""},
		{"/api/v1/iam/admin/delegations", ""},
		{"/api/v1/iam/admin/delegations/grant-role/grant", "grant-role"},
		{"/api/v1/iam/admin/delegations/revoke-role/revoke", "revoke-role"},
		{"/api/v1/iam/admin/delegations/list-deleg-role/delegations", "list-deleg-role"},
		{"", ""},
		{"/some/other/path", ""},
	}

	for _, tt := range tests {
		got := extractDelegationRoleID(tt.path)
		if got != tt.want {
			t.Errorf("extractDelegationRoleID(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestExtractDelegationRoleIDEmptyPath(t *testing.T) {
	got := extractDelegationRoleID("")
	if got != "" {
		t.Errorf("extractDelegationRoleID(\"\") = %v, want empty string", got)
	}
}

func TestExtractDelegationRoleIDWithSlashes(t *testing.T) {
	got := extractDelegationRoleID("/api/v1/iam/admin/delegations/role-456/")
	if got != "role-456" {
		t.Errorf("extractDelegationRoleID with trailing slash = %v, want role-456", got)
	}
}
