package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/operan/agent-collaboration/internal/ctxkeys"
	"github.com/operan/agent-collaboration/internal/events"
	"github.com/operan/agent-collaboration/internal/presence"
	"github.com/operan/agent-collaboration/internal/store"
)

// PresenceHandler handles presence HTTP requests.
type PresenceHandler struct {
	store    *store.PresenceStore
	manager  *presence.Manager
	eventPub *events.Publisher
}

// NewPresenceHandler creates a new PresenceHandler.
func NewPresenceHandler(s *store.PresenceStore, m *presence.Manager, pub *events.Publisher) *PresenceHandler {
	return &PresenceHandler{store: s, manager: m, eventPub: pub}
}

// Heartbeat handles POST /presence/heartbeat.
func (h *PresenceHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req struct {
		AgentID  string                 `json:"agent_id"`
		Status   string                 `json:"status"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.AgentID == "" {
		http.Error(w, `{"error":"bad-request","message":"agent_id is required"}`, http.StatusBadRequest)
		return
	}
	if req.Status == "" {
		req.Status = "online"
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	if err := h.manager.UpdateHeartbeat(tenantID, req.AgentID, req.Status, req.Metadata); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	h.eventPub.EventPresenceUpdated(tenantID, req.AgentID, req.Status, time.Now())

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id":       req.AgentID,
		"status":         req.Status,
		"last_heartbeat": time.Now().Unix(),
	})
}

// ListPresence handles GET /presence.
func (h *PresenceHandler) ListPresence(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	agentIDFilter := r.URL.Query().Get("agent_id")

	presences, err := h.store.List(r.Context(), tenantID, agentIDFilter)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"presence": presences,
	})
}

// GetPresence handles GET /presence/{agent_id}.
func (h *PresenceHandler) GetPresence(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	agentID := chi.URLParam(r, "agent_id")

	p, err := h.store.GetByAgentID(r.Context(), tenantID, agentID)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"presence not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, p)
}