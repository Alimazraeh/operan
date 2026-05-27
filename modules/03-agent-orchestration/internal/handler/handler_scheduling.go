package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// SchedulingHandler handles agent scheduling-related HTTP endpoints.
type SchedulingHandler struct {
	AgentStore    *store.AgentStore
	WorkflowStore *store.WorkflowStore
}

// NewSchedulingHandler creates a new scheduling handler.
func NewSchedulingHandler(ag *store.AgentStore, wf *store.WorkflowStore) *SchedulingHandler {
	return &SchedulingHandler{
		AgentStore:    ag,
		WorkflowStore: wf,
	}
}

// WriteJSON writes a JSON response.
func (h *SchedulingHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response.
func (h *SchedulingHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// ─── assignAgent ─────────────────────────────────────────────────────────────

// AssignAgent handles POST /agents/assign
func (h *SchedulingHandler) AssignAgent(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		WorkflowID string                 `json:"workflow_id"`
		NodeID     string                 `json:"node_id"`
		AgentID    string                 `json:"agent_id"`
		Parameters map[string]interface{} `json:"parameters,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	if req.WorkflowID == "" || req.NodeID == "" || req.AgentID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow_id, node_id, and agent_id are required")
		return
	}

	assignment := &store.AgentAssignment{
		TenantID:   tenantID,
		WorkflowID: req.WorkflowID,
		NodeID:     req.NodeID,
		AgentID:    req.AgentID,
		Parameters: req.Parameters,
		AssignedAt: time.Now().UTC(),
	}

	assignment, err := h.AgentStore.CreateAssignment(assignment)
	if err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	// Update agent availability to busy
	h.AgentStore.SetAgentAvailability(&store.AgentAvailability{
		AgentID:          req.AgentID,
		Status:           store.AgentStatusBusy,
		CurrentWorkflows: 1,
		MaxConcurrency:   5,
		LastSeenAt:       ptrTime(time.Now().UTC()),
	})

	h.WriteJSON(w, http.StatusCreated, assignment)
}

// ─── getAgentAvailability ────────────────────────────────────────────────────

// GetAgentAvailability handles GET /agents/availability
func (h *SchedulingHandler) GetAgentAvailability(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "agent_id query parameter is required")
		return
	}

	avail, err := h.AgentStore.GetAgentAvailability(agentID)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, avail)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
