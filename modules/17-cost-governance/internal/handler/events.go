package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/cost-governance/internal/ctxkeys"
	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/store"
)

// EventsHandler handles cost events endpoints.
type EventsHandler struct {
	eventStore *store.CostEventStore
	engine     *engine.Engine
}

// NewEventsHandler creates a new EventsHandler.
func NewEventsHandler(eventStore *store.CostEventStore, engine *engine.Engine) *EventsHandler {
	return &EventsHandler{eventStore: eventStore, engine: engine}
}

// IngestCostEvent handles POST /v1/cost-events
func (h *EventsHandler) IngestCostEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceModule     string  `json:"source_module"`
		AgentID          string  `json:"agent_id"`
		ModelName        string  `json:"model_name"`
		CostUSD          float64 `json:"cost_usd"`
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		EventType        string  `json:"event_type"`
		BillingTag       *string `json:"billing_tag,omitempty"`
		SourceID         *string `json:"source_id,omitempty"`
		EventTimestamp   *string `json:"event_timestamp,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.CostUSD <= 0 {
		http.Error(w, `{"error":"cost_usd must be positive"}`, http.StatusBadRequest)
		return
	}
	if req.SourceModule == "" {
		http.Error(w, `{"error":"source_module is required"}`, http.StatusBadRequest)
		return
	}

	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	eventTimestamp := time.Now()
	if req.EventTimestamp != nil {
		if t, err := time.Parse(time.RFC3339, *req.EventTimestamp); err == nil {
			eventTimestamp = t
		}
	}

	event := &store.CostEvent{
		TenantID:         tenantID,
		AgentID:          nil,
		SourceModule:     req.SourceModule,
		SourceID:         req.SourceID,
		ModelName:        &req.ModelName,
		CostUSD:          req.CostUSD,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		EventType:        req.EventType,
		BillingTag:       req.BillingTag,
		EventTimestamp:   eventTimestamp,
	}
	if req.AgentID != "" {
		event.AgentID = &req.AgentID
	}

	if err := h.eventStore.Create(r.Context(), event); err != nil {
		http.Error(w, `{"error":"failed to record cost event"}`, http.StatusInternalServerError)
		return
	}

	// Check budgets
	result, err := h.engine.CheckBudgets(r.Context(), tenantID, req.AgentID, req.CostUSD)
	if err != nil {
		http.Error(w, `{"error":"budget check failed"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"event_id":      event.ID,
		"accepted":      result.Accepted,
		"throttle_level": result.ThrottleLevel,
	})
}

// ListCostEvents handles GET /v1/cost-events
func (h *EventsHandler) ListCostEvents(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	agentID := r.URL.Query().Get("agent_id")
	sourceModule := r.URL.Query().Get("source_module")

	events, total, err := h.eventStore.List(r.Context(), tenantID, agentID, sourceModule, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"failed to list cost events"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"events":    events,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}