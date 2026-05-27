package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ScheduleHandler handles schedule-related HTTP endpoints.
type ScheduleHandler struct {
	ScheduleStore *store.ScheduleStore
	WorkflowStore *store.WorkflowStore
	AgentStore    *store.AgentStore
}

// NewScheduleHandler creates a new schedule handler.
func NewScheduleHandler(sc *store.ScheduleStore, wf *store.WorkflowStore, ag *store.AgentStore) *ScheduleHandler {
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

	sc, err := h.ScheduleStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
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

	sc, err := h.ScheduleStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
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
