package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"context"

	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// JobsHandler handles ingestion job management.
type JobsHandler struct {
	jobsStore JobsStore
	worker    Worker
}

// JobsStore is the job persistence interface.
type JobsStore interface {
	GetByID(ctx context.Context, id string) (*store.IngestionJob, error)
	ListByTenant(ctx context.Context, tenantID string, statusFilter *string, sourceID *string, page, pageSize int) ([]store.IngestionJob, int, error)
}

// Worker is the worker interface for cancellation.
type Worker interface {
	CancelJob(jobID string) bool
}

// NewJobsHandler creates a new JobsHandler.
func NewJobsHandler(js JobsStore, w Worker) *JobsHandler {
	return &JobsHandler{jobsStore: js, worker: w}
}

// ListJobs handles GET /v1/jobs
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	statusFilter := r.URL.Query().Get("status")
	sourceID := r.URL.Query().Get("source_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var sf *string
	if statusFilter != "" {
		sf = &statusFilter
	}
	var sid *string
	if sourceID != "" {
		sid = &sourceID
	}

	jobs, total, err := h.jobsStore.ListByTenant(r.Context(), tenantID, sf, sid, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"jobs":      jobs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetJob handles GET /v1/jobs/{id}
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	id = strings.TrimSuffix(id, "/cancel")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing job id")
		return
	}

	job, err := h.jobsStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var progress float64
	if job.TotalChunks > 0 {
		progress = float64(job.ProcessedChunks) / float64(job.TotalChunks) * 100
	}

	result := map[string]any{
		"id":             job.ID,
		"tenant_id":      job.TenantID,
		"source_id":      job.SourceID,
		"status":         job.Status,
		"total_chunks":   job.TotalChunks,
		"processed_chunks": job.ProcessedChunks,
		"progress_pct":   progress,
		"error_message":  job.ErrorMessage,
		"created_at":     job.CreatedAt,
	}
	if job.StartedAt != nil {
		result["started_at"] = job.StartedAt
	}
	if job.CompletedAt != nil {
		result["completed_at"] = job.CompletedAt
	}

	WriteJSON(w, http.StatusOK, result)
}

// CancelJob handles POST /v1/jobs/{id}/cancel
func (h *JobsHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	id = strings.TrimSuffix(id, "/cancel")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing job id")
		return
	}

	job, err := h.jobsStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		writeError(w, http.StatusBadRequest, "job is already "+job.Status)
		return
	}

	if !h.worker.CancelJob(id) {
		writeError(w, http.StatusBadRequest, "job not found or not running")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "job cancelled"})
}

var _ = json.Marshal