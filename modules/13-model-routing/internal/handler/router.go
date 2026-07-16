package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/operan/model-routing/internal/engine"
	"github.com/operan/model-routing/internal/events"
	"github.com/operan/model-routing/internal/middleware"
	"github.com/operan/model-routing/internal/store"
)

// Setup creates the HTTP router with all routes configured.
func Setup(cfg *middleware.JWTConfig, ruleStore *store.PGRuleStore, perfStore *store.PGPerfStore, broker events.Broker) http.Handler {
	r := chi.NewRouter()

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health — unauthenticated
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireJWT(cfg))
		r.Use(middleware.RequireTenant())

		// Resolve endpoint — main routing
		resolveHandler := NewResolveHandler(engine.NewRouter(ruleStore, perfStore), broker)
		r.Post("/v1/resolve", resolveHandler.Handle)

		// Routing rules CRUD
		rulesHandler := NewRulesHandler(ruleStore)
		r.Get("/v1/rules", rulesHandler.List)
		r.Post("/v1/rules", rulesHandler.Create)
		r.Get("/v1/rules/{id}", rulesHandler.Get)
		r.Patch("/v1/rules/{id}", rulesHandler.Update)
		r.Delete("/v1/rules/{id}", rulesHandler.Delete)

		// Performance
		perfHandler := NewPerformanceHandler(perfStore)
		r.Get("/v1/performance", perfHandler.HandleGet)

		// Available models
		modelsHandler := NewModelsHandler(ruleStore)
		r.Get("/v1/models", modelsHandler.HandleGet)
	})

	return r
}