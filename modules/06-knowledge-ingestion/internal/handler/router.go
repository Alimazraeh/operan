package handler

import (
	"time"
	"net/http"

	chiMW "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/operan/modules/06-knowledge-ingestion/internal/ctxkeys"
	"github.com/operan/modules/06-knowledge-ingestion/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// SetupRouter configures the chi router with all routes and middleware.
func SetupRouter(
	sourcesHandler *SourcesHandler,
	jobsHandler *JobsHandler,
	resultsHandler *ResultsHandler,
	ingestHandler *IngestHandler,
	authValidator *middleware.AuthValidator,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMW.RequestID)
	r.Use(chiMW.RealIP)
	r.Use(chiMW.Logger)
	r.Use(chiMW.Recoverer)
	// Timeout takes a time.Duration — a bare 60000 is 60µs and killed every
	// request's context before the first DB query.
	r.Use(chiMW.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-ID", "X-Request-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check — unauthenticated.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator), middleware.TenantMiddleware())

		// Source routes.
		r.Get("/v1/sources", sourcesHandler.ListSources)
		r.Post("/v1/sources", sourcesHandler.CreateSource)
		r.Get("/v1/sources/{id}", sourcesHandler.GetSource)
		r.Patch("/v1/sources/{id}", sourcesHandler.UpdateSource)
		r.Delete("/v1/sources/{id}", sourcesHandler.DeleteSource)

		// Ingest.
		r.Post("/v1/ingest", ingestHandler.IngestSource)

		// Job routes.
		r.Get("/v1/jobs", jobsHandler.ListJobs)
		r.Get("/v1/jobs/{id}", jobsHandler.GetJob)
		r.Post("/v1/jobs/{id}/cancel", jobsHandler.CancelJob)

		// Result routes.
		r.Get("/v1/jobs/{id}/results", resultsHandler.ListResults)
	})

	// Helper to extract tenant from context (used by middleware).
	_ = ctxkeys.TenantIDKey

	return r
}