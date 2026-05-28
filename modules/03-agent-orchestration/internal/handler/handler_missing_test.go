package handler

import (
	"errors"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

var ErrAgentNotFound = errors.New("agent not found")

// ─── DelegationHandler tests ─────────────────────────────────────────────────

func TestDelegationHandler_DelegateNodeTask(t *testing.T) {
	workflowStore := &mockDelegationWorkflowStore{
		workflows: map[string]*store.Workflow{
			"wf-1": {ID: "wf-1", TenantID: "tenant-1", DepartmentID: "dept-1", CurrentNodes: []string{"agent-1"}},
		},
	}
	agentStore := &mockDelegationAgentStore{
		agents: map[string]*store.Agent{
			"agent-2": {ID: "agent-2", TenantID: "tenant-1", Name: "Test Agent", Status: "available"},
		},
	}
	delegationStore := store.NewDelegationStore()
	h := NewDelegationHandler(delegationStore, workflowStore, agentStore)

	t.Run("creates delegation successfully", func(t *testing.T) {
		body := strings.NewReader(`{"node_id": "node-1", "delegated_agent_id": "agent-2", "reason": "original agent unavailable"}`)
		req := httptest.NewRequest("POST", "/workflows/wf-1/delegate", body)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		// Inject tenant into context (simulating TenantInjector middleware)
		ctx := middleware.SetTenantIDToContext(req.Context(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.DelegateNodeTask(w, req, "wf-1")

		if w.Code != http.StatusCreated {
			t.Errorf("Expected 201, got %d. Body: %s", w.Code, w.Body.String())
			return
		}

		var resp store.Delegation
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.WorkflowID != "wf-1" {
			t.Error("Expected workflow_id to be wf-1")
		}
		if resp.DelegatedAgentID != "agent-2" {
			t.Error("Expected delegated_agent_id to be agent-2")
		}
		if resp.Status != store.DelegationPending {
			t.Errorf("Expected pending status, got %s", resp.Status)
		}
		if resp.Reason != "original agent unavailable" {
			t.Errorf("Expected reason to match, got %s", resp.Reason)
		}
	})

	t.Run("rejects workflow not found", func(t *testing.T) {
		body := strings.NewReader(`{"node_id": "node-1", "delegated_agent_id": "agent-2", "reason": "test"}`)
		req := httptest.NewRequest("POST", "/workflows/wf-999/delegate", body)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		ctx := middleware.SetTenantIDToContext(req.Context(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.DelegateNodeTask(w, req, "wf-999")

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		body := strings.NewReader(`{"node_id": "node-1"}`)
		req := httptest.NewRequest("POST", "/workflows/wf-1/delegate", body)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		ctx := middleware.SetTenantIDToContext(req.Context(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.DelegateNodeTask(w, req, "wf-1")

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("rejects delegated agent not found", func(t *testing.T) {
		body := strings.NewReader(`{"node_id": "node-1", "delegated_agent_id": "agent-999", "reason": "test"}`)
		req := httptest.NewRequest("POST", "/workflows/wf-1/delegate", body)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		ctx := middleware.SetTenantIDToContext(req.Context(), "tenant-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.DelegateNodeTask(w, req, "wf-1")

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})

	t.Run("rejects tenant mismatch", func(t *testing.T) {
		body := strings.NewReader(`{"node_id": "node-1", "delegated_agent_id": "agent-2", "reason": "test"}`)
		req := httptest.NewRequest("POST", "/workflows/wf-1/delegate", body)
		req.Header.Set("X-Tenant-ID", "tenant-999")
		ctx := middleware.SetTenantIDToContext(req.Context(), "tenant-999")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.DelegateNodeTask(w, req, "wf-1")

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})
}

func TestDelegationHandler_ListDelegations(t *testing.T) {
	workflowStore := &mockDelegationWorkflowStore{
		workflows: map[string]*store.Workflow{
			"wf-1": {ID: "wf-1", TenantID: "tenant-1"},
		},
	}
	agentStore := &mockDelegationAgentStore{}
	delegationStore := store.NewDelegationStore()

	// Create a delegation first
	delegationStore.Create(&store.Delegation{
		ID:              "del-1",
		WorkflowID:      "wf-1",
		NodeID:          "node-1",
		OriginalAgentID: "agent-1",
		DelegatedAgentID: "agent-2",
		TenantID:        "tenant-1",
		Status:          store.DelegationPending,
		Reason:          "test delegation",
	})

	_ = NewDelegationHandler(delegationStore, workflowStore, agentStore)

	t.Run("lists delegations for workflow", func(t *testing.T) {
		// We can't test list directly since there's no HTTP endpoint for it yet
		// but we can verify the store has the delegation
		delegations := delegationStore.ListByWorkflow("wf-1")
		if len(delegations) != 1 {
			t.Errorf("Expected 1 delegation, got %d", len(delegations))
		}
		if delegations[0].ID != "del-1" {
			t.Errorf("Expected delegation ID to be del-1, got %s", delegations[0].ID)
		}
	})
}

// ─── Mocks for DelegationHandler tests ───────────────────────────────────────

type mockDelegationWorkflowStore struct {
	workflows map[string]*store.Workflow
}

func (m *mockDelegationWorkflowStore) Create(wf *store.Workflow) (*store.Workflow, error) {
	m.workflows[wf.ID] = wf
	return wf, nil
}

func (m *mockDelegationWorkflowStore) GetByID(id string) (*store.Workflow, error) {
	wf, ok := m.workflows[id]
	if !ok {
		return nil, nil
	}
	cp := *wf
	return &cp, nil
}

func (m *mockDelegationWorkflowStore) UpdateStatus(id string, status store.WorkflowStatus) error {
	return nil
}

func (m *mockDelegationWorkflowStore) UpdateCurrentNodes(id string, nodeIDs []string) error {
	return nil
}

func (m *mockDelegationWorkflowStore) List(tenantID string, page, pageSize int, status *string) ([]*store.Workflow, int, bool) {
	result := make([]*store.Workflow, 0)
	for _, wf := range m.workflows {
		if wf.TenantID == tenantID {
			cp := *wf
			result = append(result, &cp)
		}
	}
	return result, len(result), false
}

func (m *mockDelegationWorkflowStore) AddCheckpoint(store.Checkpoint) {}

func (m *mockDelegationWorkflowStore) GetCheckpoints(string) []store.Checkpoint {
	return nil
}

func (m *mockDelegationWorkflowStore) AddVariable(string, string, string, interface{}) error {
	return nil
}

func (m *mockDelegationWorkflowStore) GetVariables(string) (*store.WorkflowVariables, error) {
	return nil, nil
}

func (m *mockDelegationWorkflowStore) SetVariables(string, string, map[string]interface{}) error {
	return nil
}

func (m *mockDelegationWorkflowStore) AddEvent(string, store.ExecutionEvent) {}

func (m *mockDelegationWorkflowStore) GetExecutionHistory(string) []store.ExecutionEvent {
	return nil
}

func (m *mockDelegationWorkflowStore) Delete(string) error {
	return nil
}

type mockDelegationAgentStore struct {
	agents map[string]*store.Agent
}

func (m *mockDelegationAgentStore) CreateAssignment(*store.AgentAssignment) (*store.AgentAssignment, error) {
	return nil, nil
}

func (m *mockDelegationAgentStore) GetByID(id string) (*store.AgentAssignment, error) {
	agent, ok := m.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return &store.AgentAssignment{
		ID:       id,
		AgentID:  agent.ID,
		TenantID: agent.TenantID,
	}, nil
}

func (m *mockDelegationAgentStore) SetAgentAvailability(*store.AgentAvailability) {}

func (m *mockDelegationAgentStore) GetAgentAvailability(string) (*store.AgentAvailability, error) {
	return nil, nil
}

func (m *mockDelegationAgentStore) ListByWorkflow(string) ([]*store.AgentAssignment, error) {
	return nil, nil
}

func (m *mockDelegationAgentStore) ListAgentAvailability() []*store.AgentAvailability {
	return nil
}

func (m *mockDelegationAgentStore) ListByTenant(string) []*store.AgentAvailability {
	return nil
}
