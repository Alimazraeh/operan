package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
)

// ResultsHandler handles ingestion result queries.
type ResultsHandler struct {
	resultsStore *store.ResultsStore
}

// NewResultsHandler creates a new ResultsHandler.
func NewResultsHandler(rs *store.ResultsStore) *ResultsHandler {
	return &ResultsHandler{resultsStore: rs}
}

// ListResults handles GET /v1/jobs/{id}/results
func (h *ResultsHandler) ListResults(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	jobID = strings.TrimSuffix(jobID, "/results")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "missing job id")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	results, err := h.resultsStore.GetByJobID(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	total := len(results)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedResults := results[start:end]

	WriteJSON(w, http.StatusOK, map[string]any{
		"results":   pagedResults,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}