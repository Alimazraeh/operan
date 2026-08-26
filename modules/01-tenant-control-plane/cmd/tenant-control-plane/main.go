package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/handler"
	"github.com/operan/modules/01-tenant-control-plane/internal/config"
	"github.com/operan/modules/01-tenant-control-plane/internal/database"
	"github.com/operan/modules/01-tenant-control-plane/internal/events"
	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

func main() {
	// Set default version before config parsing.
	os.Setenv("MODULE_VERSION", "1.0.0")

	cfg := config.ParseConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config error: %v", err)
	}

	if cfg.LogLevel == "debug" {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		log.Printf("starting tenant-control-plane module v%s", cfg.Version)
		log.Printf("listen address: %s", cfg.ListenAddr)
		log.Printf("otel endpoint: %s", cfg.OTLPEndpoint)
	}

	tenantStore := store.NewTenantStore()
	secretStore := store.NewSecretStore()
	subscriptionStore := store.NewSubscriptionStore()
	billingStore := store.NewBillingStore()
	paymentMethodStore := store.NewPaymentMethodStore()
	agentStore := store.NewAgentStore()
	resourceStore := store.NewResourceStore()
	namespaceStore := store.NewNamespaceStore()
	deploymentStore := store.NewDeploymentStore()
	policyStore := store.NewPolicyStore()
	environmentStore := store.NewEnvironmentStore()

	// Durability. Reads stay in memory; writes go through to PostgreSQL and the
	// process reloads at boot. Without this the control plane loses every
	// tenant, subscription, secret, and deployment on restart.
	if cfg.DatabaseURL == "" {
		log.Printf("[TCTL] no DATABASE_URL — running in memory; every tenant, subscription, secret and deployment is lost on restart")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		pool, err := database.Connect(ctx, cfg.DatabaseURL, 0)
		if err != nil {
			// Starting without durability would look healthy and quietly
			// reintroduce the bug, so refuse: a control plane configured to
			// persist and unable to must not answer as though it can.
			cancel()
			log.Fatalf("[TCTL] database configured but unreachable: %v", err)
		}
		if err := database.Migrate(ctx, pool); err != nil {
			cancel()
			log.Fatalf("[TCTL] migrations failed: %v", err)
		}
		db := database.NewControlPlaneStore(pool)
		tenantStore.Persist(db)
		secretStore.Persist(db)
		subscriptionStore.Persist(db)
		billingStore.Persist(db)
		paymentMethodStore.Persist(db)
		agentStore.Persist(db)
		resourceStore.Persist(db)
		namespaceStore.Persist(db)
		deploymentStore.Persist(db)
		policyStore.Persist(db)
		environmentStore.Persist(db)

		loaded := make(map[string]int)
		hydrate := func(name string, f func(ctx context.Context, db *database.ControlPlaneStore) (int, error)) {
			n, err := f(ctx, db)
			if err != nil {
				cancel()
				log.Fatalf("[TCTL] could not load %s: %v", name, err)
			}
			loaded[name] = n
		}
		hydrate("tenants", tenantStore.HydrateTenants)
		hydrate("secrets", secretStore.HydrateSecrets)
		hydrate("subscriptions", subscriptionStore.HydrateSubscriptions)
		hydrate("invoices", billingStore.HydrateInvoices)
		hydrate("payment methods", paymentMethodStore.HydratePaymentMethods)
		hydrate("agents", agentStore.HydrateAgents)
		hydrate("resources", resourceStore.HydrateResources)
		hydrate("namespaces", namespaceStore.HydrateNamespaces)
		hydrate("deployments", deploymentStore.HydrateDeployments)
		hydrate("policies", policyStore.HydratePolicies)
		hydrate("environments", environmentStore.HydrateEnvironments)
		cancel()
		defer pool.Close()

		order := []string{"tenants", "secrets", "subscriptions", "invoices",
			"payment methods", "agents", "resources", "namespaces", "deployments",
			"policies", "environments"}
		var summary []string
		for _, name := range order {
			summary = append(summary, fmt.Sprintf("%d %s", loaded[name], name))
		}
		log.Printf("[TCTL] durable: loaded %s from the database", strings.Join(summary, ", "))
	}

	mux := http.NewServeMux()

	h := middleware.NewHandler(tenantStore, secretStore, subscriptionStore, billingStore,
		paymentMethodStore, agentStore, resourceStore, namespaceStore,
		deploymentStore, policyStore, environmentStore)

	// Wire the event publisher to Kafka when configured; log-only otherwise.
	if cfg.EventBusProto == "kafka" {
		broker, err := events.NewKafkaBroker(cfg.EventBusHost + ":" + cfg.EventBusPort)
		if err != nil {
			log.Printf("[WARN] event broker unavailable (%s:%s): %v — falling back to log-only", cfg.EventBusHost, cfg.EventBusPort, err)
		} else {
			h.EventPublisher = events.NewPublisherWithBroker(broker)
			defer h.EventPublisher.Close()
			log.Printf("event publisher configured for kafka broker %s:%s", cfg.EventBusHost, cfg.EventBusPort)
		}
	} else {
		log.Printf("event publisher in log-only mode (EVENT_BUS_PROTO=%s)", cfg.EventBusProto)
	}

	handler.RegisterRoutes(h, mux)

	// Liveness probe bypasses the auth/tenant middleware chain.
	chain := middleware.JWTValidator(cfg.JWTSecret, cfg.Issuer)(middleware.TenantContext(middleware.TraceID(middleware.RequestID(mux))))
	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","module":"tenant-control-plane","version":"1.0.0"}`))
	})
	root.Handle("/", chain)

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      root,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("shutdown signal received, draining connections...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("tenant-control-plane listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("tenant-control-plane stopped")
}
