// Module 05 — Department Template Engine
//
// This service provides the template factory for Operan: standardized
// department blueprints (agents, workflows, memory, governance, KPIs,
// integrations) that can be deployed and cloned across tenants.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/config"
	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/handlers"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func main() {
	cfg := config.ParseConfig()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// ─── Stores ───────────────────────────────────────────────────────────
	templateStore := store.NewTemplateStore()
	customTemplateStore := store.NewCustomTemplateStore()
	deploymentStore := store.NewDeploymentStore()
	versionStore := store.NewVersionStore()

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

	// ─── Handlers ─────────────────────────────────────────────────────────
	h := handlers.NewTemplateHandlers(
		templateStore,
		customTemplateStore,
		deploymentStore,
		versionStore,
		publisher,
		cfg.MaxPageSize,
	)

	// ─── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, h)

	// ─── Middleware chain ─────────────────────────────────────────────────
	// Logger → RequestID → TraceID → JWT Auth → Tenant Context → Rate Limit → Handlers
	var chain http.Handler = mux
	chain = middleware.Logger(chain)
	chain = middleware.RequestID(chain)
	chain = middleware.TraceID(chain)
	chain = middleware.JWTAuth(cfg.JWTSecret, chain)
	chain = middleware.TenantContext(chain)
	chain = middleware.RateLimit(100, 1*time.Minute)(chain)

	// ─── Health check ─────────────────────────────────────────────────────
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"department-template-engine","version":"1.0.0"}`))
	})

	log.Printf("Module 05 — Department Template Engine starting on :%d", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), chain); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
