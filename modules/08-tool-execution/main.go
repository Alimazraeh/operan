// Module 08 — Tool Execution
//
// This service is the secure execution layer for Operan: it registers tools,
// versions their schemas, executes them on behalf of agents, and tracks
// execution records and cost. The agent orchestrator (Module 03) calls this
// service to let agents take actions.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/operan/modules/08-tool-execution/internal/config"
	"github.com/operan/modules/08-tool-execution/internal/database"
	"github.com/operan/modules/08-tool-execution/internal/events"
	"github.com/operan/modules/08-tool-execution/internal/funnel"
	"github.com/operan/modules/08-tool-execution/internal/handlers"
	"github.com/operan/modules/08-tool-execution/internal/middleware"
	"github.com/operan/modules/08-tool-execution/internal/policyclient"
	"github.com/operan/modules/08-tool-execution/internal/schema"
	"github.com/operan/modules/08-tool-execution/internal/store"
	"github.com/operan/modules/08-tool-execution/internal/vocab"
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

	// Capability layer: the vocabulary, a tenant's providers and bindings,
	// and the immutable invocation trail.
	capabilityStore := store.NewCapabilityStore()
	providerStore := store.NewProviderStore()
	bindingStore := store.NewBindingStore()
	invocationStore := store.NewInvocationStore()
	nCaps := vocab.SeedCapabilities(capabilityStore)

	// Durability. Bindings are customer configuration and invocations are the
	// audit trail — neither may vanish on a restart. Same discipline as the
	// agent registry: configured-but-unreachable refuses to start, because a
	// capability service that looks governed while silently losing its audit
	// trail is worse than one that is down.
	if cfg.DBURL == "" {
		log.Printf("[CAPABILITY] no MODULE08_DB_URL — running in memory; bindings and the invocation trail are lost on restart")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		pool, err := database.Connect(ctx, cfg.DBURL, 10)
		if err != nil {
			cancel()
			log.Fatalf("[CAPABILITY] database configured but unreachable: %v", err)
		}
		if err := database.Migrate(ctx, pool); err != nil {
			cancel()
			log.Fatalf("[CAPABILITY] migrations failed: %v", err)
		}
		db := database.NewStore(pool)
		providerStore.Persist(db)
		bindingStore.Persist(db)
		invocationStore.Persist(db)
		np, nb, ni, err := store.Hydrate(ctx, db, providerStore, bindingStore, invocationStore)
		cancel()
		if err != nil {
			log.Fatalf("[CAPABILITY] could not rehydrate: %v", err)
		}
		defer pool.Close()
		log.Printf("[CAPABILITY] durable: %d capabilities in vocabulary; loaded %d provider(s), %d binding(s), %d invocation(s)", nCaps, np, nb, ni)
	}

	// Demo bootstrap: named tenants get the simulated provider bound to every
	// capability. Deterministic ids make this idempotent across restarts.
	if cfg.SeedSimulatedTenants != "" {
		vocab.SeedSimulatedTenants(cfg.SeedSimulatedTenants, providerStore, bindingStore, capabilityStore)
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
	h := handlers.NewToolHandlers(toolStore, versionStore, executionStore, publisher, cfg.MaxPageSize)
	capHandlers := &handlers.CapabilityHandlers{
		Funnel: &funnel.Funnel{
			Capabilities: capabilityStore,
			Providers:    providerStore,
			Bindings:     bindingStore,
			Invocations:  invocationStore,
			Validator:    schema.NewValidator(),
			Policy:       policyclient.New(cfg.PolicyURL),
		},
		MaxPageSize: cfg.MaxPageSize,
	}
	apiMux := http.NewServeMux()
	handlers.RegisterRoutes(apiMux, h)
	handlers.RegisterCapabilityRoutes(apiMux, capHandlers)

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
