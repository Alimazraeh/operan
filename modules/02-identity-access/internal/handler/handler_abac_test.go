package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/02-identity-access/internal/authentik"
	"github.com/operan/modules/02-identity-access/internal/events"
	"github.com/operan/modules/02-identity-access/internal/middleware"
)

// ---------- helpers ----------

func newABACTestServer(t *testing.T) (*ABACHandler, *httptest.Server) {
	t.Helper()
	auth := &authentik.Client{}
	store := NewABACStore()
	pub := events.NewPublisher("")
	h := NewABACHandler(auth, pub, store)

	mux := http.NewServeMux()

	// POST /abac/policies
	mux.HandleFunc("/abac/policies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreatePolicy(w, r)
		case http.MethodGet:
			h.ListPolicies(w, r)
		default:
			w.WriteHeader(405)
		}
	})

	// /abac/policies/{id}
	mux.HandleFunc("/abac/policies/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetPolicy(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			h.DeletePolicy(w, r)
			return
		}
		w.WriteHeader(405)
	})

	// POST /abac/evaluate
	mux.HandleFunc("/abac/evaluate", func(w http.ResponseWriter, r *http.Request) {
		h.Evaluate(w, r)
	})

	server := httptest.NewServer(mux)
	return h, server
}

func tenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, middleware.TenantIDKey, tenantID)
}

func setJSONBody(r *http.Request, v interface{}) {
	buf, _ := json.Marshal(v)
	r.Body = io.NopCloser(bytes.NewReader(buf))
	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = int64(len(buf))
}

