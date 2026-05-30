package handler

import (
	"bytes"
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

// ====================
// testMockGroupsAPI implements authentik.GroupsAPIOps for testing
// (renamed from mockGroupsAPI to avoid conflict with handler_delegations_test.go)
// ====================

type testMockGroupsAPI struct {
	groups         []*authentik.Group
	createFn       func(ctx context.Context, req authentik.CreateGroupRequest) (*authentik.Group, error)
	addUserFn      func(ctx context.Context, groupUUID, userUUID string) error
	removeUserFn   func(ctx context.Context, groupUUID, userUUID string) error
	listErr        error
	createErr      error
}

func (m *testMockGroupsAPI) Create(ctx context.Context, req authentik.CreateGroupRequest) (*authentik.Group, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	if m.createErr != nil {
		return nil, m.createErr
	}
	g := &authentik.Group{
		UUID: "mock-uuid-" + req.Name,
		Name: req.Name,
	}
	m.groups = append(m.groups, g)
	return g, nil
}

func (m *testMockGroupsAPI) GetByID(ctx context.Context, uuid string) (*authentik.Group, error) {
	for _, g := range m.groups {
		if g.UUID == uuid {
			return g, nil
		}
	}
	return nil, nil
}

func (m *testMockGroupsAPI) List(ctx context.Context) ([]*authentik.Group, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.groups, nil
}

func (m *testMockGroupsAPI) Update(ctx context.Context, uuid string, name string) (*authentik.Group, error) {
	return nil, nil
}

func (m *testMockGroupsAPI) Delete(ctx context.Context, uuid string) error {
	return nil
}

func (m *testMockGroupsAPI) AddUser(ctx context.Context, groupUUID, userUUID string) error {
	if m.addUserFn != nil {
		return m.addUserFn(ctx, groupUUID, userUUID)
	}
	return nil
}

func (m *testMockGroupsAPI) RemoveUser(ctx context.Context, groupUUID, userUUID string) error {
	if m.removeUserFn != nil {
		return m.removeUserFn(ctx, groupUUID, userUUID)
	}
	return nil
}

func (m *testMockGroupsAPI) GetMembers(ctx context.Context, groupUUID string) ([]string, error) {
	return nil, nil
}

// ====================
// helper to build a tenant-scoped request with principal
// ====================

func testRequestWithPrincipal(method, urlStr string, body *bytes.Reader) *http.Request {
	req := httptest.NewRequest(method, urlStr, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{
		Subject:  "admin-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	return setPrincipalInContext(req, principal)
}

// ====================
// extractUserRolesPath — additional edge cases
// ====================

func TestExtractUserRolesPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		wantID string
		wantOK bool
	}{
		{
			name:   "standard roles path",
			path:   "/api/v1/iam/users/user-123/roles",
			wantID: "user-123",
			wantOK: true,
		},
		{
			name:   "missing ID before roles",
			path:   "/api/v1/iam/users/roles",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "trailing slash after users only",
			path:   "/api/v1/iam/users/",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "extra trailing slash after roles",
			path:   "/api/v1/iam/users/abc-123/roles/",
			wantID: "abc-123",
			wantOK: true,
		},
		{
			name:   "wrong prefix",
			path:   "/api/v1/iam/roles/users/user-123",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "bare base path",
			path:   "/api/v1/iam/users",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "double slash in middle",
			path:   "/api/v1/iam/users//roles",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "roles but with extra segment",
			path:   "/api/v1/iam/users/user-123/roles/extra",
			wantID: "",
			wantOK: false,
		},
		{
			name:   "only trailing slash at end",
			path:   "/api/v1/iam/users/user-456/roles/",
			wantID: "user-456",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := extractUserRolesPath(tt.path)
			if gotID != tt.wantID {
				t.Errorf("extractUserRolesPath(%q) got ID = %q, want %q", tt.path, gotID, tt.wantID)
			}
			if gotOK != tt.wantOK {
				t.Errorf("extractUserRolesPath(%q) got OK = %v, want %v", tt.path, gotOK, tt.wantOK)
			}
		})
	}
}

// ====================
// SetRoles — full path tests (in-memory store path)
// ====================

