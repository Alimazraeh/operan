package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/operan/agent-collaboration/internal/ctxkeys"
	"github.com/operan/agent-collaboration/internal/events"
	"github.com/operan/agent-collaboration/internal/store"
)

// MessageHandler handles message HTTP requests.
type MessageHandler struct {
	store    *store.MessageStore
	eventPub *events.Publisher
}

// NewMessageHandler creates a new MessageHandler.
func NewMessageHandler(s *store.MessageStore, pub *events.Publisher) *MessageHandler {
	return &MessageHandler{store: s, eventPub: pub}
}

// SendMessage handles POST /messages.
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	senderID := ctxkeys.GetUserID(r.Context())

	var req struct {
		ChannelID   string                 `json:"channel_id"`
		Content     string                 `json:"content"`
		MessageType string                 `json:"message_type"`
		ParentID    *string                `json:"parent_id"`
		Attachments []map[string]interface{} `json:"attachments"`
		SenderName  *string                `json:"sender_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad-request","message":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, `{"error":"bad-request","message":"content is required"}`, http.StatusBadRequest)
		return
	}
	if req.ChannelID == "" {
		http.Error(w, `{"error":"bad-request","message":"channel_id is required"}`, http.StatusBadRequest)
		return
	}
	if req.MessageType == "" {
		req.MessageType = "text"
	}
	if req.Attachments == nil {
		req.Attachments = []map[string]interface{}{}
	}

	m := &store.Message{
		TenantID:    tenantID,
		ChannelID:   req.ChannelID,
		ParentID:    req.ParentID,
		SenderID:    senderID,
		SenderName:  req.SenderName,
		MessageType: req.MessageType,
		Content:     req.Content,
		Attachments: req.Attachments,
		ReplyCount:  0,
	}

	if err := h.store.Create(r.Context(), m); err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// If this is a reply, increment parent reply_count
	if m.ParentID != nil && *m.ParentID != "" {
		h.store.IncrementReplyCount(r.Context(), *m.ParentID)
	}

	h.eventPub.EventMessageSent(tenantID, m.ChannelID, m.ID, m.SenderID, m.MessageType)

	WriteJSON(w, http.StatusCreated, m)
}

// ListMessages handles GET /channels/{id}/messages.
func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	channelID := chi.URLParam(r, "channel_id")
	messageType := r.URL.Query().Get("message_type")
	replyTo := r.URL.Query().Get("reply_to")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	messages, total, err := h.store.List(r.Context(), channelID, messageType, replyTo, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"internal-error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"page":     page,
		"page_size": pageSize,
		"total":    total,
		"has_more": (page * pageSize) < total,
	})
}