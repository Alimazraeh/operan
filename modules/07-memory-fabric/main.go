// Module 07 — Memory Fabric
//
// This service is the memory layer for Operan agents: vector memory
// ingestion, semantic search, agent memory state, retention policies, and
// garbage collection. Agents (via Module 03) store and retrieve semantic
// memories here; Module 06 feeds it ingested knowledge.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/07-memory-fabric/internal/config"
	"github.com/operan/modules/07-memory-fabric/internal/events"
	"github.com/operan/modules/07-memory-fabric/internal/handlers"
	"github.com/operan/modules/07-memory-fabric/internal/middleware"
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

	log.Printf("Module 07 — Memory Fabric starting on :%d", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), root); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