func decodeJSON(t *testing.T, body []byte, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// ---------- ABAC tenant isolation tests ----------

func TestABACCreatePolicy_TenantIsolated(t *testing.T) {
	h, _ := newABACTestServer(t)
	tenantID := "tenant-A"

	policy := ABACPolicyCreateRequest{
		Name:     "allow-internal",
		Resource: "database",
		Action:   "read",
		Rule:     "ip",
		Conditions: map[string]interface{}{
			"allowed_cidrs": []interface{}{"10.0.0.0/8"},
		},
		Effect: "allow",
	}

	req, _ := http.NewRequest(http.MethodPost, "/abac/policies", nil)
	setJSONBody(req, policy)
	req = req.WithContext(tenantContext(req.Context(), tenantID))

	rec := httptest.NewRecorder()
	h.CreatePolicy(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("CreatePolicy code = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// Verify the policy was stored
	storedPolicies := h.Store.ListByTenant(tenantID)
	if len(storedPolicies) != 1 {
		t.Errorf("Store.ListByTenant(%s) got %d policies, want 1", tenantID, len(storedPolicies))
	}
	if storedPolicies[0].Name != "allow-internal" {
		t.Errorf("Policy name = %q, want %q", storedPolicies[0].Name, "allow-internal")
	}
}

func TestABACListPolicies_TenantIsolated(t *testing.T) {
	h, _ := newABACTestServer(t)

	// Create policies for two tenants via direct store access
	h.Store.Create("tenant-A", ABACPolicy{ID: "p-a1", Name: "Policy A1", Resource: "db", Action: "read"})
	h.Store.Create("tenant-A", ABACPolicy{ID: "p-a2", Name: "Policy A2", Resource: "db", Action: "write"})
	h.Store.Create("tenant-B", ABACPolicy{ID: "p-b1", Name: "Policy B1", Resource: "api", Action: "read"})

	// tenant-A should see only its policies
	req := httptest.NewRequest(http.MethodGet, "/abac/policies", nil)
	req = req.WithContext(tenantContext(req.Context(), "tenant-A"))
	rec := httptest.NewRecorder()
	h.ListPolicies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListPolicies code = %d, want 200", rec.Code)
	}

	var policies []ABACPolicy
	decodeJSON(t, rec.Body.Bytes(), &policies)
	if len(policies) != 2 {
		t.Errorf("tenant-A got %d policies, want 2", len(policies))
	}

	// tenant-B should see only its policies
	req = httptest.NewRequest(http.MethodGet, "/abac/policies", nil)
	req = req.WithContext(tenantContext(req.Context(), "tenant-B"))
	rec = httptest.NewRecorder()
	h.ListPolicies(rec, req)

	decodeJSON(t, rec.Body.Bytes(), &policies)
	if len(policies) != 1 {
		t.Errorf("tenant-B got %d policies, want 1", len(policies))
	}
}

// ---------- IP policy CIDR evaluation tests ----------

func TestEvaluateIPPolicy(t *testing.T) {
	tests := []struct {
		name       string
		attrs      map[string]interface{}
		conditions map[string]interface{}
		wantPass   bool
	}{
		{
			name:       "no client IP — pass by default",
			attrs:      map[string]interface{}{},
			conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8"}},
			wantPass:   true,
		},
		{
			name:       "invalid client IP — fail",
			attrs:      map[string]interface{}{"client_ip": "not-an-ip"},
			conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8"}},
			wantPass:   false,
		},
		{
			name:       "IP in allowed CIDR",
			attrs:      map[string]interface{}{"client_ip": "10.0.2.5"},
			conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8"}},
			wantPass:   true,
		},
		{
			name:       "IP not in allowed CIDR — fail",
			attrs:      map[string]interface{}{"client_ip": "192.168.1.1"},
			conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8"}},
			wantPass:   false,
		},
		{
			name: "IP in denied CIDR — fail (deny wins)",
			attrs: map[string]interface{}{"client_ip": "10.0.1.5"},
			conditions: map[string]interface{}{
				"allowed_cidrs": []interface{}{"10.0.0.0/8"},
				"denied_cidrs":  []interface{}{"10.0.1.0/24"},
			},
			wantPass: false,
		},
		{
			name: "IP allowed but not denied",
			attrs: map[string]interface{}{"client_ip": "10.0.2.5"},
			conditions: map[string]interface{}{
				"allowed_cidrs": []interface{}{"10.0.0.0/8"},
				"denied_cidrs":  []interface{}{"10.0.1.0/24"},
			},
			wantPass: true,
		},
		{
			name:       "only denied list — IP not denied but no allow list — fail",
			attrs:      map[string]interface{}{"client_ip": "192.168.1.1"},
			conditions: map[string]interface{}{"denied_cidrs": []interface{}{"10.0.1.0/24"}},
			wantPass:   false,
		},
		{
			name:       "only denied list — IP in denied — fail",
			attrs:      map[string]interface{}{"client_ip": "10.0.1.5"},
			conditions: map[string]interface{}{"denied_cidrs": []interface{}{"10.0.1.0/24"}},
			wantPass:   false,
		},
		{
			name: "multiple allowed CIDRs — matches second",
			attrs: map[string]interface{}{"client_ip": "172.16.0.1"},
			conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8", "172.16.0.0/12"}},
			wantPass:   true,
		},
		{
			name: "IPv6 in allowed CIDR",
			attrs: map[string]interface{}{"client_ip": "2001:db8::1"},
			conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"2001:db8::/32"}},
			wantPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateIPPolicy(tt.attrs, tt.conditions)
			if got != tt.wantPass {
				t.Errorf("evaluateIPPolicy() = %v, want %v", got, tt.wantPass)
			}
		})
	}
}

// ---------- Time policy evaluation tests ----------

func TestEvaluateTimePolicy(t *testing.T) {
	attrs := map[string]interface{}{}

	conditions := map[string]interface{}{
		"start_hour": float64(9),
		"end_hour":   float64(17),
	}

	tests := []struct {
		name string
		hour float64
		want bool
	}{
		{"midnight", 0, false},
		{"8am", 8, false},
		{"9am", 9, true},
		{"noon", 12, true},
		{"5pm", 17, true},
		{"6pm", 18, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs["time_of_day"] = tt.hour
			got := evaluateTimePolicy(attrs, conditions)
			if got != tt.want {
				t.Errorf("evaluateTimePolicy() hour=%v = %v, want %v", tt.hour, got, tt.want)
			}
		})
	}
}

// ---------- ABAC evaluation endpoint tests ----------

