package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── PipelineHandler ─────────────────────────────────────────────────────────

// PipelineHandler handles pipeline-related HTTP endpoints
type PipelineHandler struct {
	PipelineStore  repository.PipelineStoreIface
	ExecutionStore repository.ExecutionStoreIface
	HumanTaskStore repository.HumanTaskStoreIface
	Events         *events.Publisher
}

// WriteJSON writes a JSON response.
func (h *PipelineHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response.
func (h *PipelineHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	h.WriteJSON(w, status, resp)
}

// ─── CreatePipeline ──────────────────────────────────────────────────────────

// CreatePipeline handles POST /pipeline
func (h *PipelineHandler) CreatePipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string                   `json:"name"`
		Description     *string                  `json:"description,omitempty"`
		Steps           []store.PipelineStep     `json:"steps"`
		ErrorHandling   *store.PipelineErrorHandlingConfig `json:"error_handling,omitempty"`
		TimeoutMinutes  *int                     `json:"timeout_minutes,omitempty"`
		MaxRetries      *int                     `json:"max_retries,omitempty"`
		TriggerType     *store.PipelineTriggerType `json:"trigger_type,omitempty"`
		Variables       *map[string]interface{}  `json:"variables,omitempty"`
		Tags            *[]string                `json:"tags,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Name == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "name is required")
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	pipeline := &store.Pipeline{
		TenantID:       tenantID,
		Name:           req.Name,
		Description:    "",
		Steps:          req.Steps,
		ErrorHandling:  req.ErrorHandling,
		TimeoutMinutes: 0,
		MaxRetries:     0,
		TriggerType:    "",
		Variables:      nil,
		Status:         store.PipelineStatusActive,
		CreatedBy:      "anonymous",
		CreatedAt:      time.Now(),
	}
	if req.Description != nil {
		pipeline.Description = *req.Description
	}
	if req.TimeoutMinutes != nil {
		pipeline.TimeoutMinutes = *req.TimeoutMinutes
	}
	if req.MaxRetries != nil {
		pipeline.MaxRetries = *req.MaxRetries
	}
	if req.TriggerType != nil {
		pipeline.TriggerType = *req.TriggerType
	}
	if req.Variables != nil {
		pipeline.Variables = *req.Variables
	}
	if req.Tags != nil {
		pipeline.Tags = *req.Tags
	}

	created, err := h.PipelineStore.Create(pipeline)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusCreated, created)
}

// ─── ListPipelines ────────────────────────────────────────────────────────────

// ListPipelines handles GET /pipeline
func (h *PipelineHandler) ListPipelines(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	page := 1
	pageSize := 20
	status := (*string)(nil)

	q := r.URL.Query()
	if p := q.Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := q.Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if s := q.Get("status"); s != "" {
		status = &s
	}

	pipelines, total, hasMore := h.PipelineStore.List(tenantID, page, pageSize, status)

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
		"total":     total,
		"has_more":  hasMore,
	})
}

// ─── GetPipeline ──────────────────────────────────────────────────────────────

// GetPipeline handles GET /pipeline/{id}
func (h *PipelineHandler) GetPipeline(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	pipeline, err := h.PipelineStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, pipeline)
}

// ─── UpdatePipeline ───────────────────────────────────────────────────────────

// UpdatePipeline handles PATCH /pipeline/{id}
func (h *PipelineHandler) UpdatePipeline(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	var req struct {
		Name           *string                            `json:"name,omitempty"`
		Description    *string                            `json:"description,omitempty"`
		Steps          *[]store.PipelineStep              `json:"steps,omitempty"`
		ErrorHandling  *store.PipelineErrorHandlingConfig `json:"error_handling,omitempty"`
		TimeoutMinutes *int                               `json:"timeout_minutes,omitempty"`
		MaxRetries     *int                               `json:"max_retries,omitempty"`
		Status         *store.PipelineStatus              `json:"status,omitempty"`
		Variables      *map[string]interface{}            `json:"variables,omitempty"`
		Tags           *[]string                          `json:"tags,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	updated, err := h.PipelineStore.Update(id, req.Name, req.Description, req.Steps, req.ErrorHandling, req.TimeoutMinutes, req.MaxRetries, req.Status, req.Variables, req.Tags)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, updated)
}

// ─── DeletePipeline ───────────────────────────────────────────────────────────

// DeletePipeline handles DELETE /pipeline/{id}
func (h *PipelineHandler) DeletePipeline(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	err := h.PipelineStore.Delete(id)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── StartPipeline ────────────────────────────────────────────────────────────

// StartPipeline handles POST /pipeline/{id}/start
func (h *PipelineHandler) StartPipeline(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	status := store.PipelineStatusActive
	err := h.PipelineStore.UpdateStatus(id, status)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ─── StopPipeline ─────────────────────────────────────────────────────────────

// StopPipeline handles POST /pipeline/{id}/stop
func (h *PipelineHandler) StopPipeline(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	status := store.PipelineStatusInactive
	err := h.PipelineStore.UpdateStatus(id, status)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ─── GetPipelineAnalytics ────────────────────────────────────────────────────

// GetPipelineAnalytics handles GET /pipeline/{id}/analytics
func (h *PipelineHandler) GetPipelineAnalytics(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	pipeline, err := h.PipelineStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	analytics := store.PipelineAnalytics{
		TotalExecutions:       pipeline.ExecutionCount,
		SuccessRate:           pipeline.SuccessRate,
		AvgDurationMS:         pipeline.AvgDurationMS,
	}

	h.WriteJSON(w, http.StatusOK, analytics)
}

// ─── GetPipelineHistory ───────────────────────────────────────────────────────

// GetPipelineHistory handles GET /pipeline/{id}/history
func (h *PipelineHandler) GetPipelineHistory(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/pipeline/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	execs, total, _ := h.ExecutionStore.ListByPipeline(id, 1, 50, nil, 50)

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"executions": execs,
		"total":      total,
	})
}
