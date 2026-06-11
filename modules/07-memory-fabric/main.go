// Module 07 — Memory Fabric
//
// This service is the memory layer for Operan agents: vector memory
// ingestion, semantic search, agent memory state, retention policies, and
// garbage collection. Agents (via Module 03) store and retrieve semantic
// memories here; Module 06 feeds it ingested knowledge.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/operan/modules/07-memory-fabric/internal/config"
	"github.com/operan/modules/07-memory-fabric/internal/embeddings"
	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/handlers"
	"github.com/operan/modules/07-memory-fabric/internal/middleware"
	"github.com/operan/modules/07-memory-fabric/internal/persist"
	"github.com/operan/modules/07-memory-fabric/internal/store"
)

func main() {
	cfg := config.ParseConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// ─── Stores ───────────────────────────────────────────────────────────
	vectorStore := store.NewVectorStore()
	policyStore := store.NewPolicyStore()
	operationStore := store.NewOperationStore()

	// ─── Persistence (file snapshots on a mounted volume) ─────────────────
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if cfg.DataDir != "" {
		snapshots := []persist.File{
			{Name: "vectors.json", Store: vectorStore},
			{Name: "policies.json", Store: policyStore},
			{Name: "operations.json", Store: operationStore},
		}
		persist.Load(cfg.DataDir, snapshots)
		go persist.Run(shutdownCtx, cfg.DataDir, 10*time.Second, snapshots)
		log.Printf("persistence enabled: %s", cfg.DataDir)
	}

	// ─── Events ───────────────────────────────────────────────────────────
	publisher := events.NewPublisher()
	if cfg.EventBrokerURL != "" {
		broker, err := events.NewKafkaBroker(cfg.EventBrokerURL)
		if err != nil {
			log.Printf("[WARN] event broker unavailable (%s): %v — falling back to log-only", cfg.EventBrokerURL, err)
		} else {
			publisher = events.NewPublisherWithBroker(broker)
			log.Printf("event publisher configured for kafka broker %s", cfg.EventBrokerURL)
		}
	}
	defer publisher.Close()

	// ─── API routes (auth-protected) ──────────────────────────────────────
	h := handlers.NewMemoryHandlers(vectorStore, policyStore, operationStore, publisher, cfg.MaxPageSize, cfg.GCBatchSize)
	if cfg.EmbeddingsURL != "" {
		h.Embedder = embeddings.New(cfg.EmbeddingsURL, cfg.EmbeddingsAPIKey, cfg.EmbeddingsModel)
		log.Printf("embeddings gateway configured: %s (model %s)", cfg.EmbeddingsURL, cfg.EmbeddingsModel)
	} else {
		log.Printf("no embeddings gateway configured; search uses token-overlap fallback")
	}
	apiMux := http.NewServeMux()
	handlers.RegisterRoutes(apiMux, h)

	// Middleware chain (applied to API only):
	// Logger → RequestID → TraceID → JWT Auth → Tenant Context → Rate Limit
	var api http.Handler = apiMux
	api = middleware.RateLimit(100, time.Minute)(api)
	api = middleware.TenantContext(api)
	api = middleware.JWTAuth(cfg.JWTSecret, api)
	api = middleware.TraceID(api)
	api = middleware.RequestID(api)
	api = middleware.Logger(api)

	// ─── Root mux: health bypasses auth, everything else hits the API ─────
	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"memory-fabric","version":"1.0.0"}`))
	})
	root.Handle("/", api)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: root}
	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("Module 07 — Memory Fabric starting on :%d", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
	// Give the persistence loop a beat to take its final snapshot.
	time.Sleep(200 * time.Millisecond)
	log.Printf("Module 07 stopped")
}
