// Module 09 — Human Supervision
//
// The human-in-the-loop layer for Operan: approval gates that pause agent
// workflows for human decisions, escalations for incidents, interventions
// (pause/stop/restrict agents), a merged review queue, and a risk dashboard.
// Module 03's human gates and Module 10's policy engine route through here.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/09-human-supervision/internal/config"
	"github.com/operan/modules/09-human-supervision/internal/events"
	"github.com/operan/modules/09-human-supervision/internal/handlers"
	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/store"
)

func main() {
	cfg := config.ParseConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// ─── Stores ───────────────────────────────────────────────────────────
	approvalStore := store.NewApprovalStore()
	escalationStore := store.NewEscalationStore()
	interventionStore := store.NewInterventionStore()
	hitlStore := store.NewHitlStore()

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
	h := handlers.NewSupervisionHandlers(approvalStore, escalationStore, interventionStore, hitlStore, publisher, cfg.MaxPageSize)
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
		w.Write([]byte(`{"status":"healthy","module":"human-supervision","version":"1.0.0"}`))
	})
	root.Handle("/", api)

	log.Printf("Module 09 — Human Supervision starting on :%d", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), root); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
