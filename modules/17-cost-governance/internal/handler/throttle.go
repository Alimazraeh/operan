package handler

import (
	"net/http"
	"time"

	"github.com/operan/cost-governance/internal/ctxkeys"
	"github.com/operan/cost-governance/internal/engine"

	"github.com/go-chi/chi/v5"
)

// ThrottleHandler handles throttle endpoints.
type ThrottleHandler struct {
	throttleMgr *engine.ThrottleManager
}

// NewThrottleHandler creates a new ThrottleHandler.
func NewThrottleHandler(throttleMgr *engine.ThrottleManager) *ThrottleHandler {
	return &ThrottleHandler{throttleMgr: throttleMgr}
}

// GetThrottle handles GET /v1/throttle
func (h *ThrottleHandler) GetThrottle(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	state, updatedAt := h.throttleMgr.GetThrottleInfo(tenantID)

	WriteJSON(w, http.StatusOK, map[string]any{
		"throttle_state": state,
		"updated_at":     updatedAt,
	})
}

// SetThrottle handles PATCH /v1/throttle/{status}
func (h *ThrottleHandler) SetThrottle(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	status := chi.URLParam(r, "status")

	valid := map[string]bool{"none": true, "soft": true, "hard": true}
	if !valid[status] {
		http.Error(w, `{"error":"invalid throttle status; must be none, soft, or hard"}`, http.StatusBadRequest)
		return
	}

	h.throttleMgr.SetState(tenantID, status)

	WriteJSON(w, http.StatusOK, map[string]any{
		"throttle_state": status,
		"updated_at":     time.Now(),
	})
}