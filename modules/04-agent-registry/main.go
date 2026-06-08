// Package main implements the Agent Registry service (Module 04).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/config"
	"github.com/operan/modules/04-agent-registry/internal/events"
	"github.com/operan/modules/04-agent-registry/internal/handlers"
	"github.com/operan/modules/04-agent-registry/internal/middleware"
	"github.com/operan/modules/04-agent-registry/internal/store"
)

func main() {
	// ─── Parse & validate config ────────────────────────────────────────────
	cfg, err := config.ParseConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// ─── Initialize in-memory stores ────────────────────────────────────────
	agentStore := store.NewAgentStore()
	versionStore := store.NewVersionStore()
	capabilityStore := store.NewCapabilityStore()
	dependencyStore := store.NewDependencyStore()

	// ─── Create handlers ────────────────────────────────────────────────────
	h := handlers.NewAgentRegistryHandlers(
		agentStore, versionStore, capabilityStore, dependencyStore, cfg,
	)
	// Wire up Kafka broker from config
	h.EventPublisher = events.NewPublisherWithConfig(cfg)

	// ─── Build route tree ──────────────────────────────────────────────────
	router := handlers.RegisterRoutes(h)

	// ─── Apply middleware stack ─────────────────────────────────────────────
	// Chain order: JWTAuth -> ExtractTenant -> TraceID -> RequestID -> Logger
	handler := middleware.Chain(
		func(w http.ResponseWriter, r *http.Request) {
			router.ServeHTTP(w, r)
		},
		middleware.ChainJWTAuth(cfg.JWTSecret),
		middleware.ExtractTenant,
		middleware.TraceID,
		middleware.RequestID,
		middleware.Logger,
	)

	// ─── Start server ───────────────────────────────────────────────────────
	addr := cfg.ListenAddr
	srv := &http.Server{
		Addr:         addr,
		Handler:      http.HandlerFunc(handler),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		// Close event publisher before shutting down server
		h.EventPublisher.Close()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("agent-registry listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("server stopped")
}
