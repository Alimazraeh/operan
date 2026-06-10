package handlers

import (
	"net/http"
	"time"

	"github.com/operan/modules/11-observability/internal/middleware"
	"github.com/operan/modules/11-observability/internal/store"
)

// GetHealthStatus handles GET /health (contract: tenant system health,
// auth-required — the service liveness probe lives at /healthz).
func (h *ObservabilityHandlers) GetHealthStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())
	if !tenantOK(w, r, r.URL.Query().Get("tenant_id"), tenantID) {
		return
	}

	components, overall := h.Health.Overview(tenantID)
	if components == nil {
		components = []store.HealthStatus{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":      tenantID,
		"components":     components,
		"overall_status": overall,
		"last_updated":   time.Now().UTC().Format(time.RFC3339),
	})
}

// GetComponentHealth handles GET /health/{componentId}.
func (h *ObservabilityHandlers) GetComponentHealth(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantIDFromContext(r.Context())

	hs, err := h.Health.Get(tenantID, r.PathValue("componentId"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "component not found")
		return
	}
	writeJSON(w, http.StatusOK, hs)
}
