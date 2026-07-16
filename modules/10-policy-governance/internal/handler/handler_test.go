package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operan/policy-governance/internal/ctxkeys"
	"github.com/operan/policy-governance/internal/engine"
	"github.com/operan/policy-governance/internal/middleware"
	"github.com/operan/policy-governance/internal/store"
)

const testJWTSecret = "test-secret-key-for-jwt-validation"
const testIssuer = "operan-tenant-control-plane"

func makeTestJWT(tenantID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   tenantID,
		"iss":   testIssuer,
		"roles": []interface{}{"admin", "editor"},
		"exp":   jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	})
	s, _ := token.SignedString([]byte(testJWTSecret))
	return s
}

// ---- Health Endpoint ----

func TestHealth_Unauthenticated(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

// ---- Middleware: Auth ----

func TestAuthRequired_PoliciesCreate(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)

	// Build a minimal route with auth middleware
	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Post("/policies", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]string{"created": "true"})
	})

	// No auth header
	body := strings.NewReader(`{"group_id":"test","name":"Test"}`)
	req := httptest.NewRequest(http.MethodPost, "/policies", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestTenantIsolation_Mismatch(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)

	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Get("/policies", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/policies", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-mismatch")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTenantIsolation_MissingHeader(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)

	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Get("/policies", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/policies", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- Policy Validation ----

func TestCreatePolicy_MissingGroupName(t *testing.T) {
	// Test the handler directly
	h := &PolicyHandler{}

	body := strings.NewReader(`{"name":"Test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies", body)
	req.Header.Set("Content-Type", "application/json")

	// Set tenant in context
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePolicy_InvalidAction(t *testing.T) {
	h := &PolicyHandler{}

	body := strings.NewReader(`{"group_id":"test","name":"Test","action":"invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePolicy_InvalidScope(t *testing.T) {
	h := &PolicyHandler{}

	body := strings.NewReader(`{"group_id":"test","name":"Test","action":"allow","scope":"invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePolicy_InvalidResourceType(t *testing.T) {
	h := &PolicyHandler{}

	body := strings.NewReader(`{"group_id":"test","name":"Test","action":"allow","scope":"global","resource_type":"invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePolicy_InvalidEffect(t *testing.T) {
	h := &PolicyHandler{}

	body := strings.NewReader(`{"group_id":"test","name":"Test","action":"allow","scope":"global","resource_type":"all","effect":"invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- Evaluate Endpoint ----

func TestEvaluatePolicy_MissingResource(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"action_type":"send"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEvaluatePolicy_MissingActionType(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"send_email"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEvaluatePolicy_Success_DefaultDeny(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	// No matching active policies → default deny
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "group_id", "name", "description", "action",
			"scope", "resource_type", "resource_target", "condition_expression",
			"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
		}))

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"send_email","action_type":"send"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "test-request-id")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.False(t, result["allowed"].(bool))
	assert.Equal(t, "deny", result["action"])
	assert.Contains(t, result["reason"], "default deny")
}

// ---- Policy Group Validation ----

func TestCreateGroup_MissingName(t *testing.T) {
	pool, _ := pgxmock.NewPool()
	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policy-groups", body)
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreateGroup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- Audit Handler ----

func TestListAudits_MiddlewareRequired(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)

	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Get("/audit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	// No auth
	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Ensure context import is used
var _ = json.Marshal
var _ = bytes.Buffer{}

// ---- Middleware writeJSON body verification ----

func TestMiddlewareWriteJSON_Body(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error":   "unauthorized",
			"message": "missing token",
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "unauthorized")
	assert.Contains(t, w.Body.String(), "missing Authorization")
}

func TestMiddlewareWriteJSON_BodyTenantMismatch(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	req.Header.Set("X-Tenant-ID", "tenant-2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "tenant-mismatch")
}

func TestMiddlewareWriteJSON_BodyMissingTenant(t *testing.T) {
	v := middleware.NewAuthValidator(testJWTSecret, testIssuer)
	r := chi.NewRouter()
	r.Use(middleware.JWTMiddleware(v))
	r.Use(middleware.TenantMiddleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+makeTestJWT("tenant-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "X-Tenant-ID")
}

// ---- EvaluatePolicy: real policy matching ----

func TestEvaluatePolicy_AllowPolicyMatch(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	// Return a single allow policy — should produce "allowed"
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(uuid.New().String(), "tenant-1", "group-1", "allow-email",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"send_email","action_type":"send"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.True(t, result["allowed"].(bool))
	assert.Equal(t, "allow", result["action"])
	assert.Equal(t, "allow-email", result["policy_name"])
}

func TestEvaluatePolicy_DenyPolicyMatch(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(uuid.New().String(), "tenant-1", "group-1", "deny-spend",
		nil, "deny", "global", "all", nil, nil, "enforce", 90, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"run_model","action_type":"execute"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-2")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.False(t, result["allowed"].(bool))
	assert.Equal(t, "deny", result["action"])
	assert.Equal(t, "deny-spend", result["policy_name"])
}

func TestEvaluatePolicy_ProxyPolicyMatch(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(uuid.New().String(), "tenant-1", "group-1", "require-approval",
		nil, "proxy", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"deploy","action_type":"provision"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-3")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "proxy", result["action"])
	assert.Equal(t, "require-approval", result["policy_name"])
}

func TestEvaluatePolicy_PriorityDenyOverridesAllow(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	// DB returns rows ORDER BY priority DESC — high-priority first
	id1, id2 := uuid.New(), uuid.New()
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id2.String(), "tenant-1", "group-1", "high-deny",
		nil, "deny", "global", "all", nil, nil, "enforce", 90, true, nil,
		time.Now(), time.Now()).
		AddRow(id1.String(), "tenant-1", "group-1", "low-allow",
		nil, "allow", "global", "all", nil, nil, "enforce", 10, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"data_export","action_type":"download"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-4")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.False(t, result["allowed"].(bool))
	assert.Equal(t, "deny", result["action"])
	assert.Equal(t, "high-deny", result["policy_name"])
}

func TestEvaluatePolicy_ScopeAgent(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(uuid.New().String(), "tenant-1", "group-1", "agent-allow",
		nil, "allow", "agent", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"run_model","action_type":"execute","agent_id":"agent-99","department_id":"dept-5"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-5")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.True(t, result["allowed"].(bool))
	assert.Equal(t, "agent-allow", result["policy_name"])
}

// ---- ListPolicies ----

func TestListPolicies_Success(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	id := uuid.New()
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), "tenant-1", "g1", "Test Policy",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies.*LIMIT").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	pool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/policies?page=1&page_size=10", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ListPolicies(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body["policies"], 1)
	assert.Equal(t, "Test Policy", body["policies"].([]interface{})[0].(map[string]interface{})["name"])
	assert.Equal(t, float64(1), body["total"])
}

func TestListPolicies_WithScopeFilter(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(uuid.New().String(), "tenant-1", "g1", "Global Only",
		nil, "deny", "global", "all", nil, nil, "enforce", 80, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies.*scope").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	pool.ExpectQuery("SELECT COUNT.*scope").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(countRows)

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/policies?scope=global&page=1&page_size=10", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ListPolicies(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, 1, len(body["policies"].([]interface{})))
}

// ---- GetPolicy ----

func TestGetPolicy_Success(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	id := uuid.New()
	row := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), "tenant-1", "g1", "GetMe",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(row)

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Get("/policies/{id}", h.GetPolicy)
	req := httptest.NewRequest(http.MethodGet, "/policies/"+id.String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "GetMe", body["name"])
}

func TestGetPolicy_NotFound(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	pool.ExpectQuery("SELECT.*FROM policies WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Get("/policies/{id}", h.GetPolicy)
	req := httptest.NewRequest(http.MethodGet, "/policies/"+uuid.New().String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not-found")
}

func TestGetPolicy_InvalidID(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Get("/policies/{id}", h.GetPolicy)
	req := httptest.NewRequest(http.MethodGet, "/policies/not-a-uuid", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- UpdatePolicy ----

func TestUpdatePolicy_Success(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	id := uuid.New()
	// First, GetByID to fetch existing policy
	row := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), "tenant-1", "g1", "Old Name",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(row)
	// Then, Update with 11 args (name, description, action, scope, resource_type, resource_target, effect, priority, is_active, id, tenant_id)
	pool.ExpectQuery("UPDATE policies SET").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}).AddRow(time.Now()))

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Patch("/policies/{id}", h.UpdatePolicy)
	body := strings.NewReader(`{"name":"New Name","action":"deny"}`)
	req := httptest.NewRequest(http.MethodPatch, "/policies/"+id.String(), body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "New Name", result["name"])
	assert.Equal(t, "deny", result["action"])
}

func TestUpdatePolicy_NotFound(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	pool.ExpectQuery("SELECT.*FROM policies WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Patch("/policies/{id}", h.UpdatePolicy)
	req := httptest.NewRequest(http.MethodPatch, "/policies/"+uuid.New().String(), strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- DeletePolicy ----

func TestDeletePolicy_Success(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	id := uuid.New()
	pool.ExpectExec("UPDATE policies SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Delete("/policies/{id}", h.DeletePolicy)
	req := httptest.NewRequest(http.MethodDelete, "/policies/"+id.String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeletePolicy_NotFound(t *testing.T) {
	pool, err := newTestPolicyStore(t); require.NoError(t, err)
	pool.ExpectExec("UPDATE policies SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	policyStore := store.NewPolicyStore(pool)
	h := NewPolicyHandler(policyStore)

	r := chi.NewRouter()
	r.Delete("/policies/{id}", h.DeletePolicy)
	req := httptest.NewRequest(http.MethodDelete, "/policies/"+uuid.New().String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- CreateGroup success ----

func TestCreateGroup_Success(t *testing.T) {
	pool, err := newTestGroupStore(t); require.NoError(t, err)
	pool.ExpectQuery("INSERT INTO policy_groups").
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), time.Now(), time.Now()))

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	body := strings.NewReader(`{"name":"New Group","priority":75}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policy-groups", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreateGroup(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "New Group", result["name"])
	assert.NotEmpty(t, result["id"])
}

// ---- ListGroups ----

func TestListGroups_Success(t *testing.T) {
	pool, err := newTestGroupStore(t); require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "name", "description", "priority", "is_active",
		"metadata", "created_at", "updated_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", "Group A",
		nil, 70, true, pgxmock.AnyArg(),
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policy_groups.*LIMIT").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	pool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/policy-groups?page=1&page_size=10", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ListGroups(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body["groups"], 1)
	assert.Equal(t, "Group A", body["groups"].([]interface{})[0].(map[string]interface{})["name"])
}

// ---- GetGroup ----

func TestGetGroup_Success(t *testing.T) {
	pool, err := newTestGroupStore(t); require.NoError(t, err)
	id := uuid.New()
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "name", "description", "priority", "is_active",
		"metadata", "created_at", "updated_at",
	}).AddRow(
		id.String(), "tenant-1", "My Group",
		"A group", 70, true, pgxmock.AnyArg(),
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policy_groups WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	r := chi.NewRouter()
	r.Get("/groups/{id}", h.GetGroup)
	req := httptest.NewRequest(http.MethodGet, "/groups/"+id.String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "My Group", body["name"])
}

func TestGetGroup_NotFound(t *testing.T) {
	pool, err := newTestGroupStore(t); require.NoError(t, err)
	pool.ExpectQuery("SELECT.*FROM policy_groups WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	r := chi.NewRouter()
	r.Get("/groups/{id}", h.GetGroup)
	req := httptest.NewRequest(http.MethodGet, "/groups/"+uuid.New().String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "policy group not found")
}

// ---- UpdateGroup ----

func TestUpdateGroup_Success(t *testing.T) {
	pool, err := newTestGroupStore(t)
	require.NoError(t, err)
	id := uuid.New()
	// GetByID first
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "name", "description", "priority", "is_active",
		"metadata", "created_at", "updated_at",
	}).AddRow(
		id.String(), "tenant-1", "Old",
		nil, 50, true, pgxmock.AnyArg(),
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policy_groups WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)
	// Update sends: name (renamed), priority (50 from existing), is_active (true),
	// updated_at=NOW() WHERE id=$4 AND tenant_id=$5 — 5 args total
	pool.ExpectExec("UPDATE policy_groups SET").
		WithArgs(
			pgxmock.AnyArg(), // name
			pgxmock.AnyArg(), // priority
			pgxmock.AnyArg(), // is_active
			pgxmock.AnyArg(), // id (WHERE)
			pgxmock.AnyArg(), // tenant_id (WHERE)
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	r := chi.NewRouter()
	r.Patch("/groups/{id}", h.UpdateGroup)
	body := strings.NewReader(`{"name":"Renamed"}`)
	req := httptest.NewRequest(http.MethodPatch, "/groups/"+id.String(), body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Equal(t, "Renamed", result["name"])
}

// ---- DeleteGroup ----

func TestDeleteGroup_Success(t *testing.T) {
	pool, err := newTestGroupStore(t); require.NoError(t, err)
	pool.ExpectExec("UPDATE policy_groups SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	r := chi.NewRouter()
	r.Delete("/groups/{id}", h.DeleteGroup)
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+uuid.New().String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteGroup_NotFound(t *testing.T) {
	pool, err := newTestGroupStore(t); require.NoError(t, err)
	pool.ExpectExec("UPDATE policy_groups SET is_active").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	groupStore := store.NewGroupStore(pool)
	h := NewGroupHandler(groupStore)

	r := chi.NewRouter()
	r.Delete("/groups/{id}", h.DeleteGroup)
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+uuid.New().String(), nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---- ListAudits ----

func TestListAudits_Success(t *testing.T) {
	pool, err := newTestAuditStore(t); require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "denied", nil, nil, 5, nil, time.Now())
	pool.ExpectQuery("SELECT.*FROM policy_audits.*LIMIT").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	pool.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(countRows)

	auditStore := store.NewAuditStore(pool)
	h := NewAuditHandler(auditStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit?page=1&page_size=10", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-audit")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ListAudits(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body["audits"], 1)
	assert.Equal(t, "denied", body["audits"].([]interface{})[0].(map[string]interface{})["result"])
}

func TestListAudits_WithAgentFilter(t *testing.T) {
	pool, err := newTestAuditStore(t)
	require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "allowed", nil, nil, 3, nil, time.Now())
	pool.ExpectQuery("SELECT.*FROM policy_audits.*agent_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	pool.ExpectQuery("SELECT COUNT.*agent_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(countRows)

	auditStore := store.NewAuditStore(pool)
	h := NewAuditHandler(auditStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit?agent_id=agent-42&page=1&page_size=10", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-audit2")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ListAudits(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body["audits"], 1)
}

func TestListAudits_WithResultFilter(t *testing.T) {
	pool, err := newTestAuditStore(t); require.NoError(t, err)
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "policy_id", "group_id", "request_id", "agent_id",
		"resource_type", "resource_target", "requested_action", "result",
		"matched_policy_name", "matched_rule_index", "evaluation_ms", "request_data", "created_at",
	}).AddRow(
		uuid.New().String(), "tenant-1", nil, nil, nil, nil,
		"tool", nil, "send_email", "denied", nil, nil, 8, nil, time.Now())
	pool.ExpectQuery("SELECT.*FROM policy_audits.*result").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	countRows := pgxmock.NewRows([]string{"count"})
	countRows.AddRow(1)
	pool.ExpectQuery("SELECT COUNT.*result").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(countRows)

	auditStore := store.NewAuditStore(pool)
	h := NewAuditHandler(auditStore)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit?result=denied&page=1&page_size=10", nil)
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-audit3")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ListAudits(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body["audits"], 1)
}

// ---- EvaluatePolicy: condition expression ----

func TestEvaluatePolicy_ConditionMatch(t *testing.T) {
	// Use the engine directly with a mock policy store that returns a policy with a condition expression
	// The engine evaluates conditions via RuleEngine, which we've already tested.
	// Here we verify the full pipeline: handler → engine → response
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	id := uuid.New()
	// Condition: {"op":"eq","field":"cost","value":100} — cost is 100 (float64), value is 100 (float64)
	condExpr := []byte(`{"op":"eq","field":"cost","value":100}`)
	condRow := condExpr
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), "tenant-1", "g1", "cost-gated",
		nil, "deny", "global", "all", nil, &condRow, "enforce", 80, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	// cost: 100 matches condition value: 100 → condition passes → deny
	body := strings.NewReader(`{"resource":"run_model","action_type":"execute","cost":100}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-cond")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	// Condition matches (100 == 100), so deny applies
	assert.False(t, result["allowed"].(bool))
}

func TestEvaluatePolicy_ConditionNoMatch(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	id := uuid.New()
	// Condition: {"op":"eq","field":"cost","value":100} — cost is 50 != 100 → condition fails → no matching policy → default deny
	condExpr := []byte(`{"op":"eq","field":"cost","value":100}`)
	condRow := condExpr
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), "tenant-1", "g1", "cost-gated",
		nil, "deny", "global", "all", nil, &condRow, "enforce", 80, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	// cost: 50 does NOT match condition value: 100 → condition fails → default deny
	body := strings.NewReader(`{"resource":"run_model","action_type":"execute","cost":50}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-cond2")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.False(t, result["allowed"].(bool))
	assert.Contains(t, result["reason"], "default deny")
}

// ---- EvaluatePolicy: warn effect (with enforce/allow) ----

func TestEvaluatePolicy_WarnEffect(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	// DB returns policies in priority DESC order: warn policy first (priority 60),
	// then enforce/allow policy (priority 50) — warn adds warnings, allow short-circuits
	dataRows := pgxmock.NewRows([]string{
		"id", "tenant_id", "group_id", "name", "description", "action",
		"scope", "resource_type", "resource_target", "condition_expression",
		"effect", "priority", "is_active", "created_by", "created_at", "updated_at",
	}).AddRow(uuid.New().String(), "tenant-1", "g1", "warn-large",
		nil, "allow", "global", "all", nil, nil, "warn", 60, true, nil,
		time.Now(), time.Now()).
		AddRow(uuid.New().String(), "tenant-1", "g1", "allow-export",
		nil, "allow", "global", "all", nil, nil, "enforce", 50, true, nil,
		time.Now(), time.Now())
	pool.ExpectQuery("SELECT.*FROM policies WHERE tenant_id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(dataRows)

	policyStore := store.NewPolicyStore(pool)
	evaluateEngine := engine.NewEngine(policyStore, nil)
	auditStore := store.NewAuditStore(pool)
	h := NewEvaluateHandler(evaluateEngine, auditStore, nil)

	body := strings.NewReader(`{"resource":"export_data","action_type":"download"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/evaluate", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(context.Background(), ctxkeys.TenantIDKey, "tenant-1")
	ctx = context.WithValue(ctx, ctxkeys.RequestIDKey, "req-warn")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.EvaluatePolicy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	// warn effect only adds warnings, doesn't short-circuit
	assert.True(t, result["allowed"].(bool))
	warnings := result["warnings"].([]interface{})
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0].(map[string]interface{})["message"].(string), "warn-large")
}

// ---- Helper functions for handler tests ----

func newTestPolicyStore(t *testing.T) (pgxmock.PgxPoolIface, error) {
	return pgxmock.NewPool()
}

func newTestGroupStore(t *testing.T) (pgxmock.PgxPoolIface, error) {
	return pgxmock.NewPool()
}

func newTestAuditStore(t *testing.T) (pgxmock.PgxPoolIface, error) {
	return pgxmock.NewPool()
}