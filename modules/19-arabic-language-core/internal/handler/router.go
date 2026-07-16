package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/operan/arabic-language-core/internal/clients"
	"github.com/operan/arabic-language-core/internal/events"
	"github.com/operan/arabic-language-core/internal/middleware"
	"github.com/operan/arabic-language-core/internal/store"
)

// Router creates the HTTP router for all M19 endpoints.
func Router(authValidator *middleware.AuthValidator, terminologyStore *store.TerminologyStore,
	broker *events.Broker, m12Client *clients.M12Client, jwtSecret string, logger Logger) chi.Router {

	termHandler := NewTerminologyHandler(terminologyStore, broker, m12Client, jwtSecret, logger)

	r := chi.NewRouter()

	// Health check — unauthenticated.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Stats — unauthenticated.
	r.Get("/v1/stats", HandleStats)

	// Public NLP endpoints (no auth required — useful for pre-processing).
	r.Post("/v1/normalize", HandleNormalize)
	r.Post("/v1/detect-dialect", HandleDetectDialect)

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(authValidator), middleware.TenantMiddleware())

		// Terminology endpoints.
		r.Post("/v1/terminology/check", termHandler.HandleCheck)
		r.Get("/v1/terminology/glossary", termHandler.HandleListGlossary)
		r.Post("/v1/terminology/glossary", termHandler.HandleCreateGlossary)
		r.Patch("/v1/terminology/glossary/{id}", termHandler.HandleUpdateGlossary)
		r.Delete("/v1/terminology/glossary/{id}", termHandler.HandleDeleteGlossary)

		// Embeddings — delegates to M12.
		r.Post("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
			HandleEmbedArabic(w, r, m12Client, broker, terminologyStore, jwtSecret)
		})
	})

	return r
}