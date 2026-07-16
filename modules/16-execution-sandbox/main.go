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
	"github.com/operan/execution-sandbox/internal/config"
	"github.com/operan/execution-sandbox/internal/events"
	"github.com/operan/execution-sandbox/internal/handler"
	"github.com/operan/execution-sandbox/internal/middleware"
	"github.com/operan/execution-sandbox/internal/policies"
	"github.com/operan/execution-sandbox/internal/sandbox"
	"github.com/operan/execution-sandbox/internal/store"
	mig "github.com/operan/execution-sandbox/migrations"
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
	profileStore := store.NewProfileStore(pool)
	instanceStore := store.NewInstanceStore(pool)

	// Sandbox executor
	executor, err := sandbox.NewExecutor("/tmp/operan-sandbox")
	if err != nil {
		log.Fatalf("create executor: %v", err)
	}

	// Policy client
	policyClient := policies.NewPolicyClient(cfg.M10BaseURL)

	// Event publisher
	eventPub := events.NewPublisher(cfg.EventBrokerURL)

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

		profileHandler := handler.NewProfileHandler(profileStore)
		instanceHandler := handler.NewInstanceHandler(instanceStore)
		executeHandler := handler.NewExecuteHandler(executor, profileStore, instanceStore, policyClient, eventPub)

		r.Get("/sandboxes/instances", instanceHandler.ListInstances)
		r.Get("/sandboxes/instances/{id}", instanceHandler.GetInstance)
		r.Post("/sandboxes/instances/{id}/cancel", instanceHandler.CancelInstance)
		r.Get("/sandbox-profiles", profileHandler.ListProfiles)
		r.Post("/sandbox-profiles", profileHandler.CreateProfile)
		r.Get("/sandbox-profiles/{id}", profileHandler.GetProfile)
		r.Patch("/sandbox-profiles/{id}", profileHandler.UpdateProfile)
		r.Delete("/sandbox-profiles/{id}", profileHandler.DeleteProfile)
		r.Post("/sandboxes/execute", executeHandler.Execute)
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

	log.Printf("Execution Sandbox starting on :%d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
	log.Println("Server stopped")
}