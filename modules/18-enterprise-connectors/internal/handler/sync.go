package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/enterprise-connectors/internal/connectors"
	"github.com/operan/enterprise-connectors/internal/ctxkeys"
	"github.com/operan/enterprise-connectors/internal/store"
)

// SyncEngine defines the interface for sync operations.
type SyncEngine interface {
	RunSync(ctx context.Context, tenantID string, connectorID uuid.UUID, syncType string) (*connectors.SyncResult, error)
	HealthCheck(ctx context.Context, tenantID string, connectorID uuid.UUID) (*connectors.HealthCheckResult, error)
}

// SyncHandler handles HTTP requests for sync operations.
type SyncHandler struct {
	syncEngine     SyncEngine
	connectorStore *store.ConnectorStore
	syncStore      *store.SyncStore
}

// NewSyncHandler creates a new SyncHandler.
func NewSyncHandler(se SyncEngine, cs *store.ConnectorStore, ss *store.SyncStore) *SyncHandler {
	return &SyncHandler{
		syncEngine:     se,
		connectorStore: cs,
		syncStore:      ss,
	}
}

// Routes registers sync routes on the given router.
func (h *SyncHandler) Routes(r chi.Router) {
	r.Post("/v1/connectors/{id}/sync", h.TriggerSync)
	r.Get("/v1/connectors/{id}/health", h.HealthCheck)
	r.Get("/v1/sync-history", h.ListSyncHistory)
}

// TriggerSync handles POST /v1/connectors/{id}/sync.
func (h *SyncHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid connector ID"}`, http.StatusBadRequest)
		return
	}

	_, err = h.connectorStore.GetByID(r.Context(), id, tenantID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"connector not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	syncType := r.URL.Query().Get("type")
	if syncType == "" {
		syncType = "full"
	}

	result, err := h.syncEngine.RunSync(r.Context(), tenantID, id, syncType)
	if err != nil {
		http.Error(w, `{"error":"sync-failed","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"sync_id":    id.String(),
		"status":     "completed",
		"result":     result,
		"started_at": time.Now().UTC(),
	})
}

// HealthCheck handles GET /v1/connectors/{id}/health.
func (h *SyncHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid connector ID"}`, http.StatusBadRequest)
		return
	}

	_, err = h.connectorStore.GetByID(r.Context(), id, tenantID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"connector not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	result, err := h.syncEngine.HealthCheck(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, `{"error":"health-check-failed","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ListSyncHistory handles GET /v1/sync-history.
func (h *SyncHandler) ListSyncHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	connectorID := r.URL.Query().Get("connector_id")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	history, total, err := h.syncStore.List(r.Context(), tenantID, connectorID, status, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"history":  history,
		"page":     page,
		"page_size": pageSize,
		"total":    total,
		"has_more": (page * pageSize) < total,
	})
}