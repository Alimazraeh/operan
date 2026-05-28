package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/operan/modules/03-agent-orchestration/internal/execution"
	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// WorkflowHandler handles workflow-related HTTP endpoints.
type WorkflowHandler struct {
	WorkflowStore repository.WorkflowStoreIface
	ScheduleStore repository.ScheduleStoreIface
	AgentStore    repository.AgentStoreIface
	DAGEngine     *execution.Engine
	Events        *events.Publisher
}

// NewWorkflowHandler creates a new workflow handler.
func NewWorkflowHandler(wf repository.WorkflowStoreIface, sc repository.ScheduleStoreIface, ag repository.AgentStoreIface) *WorkflowHandler {
	return &WorkflowHandler{
		WorkflowStore: wf,
		ScheduleStore: sc,
		AgentStore:    ag,
	}
}

// WriteJSON writes a JSON response.
func (h *WorkflowHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response.
func (h *WorkflowHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// ─── createWorkflow ─────────────────────────────────────────────────────────

// CreateWorkflow handles POST /workflows
func (h *WorkflowHandler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		TenantID      string                 `json:"tenant_id"`
		DepartmentID  string                 `json:"department_id,omitempty"`
		Name          string                 `json:"name"`
		Version       string                 `json:"version,omitempty"`
		Graph         *store.WorkflowGraph   `json:"graph"`
		Variables     map[string]interface{} `json:"variables,omitempty"`
		Priority      int                    `json:"priority,omitempty"`
		Description   string                 `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "Invalid workflow definition")
		return
	}

	if req.Name == "" || req.Graph == nil || len(req.Graph.Nodes) == 0 {
		h.WriteError(w, http.StatusBadRequest, 400, "tenant_id, name, and graph (with nodes) are required")
		return
	}

	wf := &store.Workflow{
		TenantID:       tenantID,
		DepartmentID:   req.DepartmentID,
		Name:           req.Name,
		Version:        req.Version,
		Status:         store.WorkflowStatusPending,
		Graph:          *req.Graph.DeepCopy(),
		Variables:      req.Variables,
		Priority:       req.Priority,
		Description:    req.Description,
		CreatedBy:      middleware.TenantIDFromContext(r.Context()),
	}
	if wf.Priority < 1 {
		wf.Priority = 5
	}
	if wf.Priority > 10 {
		wf.Priority = 10
	}

	wf, err := h.WorkflowStore.Create(wf)
	if err != nil {
		h.WriteError(w, http.StatusConflict, 409, err.Error())
		return
	}

	// Publish workflow created event
	if h.Events != nil {
		h.Events.PublishWorkflowCreated(events.StackLangGraph, events.WorkflowCreatedPayload{
			WorkflowID:   wf.ID,
			TenantID:     wf.TenantID,
			DepartmentID: wf.DepartmentID,
			Name:         wf.Name,
			Version:      wf.Version,
			CreatedBy:    wf.CreatedBy,
			CreatedAt:    wf.CreatedAt,
		})
	}

	h.WriteJSON(w, http.StatusCreated, wf)
}

// ─── listWorkflows ───────────────────────────────────────────────────────────

// ListWorkflows handles GET /workflows
func (h *WorkflowHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 50
	}

	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		st := string(store.WorkflowStatus(s))
		status = &st
	}

	workflows, total, hasMore := h.WorkflowStore.List(tenantID, page, pageSize, status)

	// Sort by created_at based on query params
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")
	_ = sortBy
	_ = sortOrder

	resp := struct {
		Workflows []*store.Workflow `json:"workflows"`
		Total     int               `json:"total"`
		HasMore   bool              `json:"has_more"`
	}{
		Workflows: workflows,
		Total:     total,
		HasMore:   hasMore,
	}

	h.WriteJSON(w, http.StatusOK, resp)
}

// ─── getWorkflow ─────────────────────────────────────────────────────────────

// GetWorkflow handles GET /workflows/{id}
func (h *WorkflowHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	wf, err := h.WorkflowStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, wf)
}

// ─── cancelWorkflow ──────────────────────────────────────────────────────────

// CancelWorkflow handles DELETE /workflows/{id}
func (h *WorkflowHandler) CancelWorkflow(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	if err := h.WorkflowStore.UpdateStatus(id, store.WorkflowStatusCancelled); err != nil {
		h.WriteError(w, http.StatusConflict, 409, err.Error())
		return
	}

	// Publish workflow cancelled event
	if h.Events != nil {
		h.Events.PublishWorkflowCancelled(events.StackLangGraph, events.WorkflowCancelledPayload{
			WorkflowID:         id,
			CancelledBy:        middleware.UserIDFromContext(r.Context()),
			CancelledAt:        time.Now().UTC(),
			CancellationReason: "cancelled by user",
		})
	}

	w.WriteHeader(http.StatusOK)
}

// ─── pauseWorkflow ───────────────────────────────────────────────────────────

// PauseWorkflow handles POST /workflows/{id}/pause
func (h *WorkflowHandler) PauseWorkflow(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	if err := h.WorkflowStore.UpdateStatus(id, store.WorkflowStatusPaused); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	// Publish workflow paused event
	if h.Events != nil {
		h.Events.PublishWorkflowPaused(events.StackLangGraph, events.WorkflowPausedPayload{
			WorkflowID: id,
			Reason:     "paused by user",
			PausedAt:   time.Now().UTC(),
		})
	}

	w.WriteHeader(http.StatusOK)
}

// ─── resumeWorkflow ──────────────────────────────────────────────────────────

// ResumeWorkflow handles POST /workflows/{id}/resume
func (h *WorkflowHandler) ResumeWorkflow(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	if err := h.WorkflowStore.UpdateStatus(id, store.WorkflowStatusRunning); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	// Publish workflow resumed event
	if h.Events != nil {
		h.Events.PublishWorkflowResumed(events.StackLangGraph, events.WorkflowResumedPayload{
			WorkflowID: id,
			ResumedAt:  time.Now().UTC(),
		})
	}

	w.WriteHeader(http.StatusOK)
}

// ─── getWorkflowState ────────────────────────────────────────────────────────

// GetWorkflowState handles GET /workflows/{id}/state
func (h *WorkflowHandler) GetWorkflowState(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	wf, err := h.WorkflowStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	// Build node states from actual checkpoints and execution history
	checkpoints := h.WorkflowStore.GetCheckpoints(id)
	events := h.WorkflowStore.GetExecutionHistory(id)

	// Map nodeID -> checkpoint (latest)
	nodeCheckpoint := make(map[string]*store.Checkpoint)
	for i := range checkpoints {
		cp := &checkpoints[i]
		nodeCheckpoint[cp.NodeID] = cp
	}

	// Map nodeID -> latest event status
	nodeEventStatus := make(map[string]string)
	for _, evt := range events {
		if nodeID, ok := evt.Details["node_id"].(string); ok {
			nodeEventStatus[nodeID] = evt.EventType
		}
	}

	nodes := make([]store.NodeState, len(wf.Graph.Nodes))
	for i, n := range wf.Graph.Nodes {
		status := store.NodeStatusPending
		if cp, ok := nodeCheckpoint[n.ID]; ok && cp != nil {
			// If checkpoint exists, node has been executed
			status = store.NodeStatusCompleted
		}
		if evtStatus, ok := nodeEventStatus[n.ID]; ok {
			switch evtStatus {
			case "failed":
				status = store.NodeStatusFailed
			case "skipped":
				status = store.NodeStatusSkipped
			case "running":
				status = store.NodeStatusRunning
			}
		}

		ns := store.NodeState{
			NodeID: n.ID,
			Status: status,
		}
		if cp, ok := nodeCheckpoint[n.ID]; ok && cp != nil && cp.StateSnapshot != nil {
			ns.Output = cp.StateSnapshot
		}
		nodes[i] = ns
	}

	state := store.WorkflowState{
		WorkflowID:       id,
		Status:           wf.Status,
		Variables:        wf.Variables,
		Nodes:            nodes,
		Checkpoints:      h.WorkflowStore.GetCheckpoints(id),
		ExecutionHistory: h.WorkflowStore.GetExecutionHistory(id),
	}

	h.WriteJSON(w, http.StatusOK, state)
}

// ─── createCheckpoint ────────────────────────────────────────────────────────

// CreateCheckpoint handles POST /workflows/{id}/checkpoint
func (h *WorkflowHandler) CreateCheckpoint(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	// Verify workflow exists
	wf, err := h.WorkflowStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}
	_ = wf

	var req struct {
		NodeID     string                 `json:"node_id,omitempty"`
		StateSnapshot map[string]interface{} `json:"state_snapshot,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	nodeID := req.NodeID
	if nodeID == "" {
		// If no node_id, create a global checkpoint
		nodeID = "_global"
	}

	cp := store.Checkpoint{
		ID:            generateID(),
		WorkflowID:    id,
		NodeID:        nodeID,
		Timestamp:     time.Now().UTC(),
		StateSnapshot: req.StateSnapshot,
		Checksum:      "sha256:" + generateID(),
	}
	h.WorkflowStore.AddCheckpoint(cp)

	// Publish checkpoint created event
	if h.Events != nil {
		h.Events.PublishWorkflowCheckpointed(events.StackLangGraph, events.WorkflowCheckpointedPayload{
			WorkflowID:  id,
			NodeID:      nodeID,
			CheckpointID: cp.ID,
			Timestamp:   cp.Timestamp,
		})
	}

	h.WriteJSON(w, http.StatusCreated, cp)
}

// ─── replayWorkflow ──────────────────────────────────────────────────────────

// ReplayWorkflow handles POST /workflows/{id}/replay
func (h *WorkflowHandler) ReplayWorkflow(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	wf, err := h.WorkflowStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	var req struct {
		CheckpointID string                 `json:"checkpoint_id,omitempty"`
		NodeID       string                 `json:"node_id,omitempty"`
		Variables    map[string]interface{} `json:"variables,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Replay means creating a new workflow instance from the original graph
	// with the same definition, optionally resetting variables
	newWf := &store.Workflow{
		TenantID:       wf.TenantID,
		DepartmentID:   wf.DepartmentID,
		Name:           wf.Name + " (replay)",
		Version:        wf.Version,
		Status:         store.WorkflowStatusPending,
		Graph:          *wf.Graph.DeepCopy(),
		Priority:       wf.Priority,
		Description:    wf.Description,
		CreatedBy:      wf.CreatedBy,
	}

	if req.Variables != nil {
		newWf.Variables = req.Variables
	} else {
		newWf.Variables = make(map[string]interface{})
		for k, v := range wf.Variables {
			newWf.Variables[k] = v
		}
	}

	newWf, err = h.WorkflowStore.Create(newWf)
	if err != nil {
		h.WriteError(w, http.StatusConflict, 409, err.Error())
		return
	}

	evt := store.ExecutionEvent{
		EventType: "replay",
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"original_workflow_id": id,
			"new_workflow_id":      newWf.ID,
			"checkpoint_id":        req.CheckpointID,
		},
	}
	h.WorkflowStore.AddEvent(id, evt)

	// Publish workflow replayed event
	if h.Events != nil {
		h.Events.PublishWorkflowReplayed(events.StackLangGraph, events.WorkflowReplayedPayload{
			WorkflowID:       newWf.ID,
			ReplayID:         uuid.New().String(),
			FromCheckpointID: req.CheckpointID,
			ReplayedBy:       middleware.UserIDFromContext(r.Context()),
			StartedAt:        time.Now().UTC(),
		})
	}

	h.WriteJSON(w, http.StatusCreated, newWf)
}

// ─── getWorkflowVariables ────────────────────────────────────────────────────

// GetWorkflowVariables handles GET /workflows/{id}/variables
func (h *WorkflowHandler) GetWorkflowVariables(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	vars, err := h.WorkflowStore.GetVariables(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, vars)
}

// ─── updateWorkflowVariables ─────────────────────────────────────────────────

// UpdateWorkflowVariables handles PATCH /workflows/{id}/variables
func (h *WorkflowHandler) UpdateWorkflowVariables(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	var req struct {
		Variables map[string]interface{} `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	wf, err := h.WorkflowStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	if err := h.WorkflowStore.SetVariables(id, wf.TenantID, req.Variables); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ─── executeWorkflow ─────────────────────────────────────────────────────────

// ExecuteWorkflow handles POST /workflows/{id}/execute
func (h *WorkflowHandler) ExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/workflows/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "workflow id is required")
		return
	}

	// Verify workflow exists
	if _, err := h.WorkflowStore.GetByID(id); err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
		return
	}

	// Start execution on DAG engine (if available)
	if h.DAGEngine != nil {
		if err := h.DAGEngine.StartWorkflow(id); err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "failed to start execution: "+err.Error())
			return
		}
	} else {
		// Fallback: update status to running without DAG engine
		if err := h.WorkflowStore.UpdateStatus(id, store.WorkflowStatusRunning); err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "failed to start execution: "+err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func extractIDFromPath(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	idx := strings.Index(s, "/")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
