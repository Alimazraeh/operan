package handler

import (
	"net/http"
	"strconv"

	"github.com/operan/cost-governance/internal/ctxkeys"
	"github.com/operan/cost-governance/internal/store"
)

// AlertsHandler handles alerts endpoints.
type AlertsHandler struct {
	alertStore *store.AlertStore
}

// NewAlertsHandler creates a new AlertsHandler.
func NewAlertsHandler(alertStore *store.AlertStore) *AlertsHandler {
	return &AlertsHandler{alertStore: alertStore}
}

// ListAlerts handles GET /v1/alerts
func (h *AlertsHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	severity := r.URL.Query().Get("severity")
	alertType := r.URL.Query().Get("alert_type")
	isResolvedStr := r.URL.Query().Get("is_resolved")

	var isResolved *bool
	if isResolvedStr != "" {
		v, _ := strconv.ParseBool(isResolvedStr)
		isResolved = &v
	}

	alerts, total, err := h.alertStore.List(r.Context(), tenantID, severity, alertType, isResolved, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"failed to list alerts"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"alerts":    alerts,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}