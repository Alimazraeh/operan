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
	"github.com/operan/modules/04-agent-registry/internal/database"
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

	// ─── Stores ─────────────────────────────────────────────────────────────
	agentStore := store.NewAgentStore()
	versionStore := store.NewVersionStore()
	capabilityStore := store.NewCapabilityStore()
	dependencyStore := store.NewDependencyStore()

	// Durability. Reads stay in memory; writes go through to PostgreSQL and the
	// process reloads at boot. Without this the registry loses every agent on
	// restart — it had done so 24 times, leaving the portal reading "0 Agents"
	// and every deployed org-chart position pointing at an agent_id that no
	// longer resolved.
	//
	// Capability and dependency records are deliberately still memory-only:
	// nothing writes them in this deployment, so persisting them now would be
	// scaffolding for a path no caller takes. That is a known limit, not an
	// oversight — see README.
	if cfg.DatabaseDSN == "" {
		log.Printf("[REGISTRY] no AGENT_REGISTRY_DATABASE_DSN — running in memory; every agent is lost on restart")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		pool, err := database.Connect(ctx, cfg.DatabaseDSN, cfg.DBMaxOpen)
		if err != nil {
			// Starting without durability would look healthy and quietly
			// reintroduce the bug, so refuse: a registry configured to persist
			// and unable to must not answer as though it can.
			cancel()
			log.Fatalf("[REGISTRY] database configured but unreachable: %v", err)
		}
		if err := database.Migrate(ctx, pool); err != nil {
			cancel()
			log.Fatalf("[REGISTRY] migrations failed: %v", err)
		}
		db := database.NewAgentStore(pool)
		agentStore.Persist(db)
		versionStore.Persist(db)

		nAgents, err := agentStore.HydrateAgents(ctx, db)
		if err != nil {
			cancel()
			log.Fatalf("[REGISTRY] could not load agents: %v", err)
		}
		nVersions, err := versionStore.HydrateVersions(ctx, db)
		if err != nil {
			cancel()
			log.Fatalf("[REGISTRY] could not load agent versions: %v", err)
		}
		cancel()
		defer pool.Close()
		log.Printf("[REGISTRY] durable: loaded %d agent(s) and %d version(s) from the database", nAgents, nVersions)
	}

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

	// Liveness probe bypasses the auth/tenant middleware chain.
	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"agent-registry","version":"1.0.0"}`))
	})
	root.Handle("/", http.HandlerFunc(handler))

	// ─── Start server ───────────────────────────────────────────────────────
	addr := cfg.ListenAddr
	srv := &http.Server{
		Addr:         addr,
		Handler:      root,
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
