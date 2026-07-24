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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/operan/modules/06-knowledge-ingestion/internal/chunker"
	"github.com/operan/modules/06-knowledge-ingestion/internal/clients"
	"github.com/operan/modules/06-knowledge-ingestion/internal/config"
	"github.com/operan/modules/06-knowledge-ingestion/internal/events"
	"github.com/operan/modules/06-knowledge-ingestion/internal/handler"
	"github.com/operan/modules/06-knowledge-ingestion/internal/middleware"
	"github.com/operan/modules/06-knowledge-ingestion/internal/store"
	"github.com/operan/modules/06-knowledge-ingestion/internal/workers"
	"github.com/operan/modules/06-knowledge-ingestion/migrations"
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
	if err := migrations.RunMigrations(pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations applied")
	defer pool.Close()

	// Event broker.
	var broker events.Broker
	if cfg.EventBrokerURL != "" {
		broker = events.NoOpBroker{}
	} else {
		broker = events.NoOpBroker{}
	}

	// Logger.
	logger := log.New(os.Stdout, "[m06-knowledge-ingestion] ", log.LstdFlags)

	// Stores.
	sourcesStore := store.NewSourcesStore(pool)
	jobsStore := store.NewJobsStore(pool)
	resultsStore := store.NewResultsStore(pool)

	// Auth validator.
	authValidator := middleware.NewAuthValidator(cfg)

	// Clients.
	m12Client := clients.NewM12Client(cfg.M12BaseURL, 30000)
	m07Client := clients.NewM07Client(cfg.M07BaseURL, 30000)
	m19Client := clients.NewM19Client(cfg.M19BaseURL, 10000)

	// Chunker.
	chunkerEngine := chunker.NewAdaptiveChunker()

	// Worker.
	eventEvents := events.NewEvents(broker, logger)
	_ = eventEvents
	worker := workers.NewWorker(
		sourcesStore,
		jobsStore,
		resultsStore,
		m12Client,
		m07Client,
		m19Client,
		broker,
		chunkerEngine,
		cfg.ServiceToken,
		logger,
	)
	worker.Start(context.Background())

	// Handlers.
	sourcesHandler := handler.NewSourcesHandler(sourcesStore)
	jobsHandler := handler.NewJobsHandler(jobsStore, worker)
	resultsHandler := handler.NewResultsHandler(resultsStore)
	ingestHandler := handler.NewIngestHandler(jobsStore, worker, cfg.ServiceToken)

	// Router.
	router := handler.SetupRouter(sourcesHandler, jobsHandler, resultsHandler, ingestHandler, authValidator)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
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

	logger.Printf("starting knowledge ingestion server on port %d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server: %v", err)
	}
	logger.Println("server stopped")
}