func TestABACEvaluateEndpoint_RBACNotConfigured(t *testing.T) {
	h, _ := newABACTestServer(t)
	tenantID := "test-tenant"
	userID := "user-123"

	// Create a matching IP policy
	policy := ABACPolicy{
		ID:         "pol-001",
		Name:       "allow-internal-db",
		Resource:   "database",
		Action:     "read",
		Rule:       "ip",
		Conditions: map[string]interface{}{"allowed_cidrs": []interface{}{"10.0.0.0/8"}},
		Effect:     "allow",
	}
	if err := h.Store.Create(tenantID, policy); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Evaluate with matching IP — but Auth/UsersAPI not configured, so RBAC fails gracefully
	reqBody := map[string]interface{}{
		"actor_id":   userID,
		"resource":   "database",
		"action":     "read",
		"attributes": map[string]interface{}{"client_ip": "10.0.2.5"},
	}
	req, _ := http.NewRequest(http.MethodPost, "/abac/evaluate", nil)
	setJSONBody(req, reqBody)
	req = req.WithContext(tenantContext(req.Context(), tenantID))
	rec := httptest.NewRecorder()
	h.Evaluate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Evaluate code = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var result ABACEvaluateResult
	decodeJSON(t, rec.Body.Bytes(), &result)

	// RBAC will fail (Auth not configured), so allowed=false is expected
	if result.Allowed {
		t.Errorf("Expected allowed=false (RBAC not configured), got true")
	}
	if result.Reason != "RBAC service not configured" {
		t.Errorf("Expected reason 'RBAC service not configured', got %q", result.Reason)
	}
}

// ---------- Validate tests ----------

func TestABACPolicyCreateRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ABACPolicyCreateRequest
		wantErr bool
	}{
		{
			name:    "missing name",
			req:     ABACPolicyCreateRequest{Resource: "db", Action: "read", Rule: "ip", Conditions: map[string]interface{}{}},
			wantErr: true,
		},
		{
			name:    "missing resource",
			req:     ABACPolicyCreateRequest{Name: "p1", Action: "read", Rule: "ip", Conditions: map[string]interface{}{}},
			wantErr: true,
		},
		{
			name:    "missing action",
			req:     ABACPolicyCreateRequest{Name: "p1", Resource: "db", Rule: "ip", Conditions: map[string]interface{}{}},
			wantErr: true,
		},
		{
			name:    "missing rule",
			req:     ABACPolicyCreateRequest{Name: "p1", Resource: "db", Action: "read", Conditions: map[string]interface{}{}},
			wantErr: true,
		},
		{
			name:    "complete policy",
			req:     ABACPolicyCreateRequest{Name: "p1", Resource: "db", Action: "read", Rule: "ip", Conditions: map[string]interface{}{"a": 1}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestABACEvaluateRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ABACEvaluateRequest
		wantErr bool
	}{
		{
			name:    "missing actor_id",
			req:     ABACEvaluateRequest{Resource: "db", Action: "read"},
			wantErr: true,
		},
		{
			name:    "missing action",
			req:     ABACEvaluateRequest{ActorID: "u1", Resource: "db"},
			wantErr: true,
		},
		{
			name:    "missing resource",
			req:     ABACEvaluateRequest{ActorID: "u1", Action: "read"},
			wantErr: true,
		},
		{
			name:    "complete request",
			req:     ABACEvaluateRequest{ActorID: "u1", Resource: "db", Action: "read"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- ABACStore tests ----------

func TestABACStore_TenantIsolation(t *testing.T) {
	store := NewABACStore()

	store.Create("tenant-1", ABACPolicy{ID: "p1", Name: "Policy 1"})
	store.Create("tenant-2", ABACPolicy{ID: "p2", Name: "Policy 2"})

	policies1 := store.ListByTenant("tenant-1")
	if len(policies1) != 1 || policies1[0].ID != "p1" {
		t.Errorf("tenant-1 got %d policies, want 1", len(policies1))
	}

	policies2 := store.ListByTenant("tenant-2")
	if len(policies2) != 1 || policies2[0].ID != "p2" {
		t.Errorf("tenant-2 got %d policies, want 1", len(policies2))
	}

	policies0 := store.ListByTenant("tenant-3")
	if len(policies0) != 0 {
		t.Errorf("tenant-3 got %d policies, want 0", len(policies0))
	}
}

func TestABACStore_EvaluateByResource(t *testing.T) {
	store := NewABACStore()

	store.Create("t1", ABACPolicy{ID: "p1", Resource: "db", Action: "read"})
	store.Create("t1", ABACPolicy{ID: "p2", Resource: "db", Action: "write"})
	store.Create("t1", ABACPolicy{ID: "p3", Resource: "api", Action: "read"})

	matches := store.EvaluateByResource("t1", "db", "read")
	if len(matches) != 1 || matches[0].ID != "p1" {
		t.Errorf("EvaluateByResource(db, read) got %d matches, want 1: %+v", len(matches), matches)
	}

	matches = store.EvaluateByResource("t1", "db", "write")
	if len(matches) != 1 || matches[0].ID != "p2" {
		t.Errorf("EvaluateByResource(db, write) got %d matches, want 1", len(matches))
	}

	matches = store.EvaluateByResource("t1", "api", "read")
	if len(matches) != 1 || matches[0].ID != "p3" {
		t.Errorf("EvaluateByResource(api, read) got %d matches, want 1", len(matches))
	}
}

func TestABACStore_CreateValidation(t *testing.T) {
	store := NewABACStore()

	err := store.Create("", ABACPolicy{ID: "p1"})
	if err == nil {
		t.Error("Expected error for empty tenantID")
	}

	err = store.Create("t1", ABACPolicy{})
	if err == nil {
		t.Error("Expected error for empty policy ID")
	}
}

func TestABACStore_GetByID(t *testing.T) {
	store := NewABACStore()
	store.Create("t1", ABACPolicy{ID: "p1", Name: "Test"})

	policy, ok := store.GetByID("t1", "p1")
	if !ok {
		t.Fatal("Expected policy to exist")
	}
	if policy.Name != "Test" {
		t.Errorf("Policy name = %q, want %q", policy.Name, "Test")
	}

	_, ok = store.GetByID("t1", "nonexistent")
	if ok {
		t.Error("Expected policy to not exist")
	}
}

func TestABACStore_DeleteByID(t *testing.T) {
	store := NewABACStore()
	store.Create("t1", ABACPolicy{ID: "p1", Name: "Test"})

	deleted := store.DeleteByID("t1", "p1")
	if !deleted {
		t.Error("Expected DeleteByID to return true")
	}

	policies := store.ListByTenant("t1")
	if len(policies) != 0 {
		t.Errorf("After delete, got %d policies, want 0", len(policies))
	}

	deleted = store.DeleteByID("t1", "nonexistent")
	if deleted {
		t.Error("Expected DeleteByID to return false for non-existent policy")
	}
}

// ---------- Middleware context tests ----------

func TestTenantContextKeys(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.TenantIDKey, "test-tenant")

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "test-tenant" {
		t.Errorf("GetTenantID() = %q, want %q", tenantID, "test-tenant")
	}
}

func TestUserIDContext(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.UserIDKey, "user-123")

	userID := middleware.GetUserID(ctx)
	if userID != "user-123" {
		t.Errorf("GetUserID() = %q, want %q", userID, "user-123")
	}
}

// ---------- extractPolicyID tests ----------

func TestExtractPolicyID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/iam/abac/policies/pol-123", "pol-123"},
		{"/api/v1/iam/abac/policies/pol-456/", "pol-456"},
		{"/api/v1/iam/abac/policies/", ""},
		{"/api/v1/iam/abac/policies", ""},
		{"/api/v1/iam/abac/policies/pol/sub", "pol/sub"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPolicyID(tt.path)
			if got != tt.want {
				t.Errorf("extractPolicyID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ---------- ABACHandler.Evaluate tests ----------

func TestABACEvaluateEndpoint_InvalidJSON(t *testing.T) {
	store := NewABACStore()
	h := NewABACHandler(nil, nil, store)

	payload := `{"actor_id":"user-1"`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/evaluate", strings.NewReader(payload))
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
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACEvaluateEndpoint_MissingFields(t *testing.T) {
	store := NewABACStore()
	h := NewABACHandler(nil, nil, store)

	// Missing actor_id
	payload := `{"action":"read","resource":"api:v1:data"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/evaluate", strings.NewReader(payload))
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
	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Evaluate() missing actor_id status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

// ---------- ABACHandler.GetPolicy tests ----------

func TestABACHandlerGetPolicyNotFound(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/abac/policies/pol-nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetPolicy(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GetPolicy() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestABACHandlerGetPolicyMissingID(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/abac/policies/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetPolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetPolicy() missing ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerGetPolicySuccess(t *testing.T) {
	store := NewABACStore()

	// Insert a policy first
	policy := ABACPolicy{
		ID:          "pol-123",
		TenantID:    "tenant-1",
		Name:        "Test Policy",
		Resource:    "api:v1:data",
		Action:      "read",
		Rule:        "time",
		Conditions:  map[string]interface{}{},
		Effect:      "allow",
		CreatedAt:   "2025-01-01T00:00:00Z",
	}
	if err := store.Create("tenant-1", policy); err != nil {
		t.Fatalf("failed to create test policy: %v", err)
	}

	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/iam/abac/policies/pol-123", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.GetPolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetPolicy() success status = %v, want %v", w.Code, http.StatusOK)
		return
	}

	var result ABACPolicy
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("GetPolicy() response not valid JSON: %v", err)
	}
	if result.ID != "pol-123" {
		t.Errorf("GetPolicy() ID = %q, want %q", result.ID, "pol-123")
	}
	if result.Name != "Test Policy" {
		t.Errorf("GetPolicy() Name = %q, want %q", result.Name, "Test Policy")
	}
}

// ---------- ABACHandler.DeletePolicy tests ----------

func TestABACHandlerDeletePolicyNotFound(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/abac/policies/pol-nonexistent", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.DeletePolicy(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("DeletePolicy() not found status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestABACHandlerDeletePolicyMissingID(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/abac/policies/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.DeletePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("DeletePolicy() missing ID status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerDeletePolicySuccess(t *testing.T) {
	store := NewABACStore()

	policy := ABACPolicy{
		ID:          "pol-del-1",
		TenantID:    "tenant-1",
		Name:        "Delete Me",
		Resource:    "api:v1:data",
		Action:      "write",
		Rule:        "custom",
		Conditions:  map[string]interface{}{},
		Effect:      "deny",
		CreatedAt:   "2025-01-01T00:00:00Z",
	}
	if err := store.Create("tenant-1", policy); err != nil {
		t.Fatalf("failed to create test policy: %v", err)
	}

	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/iam/abac/policies/pol-del-1", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")

	principal := &middleware.JWTToken{
		Subject:  "user-1",
		UserType: "user",
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
	}
	req = setPrincipalInContext(req, principal)

	w := httptest.NewRecorder()
	h.DeletePolicy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DeletePolicy() success status = %v, want %v", w.Code, http.StatusOK)
	}

	// Verify deletion
	_, ok := store.GetByID("tenant-1", "pol-del-1")
	if ok {
		t.Error("DeletePolicy() policy should not exist after deletion")
	}
}

// ---------- evaluateOwnershipPolicy tests ----------

func TestEvaluateOwnershipPolicy(t *testing.T) {
	tests := []struct {
		name     string
		attrs    map[string]interface{}
		cond     map[string]interface{}
		wantPass bool
	}{
		{
			name:     "matching ownership",
			attrs:    map[string]interface{}{"resource_owner": "alice", "actor_id": "alice"},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "non-matching ownership",
			attrs:    map[string]interface{}{"resource_owner": "bob", "actor_id": "alice"},
			cond:     map[string]interface{}{},
			wantPass: false,
		},
		{
			name:     "missing resource_owner — pass",
			attrs:    map[string]interface{}{"actor_id": "alice"},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "missing actor_id — pass",
			attrs:    map[string]interface{}{"resource_owner": "alice"},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "empty strings — pass",
			attrs:    map[string]interface{}{"resource_owner": "", "actor_id": ""},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateOwnershipPolicy(tt.attrs, tt.cond)
			if got != tt.wantPass {
				t.Errorf("evaluateOwnershipPolicy(%v, %v) = %v, want %v",
					tt.attrs, tt.cond, got, tt.wantPass)
			}
		})
	}
}

// ---------- evaluateDepartmentPolicy tests ----------

func TestEvaluateDepartmentPolicy(t *testing.T) {
	tests := []struct {
		name     string
		attrs    map[string]interface{}
		cond     map[string]interface{}
		wantPass bool
	}{
		{
			name:     "matching department",
			attrs:    map[string]interface{}{"resource_department": "engineering", "actor_department": "engineering"},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "mismatched department",
			attrs:    map[string]interface{}{"resource_department": "engineering", "actor_department": "marketing"},
			cond:     map[string]interface{}{},
			wantPass: false,
		},
		{
			name:     "missing resource_department — pass",
			attrs:    map[string]interface{}{"actor_department": "engineering"},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "missing actor_department — pass",
			attrs:    map[string]interface{}{"resource_department": "engineering"},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "empty strings — pass",
			attrs:    map[string]interface{}{"resource_department": "", "actor_department": ""},
			cond:     map[string]interface{}{},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateDepartmentPolicy(context.Background(), tt.attrs, tt.cond)
			if got != tt.wantPass {
				t.Errorf("evaluateDepartmentPolicy(%v, %v) = %v, want %v",
					tt.attrs, tt.cond, got, tt.wantPass)
			}
		})
	}
}

// ---------- evaluateCustomPolicy tests ----------

func TestEvaluateCustomPolicy(t *testing.T) {
	attrs := map[string]interface{}{"key": "value"}
	cond := map[string]interface{}{"rule": "test"}

	got := evaluateCustomPolicy(context.Background(), attrs, cond)
	if !got {
		t.Errorf("evaluateCustomPolicy() = %v, want true (default pass)", got)
	}
}

// ---------- evaluateABAC tests ----------

func TestEvaluateABACNoMatchingPolicies(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	ctx := context.Background()
	attrs := &ABACEvaluateRequest{
		ActorID:  "user-1",
		Action:   "read",
		Resource: "api:v1:data",
	}

	results := h.evaluateABAC(ctx, "tenant-1", attrs, nil)
	if len(results) != 0 {
		t.Errorf("evaluateABAC() returned %d results, want 0 (no matching policies)", len(results))
	}
}

func TestEvaluateABACWithPolicies(t *testing.T) {
	store := NewABACStore()

	// Create a time policy
	policy := ABACPolicy{
		ID:          "pol-time-1",
		TenantID:    "tenant-1",
		Name:        "Business Hours Only",
		Resource:    "api:v1:data",
		Action:      "write",
		Rule:        "time",
		Conditions:  map[string]interface{}{"allowed_hours": []interface{}{9, 10, 11, 12, 13, 14, 15, 16, 17}},
		Effect:      "allow",
		CreatedAt:   "2025-01-01T00:00:00Z",
	}
	if err := store.Create("tenant-1", policy); err != nil {
		t.Fatalf("failed to create test policy: %v", err)
	}

	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	ctx := context.Background()
	attrs := &ABACEvaluateRequest{
		ActorID:  "user-1",
		Action:   "write",
		Resource: "api:v1:data",
		Attributes: map[string]interface{}{
			"current_hour": 14,
		},
	}

	results := h.evaluateABAC(ctx, "tenant-1", attrs, nil)
	if len(results) != 1 {
		t.Fatalf("evaluateABAC() returned %d results, want 1", len(results))
	}
	if !results[0].Passed {
		t.Errorf("evaluateABAC() policy %q should pass at hour 14", results[0].Name)
	}
}

func TestEvaluateABACDenyPolicy(t *testing.T) {
	store := NewABACStore()

	// Create a deny policy that always passes
	policy := ABACPolicy{
		ID:          "pol-deny-1",
		TenantID:    "tenant-1",
		Name:        "Deny All",
		Resource:    "api:v1:admin",
		Action:      "write",
		Rule:        "custom",
		Conditions:  map[string]interface{}{},
		Effect:      "deny",
		CreatedAt:   "2025-01-01T00:00:00Z",
	}
	if err := store.Create("tenant-1", policy); err != nil {
		t.Fatalf("failed to create test policy: %v", err)
	}

	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	ctx := context.Background()
	attrs := &ABACEvaluateRequest{
		ActorID:  "user-1",
		Action:   "write",
		Resource: "api:v1:admin",
	}

	results := h.evaluateABAC(ctx, "tenant-1", attrs, nil)
	if len(results) != 1 {
		t.Fatalf("evaluateABAC() returned %d results, want 1", len(results))
	}
	if results[0].Passed {
		t.Errorf("evaluateABAC() deny policy %q should NOT pass (effect=deny inverts result)", results[0].Name)
	}
}

// ---------- ABACHandler.CreatePolicy tests ----------

func TestABACHandlerCreatePolicyMissingName(t *testing.T) {
	h := newTestABACHandler()

	// Missing name
	payload := `{"resource":"api:v1:data","action":"read","rule":"time","conditions":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePolicy() missing name status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerCreatePolicyMissingResource(t *testing.T) {
	h := newTestABACHandler()

	// Missing resource
	payload := `{"name":"Test","action":"read","rule":"time","conditions":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePolicy() missing resource status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerCreatePolicyMissingAction(t *testing.T) {
	h := newTestABACHandler()

	// Missing action
	payload := `{"name":"Test","resource":"api:v1:data","rule":"time","conditions":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePolicy() missing action status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerCreatePolicyMissingRule(t *testing.T) {
	h := newTestABACHandler()

	// Missing rule
	payload := `{"name":"Test","resource":"api:v1:data","action":"read","conditions":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePolicy() missing rule status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerCreatePolicyMissingConditions(t *testing.T) {
	h := newTestABACHandler()

	// Missing conditions
	payload := `{"name":"Test","resource":"api:v1:data","action":"read","rule":"time"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePolicy() missing conditions status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerCreatePolicyInvalidJSON(t *testing.T) {
	h := newTestABACHandler()

	payload := `{"name":"Test"`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("CreatePolicy() invalid JSON status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestABACHandlerCreatePolicySuccess(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	// Valid policy
	payload := `{"name":"Test Policy","resource":"api:v1:data","action":"read","rule":"time","conditions":{"allowed_hours":[9,10,11,12,13,14,15,16,17]},"effect":"allow"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("CreatePolicy() success status = %v, want %v", w.Code, http.StatusCreated)
		return
	}

	var result ABACPolicy
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("CreatePolicy() response not valid JSON: %v", err)
	}
	if result.Name != "Test Policy" {
		t.Errorf("CreatePolicy() Name = %q, want %q", result.Name, "Test Policy")
	}
	if result.Resource != "api:v1:data" {
		t.Errorf("CreatePolicy() Resource = %q, want %q", result.Resource, "api:v1:data")
	}
	if result.Effect != "allow" {
		t.Errorf("CreatePolicy() Effect = %q, want %q", result.Effect, "allow")
	}
	if result.Effect == "" {
		t.Error("CreatePolicy() default effect should be 'allow'")
	}
}

func TestABACHandlerCreatePolicyDefaultEffect(t *testing.T) {
	store := NewABACStore()
	h := &ABACHandler{
		RBACHandler: &RBACHandler{Auth: nil},
		Auth:        nil,
		Store:       store,
		Pub:         nil,
	}

	// No effect specified — should default to "allow"
	payload := `{"name":"Test Default","resource":"api:v1:data","action":"read","rule":"custom","conditions":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/abac/policies", strings.NewReader(payload))
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
	h.CreatePolicy(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("CreatePolicy() default effect status = %v, want %v", w.Code, http.StatusCreated)
		return
	}

	var result ABACPolicy
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("CreatePolicy() response not valid JSON: %v", err)
	}
	if result.Effect != "allow" {
		t.Errorf("CreatePolicy() default effect = %q, want %q", result.Effect, "allow")
	}
}
