package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	srv := testServer()

	// Register a tool to execute.
	w := do(t, srv, http.MethodPost, "/tools/register", "t1",
		`{"name":"calc","cost_per_call":{"amount":0.5,"currency":"USD"}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: %s", w.Body.String())
	}

	// Execute
	w = do(t, srv, http.MethodPost, "/execute", "t1",
		`{"agent_id":"agent-1","tool":"calc","input":{"x":2}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("execute = %d, body %s", w.Code, w.Body.String())
	}
	exec := decode(t, w)
	execID, _ := exec["id"].(string)
	if exec["status"] != "completed" {
		t.Errorf("execute status = %v, want completed", exec["status"])
	}
	if exec["output"] == nil {
		t.Error("execute should produce output")
	}

	// Execute validation + missing tool
	if w := do(t, srv, http.MethodPost, "/execute", "t1", `{"tool":"calc"}`); w.Code != http.StatusBadRequest {
		t.Errorf("execute no agent = %d, want 400", w.Code)
	}
	if w := do(t, srv, http.MethodPost, "/execute", "t1", `{"agent_id":"a","tool":"ghost"}`); w.Code != http.StatusNotFound {
		t.Errorf("execute unknown tool = %d, want 404", w.Code)
	}

	// List executions
	w = do(t, srv, http.MethodGet, "/executions", "t1", "")
	if w.Code != http.StatusOK || int(decode(t, w)["total"].(float64)) != 1 {
		t.Errorf("list executions = %d, body %s", w.Code, w.Body.String())
	}

	// Get execution
	if w := do(t, srv, http.MethodGet, "/executions/"+execID, "t1", ""); w.Code != http.StatusOK {
		t.Errorf("get execution = %d", w.Code)
	}
	if w := do(t, srv, http.MethodGet, "/executions/nope", "t1", ""); w.Code != http.StatusNotFound {
		t.Errorf("get missing execution = %d, want 404", w.Code)
	}

	// Retry a completed execution -> 409 (only failed may retry)
	if w := do(t, srv, http.MethodPost, "/executions/"+execID+"/retry", "t1", ""); w.Code != http.StatusConflict {
		t.Errorf("retry completed = %d, want 409", w.Code)
	}
	if w := do(t, srv, http.MethodPost, "/executions/nope/retry", "t1", ""); w.Code != http.StatusNotFound {
		t.Errorf("retry missing = %d, want 404", w.Code)
	}

	// Cost summary
	w = do(t, srv, http.MethodGet, "/cost?tool=calc", "t1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("cost = %d", w.Code)
	}
	cost := decode(t, w)
	tc, _ := cost["total_cost"].(map[string]interface{})
	if cost["total_calls"].(float64) != 1 || tc["amount"].(float64) != 0.5 {
		t.Errorf("cost summary wrong: %v", cost)
	}
}

func TestRetryFailedExecution(t *testing.T) {
	// Drive the store directly to create a failed execution, then retry via HTTP.
	toolStore := store.NewToolStore()
	versionStore := store.NewVersionStore()
	execStore := store.NewExecutionStore()
	h := NewToolHandlers(toolStore, versionStore, execStore, events.NewPublisher(), 100)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	srv := middleware.RequestID(middleware.TenantContext(mux))

	_, _ = toolStore.Create(&store.Tool{TenantID: "t1", Name: "calc"})
	failed, _ := execStore.Create(&store.ToolExecution{TenantID: "t1", AgentID: "a1", Tool: "calc"})
	_, _ = execStore.Update(failed.ID, "t1", func(e *store.ToolExecution) {
		e.Status = store.ExecFailed
		e.ErrorMessage = "boom"
	})

	w := do(t, srv, http.MethodPost, "/executions/"+failed.ID+"/retry", "t1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("retry failed exec = %d, body %s", w.Code, w.Body.String())
	}
	exec := decode(t, w)
	if exec["status"] != "completed" || exec["retry_count"].(float64) != 1 {
		t.Errorf("retry result wrong: %v", exec)
	}
}
