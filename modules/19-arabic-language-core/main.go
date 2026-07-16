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
	"github.com/operan/arabic-language-core/internal/clients"
	"github.com/operan/arabic-language-core/internal/config"
	"github.com/operan/arabic-language-core/internal/events"
	"github.com/operan/arabic-language-core/internal/handler"
	authMW "github.com/operan/arabic-language-core/internal/middleware"
	"github.com/operan/arabic-language-core/internal/store"
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
	brokerLogger := log.New(os.Stdout, "[arabic-language-core/events] ", log.LstdFlags)
	var broker *events.Broker
	if cfg.EventBrokerURL != "" {
		kp, err := events.NewKafkaPublisherFromURL(cfg.EventBrokerURL, brokerLogger)
		if err != nil {
			log.Printf("event broker init warning: %v (continuing without Kafka)", err)
		}
		broker = events.NewBroker(kp)
		defer kp.Close()
	} else {
		broker = events.NewBroker(events.NoOpBroker{})
	}

	// Logger.
	logger := log.New(os.Stdout, "[arabic-language-core] ", log.LstdFlags)

	// Stores.
	terminologyStore := store.NewTerminologyStore(pool)

	// Auth validator.
	authValidator := authMW.NewAuthValidator(cfg)

	// M12 client.
	var m12Client *clients.M12Client
	if cfg.M12BaseURL != "" {
		m12Client = clients.NewM12Client(cfg.M12BaseURL)
	}

	// Router.
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

	// Stats — unauthenticated.
	r.Get("/v1/stats", handler.HandleStats)

	// Public NLP endpoints.
	r.Post("/v1/normalize", handler.HandleNormalize)
	r.Post("/v1/detect-dialect", handler.HandleDetectDialect)

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(authMW.JWTMiddleware(authValidator), authMW.TenantMiddleware())

		termHandler := handler.NewTerminologyHandler(terminologyStore, broker, m12Client, cfg.JWTSecret, logger)
		r.Post("/v1/terminology/check", termHandler.HandleCheck)
		r.Get("/v1/terminology/glossary", termHandler.HandleListGlossary)
		r.Post("/v1/terminology/glossary", termHandler.HandleCreateGlossary)
		r.Patch("/v1/terminology/glossary/{id}", termHandler.HandleUpdateGlossary)
		r.Delete("/v1/terminology/glossary/{id}", termHandler.HandleDeleteGlossary)

		r.Post("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
			handler.HandleEmbedArabic(w, r, m12Client, broker, terminologyStore, cfg.JWTSecret)
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      r,
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

	logger.Printf("starting Arabic Language Core server on port %d", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server: %v", err)
	}
	logger.Println("server stopped")
}