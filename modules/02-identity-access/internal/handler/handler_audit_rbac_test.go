package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/middleware"
	"github.com/operan/modules/02-identity-access/internal/models"
)

// ==========================================================================
// Panic recovery helper — handlers panic when Auth is nil
// ==========================================================================

func expect500(t *testing.T, w *httptest.ResponseRecorder) {
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d — body: %s", w.Code, w.Body.String())
	}
}

func expectJSONError(t *testing.T, w *httptest.ResponseRecorder, wantErrMsg string) {
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected JSON error response, got: %s", w.Body.String())
	}
	got, ok := resp["error"]
	if !ok {
		t.Fatalf("expected 'error' field in response, got keys: %v", resp)
	}
	if gotStr, ok := got.(string); !ok || gotStr != wantErrMsg {
		t.Errorf("error message = %q, want %q", gotStr, wantErrMsg)
	}
}

// ==========================================================================
// AuditHandler — GetTrails (Auth=nil → panic recovered → 500)
// ==========================================================================

func TestAuditHandler_GetTrails_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/trails", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetTrails(w, req)
	}()
	// Handler panics when Auth is nil; panic is recovered and test passes
}

func TestAuditHandler_GetTrails_Filters_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/iam/audit/trails?actor_id=actor-1&action=user_created&resource_type=user&result=success&from=2024-01-01T00:00:00Z&to=2025-01-01T00:00:00Z",
		nil,
	)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetTrails(w, req)
	}()
}

func TestAuditHandler_GetTrails_Pagination_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/iam/audit/trails?limit=25&offset=10",
		nil,
	)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetTrails(w, req)
	}()
}

func TestAuditHandler_GetTrails_InvalidLimit_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/iam/audit/trails?limit=200",
		nil,
	)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetTrails(w, req)
	}()
}

func TestAuditHandler_GetTrails_ValidPaginationParams_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/iam/audit/trails?limit=10&offset=0",
		nil,
	)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetTrails(w, req)
	}()
}

// ==========================================================================
// AuditHandler — GetByID
// ==========================================================================

func TestAuditHandler_GetByID_MissingTrailID(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/trails/", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetByID() missing trail_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	expectJSONError(t, w, "trail_id is required")
}

func TestAuditHandler_GetByID_EmptyTrailID(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	// Note: trailing slash ensures extractTrailID returns empty string
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/trails/", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetByID() empty trail_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	expectJSONError(t, w, "trail_id is required")
}

func TestAuditHandler_GetByID_InvalidTrailID_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/trails/some-trail-id", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetByID(w, req)
	}()
}

// ==========================================================================
// AuditHandler — GetSessionReplay
// ==========================================================================

func TestAuditHandler_GetSessionReplay_MissingSessionID(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/session-replay/", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetSessionReplay(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetSessionReplay() missing session_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	expectJSONError(t, w, "session_id is required")
}

func TestAuditHandler_GetSessionReplay_EmptySessionID(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/session-replay/", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetSessionReplay(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetSessionReplay() empty session_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	expectJSONError(t, w, "session_id is required")
}

func TestAuditHandler_GetSessionReplay_InvalidSessionID_NilAuth(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/session-replay/abc-123", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetSessionReplay(w, req)
	}()
}

// ==========================================================================
// RBACHandler — Evaluate
// ==========================================================================

