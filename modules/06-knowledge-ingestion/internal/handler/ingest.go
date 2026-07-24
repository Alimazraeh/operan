package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// jobCreator abstracts job creation for the ingest handler.
type jobCreator interface {
	Create(ctx context.Context, job *store.IngestionJob) error
}

// jobStarter abstracts job processing for the ingest handler.
type jobStarter interface {
	ProcessJob(ctx context.Context, jobID string, serviceToken string)
}

// IngestHandler handles ingestion trigger endpoints.
type IngestHandler struct {
	jobsStore    jobCreator
	worker       jobStarter
	serviceToken string
}

// NewIngestHandler creates a new IngestHandler.
func NewIngestHandler(js jobCreator, w jobStarter, serviceToken string) *IngestHandler {
	return &IngestHandler{jobsStore: js, worker: w, serviceToken: serviceToken}
}

// IngestSource handles POST /v1/ingest
func (h *IngestHandler) IngestSource(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	var req struct {
		SourceID string `json:"source_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SourceID == "" {
		writeError(w, http.StatusBadRequest, "source_id is required")
		return
	}

	job := &store.IngestionJob{
		TenantID:        tenantID,
		SourceID:        req.SourceID,
		Status:          "pending",
		TotalChunks:     0,
		ProcessedChunks: 0,
		CreatedAt:       time.Now(),
	}

	if err := h.jobsStore.Create(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job: "+err.Error())
		return
	}

	// The job outlives this request — never hand it the request context
	// (it cancels the moment the response is written). Forward the caller's
	// token so downstream writes (M07) run under the caller's identity;
	// the configured service token is only a fallback for recovery runs.
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = h.serviceToken
	}
	h.worker.ProcessJob(context.Background(), job.ID, token)

	WriteJSON(w, http.StatusOK, map[string]any{
		"job_id":       job.ID,
		"status":       job.Status,
		"total_chunks": 0,
	})
}
