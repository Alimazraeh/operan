package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/execution"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── ExecuteWorkflow tests ────────────────────────────────────────────────────

func TestExecuteWorkflow(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		h := newTestWorkflowHandler(t)
		req := httptest.NewRequest("POST", "/workflows/non-existent/execute", nil)
		w := httptest.NewRecorder()
		h.ExecuteWorkflow(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})

	t.Run("success without DAG engine", func(t *testing.T) {
		h := newTestWorkflowHandler(t)
		wf, _ := h.WorkflowStore.Create(&store.Workflow{
			TenantID: "tenant-1",
			Name:     "Test Workflow",
			Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
		})
		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/execute", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ExecuteWorkflow(w, req)
		if w.Code != http.StatusNoContent {
			t.Errorf("Expected 204, got %d", w.Code)
		}
		got, _ := h.WorkflowStore.GetByID(wf.ID)
		if got.Status != store.WorkflowStatusRunning {
			t.Errorf("Expected running status, got %s", got.Status)
		}
	})

	t.Run("success with DAG engine", func(t *testing.T) {
		// Create a node handler that simply returns processed output
		nodeHdlr := func(ctx context.Context, node store.WorkflowNode, workflowID string, variables map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"status": "processed"}, nil
		}
		wfStore := store.NewWorkflowStore()
		h := NewWorkflowHandler(wfStore, store.NewScheduleStore(), store.NewAgentStore())
		h.DAGEngine = execution.NewEngine(wfStore, events.NewPublisher(), nodeHdlr, events.StackLangGraph)
		wf, _ := wfStore.Create(&store.Workflow{
			TenantID: "tenant-1",
			Name:     "Test Workflow",
			Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1", Type: "agent"}}},
		})
		req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/execute", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ExecuteWorkflow(w, req)
		if w.Code != http.StatusNoContent {
			t.Errorf("Expected 204, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

// ─── EscalationHandler tests ──────────────────────────────────────────────────

func TestEscalationHandler_ListWorkflowEscalations(t *testing.T) {
	escStore := store.NewEscalationStore()
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{TenantID: "tenant-1", Name: "Test"})
	escStore.Create(&store.Escalation{
		ID:         "esc-1",
		WorkflowID: wf.ID,
		TenantID:   "tenant-1",
		Severity:   store.EscalationHigh,
		Reason:     "node failed",
		Status:     store.EscalationPending,
	})
	h := NewEscalationHandler(escStore, wfStore)

	t.Run("lists escalations", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/workflows/"+wf.ID+"/escalations", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ListWorkflowEscalations(w, req, wf.ID)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["total"].(float64) != 1 {
			t.Errorf("Expected total 1, got %v", resp["total"])
		}
	})

	t.Run("workflow not found", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/workflows/non-existent/escalations", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ListWorkflowEscalations(w, req, "non-existent")
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestEscalationHandler_CreateEscalation(t *testing.T) {
	escStore := store.NewEscalationStore()
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{TenantID: "tenant-1", Name: "Test"})
	h := NewEscalationHandler(escStore, wfStore)

	t.Run("creates escalation", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"severity": "high",
			"reason":   "node failed",
			"node_id":  "n1",
		})
				req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/escalations", bytes.NewReader(body))
		req = setTenant(req)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.CreateEscalation(w, req, wf.ID)
		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["severity"] != "high" {
			t.Errorf("Expected severity 'high', got %v", resp["severity"])
		}
	})

	t.Run("invalid body", func(t *testing.T) {
				req := httptest.NewRequest("POST", "/workflows/"+wf.ID+"/escalations", bytes.NewReader([]byte("not json")))
		req = setTenant(req)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.CreateEscalation(w, req, wf.ID)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})
}

