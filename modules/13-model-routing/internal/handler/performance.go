package handler

import (
	"log"
	"net/http"

	"github.com/operan/model-routing/internal/ctxkeys"
	"github.com/operan/model-routing/internal/store"
)

// PerformanceHandler handles performance metrics endpoints.
type PerformanceHandler struct {
	store store.PerfStore
}

// NewPerformanceHandler creates a new performance handler.
func NewPerformanceHandler(s store.PerfStore) *PerformanceHandler {
	return &PerformanceHandler{store: s}
}

// HandleGet handles GET /v1/performance
func (h *PerformanceHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	modelID := r.URL.Query().Get("model_id")
	taskType := r.URL.Query().Get("task_type")

	var metrics interface{}
	var err error

	if modelID != "" && taskType != "" {
		m, err := h.store.GetByModelAndTask(tenantID, modelID, taskType)
		if err != nil {
			log.Printf("get performance error: %v", err)
			WriteError(w, http.StatusInternalServerError, "failed to get performance")
			return
		}
		if m == nil {
			WriteJSON(w, http.StatusOK, map[string]interface{}{
				"model_id": modelID, "task_type": taskType,
			})
			return
		}
		metrics = m
	} else if modelID != "" {
		metrics, err = h.store.GetByModel(tenantID, modelID)
	} else if taskType != "" {
		metrics, err = h.store.GetByTaskType(tenantID, taskType)
	} else {
		metrics, err = h.store.GetByModel(tenantID, "")
	}

	if err != nil {
		log.Printf("get performance error: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to get performance")
		return
	}

	WriteJSON(w, http.StatusOK, metrics)
}