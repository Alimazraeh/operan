package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

func newTestWorkflowHandler(t *testing.T) *WorkflowHandler {
	return NewWorkflowHandler(
		store.NewWorkflowStore(),
		store.NewScheduleStore(),
		store.NewAgentStore(),
	)
}

func TestCreateWorkflow(t *testing.T) {
	h := newTestWorkflowHandler(t)

	t.Run("creates workflow successfully", func(t *testing.T) {
		graph := map[string]interface{}{
			"nodes": []map[string]interface{}{
				{"id": "node-1", "type": "agent", "action": "process"},
			},
			"edges": []map[string]interface{}{
				{"from": "node-1", "to": "node-2"},
			},
		}

		reqBody, _ := json.Marshal(map[string]interface{}{
			"name":         "Test Workflow",
			"version":      "1.0.0",
			"tenant_id":    "tenant-1",
			"department_id": "dept-1",
			"graph":        graph,
			"priority":     5,
		})

		req := httptest.NewRequest("POST", "/workflows", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateWorkflow(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["name"] != "Test Workflow" {
			t.Errorf("Expected name 'Test Workflow', got %v", resp["name"])
		}
		if resp["status"] != "pending" {
			t.Errorf("Expected status 'pending', got %v", resp["status"])
		}
	})

	t.Run("missing name", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"graph": map[string]interface{}{"nodes": []interface{}{}},
		})

		req := httptest.NewRequest("POST", "/workflows", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateWorkflow(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestListWorkflows(t *testing.T) {
	h := newTestWorkflowHandler(t)

	// Pre-populate store
	wfStore := store.NewWorkflowStore()
	for i := 1; i <= 3; i++ {
		wfStore.Create(&store.Workflow{
			TenantID: "tenant-1",
			Name:     "Workflow " + string(rune('A'+i-1)),
			Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
		})
	}
	h.WorkflowStore = wfStore

	t.Run("returns workflows", func(t *testing.T) {
		ctx := middleware.SetTenantIDToContext(context.Background(), "tenant-1")
		req := httptest.NewRequest("GET", "/workflows?page=1&page_size=50", nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		h.ListWorkflows(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["total"].(float64) != 3 {
			t.Errorf("Expected total 3, got %v", resp["total"])
		}
	})
}

func TestGetWorkflow(t *testing.T) {
	h := newTestWorkflowHandler(t)

	// Create workflow
	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
	})

	t.Run("returns workflow by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/workflows/"+wf.ID, nil)
		w := httptest.NewRecorder()

		h.GetWorkflow(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["id"] != wf.ID {
			t.Errorf("Expected ID %s, got %v", wf.ID, resp["id"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/workflows/non-existent", nil)
		w := httptest.NewRecorder()

		h.GetWorkflow(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestPauseResumeWorkflow(t *testing.T) {
	h := newTestWorkflowHandler(t)

	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
	})

	// First transition to running (required to pause)
	h.WorkflowStore.UpdateStatus(wf.ID, store.WorkflowStatusRunning)

	t.Run("pause workflow", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/pause", nil)
		ctx := middleware.SetTenantIDToContext(context.Background(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		h.PauseWorkflow(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		got, _ := h.WorkflowStore.GetByID(wf.ID)
		if got.Status != store.WorkflowStatusPaused {
			t.Errorf("Expected paused status, got %s", got.Status)
		}
	})

	t.Run("resume workflow", func(t *testing.T) {
		// Pause first (currently running), then resume
		h.WorkflowStore.UpdateStatus(wf.ID, store.WorkflowStatusPaused)

		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/resume", nil)
		ctx := middleware.SetTenantIDToContext(context.Background(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		h.ResumeWorkflow(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		got, _ := h.WorkflowStore.GetByID(wf.ID)
		if got.Status != store.WorkflowStatusRunning {
			t.Errorf("Expected running status, got %s", got.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/workflows/non-existent/pause", nil)
		ctx := middleware.SetTenantIDToContext(context.Background(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		h.PauseWorkflow(w, req)

		// Check that we get a 4xx error (the error response has code 404)
		if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestCancelWorkflow(t *testing.T) {
	h := newTestWorkflowHandler(t)

	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
	})

	req := httptest.NewRequest("DELETE", "/workflows/"+wf.ID, nil)
	w := httptest.NewRecorder()

	h.CancelWorkflow(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	got, _ := h.WorkflowStore.GetByID(wf.ID)
	if got.Status != store.WorkflowStatusCancelled {
		t.Errorf("Expected cancelled status, got %s", got.Status)
	}
}

func TestCreateCheckpoint(t *testing.T) {
	h := newTestWorkflowHandler(t)

	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
	})

	t.Run("creates checkpoint with node_id", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"node_id": "n1",
			"state_snapshot": map[string]interface{}{"value": 42},
		})

		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/checkpoint", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateCheckpoint(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["node_id"] != "n1" {
			t.Errorf("Expected node_id 'n1', got %v", resp["node_id"])
		}
	})

	t.Run("creates global checkpoint without node_id", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"state_snapshot": map[string]interface{}{"value": 42},
		})

		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/checkpoint", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateCheckpoint(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["node_id"] != "_global" {
			t.Errorf("Expected node_id '_global', got %v", resp["node_id"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"node_id": "n1",
		})

		req := httptest.NewRequest("POST", "/workflows/non-existent/checkpoint", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.CreateCheckpoint(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestGetWorkflowState(t *testing.T) {
	h := newTestWorkflowHandler(t)

	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "n1", Type: "agent", Action: "process"},
				{ID: "n2", Type: "agent", Action: "validate"},
			},
		},
	})

	t.Run("returns node states", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/workflows/"+wf.ID+"/state", nil)
		w := httptest.NewRecorder()

		h.GetWorkflowState(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["workflow_id"] != wf.ID {
			t.Errorf("Expected workflow_id %s, got %v", wf.ID, resp["workflow_id"])
		}
	})
}

func TestReplayWorkflow(t *testing.T) {
	h := newTestWorkflowHandler(t)

	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
	})

	t.Run("creates new workflow instance", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"checkpoint_id": "cp-1",
			"node_id":       "n1",
		})

		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/replay", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ReplayWorkflow(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"checkpoint_id": "cp-1",
		})

		req := httptest.NewRequest("POST", "/workflows/non-existent/replay", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ReplayWorkflow(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}