func TestEscalationHandler_AcknowledgeEscalation(t *testing.T) {
	escStore := store.NewEscalationStore()
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{TenantID: "tenant-1", Name: "Test"})
	escStore.Create(&store.Escalation{
		ID:         "esc-1",
		WorkflowID: wf.ID,
		TenantID:   "tenant-1",
		Status:     store.EscalationPending,
	})
	h := NewEscalationHandler(escStore, wfStore)

	t.Run("acknowledges pending escalation", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/escalations/esc-1/acknowledge", nil)
		req = setTenant(req)
		h.AcknowledgeEscalation(w, req, "esc-1")
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		esc, _ := escStore.GetByID("esc-1")
		if esc.Status != store.EscalationAcknowledged {
			t.Errorf("Expected acknowledged, got %s", esc.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/escalations/non-existent/acknowledge", nil)
		h.AcknowledgeEscalation(w, req, "non-existent")
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestEscalationHandler_ResolveEscalation(t *testing.T) {
	escStore := store.NewEscalationStore()
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{TenantID: "tenant-1", Name: "Test"})
	escStore.Create(&store.Escalation{
		ID:         "esc-1",
		WorkflowID: wf.ID,
		TenantID:   "tenant-1",
		Status:     store.EscalationAcknowledged,
	})
	h := NewEscalationHandler(escStore, wfStore)

	t.Run("resolves acknowledged escalation", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/escalations/esc-1/resolve", nil)
		req = setTenant(req)
		h.ResolveEscalation(w, req, "esc-1")
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		esc, _ := escStore.GetByID("esc-1")
		if esc.Status != store.EscalationResolved {
			t.Errorf("Expected resolved, got %s", esc.Status)
		}
	})
}

// ─── RetryHandler tests ───────────────────────────────────────────────────────

func TestRetryHandler_ListWorkflowRetryRecords(t *testing.T) {
	retryStore := store.NewRetryRecordStore()
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{TenantID: "tenant-1", Name: "Test"})
	retryStore.Create(&store.RetryRecord{
		ID:         "retry-1",
		WorkflowID: wf.ID,
		TenantID:   "tenant-1",
		NodeID:     "n1",
		Status:     store.RetryPending,
	})
	h := NewRetryHandler(retryStore, wfStore, store.NewExecutionStore())

	t.Run("lists retry records", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/workflows/"+wf.ID+"/retries", nil)
		req = setTenant(req)
		h.ListWorkflowRetryRecords(w, req, wf.ID)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["total"].(float64) != 1 {
			t.Errorf("Expected total 1, got %v", resp["total"])
		}
	})
}

func TestRetryHandler_RetryNode(t *testing.T) {
	retryStore := store.NewRetryRecordStore()
	wfStore := store.NewWorkflowStore()
	wfStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		ID:       "wf-1",
	})
	h := NewRetryHandler(retryStore, wfStore, store.NewExecutionStore())

	t.Run("creates retry record", func(t *testing.T) {
				req := httptest.NewRequest("POST", "/workflows/wf-1/nodes/n1/retry", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.RetryNode(w, req, "wf-1", "n1")
		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["workflow_id"] != "wf-1" {
			t.Errorf("Expected workflow_id 'wf-1', got %v", resp["workflow_id"])
		}
	})
}

// ─── NodesResultsHandler tests ────────────────────────────────────────────────

func TestNodesResultsHandler_ListWorkflowNodes(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test",
		Graph: store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "n1", Type: "agent"},
				{ID: "n2", Type: "tool"},
			},
		},
	})
	h := NewNodesResultsHandler(wfStore)

	t.Run("lists nodes", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/workflows/"+wf.ID+"/nodes", nil)
		req = setTenant(req)
		h.ListWorkflowNodes(w, req, wf.ID)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/workflows/non-existent/nodes", nil)
		h.ListWorkflowNodes(w, req, "non-existent")
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestNodesResultsHandler_ListWorkflowResults(t *testing.T) {
	wfStore := store.NewWorkflowStore()
	wf, _ := wfStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1"}}},
	})
	wfStore.AddCheckpoint(store.Checkpoint{
		ID:            "cp-1",
		WorkflowID:    wf.ID,
		NodeID:        "n1",
		Timestamp:     time.Now().UTC(),
		StateSnapshot: map[string]interface{}{"value": 42},
		Checksum:      "sha256:abc",
	})
	h := NewNodesResultsHandler(wfStore)

	t.Run("lists results", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/workflows/"+wf.ID+"/results", nil)
		req = setTenant(req)
		h.ListWorkflowResults(w, req, wf.ID)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["total"].(float64) != 1 {
			t.Errorf("Expected total 1, got %v", resp["total"])
		}
	})
}

