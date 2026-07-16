package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/operan/agent-collaboration/internal/ctxkeys"
	"github.com/operan/agent-collaboration/internal/events"
	"github.com/operan/agent-collaboration/internal/store"
)

// HandoffHandler handles handoff HTTP requests.
type HandoffHandler struct {
	store    *store.HandoffStore
	eventPub *events.Publisher
}

// NewHandoffHandler creates a new HandoffHandler.
func NewHandoffHandler(s *store.HandoffStore, pub *events.Publisher) *HandoffHandler {
	return &HandoffHandler{store: s, eventPub: pub}
}

// CreateHandoff handles POST /handoffs.
func (h *HandoffHandler) CreateHandoff(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	fromAgentID := ctxkeys.GetUserID(r.Context())

	var req struct {
		ToAgentID      string                 `json:"to_agent_id"`
		Title          string                 `json:"title"`
		Description    *string                `json:"description"`
		Context        map[string]interface{} `json:"context"`
		Priority       string                 `json:"priority"`
		ChannelID      *string                `json:"channel_id"`
		ExpiresInSeconds *int                 `json:"expires_in_seconds"`
		ParentMessageID *string               `json:"parent_message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.ToAgentID == "" {
		http.Error(w, `{"error":"bad-request","message":"to_agent_id is required"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"bad-request","message":"title is required"}`, http.StatusBadRequest)
		return
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if req.Context == nil {
		req.Context = make(map[string]interface{})
	}

	now := time.Now()
	var expiresAt *time.Time
	if req.ExpiresInSeconds != nil && *req.ExpiresInSeconds > 0 {
		exp := now.Add(time.Duration(*req.ExpiresInSeconds) * time.Second)
		expiresAt = &exp
	}

	handoff := &store.Handoff{
		TenantID:        tenantID,
		FromAgentID:     fromAgentID,
		ToAgentID:       req.ToAgentID,
		ChannelID:       req.ChannelID,
		ParentMessageID: req.ParentMessageID,
		Title:           req.Title,
		Description:     req.Description,
		Context:         req.Context,
		Priority:        req.Priority,
		Status:          "pending",
		ExpiresAt:       expiresAt,
	}

	if err := h.store.Create(r.Context(), handoff); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	channelID := ""
	if handoff.ChannelID != nil {
		channelID = *handoff.ChannelID
	}
	h.eventPub.EventHandoffCreated(tenantID, handoff.ID, fromAgentID, req.ToAgentID, req.Priority, channelID)

	WriteJSON(w, http.StatusCreated, handoff)
}

// ListHandoffs handles GET /handoffs.
func (h *HandoffHandler) ListHandoffs(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	toAgentID := r.URL.Query().Get("to_agent_id")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	handoffs, total, err := h.store.List(r.Context(), tenantID, toAgentID, status, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"handoffs":  handoffs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"has_more":  (page * pageSize) < total,
	})
}

// AcceptHandoff handles POST /handoffs/{id}/accept.
func (h *HandoffHandler) AcceptHandoff(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	agentID := ctxkeys.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")

	var req struct {
		Response *string `json:"response"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.store.AcceptHandoff(r.Context(), idStr, agentID, strVal(req.Response)); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	h.eventPub.EventHandoffAccepted(tenantID, idStr, agentID)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "accepted", "handoff_id": idStr})
}

// CompleteHandoff handles POST /handoffs/{id}/complete.
func (h *HandoffHandler) CompleteHandoff(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	agentID := ctxkeys.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")

	var req struct {
		Response *string `json:"response"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.store.CompleteHandoff(r.Context(), idStr, agentID, strVal(req.Response)); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	h.eventPub.EventHandoffCompleted(tenantID, idStr, agentID, 0)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "completed", "handoff_id": idStr})
}

// RejectHandoff handles POST /handoffs/{id}/reject.
func (h *HandoffHandler) RejectHandoff(w http.ResponseWriter, r *http.Request) {
	_ = ctxkeys.GetTenantID(r.Context())
	agentID := ctxkeys.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")

	var req struct {
		Response *string `json:"response"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.store.RejectHandoff(r.Context(), idStr, agentID, strVal(req.Response)); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected", "handoff_id": idStr})
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}