package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/operan/policy-governance/internal/config"
	"github.com/operan/policy-governance/internal/engine"
	"github.com/operan/policy-governance/internal/events"
	"github.com/operan/policy-governance/internal/handler"
	"github.com/operan/policy-governance/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	policyStore := store.NewPolicyStore(pool)
	groupStore := store.NewGroupStore(pool)
	auditStore := store.NewAuditStore(pool)

	eventPub := events.NewPublisher(cfg.EventBrokerURL)
	policyEngine := engine.NewEngine(policyStore, eventPub)

	router := handler.SetupRouter(policyStore, groupStore, auditStore, policyEngine, eventPub, cfg.JWTSecret, cfg.Issuer)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigch
		log.Printf("received signal %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("server forced to shutdown: %v", err)
		}
	}()

	log.Printf("Policy Governance Engine listening on port %s", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}