// Module 08 — Tool Execution
//
// This service is the secure execution layer for Operan: it registers tools,
// versions their schemas, executes them on behalf of agents, and tracks
// execution records and cost. The agent orchestrator (Module 03) calls this
// service to let agents take actions.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/08-tool-execution/internal/config"
	"github.com/operan/modules/08-tool-execution/internal/events"
	"github.com/operan/modules/08-tool-execution/internal/handlers"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

func main() {
	cfg := config.ParseConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// ─── Stores ───────────────────────────────────────────────────────────
	toolStore := store.NewToolStore()
	versionStore := store.NewVersionStore()
	executionStore := store.NewExecutionStore()

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
	h := handlers.NewToolHandlers(toolStore, versionStore, executionStore, publisher, cfg.MaxPageSize)
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
		w.Write([]byte(`{"status":"healthy","module":"tool-execution","version":"1.0.0"}`))
	})
	root.Handle("/", api)

	log.Printf("Module 08 — Tool Execution starting on :%d", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), root); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
