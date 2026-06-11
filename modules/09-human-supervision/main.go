// Module 09 — Human Supervision
//
// The human-in-the-loop layer for Operan: approval gates that pause agent
// workflows for human decisions, escalations for incidents, interventions
// (pause/stop/restrict agents), a merged review queue, and a risk dashboard.
// Module 03's human gates and Module 10's policy engine route through here.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/operan/modules/09-human-supervision/internal/config"
	"github.com/operan/modules/09-human-supervision/internal/events"
	"github.com/operan/modules/09-human-supervision/internal/handlers"
	"github.com/operan/modules/09-human-supervision/internal/middleware"
	"github.com/operan/modules/09-human-supervision/internal/persist"
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

	// ─── Persistence (file snapshots on a mounted volume) ─────────────────
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if cfg.DataDir != "" {
		snapshots := []persist.File{
			{Name: "approvals.json", Store: approvalStore},
			{Name: "escalations.json", Store: escalationStore},
			{Name: "interventions.json", Store: interventionStore},
			{Name: "hitl.json", Store: hitlStore},
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

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: root}
	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("Module 09 — Human Supervision starting on :%d", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond) // final snapshot
	log.Printf("Module 09 stopped")
}
