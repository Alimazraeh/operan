// Module 05 — Department Lifecycle Engine
//
// The department factory for Operan: standardized department blueprints
// (agents, workflows, memory, governance, KPIs, org charts, service
// portfolios, value streams) that deploy into living Department instances —
// agents registered in Module 04, memory provisioned in Module 07 — with
// snapshot persistence so departments survive restarts.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/config"
	"github.com/operan/modules/05-department-template-engine/internal/deploy"
	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/handlers"
	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/persist"
	"github.com/operan/modules/05-department-template-engine/internal/seed"
	"github.com/operan/modules/05-department-template-engine/internal/store"
	"github.com/operan/modules/05-department-template-engine/internal/workloop"
)

func main() {
	cfg := config.ParseConfig()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// ─── Built-in template catalog (embedded; fail fast on drift) ─────────
	if err := seed.LoadCatalog(templatesFS, "templates"); err != nil {
		log.Fatalf("Fatal: template catalog: %v", err)
	}
	log.Printf("template catalog loaded: %d built-in templates", len(seed.Catalog()))

	// ─── Stores ───────────────────────────────────────────────────────────
	templateStore := store.NewTemplateStore()
	customTemplateStore := store.NewCustomTemplateStore()
	deploymentStore := store.NewDeploymentStore()
	versionStore := store.NewVersionStore()
	departmentStore := store.NewDepartmentStore()
	requestStore := store.NewRequestStore()

	// ─── Persistence (snapshot restore + periodic save) ───────────────────
	persistFiles := []persist.File{
		{Name: "templates.json", Store: templateStore},
		{Name: "custom_templates.json", Store: customTemplateStore},
		{Name: "deployments.json", Store: deploymentStore},
		{Name: "versions.json", Store: versionStore},
		{Name: "departments.json", Store: departmentStore},
		{Name: "requests.json", Store: requestStore},
	}
	persist.Load(cfg.DataDir, persistFiles)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go persist.Run(ctx, cfg.DataDir, time.Duration(cfg.SnapshotInterval)*time.Second, persistFiles)

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
	h.DepartmentStore = departmentStore // shared with the persistence loop
	h.RequestStore = requestStore

	// ─── The department work loop: request dispatch + run poller ─────────
	orchClient := &clients.OrchestrationClient{BaseURL: cfg.OrchestrationURL}
	loop := workloop.New(requestStore, departmentStore, templateStore, orchClient, publisher)
	h.Dispatcher = loop
	go loop.Run(ctx)
	h.Orchestrator = &deploy.Orchestrator{
		Deployments:   deploymentStore,
		Departments:   departmentStore,
		Publisher:     publisher,
		Registry:      &clients.RegistryClient{BaseURL: cfg.RegistryURL},
		Memory:        &clients.MemoryClient{BaseURL: cfg.MemoryURL},
		Orchestration: &clients.OrchestrationClient{BaseURL: cfg.OrchestrationURL},
	}

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

	// Liveness probe bypasses the auth/tenant middleware chain.
	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"department-template-engine","version":"1.0.0"}`))
	})
	root.Handle("/", chain)

	// Graceful shutdown: the signal context (also driving the persist loop)
	// stops the HTTP server so SIGTERM actually terminates the process after
	// a final snapshot — required for clean pod rollovers.
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: root}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("Module 05 — Department Template Engine starting on :%d", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
	// Final snapshot before exit (persist.Run also saves on ctx cancel).
	persist.SaveAll(cfg.DataDir, persistFiles)
	log.Printf("Module 05 stopped cleanly")
}
