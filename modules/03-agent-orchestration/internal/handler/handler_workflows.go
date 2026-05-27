package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// WorkflowHandler handles workflow-related HTTP endpoints.
type WorkflowHandler struct {
	WorkflowStore *store.WorkflowStore
	ScheduleStore *store.ScheduleStore
	AgentStore    *store.AgentStore
}

// NewWorkflowHandler creates a new workflow handler.
func NewWorkflowHandler(wf *store.WorkflowStore, sc *store.ScheduleStore, ag *store.AgentStore) *WorkflowHandler {
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

	h.WriteJSON(w, http.StatusCreated, wf)
}

// ─── listWorkflows ───────────────────────────────────────────────────────────

// ListWorkflows handles GET /workflows
func (h *WorkflowHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	page++ // convert from 0-based offset to 1-based page
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
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

	nodes := make([]store.NodeState, len(wf.Graph.Nodes))
	for i, n := range wf.Graph.Nodes {
		nodes[i] = store.NodeState{
			NodeID: n.ID,
			Status: store.NodeStatusPending,
		}
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

	cp := store.Checkpoint{
		NodeID:    id,
		Timestamp: time.Now().UTC(),
	}
	h.WorkflowStore.AddCheckpoint(cp)

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

	var req struct {
		CheckpointID string                 `json:"checkpoint_id,omitempty"`
		NodeID       string                 `json:"node_id,omitempty"`
		Variables    map[string]interface{} `json:"variables,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	wf, err := h.WorkflowStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	if req.Variables != nil {
		for k, v := range req.Variables {
			wf.Variables[k] = v
		}
		h.WorkflowStore.SetVariables(id, wf.TenantID, wf.Variables)
	}

	evt := store.ExecutionEvent{
		EventType: "replay",
		Timestamp: time.Now().UTC(),
		Details:   map[string]interface{}{"checkpoint_id": req.CheckpointID},
	}
	h.WorkflowStore.AddEvent(id, evt)

	w.WriteHeader(http.StatusCreated)
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
