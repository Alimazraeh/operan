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
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/model-abstraction/internal/config"
	"github.com/operan/model-abstraction/internal/handler"
	authMW "github.com/operan/model-abstraction/internal/middleware"
	"github.com/operan/model-abstraction/internal/store"

	"github.com/operan/model-abstraction/internal/events"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Database connection.
	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("cannot create DB pool: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("cannot reach database: %v", err)
	}
	defer pool.Close()

	// Event broker.
	broker := events.NewBroker(events.NoOpBroker{})

	// Logger.
	logger := log.New(os.Stdout, "[model-abstraction] ", log.LstdFlags)

	// Stores.
	providersStore := store.NewProvidersStore(pool)
	registryStore := store.NewRegistryStore(pool)
	callsStore := store.NewCallsStore(pool)

	// Auth validator.
	authValidator := authMW.NewAuthValidator(cfg)

	// Handlers.
	completionHandler := handler.NewCompletionHandler(
		registryStore, providersStore, cfg, broker, callsStore, logger)
	embeddingsHandler := handler.NewEmbeddingsHandler(
		registryStore, providersStore, cfg, callsStore, logger)
	providersHandler := handler.NewProvidersHandler(providersStore, cfg)
	modelsHandler := handler.NewModelsHandler(registryStore, providersStore)

	// Middleware chain.
	authChain := authMW.JWTMiddleware(authValidator)
	tenantChain := authMW.TenantMiddleware()
	rbacAdmin := authMW.RBACMiddleware("model_admin")

	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60000))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-ID", "X-Workflow-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check — unauthenticated.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(authChain, tenantChain)

		// Completions.
		r.Post("/v1/models/completions", completionHandler.POST)

		// Embeddings.
		r.Post("/v1/models/embeddings", embeddingsHandler.POST)

		// Providers CRUD (admin-only for write).
		r.Get("/v1/model-providers", providersHandler.HandleGET)
		r.Post("/v1/model-providers", rbacAdmin(http.HandlerFunc(providersHandler.HandlePOST)).(http.HandlerFunc))
		r.Patch("/v1/model-providers/{id}", rbacAdmin(http.HandlerFunc(providersHandler.HandlePATCH)).(http.HandlerFunc))
		r.Delete("/v1/model-providers/{id}", rbacAdmin(http.HandlerFunc(providersHandler.HandleDELETE)).(http.HandlerFunc))

		// Models CRUD.
		r.Get("/v1/model-registry", modelsHandler.HandleGET)
		r.Post("/v1/model-registry", rbacAdmin(http.HandlerFunc(modelsHandler.HandlePOST)).(http.HandlerFunc))
		r.Patch("/v1/model-registry/{id}", rbacAdmin(http.HandlerFunc(modelsHandler.HandlePATCH)).(http.HandlerFunc))
	})

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:     r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		logger.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	logger.Printf("starting model abstraction server on port %d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server: %v", err)
	}
	logger.Println("server stopped")
}