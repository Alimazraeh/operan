package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/08-tool-execution/internal/events"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// testServer builds the full router wrapped in the tenant-context middleware so
// tests exercise real routing and tenant scoping (JWT auth is applied
// separately in production and skipped here).
func testServer() http.Handler {
	h := NewToolHandlers(store.NewToolStore(), store.NewVersionStore(), store.NewExecutionStore(), events.NewPublisher(), 100)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return middleware.RequestID(middleware.TenantContext(mux))
}

func do(t *testing.T, srv http.Handler, method, path, tenant, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	}
	if tenant != "" {
		r.Header.Set("X-Tenant-ID", tenant)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

func TestMissingTenantRejected(t *testing.T) {
	srv := testServer()
	if w := do(t, srv, http.MethodGet, "/tools", "", ""); w.Code != http.StatusBadRequest {
		t.Errorf("missing tenant = %d, want 400", w.Code)
	}
}

func TestRegisterListGetUpdate(t *testing.T) {
	srv := testServer()

	// Register
	w := do(t, srv, http.MethodPost, "/tools/register", "t1",
		`{"name":"web-search","category":"knowledge","cost_per_call":{"amount":0.01,"currency":"USD"}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("register = %d, body %s", w.Code, w.Body.String())
	}
	tool := decode(t, w)
	id, _ := tool["id"].(string)
	if id == "" || tool["version"] != "1.0.0" || tool["status"] != "active" {
		t.Fatalf("register defaults wrong: %v", tool)
	}

	// Register validation: missing name
	if w := do(t, srv, http.MethodPost, "/tools/register", "t1", `{}`); w.Code != http.StatusBadRequest {
		t.Errorf("register no name = %d, want 400", w.Code)
	}
	// Register tenant mismatch
	if w := do(t, srv, http.MethodPost, "/tools/register", "t1", `{"name":"x","tenant_id":"other"}`); w.Code != http.StatusConflict {
		t.Errorf("register tenant mismatch = %d, want 409", w.Code)
	}

	// List
	w = do(t, srv, http.MethodGet, "/tools", "t1", "")
	if w.Code != http.StatusOK || int(decode(t, w)["total"].(float64)) != 1 {
		t.Errorf("list = %d, body %s", w.Code, w.Body.String())
	}

	// Get
	if w := do(t, srv, http.MethodGet, "/tools/"+id, "t1", ""); w.Code != http.StatusOK {
		t.Errorf("get = %d", w.Code)
	}
	// Cross-tenant get -> 404
	if w := do(t, srv, http.MethodGet, "/tools/"+id, "other", ""); w.Code != http.StatusNotFound {
		t.Errorf("cross-tenant get = %d, want 404", w.Code)
	}

	// Update
	w = do(t, srv, http.MethodPatch, "/tools/"+id, "t1", `{"status":"deprecated","description":"updated"}`)
	if w.Code != http.StatusOK || decode(t, w)["status"] != "deprecated" {
		t.Errorf("update = %d, body %s", w.Code, w.Body.String())
	}
	// Update missing -> 404
	if w := do(t, srv, http.MethodPatch, "/tools/nope", "t1", `{"status":"x"}`); w.Code != http.StatusNotFound {
		t.Errorf("update missing = %d, want 404", w.Code)
	}

	// Versions
	w = do(t, srv, http.MethodGet, "/tools/"+id+"/versions", "t1", "")
	if w.Code != http.StatusOK || int(decode(t, w)["total"].(float64)) != 1 {
		t.Errorf("versions = %d, body %s", w.Code, w.Body.String())
	}
	if w := do(t, srv, http.MethodGet, "/tools/nope/versions", "t1", ""); w.Code != http.StatusNotFound {
		t.Errorf("versions missing tool = %d, want 404", w.Code)
	}
}

func TestExecuteFlow(t *testing.T) {
	// The echo executor is gone on purpose: it stamped records completed while
	// doing nothing. /execute must say so with 410, not 404 — a caller has to
	// learn it was removed, not wonder if they mistyped the path.
	srv := testServer()
	w := do(t, srv, http.MethodPost, "/execute", "t1", `{"agent_id":"a1","tool":"x"}`)
	if w.Code != http.StatusGone {
		t.Fatalf("execute = %d, want 410 (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "/invoke") {
		t.Fatalf("the 410 must point at the governed door: %s", w.Body.String())
	}
}

func TestRetryFailedExecution(t *testing.T) {
	// Same removal as /execute: the governed path does not "retry" a record —
	// a new invocation is a new, fully checked attempt.
	srv := testServer()
	w := do(t, srv, http.MethodPost, "/executions/whatever/retry", "t1", "")
	if w.Code != http.StatusGone {
		t.Fatalf("retry = %d, want 410 (%s)", w.Code, w.Body.String())
	}
}
