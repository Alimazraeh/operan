package handlers

import (
	"net/http"
	"strconv"

	"github.com/operan/modules/05-department-template-engine/internal/middleware"
)

// ListBriefings handles GET /briefings — the tenant's operating-cadence
// digests, newest first. Optional query: department_id, limit (default 10).
func (h *TemplateHandlers) ListBriefings(w http.ResponseWriter, r *http.Request) {
	reqID := middleware.RequestIDFromContext(r.Context())
	tenantID := middleware.TenantIDFromContext(r.Context())
	if h.Briefings == nil {
		writeError(w, http.StatusServiceUnavailable, "about:blank", "Unavailable",
			"briefing store not configured", r.URL.Path, reqID)
		return
	}
	limit := 10
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	items := h.Briefings.List(tenantID, r.URL.Query().Get("department_id"), limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": items,
		"meta": map[string]interface{}{"total": len(items)},
	})
}