// ─── AgentWorkersHandler tests ────────────────────────────────────────────────

func TestAgentWorkersHandler_GetAgentWorkers(t *testing.T) {
	agStore := store.NewAgentStore()
	agStore.CreateAssignment(&store.AgentAssignment{
		ID:         "agent-1",
		WorkflowID: "wf-1",
		TenantID:   "tenant-1",
	})
	h := NewAgentWorkersHandler(agStore)

	t.Run("returns workers for agent", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/orchestration/agents/agent-1/workers", nil)
		h.GetAgentWorkers(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("agent not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/orchestration/agents/non-existent/workers", nil)
		h.GetAgentWorkers(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

// ─── StackHealthHandler tests ─────────────────────────────────────────────────

func TestStackHealthHandler_GetStackHealth(t *testing.T) {
	healthStore := store.NewStackHealthStore()
	h := NewStackHealthHandler(healthStore)

	t.Run("returns health status", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/stack/health", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.GetStackHealth(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})
}

func TestStackHealthHandler_LangGraph(t *testing.T) {
	healthStore := store.NewStackHealthStore()
	h := NewStackHealthHandler(healthStore)

	t.Run("list langgraphs (empty)", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/stack/langgraph/graphs", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ListLangGraphs(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("create langgraph", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":       "test-graph",
			"graph_def":  map[string]interface{}{"nodes": []string{"n1"}},
		})
				req := httptest.NewRequest("POST", "/stack/langgraph/graphs", bytes.NewReader(body))
		req = setTenant(req)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.CreateLangGraph(w, req)
		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestStackHealthHandler_Celery(t *testing.T) {
	healthStore := store.NewStackHealthStore()
	h := NewStackHealthHandler(healthStore)

	t.Run("list celery queues (empty)", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/stack/celery/queues", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h.ListCeleryQueues(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("create celery queue", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":    "default",
			"backend": "rpc://",
		})
				req := httptest.NewRequest("POST", "/stack/celery/queues", bytes.NewReader(body))
		req = setTenant(req)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.CreateCeleryQueue(w, req)
		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d", w.Code)
		}
	})
}

// ─── ListAgents test ──────────────────────────────────────────────────────────

func TestListAgents(t *testing.T) {
	agStore := store.NewAgentStore()
	h := ListAgents(agStore)

	t.Run("returns empty list", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/agents", nil)
		req = setTenant(req)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["total"].(float64) != 0 {
			t.Errorf("Expected total 0, got %v", resp["total"])
		}
	})
}

// ─── UpdateWorkflowVariables test ─────────────────────────────────────────────

func TestUpdateWorkflowVariables(t *testing.T) {
	h := newTestWorkflowHandler(t)
	wf, _ := h.WorkflowStore.Create(&store.Workflow{
		TenantID: "tenant-1",
		Name:     "Test",
		Graph:    store.WorkflowGraph{Nodes: []store.WorkflowNode{{ID: "n1"}}},
	})

	t.Run("updates variables", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"variables": map[string]interface{}{"key1": "value1", "key2": 42},
		})
		req := httptest.NewRequest("PATCH", "/workflows/"+wf.ID+"/variables", bytes.NewReader(body))
		req = setTenant(req)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.UpdateWorkflowVariables(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/workflows/"+wf.ID+"/variables", bytes.NewReader([]byte("not json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.UpdateWorkflowVariables(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})
}
