package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/operan/model-routing/internal/config"
	"github.com/operan/model-routing/internal/events"
	"github.com/operan/model-routing/internal/handler"
	"github.com/operan/model-routing/internal/middleware"
	"github.com/operan/model-routing/internal/store"
)

//go:embed migrations/001_create_schema.sql
var migrationsSQL string

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Connect to PostgreSQL
	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("database ping: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if _, err := pool.Exec(context.Background(), migrationsSQL); err != nil {
		log.Printf("migration warning: %v", err)
	}

	// Setup stores
	ruleStore := store.NewPGRuleStore(pool)
	perfStore := store.NewPGPerfStore(pool)

	// Setup Kafka broker
	var broker events.Broker
	if cfg.EventBrokerURL != "" {
		broker, err = events.NewKafkaBroker(strings.Split(cfg.EventBrokerURL, ","))
		if err != nil {
			log.Printf("kafka broker: %v (proceeding without Kafka)", err)
		}
	}
	if broker == nil {
		broker = events.NoOpBroker{}
	}
	defer broker.Close()

	// Setup JWT config
	jwtCfg := &middleware.JWTConfig{
		Secret: []byte(cfg.JWTSecret),
	}

	// Setup HTTP router
	router := handler.Setup(jwtCfg, ruleStore, perfStore, broker)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	go func() {
		log.Printf("model-routing listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("stopped")
}