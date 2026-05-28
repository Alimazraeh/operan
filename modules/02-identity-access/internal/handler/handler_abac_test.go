package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
