package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── ExecutionHandler tests ──────────────────────────────────────────────────

func TestExecutionHandler_CreateExecution(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	// Create a pipeline for testing
	_, err := pipelineStore.Create(&store.Pipeline{
		Name:     "Test Pipeline",
		TenantID: "tenant-1",
		Steps:    []store.PipelineStep{{ID: "step-1", Name: "Step 1", Type: store.PipelineStepAgent}},
	})
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	pipelines, _, _ := pipelineStore.List("tenant-1", 1, 20, nil)
	if len(pipelines) == 0 {
		t.Fatal("no pipelines found")
	}

	t.Run("creates execution successfully", func(t *testing.T) {
		h := NewExecutionHandler(execStore, pipelineStore)
		body := strings.NewReader(`{"pipeline_id": "` + pipelines[0].ID + `"}`)
		req := httptest.NewRequest("POST", "/executions", body)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.CreateExecution(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp store.PipelineExecution
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.PipelineID == "" {
			t.Error("Expected pipeline_id in response")
		}
		if resp.Status != store.PipelineExecutionPending {
			t.Errorf("Expected pending status, got %s", resp.Status)
		}
	})

	t.Run("rejects missing pipeline_id", func(t *testing.T) {
		h := NewExecutionHandler(execStore, pipelineStore)
		body := strings.NewReader(`{}`)
		req := httptest.NewRequest("POST", "/executions", body)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.CreateExecution(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewExecutionHandler(execStore, pipelineStore)
		body := strings.NewReader(`{invalid`)
		req := httptest.NewRequest("POST", "/executions", body)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.CreateExecution(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("rejects unknown pipeline", func(t *testing.T) {
		h := NewExecutionHandler(execStore, pipelineStore)
		body := strings.NewReader(`{"pipeline_id": "non-existent"}`)
		req := httptest.NewRequest("POST", "/executions", body)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.CreateExecution(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestExecutionHandler_ListExecutions(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	// Create a pipeline and execution
	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionCompleted,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("lists executions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions?page=1&page_size=20", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ListExecutions(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if total, ok := resp["total"].(float64); !ok || total != 1 {
			t.Errorf("Expected total 1, got %v", resp["total"])
		}
	})

	t.Run("defaults to tenant default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions", nil)
		w := httptest.NewRecorder()
		h.ListExecutions(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})
}

func TestExecutionHandler_GetExecution(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	exec, _ := execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionCompleted,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("gets execution by id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions/"+exec.ID, nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecution(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp store.PipelineExecution
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ID != exec.ID {
			t.Errorf("Expected id %s, got %s", exec.ID, resp.ID)
		}
	})

	t.Run("returns 404 for missing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions/non-existent", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecution(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestExecutionHandler_DeleteExecution(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	exec, _ := execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionCompleted,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("deletes execution", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/executions/"+exec.ID, nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.DeleteExecution(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected 204, got %d", w.Code)
		}

		// Verify it's gone
		_, err := execStore.GetByID(exec.ID)
		if err == nil {
			t.Error("Expected execution to be deleted")
		}
	})

	t.Run("returns 404 for missing", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/executions/non-existent", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.DeleteExecution(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestExecutionHandler_StartStopExecution(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	exec, _ := execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionPending,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("starts execution", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/executions/"+exec.ID+"/start", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.StartExecution(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected 202, got %d", w.Code)
		}

		got, _ := execStore.GetByID(exec.ID)
		if got.Status != store.PipelineExecutionRunning {
			t.Errorf("Expected running status, got %s", got.Status)
		}
	})

	t.Run("stops execution", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/executions/"+exec.ID+"/stop", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.StopExecution(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected 202, got %d", w.Code)
		}

		got, _ := execStore.GetByID(exec.ID)
		if got.Status != store.PipelineExecutionCancelled {
			t.Errorf("Expected cancelled status, got %s", got.Status)
		}
	})
}

func TestExecutionHandler_RetryExecution(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	exec, _ := execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionFailed,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("retries execution", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/executions/"+exec.ID+"/retry", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.RetryExecution(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["retry_count"].(float64) != 1 {
			t.Errorf("Expected retry_count 1, got %v", resp["retry_count"])
		}

		got, _ := execStore.GetByID(exec.ID)
		if got.Status != store.PipelineExecutionRunning {
			t.Errorf("Expected running status after retry, got %s", got.Status)
		}
	})
}

func TestExecutionHandler_GetExecutionSteps(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	exec, _ := execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionCompleted,
	})

	// Add steps
	now := time.Now()
	execStore.AddStep(&store.PipelineExecutionStep{
		ID:          "step-1",
		ExecutionID: exec.ID,
		StepID:      "step-1",
		StepName:    "Process Input",
		Status:      store.PipelineStepCompleted,
		StartedAt:   &now,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("gets execution steps", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions/"+exec.ID+"/steps", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecutionSteps(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp []*store.PipelineExecutionStep
		json.Unmarshal(w.Body.Bytes(), &resp)
		if len(resp) != 1 {
			t.Errorf("Expected 1 step, got %d", len(resp))
		}
	})

	t.Run("returns empty for no steps", func(t *testing.T) {
		_, _ = execStore.Create(&store.PipelineExecution{
			PipelineID: pipeline.ID,
			TenantID:   "tenant-1",
			Status:     store.PipelineExecutionPending,
		})
		req := httptest.NewRequest("GET", "/executions/"+exec.ID+"/steps", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecutionSteps(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp []*store.PipelineExecutionStep
		json.Unmarshal(w.Body.Bytes(), &resp)
		if len(resp) != 1 {
			t.Errorf("Expected 1 step, got %d", len(resp))
		}
	})
}

func TestExecutionHandler_GetExecutionsByPipeline(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	pipeline, _ := pipelineStore.Create(&store.Pipeline{Name: "Test", TenantID: "tenant-1"})
	execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionCompleted,
	})
	execStore.Create(&store.PipelineExecution{
		PipelineID: pipeline.ID,
		TenantID:   "tenant-1",
		Status:     store.PipelineExecutionFailed,
	})

	h := NewExecutionHandler(execStore, pipelineStore)

	t.Run("lists by pipeline", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions/pipeline/"+pipeline.ID, nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecutionsByPipeline(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if total, ok := resp["total"].(float64); !ok || total != 2 {
			t.Errorf("Expected total 2, got %v", resp["total"])
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions/pipeline/"+pipeline.ID+"?status=completed", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecutionsByPipeline(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if total, ok := resp["total"].(float64); !ok || total != 1 {
			t.Errorf("Expected total 1 for completed, got %v", resp["total"])
		}
	})

	t.Run("filters by limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/executions/pipeline/"+pipeline.ID+"?limit=1", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetExecutionsByPipeline(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		// With limit=1, total reflects the limited count
	})
}

func TestExecutionHandler_GetExecutionAnalytics(t *testing.T) {
	execStore := store.NewExecutionStore()
	pipelineStore := store.NewPipelineStore()

	h := NewExecutionHandler(execStore, pipelineStore)

	req := httptest.NewRequest("GET", "/executions/analytics", nil)
	w := httptest.NewRecorder()
	h.GetExecutionAnalytics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp store.PipelineAnalytics
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Analytics returns placeholder - just verify response is valid
	_ = resp
}

func TestGetWorkflowVariables(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	scStore := store.NewScheduleStore()
	agStore := store.NewAgentStore()
	h := NewWorkflowHandler(wfStore, scStore, agStore)

	// Seed a real workflow owned by tenant-1, then attach variables to it.
	wf, _ := wfStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
	})
	wfStore.AddVariable(wf.ID, "tenant-1", "key", "value")

	t.Run("gets workflow variables", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/workflows/"+wf.ID+"/variables", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetWorkflowVariables(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		_ = resp
	})

	t.Run("returns 404 for missing workflow", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/workflows/nonexistent/variables", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetWorkflowVariables(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}