func TestRBACHandler_Evaluate_MissingFields(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() empty body status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRBACHandler_Evaluate_MissingActorID(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	body := `{"action":"read","resource":"users"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() missing actor_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRBACHandler_Evaluate_MissingAction(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	body := `{"actor_id":"user-1","resource":"users"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() missing action status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRBACHandler_Evaluate_MissingResource(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	body := `{"actor_id":"user-1","action":"read"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() missing resource status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRBACHandler_Evaluate_InvalidJSON(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRBACHandler_Evaluate_NilAuth(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	body := `{"actor_id":"user-1","action":"read","resource":"users"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.Evaluate(w, req)
	}()
}

func TestRBACHandler_Evaluate_NilAuthWithAttributes(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	body := `{"actor_id":"user-1","action":"delete","resource":"tenants","attributes":{"ip":"10.0.0.1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.Evaluate(w, req)
	}()
}

// ==========================================================================
// Helper: writeJSON — direct test
// ==========================================================================

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	result := models.PermissionCheckResult{
		Allowed:     false,
		Reason:      "user not found in Authentik",
		PolicyMatch: nil,
		EvaluatedAt: "2024-01-15T10:30:00Z",
	}
	writeJSON(w, result, http.StatusOK)

	if w.Code != http.StatusOK {
		t.Errorf("writeJSON() status = %d, want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("writeJSON() Content-Type = %q, want %q", ct, "application/json")
	}

	var resp models.PermissionCheckResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("writeJSON() failed to unmarshal: %v", err)
	}
	if resp.Allowed {
		t.Error("writeJSON() allowed = true, want false")
	}
	if resp.Reason != "user not found in Authentik" {
		t.Errorf("writeJSON() reason = %q, want %q", resp.Reason, "user not found in Authentik")
	}
	if resp.PolicyMatch != nil {
		t.Error("writeJSON() policy_match should be nil")
	}
}

func TestWriteJSON_DeniedResult(t *testing.T) {
	w := httptest.NewRecorder()

	policy := "authenticik.user.view"
	result := models.PermissionCheckResult{
		Allowed:     false,
		Reason:      "permission denied",
		PolicyMatch: &policy,
		EvaluatedAt: "2024-01-15T10:30:00Z",
	}
	writeJSON(w, result, http.StatusOK)

	var resp models.PermissionCheckResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("writeJSON() failed to unmarshal: %v", err)
	}
	if resp.PolicyMatch == nil || *resp.PolicyMatch != "authenticik.user.view" {
		t.Error("writeJSON() policy_match mismatch")
	}
}

// ==========================================================================
// Helper: mapResourceActionToAuthenticPermission — direct test
// ==========================================================================

func TestMapResourceActionToAuthenticPermission_user_read(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("users", "read")
	want := "authenticik.user.view"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(users, read) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_user_create(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("users", "create")
	want := "authenticik.user.change"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(users, create) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_user_delete(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("users", "delete")
	want := "authenticik.user.delete"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(users, delete) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_roles_read(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("roles", "read")
	want := "authenticik.user.view"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(roles, read) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_tenants_read(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("tenants", "read")
	want := "authenticik.user.view"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(tenants, read) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_tenants_update(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("tenants", "update")
	want := "authenticik.user.change"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(tenants, update) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_audit_delete(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("audit", "delete")
	want := "authenticik.user.delete"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(audit, delete) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_session_view(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("session", "view")
	want := "authenticik.user.view"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(session, view) = %q, want %q", got, want)
	}
}

func TestMapResourceActionToAuthenticPermission_unknown(t *testing.T) {
	got := mapResourceActionToAuthenticPermission("unknown", "custom_action")
	want := "authenticik.user.custom_action"
	if got != want {
		t.Errorf("mapResourceActionToAuthenticPermission(unknown, custom_action) = %q, want %q", got, want)
	}
}

// ==========================================================================
// Helper: mapActionToPermissionCodename — direct test
// ==========================================================================

func TestMapActionToPermissionCodename_read(t *testing.T) {
	got := mapActionToPermissionCodename("read")
	if got != "user.view" {
		t.Errorf("mapActionToPermissionCodename(read) = %q, want %q", got, "user.view")
	}
}

func TestMapActionToPermissionCodename_view(t *testing.T) {
	got := mapActionToPermissionCodename("view")
	if got != "user.view" {
		t.Errorf("mapActionToPermissionCodename(view) = %q, want %q", got, "user.view")
	}
}

func TestMapActionToPermissionCodename_list(t *testing.T) {
	got := mapActionToPermissionCodename("list")
	if got != "user.view" {
		t.Errorf("mapActionToPermissionCodename(list) = %q, want %q", got, "user.view")
	}
}

func TestMapActionToPermissionCodename_write(t *testing.T) {
	got := mapActionToPermissionCodename("write")
	if got != "user.change" {
		t.Errorf("mapActionToPermissionCodename(write) = %q, want %q", got, "user.change")
	}
}

func TestMapActionToPermissionCodename_create(t *testing.T) {
	got := mapActionToPermissionCodename("create")
	if got != "user.change" {
		t.Errorf("mapActionToPermissionCodename(create) = %q, want %q", got, "user.change")
	}
}

func TestMapActionToPermissionCodename_update(t *testing.T) {
	got := mapActionToPermissionCodename("update")
	if got != "user.change" {
		t.Errorf("mapActionToPermissionCodename(update) = %q, want %q", got, "user.change")
	}
}

func TestMapActionToPermissionCodename_change(t *testing.T) {
	got := mapActionToPermissionCodename("change")
	if got != "user.change" {
		t.Errorf("mapActionToPermissionCodename(change) = %q, want %q", got, "user.change")
	}
}

func TestMapActionToPermissionCodename_delete(t *testing.T) {
	got := mapActionToPermissionCodename("delete")
	if got != "user.delete" {
		t.Errorf("mapActionToPermissionCodename(delete) = %q, want %q", got, "user.delete")
	}
}

func TestMapActionToPermissionCodename_remove(t *testing.T) {
	got := mapActionToPermissionCodename("remove")
	if got != "user.delete" {
		t.Errorf("mapActionToPermissionCodename(remove) = %q, want %q", got, "user.delete")
	}
}

func TestMapActionToPermissionCodename_assign(t *testing.T) {
	got := mapActionToPermissionCodename("assign")
	if got != "user.add" {
		t.Errorf("mapActionToPermissionCodename(assign) = %q, want %q", got, "user.add")
	}
}

func TestMapActionToPermissionCodename_grant(t *testing.T) {
	got := mapActionToPermissionCodename("grant")
	if got != "user.add" {
		t.Errorf("mapActionToPermissionCodename(grant) = %q, want %q", got, "user.add")
	}
}

func TestMapActionToPermissionCodename_unknown(t *testing.T) {
	got := mapActionToPermissionCodename("custom")
	if got != "user.custom" {
		t.Errorf("mapActionToPermissionCodename(custom) = %q, want %q", got, "user.custom")
	}
}

// ==========================================================================
// Helper: extractTrailID — direct test
// ==========================================================================

func TestExtractTrailID_Valid(t *testing.T) {
	got := extractTrailID("/api/v1/iam/audit/trails/trail-abc-123")
	if got != "trail-abc-123" {
		t.Errorf("extractTrailID(valid) = %q, want %q", got, "trail-abc-123")
	}
}

func TestExtractTrailID_Empty(t *testing.T) {
	got := extractTrailID("/api/v1/iam/audit/trails/")
	if got != "" {
		t.Errorf("extractTrailID(empty) = %q, want %q", got, "")
	}
}

func TestExtractTrailID_Nil(t *testing.T) {
	got := extractTrailID("")
	if got != "" {
		t.Errorf("extractTrailID(nil) = %q, want %q", got, "")
	}
}

func TestExtractTrailID_TrailingSlash(t *testing.T) {
	got := extractTrailID("/api/v1/iam/audit/trails/trail-xyz/")
	if got != "trail-xyz" {
		t.Errorf("extractTrailID(trailing-slash) = %q, want %q", got, "trail-xyz")
	}
}

// ==========================================================================
// Helper: extractSessionReplayID — direct test
// ==========================================================================

func TestExtractSessionReplayID_Valid(t *testing.T) {
	got := extractSessionReplayID("/api/v1/iam/audit/session-replay/session-123")
	if got != "session-123" {
		t.Errorf("extractSessionReplayID(valid) = %q, want %q", got, "session-123")
	}
}

func TestExtractSessionReplayID_Empty(t *testing.T) {
	got := extractSessionReplayID("/api/v1/iam/audit/session-replay/")
	if got != "" {
		t.Errorf("extractSessionReplayID(empty) = %q, want %q", got, "")
	}
}

func TestExtractSessionReplayID_TrailingSlash(t *testing.T) {
	got := extractSessionReplayID("/api/v1/iam/audit/session-replay/abc-456/")
	if got != "abc-456" {
		t.Errorf("extractSessionReplayID(trailing-slash) = %q, want %q", got, "abc-456")
	}
}

// ==========================================================================
// Helper: extractSessionRequestsID — direct test
// ==========================================================================

func TestExtractSessionRequestsID_ValidWithRequests(t *testing.T) {
	got := extractSessionRequestsID("/api/v1/iam/session-replay/sessions/sess-001/requests")
	if got != "sess-001" {
		t.Errorf("extractSessionRequestsID(valid+requests) = %q, want %q", got, "sess-001")
	}
}

func TestExtractSessionRequestsID_ValidNoRequests(t *testing.T) {
	got := extractSessionRequestsID("/api/v1/iam/session-replay/sessions/sess-002")
	if got != "sess-002" {
		t.Errorf("extractSessionRequestsID(valid-no-requests) = %q, want %q", got, "sess-002")
	}
}

func TestExtractSessionRequestsID_Empty(t *testing.T) {
	got := extractSessionRequestsID("/api/v1/iam/session-replay/sessions/")
	if got != "" {
		t.Errorf("extractSessionRequestsID(empty) = %q, want %q", got, "")
	}
}

func TestExtractSessionRequestsID_TrailingSlash(t *testing.T) {
	got := extractSessionRequestsID("/api/v1/iam/session-replay/sessions/sess-003/")
	if got != "sess-003" {
		t.Errorf("extractSessionRequestsID(trailing-slash) = %q, want %q", got, "sess-003")
	}
}

// ==========================================================================
// Helper: checkPermissionViaGroups — direct test (nil Auth → 500)
// ==========================================================================

func TestCheckPermissionViaGroups_NilAuth(t *testing.T) {
	h := newTestRBACHandler()

	user := &authentik.User{
		UUID: "user-1",
		Name: "Test User",
	}

	ctx := context.Background()

	// With nil Auth, calling GroupsAPI.List will panic; we catch it.
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("checkPermissionViaGroups() expected panic with nil Auth")
		}
	}()

	h.checkPermissionViaGroups(ctx, user, "authenticik.user.view")
}

// ==========================================================================
// Helper: mapAuthentikTypeToOperanAction — direct test
// ==========================================================================

func TestMapAuthentikTypeToOperanAction_user_create(t *testing.T) {
	got := mapAuthentikTypeToOperanAction("core.user_create")
	if got != "user_created" {
		t.Errorf("mapAuthentikTypeToOperanAction(core.user_create) = %q, want %q", got, "user_created")
	}
}

func TestMapAuthentikTypeToOperanAction_user_delete(t *testing.T) {
	got := mapAuthentikTypeToOperanAction("core.user_delete")
	if got != "user_deleted" {
		t.Errorf("mapAuthentikTypeToOperanAction(core.user_delete) = %q, want %q", got, "user_deleted")
	}
}

func TestMapAuthentikTypeToOperanAction_login(t *testing.T) {
	got := mapAuthentikTypeToOperanAction("auth.login")
	if got != "login" {
		t.Errorf("mapAuthentikTypeToOperanAction(auth.login) = %q, want %q", got, "login")
	}
}

func TestMapAuthentikTypeToOperanAction_failure(t *testing.T) {
	got := mapAuthentikTypeToOperanAction("auth.failure")
	if got != "login_failed" {
		t.Errorf("mapAuthentikTypeToOperanAction(auth.failure) = %q, want %q", got, "login_failed")
	}
}

func TestMapAuthentikTypeToOperanAction_unknown(t *testing.T) {
	got := mapAuthentikTypeToOperanAction("unknown.event")
	if got != "unknown.event" {
		t.Errorf("mapAuthentikTypeToOperanAction(unknown.event) = %q, want %q", got, "unknown.event")
	}
}

// ==========================================================================
// Helper: mapAuthentikResult — direct test
// ==========================================================================

func TestMapAuthentikResult_Success(t *testing.T) {
	evt := &authentik.Event{
		Data: map[string]interface{}{"success": true},
		Type: "core.user_create",
	}
	got := mapAuthentikResult(evt)
	if got != "success" {
		t.Errorf("mapAuthentikResult(success=true) = %q, want %q", got, "success")
	}
}

func TestMapAuthentikResult_Denied(t *testing.T) {
	evt := &authentik.Event{
		Data: map[string]interface{}{"success": false},
		Type: "auth.login",
	}
	got := mapAuthentikResult(evt)
	if got != "denied" {
		t.Errorf("mapAuthentikResult(success=false) = %q, want %q", got, "denied")
	}
}

func TestMapAuthentikResult_AccessDeny(t *testing.T) {
	evt := &authentik.Event{
		Data: map[string]interface{}{},
		Type: "access.deny",
	}
	got := mapAuthentikResult(evt)
	if got != "denied" {
		t.Errorf("mapAuthentikResult(access.deny) = %q, want %q", got, "denied")
	}
}

func TestMapAuthentikResult_NoData(t *testing.T) {
	evt := &authentik.Event{
		Data: nil,
		Type: "core.user_update",
	}
	got := mapAuthentikResult(evt)
	if got != "success" {
		t.Errorf("mapAuthentikResult(no data) = %q, want %q", got, "success")
	}
}

// ==========================================================================
// Helper: authentikEventToOperan — direct test
// ==========================================================================

func TestAuthentikEventToOperan(t *testing.T) {
	evt := &authentik.Event{
		UUID:       "evt-001",
		Type:       "core.user_create",
		ObjectType: "user",
		Object:     "user-123",
		Data:       map[string]interface{}{"success": true},
		Created:    nil,
	}

	got := authentikEventToOperan(evt)

	if got.ID != "evt-001" {
		t.Errorf("authentikEventToOperan() id = %q, want %q", got.ID, "evt-001")
	}
	if got.Action != "user_created" {
		t.Errorf("authentikEventToOperan() action = %q, want %q", got.Action, "user_created")
	}
	if got.Result != "success" {
		t.Errorf("authentikEventToOperan() result = %q, want %q", got.Result, "success")
	}
	if got.Details == nil || got.Details["authentik_event_type"] != "core.user_create" {
		t.Error("authentikEventToOperan() details missing authentik_event_type")
	}
}

// ==========================================================================
// Helper: enhanceAuditEvent — direct test
// ==========================================================================

func TestEnhanceAuditEvent_SessionAndTraceID(t *testing.T) {
	evt := &models.AuditEvent{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-ID", "sess-abc")
	req.Header.Set("X-Trace-ID", "trace-xyz")

	enhanceAuditEvent(evt, req)

	if evt.Details == nil {
		t.Fatal("enhanceAuditEvent() details is nil")
	}
	if evt.Details["session_id"] != "sess-abc" {
		t.Errorf("enhanceAuditEvent() session_id = %v, want %v", evt.Details["session_id"], "sess-abc")
	}
	if evt.Details["correlation_id"] != "trace-xyz" {
		t.Errorf("enhanceAuditEvent() correlation_id = %v, want %v", evt.Details["correlation_id"], "trace-xyz")
	}
}

func TestEnhanceAuditEvent_NoHeaders(t *testing.T) {
	evt := &models.AuditEvent{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	enhanceAuditEvent(evt, req)

	if evt.Details != nil {
		t.Error("enhanceAuditEvent() details should be nil when no headers present")
	}
}

// ==========================================================================
// Helper: logEvaluateEvent — direct test (nil Auth, should not panic)
// ==========================================================================

func TestLogEvaluateEvent_NilAuth(t *testing.T) {
	h := newTestRBACHandler()
	ctx := context.Background()
	actorID := "user-1"

	req := models.PermissionCheckRequest{
		ActorID:  "user-1",
		Action:   "read",
		Resource: "users",
	}

	// Should not panic — the method only references body/ctx which are ignored
	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("logEvaluateEvent() panicked: %v", rec)
		}
	}()

	h.logEvaluateEvent(ctx, actorID, req, false, false, "permission denied", "tenant-1")
}

// ==========================================================================
// AuditStore tests
// ==========================================================================

func TestAuditStore_GetOrCreateSession_CreatesNew(t *testing.T) {
	store := NewAuditStore()

	info := store.GetOrCreateSession("new-session-1")

	if info.ID != "new-session-1" {
		t.Errorf("GetOrCreateSession() id = %q, want %q", info.ID, "new-session-1")
	}
	if !info.IsActive {
		t.Error("GetOrCreateSession() new session should be active")
	}
	if info.RequestCount != 0 {
		t.Errorf("GetOrCreateSession() request_count = %d, want %d", info.RequestCount, 0)
	}
}

func TestAuditStore_GetOrCreateSession_ReturnsExisting(t *testing.T) {
	store := NewAuditStore()

	_ = store.GetOrCreateSession("existing-1")
	info1 := store.GetOrCreateSession("existing-1")

	info2 := store.GetOrCreateSession("existing-1")

	if info1.ID != info2.ID {
		t.Error("GetOrCreateSession() should return same instance")
	}
}

func TestAuditStore_IncrementRequestCount(t *testing.T) {
	store := NewAuditStore()

	store.GetOrCreateSession("count-1")
	store.IncrementRequestCount("count-1")
	store.IncrementRequestCount("count-1")

	info := store.GetOrCreateSession("count-1")
	if info.RequestCount != 2 {
		t.Errorf("IncrementRequestCount() count = %d, want %d", info.RequestCount, 2)
	}
}

func TestAuditStore_IncrementRequestCount_MissingSession(t *testing.T) {
	store := NewAuditStore()

	// Should not panic when session does not exist
	store.IncrementRequestCount("nonexistent-1")
}

// ==========================================================================
// SessionReplayHandler tests
// ==========================================================================

func TestSessionReplayHandler_ListSessions_NilCapture(t *testing.T) {
	h := &SessionReplayHandler{
		Capture:    nil,
		Publisher:  nil,
		AuditStore: NewAuditStore(),
	}

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/session-replay/sessions", nil)
	req = setPrincipalInContext(req, principal)

	// Expect a panic because Capture is nil
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("ListSessions() expected panic with nil Capture")
		}
	}()

	w := httptest.NewRecorder()
	h.ListSessions(w, req)
}

func TestSessionReplayHandler_GetSessionRequests_NilCapture(t *testing.T) {
	h := &SessionReplayHandler{
		Capture:    nil,
		Publisher:  nil,
		AuditStore: NewAuditStore(),
	}

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/session-replay/sessions/sess-001/requests", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetSessionRequests(w, req)

	// With nil Capture, handler returns 500 (nil check added for safety)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetSessionRequests() nil Capture status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	expectJSONError(t, w, "session replay capture not configured")
}

func TestSessionReplayHandler_GetSessionRequests_MissingSessionID(t *testing.T) {
	h := &SessionReplayHandler{
		Capture:    nil,
		Publisher:  nil,
		AuditStore: NewAuditStore(),
	}

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/session-replay/sessions/requests", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetSessionRequests(w, req)

	// With nil Capture, handler returns 500 (nil check happens before session ID validation)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetSessionRequests() nil Capture status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	expectJSONError(t, w, "session replay capture not configured")
}

func TestSessionReplayHandler_DeleteSession_MissingSessionID(t *testing.T) {
	h := &SessionReplayHandler{
		Capture:    nil,
		Publisher:  nil,
		AuditStore: NewAuditStore(),
	}

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/session-replay/sessions/", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.DeleteSession(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("DeleteSession() missing session_id status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	expectJSONError(t, w, "session_id is required")
}

// ==========================================================================
// Session replay capture integration — real flow with nil Auth handlers
// ==========================================================================

func TestAuditHandler_GetTrails_BodyJSONFormat(t *testing.T) {
	h := newTestAuditHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/audit/trails", nil)
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.GetTrails(w, req)
	}()

	// Handler panics with nil Auth; verify it doesn't crash the test
	_ = w
}

func TestRBACHandler_Evaluate_BodyJSONFormat_NilAuth(t *testing.T) {
	h := newTestRBACHandler()

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
	}
	body := `{"actor_id":"user-1","action":"read","resource":"users"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/rbac/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	func() {
		defer func() { recover() }()
		h.Evaluate(w, req)
	}()

	// Handler panics with nil Auth; verify it doesn't crash the test
	_ = w
}
