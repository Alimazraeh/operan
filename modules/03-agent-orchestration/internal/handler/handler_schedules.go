package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ScheduleHandler handles schedule-related HTTP endpoints.
type ScheduleHandler struct {
	ScheduleStore repository.ScheduleStoreIface
	WorkflowStore repository.WorkflowStoreIface
	AgentStore    repository.AgentStoreIface
	Events        *events.Publisher
}

// NewScheduleHandler creates a new schedule handler.
func NewScheduleHandler(sc repository.ScheduleStoreIface, wf repository.WorkflowStoreIface, ag repository.AgentStoreIface) *ScheduleHandler {
	return &ScheduleHandler{
		ScheduleStore: sc,
		WorkflowStore: wf,
		AgentStore:    ag,
	}
}

// WriteJSON writes a JSON response.
func (h *ScheduleHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response.
func (h *ScheduleHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := middleware.ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: generateID(),
	}
	h.WriteJSON(w, status, resp)
}

// ─── scheduleWorkflow ────────────────────────────────────────────────────────

// ScheduleWorkflow handles POST /schedules
func (h *ScheduleHandler) ScheduleWorkflow(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	var req struct {
		TenantID           string                 `json:"tenant_id"`
		Name               string                 `json:"name"`
		Cron               string                 `json:"cron"`
		WorkflowTemplateID string                 `json:"workflow_template_id"`
		Variables          map[string]interface{} `json:"variables,omitempty"`
		Enabled            bool                   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	if req.Name == "" || req.Cron == "" || req.WorkflowTemplateID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "name, cron, and workflow_template_id are required")
		return
	}

	sc := &store.Schedule{
		TenantID:           tenantID,
		Name:               req.Name,
		Cron:               req.Cron,
		WorkflowTemplateID: req.WorkflowTemplateID,
		Variables:          req.Variables,
		Enabled:            req.Enabled,
	}

	sc, err := h.ScheduleStore.Create(sc)
	if err != nil {
		h.WriteError(w, http.StatusConflict, 409, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusCreated, sc)
}

// ─── getSchedule ─────────────────────────────────────────────────────────────

// GetSchedule handles GET /schedules/{id}
func (h *ScheduleHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "schedule id is required")
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())

	sc, err := h.ScheduleStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "schedule not found")
		return
	}

	h.WriteJSON(w, http.StatusOK, sc)
}

// ─── updateSchedule ──────────────────────────────────────────────────────────

// UpdateSchedule handles PATCH /schedules/{id}
func (h *ScheduleHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "schedule id is required")
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())

	// Verify ownership
	if _, err := h.ScheduleStore.GetByIDAndTenant(id, tenantID); err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "schedule not found")
		return
	}

	var req struct {
		Name               *string                 `json:"name,omitempty"`
		Cron               *string                 `json:"cron,omitempty"`
		WorkflowTemplateID *string                 `json:"workflow_template_id,omitempty"`
		Variables          *map[string]interface{} `json:"variables,omitempty"`
		Enabled            *bool                   `json:"enabled,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	sc, err := h.ScheduleStore.Patch(id, req.Name, req.Cron, req.WorkflowTemplateID, req.Variables, req.Enabled)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, sc)
}

// ─── deleteSchedule ──────────────────────────────────────────────────────────

// DeleteSchedule handles DELETE /schedules/{id}
func (h *ScheduleHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "schedule id is required")
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())

	// Verify ownership
	if _, err := h.ScheduleStore.GetByIDAndTenant(id, tenantID); err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "schedule not found")
		return
	}

	if err := h.ScheduleStore.Delete(id); err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── triggerSchedule ─────────────────────────────────────────────────────────

// TriggerSchedule handles POST /schedules/{id}/trigger
func (h *ScheduleHandler) TriggerSchedule(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "schedule id is required")
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())

	sc, err := h.ScheduleStore.GetByIDAndTenant(id, tenantID)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "schedule not found")
		return
	}

	evt := store.ExecutionEvent{
		EventType: "schedule_triggered",
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"schedule_id":          id,
			"workflow_template_id": sc.WorkflowTemplateID,
		},
	}
	h.WorkflowStore.AddEvent(sc.WorkflowTemplateID, evt)

	// Publish schedule triggered event
	if h.Events != nil {
		h.Events.PublishScheduleTriggered(events.StackLangGraph, events.ScheduleTriggeredPayload{
			ScheduleID:     id,
			WorkflowID:     sc.WorkflowTemplateID,
			TriggeredBy:    middleware.UserIDFromContext(r.Context()),
			CronExpression: sc.Cron,
		})
	}

	w.WriteHeader(http.StatusCreated)
}

func extractScheduleIDFromPath(path string) string {
	// /schedules/{id} or /schedules/{id}/trigger
	const prefix = "/schedules/"
	s := path[len(prefix):]
	idx := 0
	for idx < len(s) && s[idx] != '/' {
		idx++
	}
	return s[:idx]
}

// ─── listSchedules ───────────────────────────────────────────────────────────

// ListSchedules handles GET /schedules with pagination.
func (h *ScheduleHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	// Parse pagination params (1-based page/page_size, fallback to offset/limit)
	page := 1
	pageSize := 20
	if p := r.URL.Query().Get("page"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", &page); err != nil {
			page = 1
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if _, err := fmt.Sscanf(ps, "%d", &pageSize); err != nil {
			pageSize = 20
		}
	}

	enabled := r.URL.Query().Get("enabled")
	var enabledFilter *bool
	if enabled != "" {
		b := enabled == "true"
		enabledFilter = &b
	}

	schedules, total, hasMore := h.ScheduleStore.List(tenantID, page, pageSize, enabledFilter)
	h.WriteJSON(w, http.StatusOK, middleware.PaginatedResponse[store.Schedule]{
		Data:    schedules,
		Total:   total,
		HasMore: hasMore,
	})
}

// ─── pauseSchedule ───────────────────────────────────────────────────────────

// PauseSchedule handles POST /schedules/{id}/pause
func (h *ScheduleHandler) PauseSchedule(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "schedule id is required")
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())

	// Verify ownership
	if _, err := h.ScheduleStore.GetByIDAndTenant(id, tenantID); err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "schedule not found")
		return
	}

	sc, err := h.ScheduleStore.Patch(id, nil, nil, nil, nil, ptrBool(false))
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, sc)
}

// ─── resumeSchedule ──────────────────────────────────────────────────────────

// ResumeSchedule handles POST /schedules/{id}/resume
func (h *ScheduleHandler) ResumeSchedule(w http.ResponseWriter, r *http.Request) {
	id := extractScheduleIDFromPath(r.URL.Path)
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "schedule id is required")
		return
	}

	tenantID := middleware.TenantIDFromContext(r.Context())

	// Verify ownership
	if _, err := h.ScheduleStore.GetByIDAndTenant(id, tenantID); err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "schedule not found")
		return
	}

	sc, err := h.ScheduleStore.Patch(id, nil, nil, nil, nil, ptrBool(true))
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, sc)
}

func ptrBool(b bool) *bool {
	return &b
}
