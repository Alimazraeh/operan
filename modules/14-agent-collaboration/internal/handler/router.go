package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	osm "github.com/operan/agent-collaboration/internal/middleware"
	"github.com/operan/agent-collaboration/internal/events"
	"github.com/operan/agent-collaboration/internal/presence"
	"github.com/operan/agent-collaboration/internal/store"
)

// SetupRouter creates the chi router with all routes.
func SetupRouter(channelStore *store.ChannelStore, messageStore *store.MessageStore,
	handoffStore *store.HandoffStore, presenceStore *store.PresenceStore,
	presenceManager *presence.Manager, eventPub *events.Publisher,
) http.Handler {
	r := chi.NewRouter()
	r.Use(osm.SetupCORS())
	r.Use(osm.Logger)
	r.Use(osm.RequestID)
	r.Use(osm.TraceID)

	// Health endpoint — unauthenticated
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes
	channelHandler := NewChannelHandler(channelStore, eventPub)
	messageHandler := NewMessageHandler(messageStore, eventPub)
	handoffHandler := NewHandoffHandler(handoffStore, eventPub)
	presenceHandler := NewPresenceHandler(presenceStore, presenceManager, eventPub)

	r.Group(func(r chi.Router) {
		r.Use(osm.JWTMiddleware(osm.NewAuthValidator("", ""))) // placeholder — main.go overrides
		r.Use(osm.TenantMiddleware())

		// Channels
		r.Post("/channels", channelHandler.CreateChannel)
		r.Get("/channels", channelHandler.ListChannels)
		r.Get("/channels/{id}", channelHandler.GetChannel)
		r.Patch("/channels/{id}", channelHandler.UpdateChannel)
		r.Delete("/channels/{id}", channelHandler.DeleteChannel)
		r.Post("/channels/{id}/join", channelHandler.JoinChannel)
		r.Delete("/channels/{id}/leave", channelHandler.LeaveChannel)

		// Messages
		r.Post("/messages", messageHandler.SendMessage)
		r.Get("/channels/{channel_id}/messages", messageHandler.ListMessages)

		// Handoffs
		r.Get("/handoffs", handoffHandler.ListHandoffs)
		r.Post("/handoffs", handoffHandler.CreateHandoff)
		r.Post("/handoffs/{id}/accept", handoffHandler.AcceptHandoff)
		r.Post("/handoffs/{id}/complete", handoffHandler.CompleteHandoff)
		r.Post("/handoffs/{id}/reject", handoffHandler.RejectHandoff)

		// Presence
		r.Get("/presence", presenceHandler.ListPresence)
		r.Get("/presence/{agent_id}", presenceHandler.GetPresence)
		r.Post("/presence/heartbeat", presenceHandler.Heartbeat)
	})

	return r
}