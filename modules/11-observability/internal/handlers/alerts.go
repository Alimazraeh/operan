package handlers

import (
	"net/http"
	"strconv"

	"github.com/operan/modules/11-observability/internal/middleware"
	"github.com/operan/modules/11-observability/internal/store"
)

// ListAlerts handles GET /alerts.
func (h *ObservabilityHandlers) ListAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if !tenantOK(w, r, r.URL.Query().Get("tenant_id"), tenantID) {
		return
	}
	page, pageSize := h.pagination(r)

	if v := queryPtr(r, "severity"); v != nil && !store.ValidAlertSeverity(*v) {
		writeError(w, r, http.StatusBadRequest, "invalid severity filter")
		return
	}
	var resolved *bool
	if v := r.URL.Query().Get("resolved"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "resolved must be a boolean")
			return
		}
		resolved = &b
	}

	items, total, hasMore := h.Alerts.List(tenantID, page, pageSize, queryPtr(r, "severity"), resolved)
	if items == nil {
		items = []store.Alert{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  hasMore,
	})
}

// ResolveAlert handles POST /alerts/{id}/resolve.
func (h *ObservabilityHandlers) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	alert, err := h.Alerts.Resolve(r.PathValue("id"), tenantID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "alert not found")
		return
	}
	writeJSON(w, http.StatusOK, alert)
}
