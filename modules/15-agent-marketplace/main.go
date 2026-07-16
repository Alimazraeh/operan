package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/operan/agent-marketplace/internal/clients"
	"github.com/operan/agent-marketplace/internal/config"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/handler"
	"github.com/operan/agent-marketplace/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Database connection
	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	defer pool.Close()

	// Initialize stores
	listingStore := store.NewListingStore(pool)
	subStore := store.NewSubscriptionStore(pool)
	reviewStore := store.NewReviewStore(pool)

	// Initialize clients
	m04Client := clients.NewM04Client(cfg.M04BaseURL)
	m03Client := clients.NewM03Client(cfg.M03BaseURL)

	// Initialize event publisher
	evtPub := events.NewPublisher(cfg.EventBrokerURL)

	// Build router
	m04Token := os.Getenv("M04_TOKEN")
	router := handler.SetupRouter(listingStore, subStore, reviewStore, evtPub, m04Client, m03Client, m04Token)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		log.Printf("marketplace: listening on :%d", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("marketplace: shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
	log.Println("marketplace: stopped")
}