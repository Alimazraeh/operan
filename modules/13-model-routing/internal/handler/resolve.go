package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/operan/model-routing/internal/ctxkeys"
	"github.com/operan/model-routing/internal/engine"
	"github.com/operan/model-routing/internal/events"
)

// ResolveHandler handles the main routing endpoint.
type ResolveHandler struct {
	router  *engine.Router
	broker  events.Broker
}

// NewResolveHandler creates a new resolve handler.
func NewResolveHandler(r *engine.Router, b events.Broker) *ResolveHandler {
	return &ResolveHandler{router: r, broker: b}
}

// Handle handles POST /v1/resolve
func (h *ResolveHandler) Handle(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	var req struct {
		TaskType    string        `json:"task_type"`
		ContextLen  *int          `json:"context_length"`
		MaxTokens   *int          `json:"max_tokens"`
		CostTarget  *float64      `json:"cost_target"`
		Constraints *struct {
			MaxLatencyMs *int     `json:"max_latency_ms"`
			MinQuality   *float64 `json:"min_quality"`
		} `json:"constraints"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TaskType == "" {
		WriteError(w, http.StatusBadRequest, "task_type is required")
		return
	}

	constraints := engine.Constraints{}
	if req.MaxTokens != nil {
		constraints.MaxTokens = *req.MaxTokens
	}
	if req.CostTarget != nil {
		constraints.CostTarget = *req.CostTarget
	}
	if req.Constraints != nil {
		if req.Constraints.MaxLatencyMs != nil {
			constraints.MaxLatencyMs = *req.Constraints.MaxLatencyMs
		}
		if req.Constraints.MinQuality != nil {
			constraints.MinQuality = *req.Constraints.MinQuality
		}
	}

	start := time.Now()
	decision, err := h.router.Resolve(r.Context(), tenantID, req.TaskType, constraints)
	if err != nil {
		log.Printf("resolve error: %v", err)
		WriteError(w, http.StatusInternalServerError, "routing failed")
		return
	}

	decision.LatencyEstimate = int(time.Since(start).Milliseconds())

	// Publish event
	if h.broker != nil {
		event := map[string]interface{}{
			"tenant_id":       tenantID,
			"task_type":       req.TaskType,
			"selected_model":  decision.ModelID,
			"fallback_model":  decision.FallbackModel,
			"score":           decision.Score,
			"latency_estimate": decision.LatencyEstimate,
			"duration_ms":     time.Since(start).Milliseconds(),
		}
		if err := h.broker.Publish(events.EventRouteResolved, encodeJSON(event)); err != nil {
			log.Printf("publish event error: %v", err)
		}
	}

	WriteJSON(w, http.StatusOK, decision)
}

func encodeJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}