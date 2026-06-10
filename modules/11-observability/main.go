// Module 11 — Observability
//
// The platform's eyes: ingests events from every Operan module into trace
// spans, metrics, alerts, and component health, and serves them through the
// observability API. The Kafka consumer turns the platform event mesh into
// queryable telemetry.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/11-observability/internal/config"
	"github.com/operan/modules/11-observability/internal/consumer"
	"github.com/operan/modules/11-observability/internal/events"
	"github.com/operan/modules/11-observability/internal/handlers"
	"github.com/operan/modules/11-observability/internal/middleware"
	"github.com/operan/modules/11-observability/internal/store"
)

func main() {
	cfg := config.ParseConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// ─── Stores ───────────────────────────────────────────────────────────
	metricStore := store.NewMetricStore()
	spanStore := store.NewSpanStore()
	alertStore := store.NewAlertStore()
	healthStore := store.NewHealthStore()

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

	// ─── Platform event consumer ──────────────────────────────────────────
	ingestor := consumer.NewIngestor(spanStore, metricStore, alertStore, healthStore, publisher)
	if cfg.EventBrokerURL != "" {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ingestor.Run(ctx, cfg.EventBrokerURL, cfg.ConsumerGroup, cfg.ConsumeTopics)
	} else {
		log.Printf("[CONSUMER] no event broker configured; platform event ingestion disabled")
	}

	// ─── API routes (auth-protected) ──────────────────────────────────────
	h := handlers.NewObservabilityHandlers(metricStore, spanStore, alertStore, healthStore, publisher, cfg.MaxPageSize)
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

	// ─── Root mux: /healthz liveness bypasses auth; the contract's
	// tenant-scoped GET /health is part of the authenticated API. ──────────
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"observability","version":"1.0.0"}`))
	})
	root.Handle("/", api)

	log.Printf("Module 11 — Observability starting on :%d", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), root); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
