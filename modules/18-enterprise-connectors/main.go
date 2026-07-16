package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/enterprise-connectors/internal/clients"
	"github.com/operan/enterprise-connectors/internal/config"
	"github.com/operan/enterprise-connectors/internal/connectors"
	"github.com/operan/enterprise-connectors/internal/events"
	"github.com/operan/enterprise-connectors/internal/handler"
	"github.com/operan/enterprise-connectors/internal/middleware"
	"github.com/operan/enterprise-connectors/internal/store"
	"github.com/operan/enterprise-connectors/internal/sync"
	mig "github.com/operan/enterprise-connectors/migrations"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation: %v", err)
	}

	// Database connection
	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("ping database: %v", err)
	}
	defer pool.Close()

	if err := mig.RunMigrations(pool); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("Database migrations applied")

	// Stores
	connectorStore := store.NewConnectorStore(pool)
	syncStore := store.NewSyncStore(pool)

	// M04 client
	m04Client := clients.NewM04Client(cfg.M04BaseURL)

	// Event publisher
	eventPub := events.NewPublisher(cfg.EventBrokerURL)

	// Connector registry
	registry := connectors.NewRegistry()
	registry.Register(&connectors.SMTPConnector{})
	registry.Register(&connectors.SalesforceConnector{})
	registry.Register(&connectors.HubSpotConnector{})
	registry.Register(&connectors.M365Connector{})
	registry.Register(&connectors.SAPConnector{})
	registry.Register(&connectors.RESTConnector{})

	// Sync engine
	syncEngine := sync.NewEngine(connectorStore, syncStore, m04Client, eventPub, registry)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.SetupCORS())
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.TraceID)

	// Health — unauthenticated
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes
	authValidator := middleware.NewAuthValidator(cfg.JWTSecret, cfg.Issuer)
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator))
		r.Use(middleware.TenantMiddleware())

		connHandler := handler.NewConnectorHandler(connectorStore)
		syncHandler := handler.NewSyncHandler(syncEngine, connectorStore, syncStore)
		toolsHandler := handler.NewToolsHandler(registry)

		connHandler.Routes(r)
		syncHandler.Routes(r)
		toolsHandler.Routes(r)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigch
		log.Printf("Received signal %v, shutting down...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown: %v", err)
		}
	}()

	log.Printf("Enterprise Connector Fabric starting on :%d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
	log.Println("Server stopped")
}