package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/operan/execution-sandbox/internal/events"
	osm "github.com/operan/execution-sandbox/internal/middleware"
	"github.com/operan/execution-sandbox/internal/policies"
	"github.com/operan/execution-sandbox/internal/sandbox"
	"github.com/operan/execution-sandbox/internal/store"
)

// SetupRouter creates the chi router with all routes.
func SetupRouter(profileStore *store.ProfileStore, instanceStore *store.InstanceStore,
	executor *sandbox.Executor, policyClient *policies.PolicyClient, eventPub *events.Publisher,
) http.Handler {
	r := chi.NewRouter()
	r.Use(osm.SetupCORS())
	r.Use(osm.Logger)
	r.Use(osm.RequestID)
	r.Use(osm.TraceID)

	// Health endpoint — unauthenticated
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated routes
	profileHandler := NewProfileHandler(profileStore)
	instanceHandler := NewInstanceHandler(instanceStore)
	executeHandler := NewExecuteHandler(executor, profileStore, instanceStore, policyClient, eventPub)

	r.Group(func(r chi.Router) {
		r.Use(osm.JWTMiddleware(osm.NewAuthValidator("", ""))) // placeholder — main.go overrides
		r.Use(osm.TenantMiddleware())

		// Sandbox instances
		r.Get("/sandboxes/instances", instanceHandler.ListInstances)
		r.Get("/sandboxes/instances/{id}", instanceHandler.GetInstance)
		r.Post("/sandboxes/instances/{id}/cancel", instanceHandler.CancelInstance)

		// Sandbox profiles
		r.Get("/sandbox-profiles", profileHandler.ListProfiles)
		r.Post("/sandbox-profiles", profileHandler.CreateProfile)
		r.Get("/sandbox-profiles/{id}", profileHandler.GetProfile)
		r.Patch("/sandbox-profiles/{id}", profileHandler.UpdateProfile)
		r.Delete("/sandbox-profiles/{id}", profileHandler.DeleteProfile)

		// Execute tool in sandbox
		r.Post("/sandboxes/execute", executeHandler.Execute)
	})

	return r
}