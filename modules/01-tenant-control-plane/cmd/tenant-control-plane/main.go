package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/handler"
	"github.com/operan/modules/01-tenant-control-plane/internal/config"
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

	mux := http.NewServeMux()

	h := middleware.NewHandler(tenantStore, secretStore, subscriptionStore, billingStore)

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

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      middleware.JWTValidator(cfg.JWTSecret, cfg.Issuer)(middleware.TenantContext(middleware.TraceID(middleware.RequestID(mux)))),
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