func TestSetRoles_Success(t *testing.T) {
	h := NewTestUserHandler()

	// First create a user in the in-memory store via the Create handler
	createPayload := `{"email":"setroles@test.com","display_name":"SetRoles User","role_ids":["viewer"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{
		Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"},
	}
	createReq = setPrincipalInContext(createReq, principal)
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("Create returned %d, body: %s", createW.Code, createW.Body.String())
	}

	// Extract the user ID from the created response
	created := map[string]interface{}{}
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	userID := created["id"].(string)

	// Now call SetRoles
	rolesPayload := `{"role_ids":["admin","viewer"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/"+userID+"/roles", strings.NewReader(rolesPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	result := map[string]interface{}{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["user_id"] != userID {
		t.Errorf("SetRoles() user_id = %v, want %v", result["user_id"], userID)
	}
}

func TestSetRoles_EmptyUserID(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("SetRoles() empty user_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetRoles_InvalidJSON(t *testing.T) {
	h := NewTestUserHandler()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("SetRoles() invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetRoles_MissingTenant(t *testing.T) {
	h := NewTestUserHandler()

	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("SetRoles() missing tenant status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestSetRoles_ValidateError_EmptyRoleIDs(t *testing.T) {
	h := NewTestUserHandler()

	// Create a user first so GetByID doesn't fail
	createPayload := `{"email":"emptyroles@test.com","display_name":"EmptyRoles User","role_ids":["viewer"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", strings.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	createReq = setPrincipalInContext(createReq, principal)
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("Create returned %d", createW.Code)
	}

	created := map[string]interface{}{}
	json.NewDecoder(createW.Body).Decode(&created)
	userID := created["id"].(string)

	// Call SetRoles with empty role_ids
	payload := `{"role_ids":[]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/"+userID+"/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("SetRoles() empty role_ids status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ====================
// SetRoles with authentik path (GroupsAPI involved)
// ====================

func TestSetRoles_AuthentikPath_Success(t *testing.T) {
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{
			{UUID: "group-admin", Name: "admin"},
			{UUID: "group-viewer", Name: "viewer"},
		},
	}
	mockUsers := &mockAuthentikUsers{}
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI:  mockUsers,
			GroupsAPI: mockGroups,
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"role_ids":["admin","viewer"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/auth-user-uuid/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() authentik path status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify that RemoveUser was called for non-system groups and AddUser for requested roles
	if len(mockGroups.groups) == 0 {
		t.Error("SetRoles() expected mockGroups to remain populated")
	}
}

func TestSetRoles_AuthentikPath_ListGroupsError(t *testing.T) {
	mockGroups := &testMockGroupsAPI{listErr: &authentik.APIError{StatusCode: 500, Message: "internal error", Path: "/api/v3/core/groups/"}}
	mockUsers := &mockAuthentikUsers{}
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI:  mockUsers,
			GroupsAPI: mockGroups,
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/user-123/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("SetRoles() groups list error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestSetRoles_AuthentikPath_GroupNotFound_Skipped(t *testing.T) {
	// Group list returns groups but none match the requested role "unknown-role"
	// The handler silently skips roles not found in the group map
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{
			{UUID: "group-staff", Name: "staff"},
		},
	}
	mockUsers := &mockAuthentikUsers{}
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI:  mockUsers,
			GroupsAPI: mockGroups,
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"role_ids":["nonexistent-role"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/auth-user-uuid/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() nonexistent role status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	// Group count should remain unchanged — role not found means nothing to add
	if len(mockGroups.groups) != 1 {
		t.Errorf("SetRoles() expected 1 group (unchanged), got %d", len(mockGroups.groups))
	}
}

func TestSetRoles_AuthentikPath_GroupCreateFails_NonBlocking(t *testing.T) {
	// Group creation error is non-fatal — handler should still return 200
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{},
		createErr: &authentik.APIError{StatusCode: 500, Message: "create failed", Path: "/api/v3/core/groups/"},
	}
	mockUsers := &mockAuthentikUsers{}
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI:  mockUsers,
			GroupsAPI: mockGroups,
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"role_ids":["will-fail-group"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/auth-user-uuid/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() group create fails — expected 200 (non-fatal), got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestSetRoles_AuthentikPath_SystemGroupSkipped(t *testing.T) {
	// Groups with "staff" prefix should be skipped during removal
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{
			{UUID: "group-staff", Name: "staff"},
			{UUID: "group-admin", Name: "admin"},
		},
	}
	mockUsers := &mockAuthentikUsers{}
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI:  mockUsers,
			GroupsAPI: mockGroups,
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/auth-user-uuid/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() system group skip status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// ====================
// addToGroup — tests the unexported function via a wrapper
// ====================

func TestAddToGroup_GroupFound_CallsAddUser(t *testing.T) {
	addCalled := false
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{
			{UUID: "group-dev", Name: "developers"},
		},
		addUserFn: func(ctx context.Context, groupUUID, userUUID string) error {
			addCalled = true
			if groupUUID != "group-dev" {
				t.Errorf("AddUser called with groupUUID = %q, want %q", groupUUID, "group-dev")
			}
			if userUUID != "user-123" {
				t.Errorf("AddUser called with userUUID = %q, want %q", userUUID, "user-123")
			}
			return nil
		},
	}
	auth := &authentik.Client{GroupsAPI: mockGroups}

	err := addToGroup(context.Background(), auth, "user-123", "developers")
	if err != nil {
		t.Errorf("addToGroup() error = %v, want nil", err)
	}
	if !addCalled {
		t.Error("addToGroup() did not call AddUser for matching group")
	}
}

func TestAddToGroup_GroupNotFound_CreatesGroup(t *testing.T) {
	createCalled := false
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{},
		createFn: func(ctx context.Context, req authentik.CreateGroupRequest) (*authentik.Group, error) {
			createCalled = true
			if req.Name != "new-role" {
				t.Errorf("Create called with Name = %q, want %q", req.Name, "new-role")
			}
			return &authentik.Group{UUID: "new-uuid", Name: req.Name}, nil
		},
	}
	auth := &authentik.Client{GroupsAPI: mockGroups}

	err := addToGroup(context.Background(), auth, "user-456", "new-role")
	if err != nil {
		t.Errorf("addToGroup() error = %v, want nil", err)
	}
	if !createCalled {
		t.Error("addToGroup() did not call Create for missing group")
	}
}

func TestAddToGroup_CreateFails_NonFatal(t *testing.T) {
	mockGroups := &testMockGroupsAPI{
		groups:    []*authentik.Group{},
		createErr: &authentik.APIError{StatusCode: 500, Message: "create failed"},
	}
	auth := &authentik.Client{GroupsAPI: mockGroups}

	// Create error should be silently swallowed (non-fatal)
	err := addToGroup(context.Background(), auth, "user-789", "missing-role")
	if err != nil {
		t.Errorf("addToGroup() expected no error (create is non-fatal), got %v", err)
	}
}

func TestAddToGroup_ListFails(t *testing.T) {
	mockGroups := &testMockGroupsAPI{
		listErr:   &authentik.APIError{StatusCode: 500, Message: "list failed"},
		createErr: nil,
	}
	auth := &authentik.Client{GroupsAPI: mockGroups}

	err := addToGroup(context.Background(), auth, "user-999", "any-role")
	if err == nil {
		t.Error("addToGroup() expected error when List fails, got nil")
	}
}

// ====================
// isConflictError
// ====================

func TestIsConflictError_Cases(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{
			name:  "409 conflict",
			err:   &authentik.APIError{StatusCode: 409, Message: "duplicate", Path: "/api/v3/core/users/"},
			want:  true,
		},
		{
			name:  "400 bad request",
			err:   &authentik.APIError{StatusCode: 400, Message: "exists", Path: "/api/v3/core/users/"},
			want:  true,
		},
		{
			name:  "500 server error",
			err:   &authentik.APIError{StatusCode: 500, Message: "error", Path: "/api/v3/core/users/"},
			want:  false,
		},
		{
			name:  "404 not found",
			err:   &authentik.APIError{StatusCode: 404, Message: "not found", Path: "/api/v3/core/users/"},
			want:  false,
		},
		{
			name:  "201 created",
			err:   &authentik.APIError{StatusCode: 201, Message: "created", Path: "/api/v3/core/users/"},
			want:  false,
		},
		{
			name:  "wrapped error",
			err:   &models.ValidationError{Message: "invalid format"},
			want:  false,
		},
		{
			name:  "standard error",
			err:   &models.ValidationError{Message: "bad input"},
			want:  false,
		},
		{
			name:  "nil error",
			err:   nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConflictError(tt.err)
			if got != tt.want {
				t.Errorf("isConflictError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ====================
// SetRoles — invalid user ID (non-UUID path segment)
// ====================

func TestSetRoles_InvalidUserPathSegment(t *testing.T) {
	h := NewTestUserHandler()

	// Path /api/v1/iam/users/roles — missing user ID entirely
	payload := `{"role_ids":["admin"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("SetRoles() missing user ID in path status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ====================
// SetRoles — with Authentik path and non-matching role
// ====================

func TestSetRoles_AuthentikPath_RoleNotInGroupMap(t *testing.T) {
	// Group list returns groups but none match the requested role "unknown-role"
	mockGroups := &testMockGroupsAPI{
		groups: []*authentik.Group{
			{UUID: "group-staff", Name: "staff"},
			{UUID: "group-devs", Name: "developers"},
		},
	}
	mockUsers := &mockAuthentikUsers{}
	h := NewUserHandler(
		&authentik.Client{
			UsersAPI:  mockUsers,
			GroupsAPI: mockGroups,
		},
		store.NewUserStore(),
		store.NewAuditStore(),
		events.NewPublisher(""),
	)

	// Request a role that doesn't exist in the group map
	payload := `{"role_ids":["unknown-role"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/iam/users/auth-user-uuid/roles", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	principal := &middleware.JWTToken{Subject: "admin-1", UserType: "user", TenantID: "tenant-1", Roles: []string{"admin"}}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.SetRoles(w, req)

	// Should still return 200 (role not found is silently skipped)
	if w.Code != http.StatusOK {
		t.Errorf("SetRoles() unknown role status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// ====================
// extractUserRolesPath — nil/empty path
// ====================

func TestExtractUserRolesPath_EmptyPath(t *testing.T) {
	id, ok := extractUserRolesPath("")
	if id != "" || ok {
		t.Errorf("extractUserRolesPath(\"\") = (%q, %v), want (\"\", false)", id, ok)
	}
}

func TestExtractUserRolesPath_JustPrefix(t *testing.T) {
	id, ok := extractUserRolesPath("/api/v1/iam/users/")
	if id != "" || ok {
		t.Errorf("extractUserRolesPath(\"/api/v1/iam/users/\") = (%q, %v), want (\"\", false)", id, ok)
	}
}
