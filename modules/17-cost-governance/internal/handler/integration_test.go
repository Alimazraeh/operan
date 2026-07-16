package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Handler integration tests ───

func TestCreateBudget_BadRequest(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"description":"test","budget_amount":0,"period":"monthly"}`)
	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("POST", "/v1/budgets", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateBudget_MissingPeriod(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"description":"test","budget_amount":100}`)
	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("POST", "/v1/budgets", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateBudget_InvalidJSON(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{invalid json}`)
	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("POST", "/v1/budgets", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChainedAuthMiddleware(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"description":"test","budget_amount":100,"period":"monthly"}`)
	token := createTestToken("test-secret", "correct-tenant")
	req := httptest.NewRequest("POST", "/v1/budgets", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "wrong-tenant")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for tenant mismatch, got %d", w.Code)
	}
}

func TestGetThrottle_NoState(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("GET", "/v1/throttle", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetThrottle (no state): expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["throttle_state"] != nil {
		t.Logf("throttle_state: %v", resp["throttle_state"])
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestGetThrottle_HardState(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	throttleMgr.SetState("test-tenant", "hard")

	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("GET", "/v1/throttle", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetThrottle (hard): expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["throttle_state"] != "hard" {
		t.Errorf("expected hard, got %v", resp["throttle_state"])
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestSetThrottle(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("PATCH", "/v1/throttle/soft", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetThrottle: expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["throttle_state"] != "soft" {
		t.Errorf("expected soft, got %v", resp["throttle_state"])
	}

	if throttleMgr.GetState("test-tenant") != "soft" {
		t.Error("throttle manager state not updated")
	}
}

func TestSetThrottle_InvalidStatus(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("PATCH", "/v1/throttle/invalid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCostEvents_IngestBadCost(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"source_module":"m12","cost_usd":-1,"agent_id":"a1"}`)
	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("POST", "/v1/cost-events", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestCostEvents_IngestMissingSource(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	body := strings.NewReader(`{"cost_usd":1.5}`)
	token := createTestToken("test-secret", "test-tenant")
	req := httptest.NewRequest("POST", "/v1/cost-events", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestRouter_NonExistentRoute(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestRouter_CORSHeaders(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/v1/budgets", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK || w.Code == http.StatusNoContent {
		allowedOrigins := w.Header().Get("Access-Control-Allow-Origin")
		if allowedOrigins != "*" {
			t.Errorf("expected CORS wildcard, got: %s", allowedOrigins)
		}
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestRouter_HandlesOptions(t *testing.T) {
	router, throttleMgr, cleanup := setupTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// chi allows OPTIONS for CORS preflight on all routes
	// Health returns 405 since chi doesn't have OPTIONS handler by default
	if w.Code != http.StatusOK && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("OPTIONS /health: expected 200/405, got %d", w.Code)
	}

	throttleMgr.SetState("test-tenant", "none")
}

func TestBudgetEndpoints_Unauthenticated(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/v1/budgets"},
		{"GET", "/v1/budgets"},
		{"GET", "/v1/budgets/some-id"},
		{"PATCH", "/v1/budgets/some-id"},
		{"DELETE", "/v1/budgets/some-id"},
		{"POST", "/v1/cost-events"},
		{"GET", "/v1/cost-events"},
		{"GET", "/v1/summary"},
		{"GET", "/v1/alerts"},
		{"GET", "/v1/throttle"},
		{"PATCH", "/v1/throttle/hard"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: expected 401, got %d", tt.method, tt.path, w.Code)
			}
		})
	}
}