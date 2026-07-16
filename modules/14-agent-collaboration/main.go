package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/agent-collaboration/internal/config"
	"github.com/operan/agent-collaboration/internal/events"
	"github.com/operan/agent-collaboration/internal/handler"
	"github.com/operan/agent-collaboration/internal/middleware"
	osm "github.com/operan/agent-collaboration/internal/middleware"
	"github.com/operan/agent-collaboration/internal/presence"
	"github.com/operan/agent-collaboration/internal/store"
	mig "github.com/operan/agent-collaboration/migrations"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation: %v", err)
	}

	// Database connection
	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("ping database: %v", err)
	}
	defer pool.Close()

	if err := mig.RunMigrations(pool); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("Database migrations applied")

	// Stores
	channelStore := store.NewChannelStore(pool)
	messageStore := store.NewMessageStore(pool)
	handoffStore := store.NewHandoffStore(pool)
	presenceStore := store.NewPresenceStore(pool)

	// Presence manager
	presenceManager := presence.NewManager(presenceStore)
	presenceManager.Start()
	defer presenceManager.Stop()

	// Event publisher
	eventPub := events.NewPublisher(cfg.EventBrokerURL)

	// Router
	r := chi.NewRouter()
	r.Use(osm.SetupCORS())
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.TraceID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authValidator := middleware.NewAuthValidator(cfg.JWTSecret, cfg.Issuer)
	r.Group(func(r chi.Router) {
		r.Use(osm.JWTMiddleware(authValidator))
		r.Use(osm.TenantMiddleware())

		channelHandler := handler.NewChannelHandler(channelStore, eventPub)
		messageHandler := handler.NewMessageHandler(messageStore, eventPub)
		handoffHandler := handler.NewHandoffHandler(handoffStore, eventPub)
		presenceHandler := handler.NewPresenceHandler(presenceStore, presenceManager, eventPub)

		r.Post("/channels", channelHandler.CreateChannel)
		r.Get("/channels", channelHandler.ListChannels)
		r.Get("/channels/{id}", channelHandler.GetChannel)
		r.Patch("/channels/{id}", channelHandler.UpdateChannel)
		r.Delete("/channels/{id}", channelHandler.DeleteChannel)
		r.Post("/channels/{id}/join", channelHandler.JoinChannel)
		r.Delete("/channels/{id}/leave", channelHandler.LeaveChannel)

		r.Post("/messages", messageHandler.SendMessage)
		r.Get("/channels/{channel_id}/messages", messageHandler.ListMessages)

		r.Get("/handoffs", handoffHandler.ListHandoffs)
		r.Post("/handoffs", handoffHandler.CreateHandoff)
		r.Post("/handoffs/{id}/accept", handoffHandler.AcceptHandoff)
		r.Post("/handoffs/{id}/complete", handoffHandler.CompleteHandoff)
		r.Post("/handoffs/{id}/reject", handoffHandler.RejectHandoff)

		r.Get("/presence", presenceHandler.ListPresence)
		r.Get("/presence/{agent_id}", presenceHandler.GetPresence)
		r.Post("/presence/heartbeat", presenceHandler.Heartbeat)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigch
		log.Printf("Received signal %v, shutting down...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown: %v", err)
		}
	}()

	log.Printf("Agent Collaboration starting on :%d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
	log.Println("Server stopped")
}