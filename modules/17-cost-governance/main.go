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

	"github.com/operan/cost-governance/internal/config"
	"github.com/operan/cost-governance/internal/consumers"
	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/events"
	"github.com/operan/cost-governance/internal/handler"
	"github.com/operan/cost-governance/internal/middleware"
	"github.com/operan/cost-governance/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Connect to PostgreSQL
	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	defer pool.Close()

	log.Println("connected to database")

	// Create event broker
	broker := events.NewBroker()

	// Create stores
	budgetStore := store.NewBudgetStore(pool)
	eventStore := store.NewCostEventStore(pool)
	alertStore := store.NewAlertStore(pool)

	// Create throttle manager
	throttleMgr := engine.NewThrottleManager()

	// Create budget check engine
	budgetEngine := engine.NewEngine(budgetStore, eventStore, alertStore, throttleMgr)

	// Create cost event consumer
	costConsumer := consumers.NewCostEventConsumer(broker, eventStore, budgetEngine)

	// Create auth validator
	authValidator := middleware.NewAuthValidator(cfg)

	// Create router
	r := handler.SetupRouter(authValidator, budgetStore, eventStore, alertStore, budgetEngine, throttleMgr, costConsumer)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("server forced to shutdown: %v", err)
		}
	}()

	log.Printf("cost-governance listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}

	log.Println("server stopped")
}