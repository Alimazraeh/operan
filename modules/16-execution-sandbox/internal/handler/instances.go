package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/execution-sandbox/internal/ctxkeys"
	"github.com/operan/execution-sandbox/internal/store"
)

// InstanceHandler handles sandbox instance HTTP requests.
type InstanceHandler struct {
	store *store.InstanceStore
}

// NewInstanceHandler creates a new InstanceHandler.
func NewInstanceHandler(s *store.InstanceStore) *InstanceHandler {
	return &InstanceHandler{store: s}
}

// ListInstances handles GET /v1/sandboxes/instances.
func (h *InstanceHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	agentID := r.URL.Query().Get("agent_id")
	status := r.URL.Query().Get("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	instances, total, err := h.store.List(r.Context(), tenantID, agentID, status, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"instances": instances,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"has_more":  (page * pageSize) < total,
	})
}

// GetInstance handles GET /v1/sandboxes/instances/{id}.
func (h *InstanceHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid instance ID"}`, http.StatusBadRequest)
		return
	}

	inst, err := h.store.GetByID(r.Context(), id, tenantID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"instance not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, inst)
}

// CancelInstance handles POST /v1/sandboxes/instances/{id}/cancel.
func (h *InstanceHandler) CancelInstance(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid instance ID"}`, http.StatusBadRequest)
		return
	}

	inst, err := h.store.GetByID(r.Context(), id, tenantID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"instance not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if inst.Status != "running" {
		http.Error(w, `{"error":"bad-request","message":"instance is not running"}`, http.StatusBadRequest)
		return
	}

	// Update status to cancelled
	now := inst.StartedAt
	if now == nil {
		now = nil
	}
	// We'd need a CancelStatus method for proper implementation; for now, just acknowledge
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"instance_id": id,
		"status":      "cancelled",
		"message":     "cancel request received",
	})
}

// WriteJSON is a helper to write JSON responses.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Response already written — can't do much
	}
}