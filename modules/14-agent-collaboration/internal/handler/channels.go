package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/agent-collaboration/internal/ctxkeys"
	"github.com/operan/agent-collaboration/internal/events"
	"github.com/operan/agent-collaboration/internal/store"
)

// ChannelHandler handles channel HTTP requests.
type ChannelHandler struct {
	store     *store.ChannelStore
	eventPub  *events.Publisher
}

// NewChannelHandler creates a new ChannelHandler.
func NewChannelHandler(s *store.ChannelStore, pub *events.Publisher) *ChannelHandler {
	return &ChannelHandler{store: s, eventPub: pub}
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Response already written — can't do much
	}
}

// ListChannels handles GET /channels.
func (h *ChannelHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	channelType := r.URL.Query().Get("channel_type")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	channels, total, err := h.store.List(r.Context(), tenantID, channelType, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channels,
		"page":     page,
		"page_size": pageSize,
		"total":    total,
		"has_more": (page * pageSize) < total,
	})
}

// CreateChannel handles POST /channels.
func (h *ChannelHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	userID := ctxkeys.GetUserID(r.Context())

	var req struct {
		Name        string                 `json:"name"`
		Description *string                `json:"description"`
		ChannelType string                 `json:"channel_type"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"bad-request","message":"name is required"}`, http.StatusBadRequest)
		return
	}
	if req.ChannelType == "" {
		req.ChannelType = "general"
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	ch := &store.Channel{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		ChannelType: req.ChannelType,
		CreatorID:   userID,
		MaxMembers:  50,
		IsPublic:    false,
		Metadata:    req.Metadata,
	}

	if err := h.store.Create(r.Context(), ch); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Auto-add creator as member
	h.store.AddMember(r.Context(), ch.ID, userID, "owner")

	WriteJSON(w, http.StatusCreated, ch)
}

// GetChannel handles GET /channels/{id}.
func (h *ChannelHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid channel ID"}`, http.StatusBadRequest)
		return
	}

	ch, err := h.store.GetByID(r.Context(), id.String())
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"channel not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if ch.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden","message":"tenant mismatch"}`, http.StatusForbidden)
		return
	}

	// Add member count
	memberCount, _ := h.store.MemberCount(r.Context(), ch.ID)
	chResp := struct {
		*store.Channel
		MemberCount int `json:"member_count"`
	}{ch, memberCount}

	WriteJSON(w, http.StatusOK, chResp)
}

// UpdateChannel handles PATCH /channels/{id}.
func (h *ChannelHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	ch, err := h.store.GetByID(r.Context(), idStr)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"channel not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if ch.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden","message":"tenant mismatch"}`, http.StatusForbidden)
		return
	}

	var req struct {
		Name        *string                `json:"name"`
		Description *string                `json:"description"`
		ChannelType *string                `json:"channel_type"`
		MaxMembers  *int                   `json:"max_members"`
		IsPublic    *bool                  `json:"is_public"`
		Metadata    *map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		ch.Name = *req.Name
	}
	if req.Description != nil {
		ch.Description = req.Description
	}
	if req.ChannelType != nil {
		ch.ChannelType = *req.ChannelType
	}
	if req.MaxMembers != nil {
		ch.MaxMembers = *req.MaxMembers
	}
	if req.IsPublic != nil {
		ch.IsPublic = *req.IsPublic
	}
	if req.Metadata != nil {
		ch.Metadata = *req.Metadata
	}

	if err := h.store.Update(r.Context(), ch); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, ch)
}

// DeleteChannel handles DELETE /channels/{id}.
func (h *ChannelHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	if err := h.store.Delete(r.Context(), idStr, tenantID); err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"channel not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// JoinChannel handles POST /channels/{id}/join.
func (h *ChannelHandler) JoinChannel(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	agentID := ctxkeys.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")

	ch, err := h.store.GetByID(r.Context(), idStr)
	if err != nil {
		if err == store.ErrNotFound {
			http.Error(w, `{"error":"not-found","message":"channel not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if ch.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden","message":"tenant mismatch"}`, http.StatusForbidden)
		return
	}

	// Check membership
	existing, _ := h.store.IsMember(r.Context(), idStr, agentID)
	if existing {
		http.Error(w, `{"error":"conflict","message":"already a member"}`, http.StatusConflict)
		return
	}

	if err := h.store.AddMember(r.Context(), idStr, agentID, "member"); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Publish event
	h.eventPub.EventChannelJoined(tenantID, idStr, agentID)

	WriteJSON(w, http.StatusOK, map[string]string{
		"channel_id": idStr,
		"agent_id":   agentID,
		"status":     "joined",
	})
}

// LeaveChannel handles DELETE /channels/{id}/leave.
func (h *ChannelHandler) LeaveChannel(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	agentID := ctxkeys.GetUserID(r.Context())
	idStr := chi.URLParam(r, "id")

	// Check membership
	existing, err := h.store.IsMember(r.Context(), idStr, agentID)
	if err != nil || !existing {
		http.Error(w, `{"error":"not-found","message":"not a member"}`, http.StatusNotFound)
		return
	}

	if err := h.store.RemoveMember(r.Context(), idStr, agentID); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	h.eventPub.EventChannelLeft(tenantID, idStr, agentID)

	w.WriteHeader(http.StatusNoContent)
}