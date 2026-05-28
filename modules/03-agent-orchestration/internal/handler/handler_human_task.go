package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── HumanTaskHandler ────────────────────────────────────────────────────────

// HumanTaskHandler handles human-task-related HTTP endpoints
type HumanTaskHandler struct {
	HumanTaskStore repository.HumanTaskStoreIface
	ExecutionStore repository.ExecutionStoreIface
	Events         *events.Publisher
}

// WriteJSON writes a JSON response.
func (h *HumanTaskHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response.
func (h *HumanTaskHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	h.WriteJSON(w, status, resp)
}

// ─── CreateHumanTask ─────────────────────────────────────────────────────────

// CreateHumanTask handles POST /human-tasks
func (h *HumanTaskHandler) CreateHumanTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PipelineExecutionID string                        `json:"pipeline_execution_id"`
		StepID              string                        `json:"step_id,omitempty"`
		AssigneeType        *store.HumanTaskAssigneeType  `json:"assignee_type,omitempty"`
		AssigneeID          string                        `json:"assignee_id"`
		TaskType            *store.HumanTaskType          `json:"task_type,omitempty"`
		Instructions        string                        `json:"instructions"`
		Context             map[string]interface{}        `json:"context,omitempty"`
		TimeoutMinutes      *int                          `json:"timeout_minutes,omitempty"`
		Label               *string                       `json:"label,omitempty"`
		Priority            *store.HumanTaskPriority      `json:"priority,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.AssigneeID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "assignee_id is required")
		return
	}
	if req.Instructions == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "instructions are required")
		return
	}

	// Verify execution exists
	_, err := h.ExecutionStore.GetByID(req.PipelineExecutionID)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "execution not found")
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	task := &store.HumanTask{
		TenantID:            tenantID,
		PipelineExecutionID: req.PipelineExecutionID,
		StepID:              req.StepID,
		AssigneeID:          req.AssigneeID,
		TaskType:            "",
		Instructions:        req.Instructions,
		Context:             req.Context,
		Status:              store.HumanTaskStatusPending,
		CreatedAt:           time.Now(),
	}
	if req.AssigneeType != nil {
		task.AssigneeType = *req.AssigneeType
	}
	if req.TaskType != nil {
		task.TaskType = *req.TaskType
	}
	if req.TimeoutMinutes != nil {
		task.TimeoutMinutes = *req.TimeoutMinutes
	}
	if req.Label != nil {
		task.Label = *req.Label
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}

	created, err := h.HumanTaskStore.Create(task)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusCreated, created)
}

// ─── ListHumanTasks ───────────────────────────────────────────────────────────

// ListHumanTasks handles GET /human-tasks
func (h *HumanTaskHandler) ListHumanTasks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	status := (*string)(nil)
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}

	tasks, total := h.HumanTaskStore.List(tenantID, status)

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": total,
	})
}

// ─── GetHumanTask ─────────────────────────────────────────────────────────────

// GetHumanTask handles GET /human-tasks/{id}
func (h *HumanTaskHandler) GetHumanTask(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/human-tasks/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "human task id is required")
		return
	}

	task, err := h.HumanTaskStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, task)
}

// ─── RespondToHumanTask ───────────────────────────────────────────────────────

// RespondToHumanTask handles POST /human-tasks/{id}/respond
func (h *HumanTaskHandler) RespondToHumanTask(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/human-tasks/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "human task id is required")
		return
	}

	var req struct {
		Action    string                 `json:"action"`
		Response  map[string]interface{} `json:"response,omitempty"`
		Comments  string                 `json:"comments,omitempty"`
		RespondedBy string               `json:"responded_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Action == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "action is required")
		return
	}

	updated, err := h.HumanTaskStore.Respond(id, req.Action, req.Response, req.RespondedBy, req.Comments)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, updated)
}

// ─── GetPendingTasks ──────────────────────────────────────────────────────────

// GetPendingTasks handles GET /human-tasks/pending
func (h *HumanTaskHandler) GetPendingTasks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	status := string(store.HumanTaskStatusPending)
	tasks, total := h.HumanTaskStore.List(tenantID, &status)

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": total,
	})
}

// ─── GetTasksByExecution ──────────────────────────────────────────────────────

// GetTasksByExecution handles GET /human-tasks/execution/{executionId}
func (h *HumanTaskHandler) GetTasksByExecution(w http.ResponseWriter, r *http.Request) {
	executionID := extractIDFromPath(r.URL.Path, "/human-tasks/execution/")
	if executionID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	// The store doesn't have a direct by-execution list, but we can get all
	// tenant tasks and filter. For a proper implementation, we'd add
	// ListByExecution to the store. For now, return all pending tasks
	// that match the execution.
	status := string(store.HumanTaskStatusPending)
	tasks, _ := h.HumanTaskStore.List(r.Header.Get("X-Tenant-ID"), &status)

	// Filter to matching execution — in production this would be a direct query
	var filtered []*store.HumanTask
	for _, t := range tasks {
		if t.PipelineExecutionID == executionID {
			filtered = append(filtered, t)
		}
	}

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":   filtered,
		"total":   len(filtered),
	})
}

// ─── CancelHumanTask ──────────────────────────────────────────────────────────

// CancelHumanTask handles POST /human-tasks/{id}/cancel
func (h *HumanTaskHandler) CancelHumanTask(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/human-tasks/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "human task id is required")
		return
	}

	// Get current task, update to cancelled
	task, err := h.HumanTaskStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	if task.Status != store.HumanTaskStatusPending {
		h.WriteError(w, http.StatusBadRequest, 400, "only pending tasks can be cancelled")
		return
	}

	// Respond with "cancel" action
	updated, err := h.HumanTaskStore.Respond(id, "cancel", nil, "system", "cancelled by user")
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, updated)
}
