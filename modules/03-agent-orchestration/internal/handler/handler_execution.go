package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/middleware"
	"github.com/operan/modules/03-agent-orchestration/internal/repository"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// ─── ExecutionHandler ────────────────────────────────────────────────────────

// ExecutionHandler handles execution-related HTTP endpoints
type ExecutionHandler struct {
	ExecutionStore repository.ExecutionStoreIface
	PipelineStore  repository.PipelineStoreIface
	Events         *events.Publisher
}

// WriteJSON writes a JSON response.
func (h *ExecutionHandler) WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes an error response.
func (h *ExecutionHandler) WriteError(w http.ResponseWriter, status int, code int, message string) {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	h.WriteJSON(w, status, resp)
}

// ─── CreateExecution ─────────────────────────────────────────────────────────

// CreateExecution handles POST /executions
func (h *ExecutionHandler) CreateExecution(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PipelineID string                 `json:"pipeline_id"`
		Inputs     map[string]interface{} `json:"inputs,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteError(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.PipelineID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline_id is required")
		return
	}

	// Verify pipeline exists
	_, err := h.PipelineStore.GetByID(req.PipelineID)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, "pipeline not found")
		return
	}

	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	execution := &store.PipelineExecution{
		PipelineID: req.PipelineID,
		TenantID:   tenantID,
		Status:     store.PipelineExecutionPending,
		Inputs:     req.Inputs,
		CreatedAt:  time.Now(),
	}

	created, err := h.ExecutionStore.Create(execution)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusCreated, created)
}

// ─── ListExecutions ───────────────────────────────────────────────────────────

// ListExecutions handles GET /executions
func (h *ExecutionHandler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	page := 1
	pageSize := 20

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

	executions, total, hasMore := h.ExecutionStore.ListByTenant(tenantID, page, pageSize)

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"executions": executions,
		"total":      total,
		"has_more":   hasMore,
	})
}

// ─── GetExecution ─────────────────────────────────────────────────────────────

// GetExecution handles GET /executions/{id}
func (h *ExecutionHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/executions/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	execution, err := h.ExecutionStore.GetByID(id)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, execution)
}

// ─── DeleteExecution ──────────────────────────────────────────────────────────

// DeleteExecution handles DELETE /executions/{id}
func (h *ExecutionHandler) DeleteExecution(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/executions/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	err := h.ExecutionStore.Delete(id)
	if err != nil {
		h.WriteError(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── StartExecution ───────────────────────────────────────────────────────────

// StartExecution handles POST /executions/{id}/start
func (h *ExecutionHandler) StartExecution(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/executions/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	status := store.PipelineExecutionRunning
	err := h.ExecutionStore.UpdateStatus(id, status)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ─── StopExecution ────────────────────────────────────────────────────────────

// StopExecution handles POST /executions/{id}/stop
func (h *ExecutionHandler) StopExecution(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/executions/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	status := store.PipelineExecutionCancelled
	err := h.ExecutionStore.UpdateStatus(id, status)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ─── RetryExecution ───────────────────────────────────────────────────────────

// RetryExecution handles POST /executions/{id}/retry
func (h *ExecutionHandler) RetryExecution(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/executions/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	retryCount := h.ExecutionStore.IncrementRetryCount(id)

	status := store.PipelineExecutionRunning
	err := h.ExecutionStore.UpdateStatus(id, status)
	if err != nil {
		h.WriteError(w, http.StatusNotFound, 404, err.Error())
		return
	}

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"execution_id": id,
		"retry_count":  retryCount,
	})
}

// ─── GetExecutionSteps ────────────────────────────────────────────────────────

// GetExecutionSteps handles GET /executions/{id}/steps
func (h *ExecutionHandler) GetExecutionSteps(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/executions/")
	if id == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "execution id is required")
		return
	}

	steps := h.ExecutionStore.GetSteps(id)

	h.WriteJSON(w, http.StatusOK, steps)
}

// ─── GetExecutionsByPipeline ─────────────────────────────────────────────────

// GetExecutionsByPipeline handles GET /executions/pipeline/{pipelineId}
func (h *ExecutionHandler) GetExecutionsByPipeline(w http.ResponseWriter, r *http.Request) {
	pipelineID := extractIDFromPath(r.URL.Path, "/executions/pipeline/")
	if pipelineID == "" {
		h.WriteError(w, http.StatusBadRequest, 400, "pipeline id is required")
		return
	}

	page := 1
	pageSize := 20
	limit := 0
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
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if s := q.Get("status"); s != "" {
		status = &s
	}

	executions, total, hasMore := h.ExecutionStore.ListByPipeline(pipelineID, page, pageSize, status, limit)

	h.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"executions": executions,
		"total":      total,
		"has_more":   hasMore,
	})
}

// ─── GetExecutionAnalytics ────────────────────────────────────────────────────

// GetExecutionAnalytics handles GET /executions/analytics
func (h *ExecutionHandler) GetExecutionAnalytics(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	// Get all executions for the tenant
	execs, total, _ := h.ExecutionStore.ListByTenant(tenantID, 1, 0)

	// Aggregate analytics
	totalExecutions := total
	completed := 0
	failed := 0
	cancelled := 0
	totalDuration := float64(0)

	for _, ex := range execs {
		switch ex.Status {
		case store.PipelineExecutionCompleted:
			completed++
			totalDuration += ex.DurationMS
		case store.PipelineExecutionFailed:
			failed++
		case store.PipelineExecutionCancelled:
			cancelled++
		}
	}

	avgDuration := float64(0)
	if completed > 0 {
		avgDuration = totalDuration / float64(completed)
	}

	successRate := 0.0
	if totalExecutions > 0 {
		successRate = float64(completed) / float64(totalExecutions) * 100
	}

	analytics := store.PipelineAnalytics{
		TotalExecutions:       totalExecutions,
		CompletedExecutions:   completed,
		FailedExecutions:      failed,
		CancelledExecutions:   cancelled,
		SuccessRate:           successRate,
		AvgDurationMS:         avgDuration,
	}

	h.WriteJSON(w, http.StatusOK, analytics)
}
