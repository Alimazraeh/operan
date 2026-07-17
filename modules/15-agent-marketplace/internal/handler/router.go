package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/operan/agent-marketplace/internal/clients"
	"github.com/operan/agent-marketplace/internal/deploy"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/middleware"
	"github.com/operan/agent-marketplace/internal/store"
)

// SetupRouter creates the chi router with all marketplace routes.
func SetupRouter(listingStore *store.ListingStore, subStore *store.SubscriptionStore,
	reviewStore *store.ReviewStore, evtPub *events.Publisher,
	m04Client *clients.M04Client, m03Client *clients.M03Client,
	m04Token string) http.Handler {

	lh := NewListingsHandler(listingStore, subStore, evtPub, nil)
	sh := NewSubscriptionsHandler(subStore, listingStore, evtPub)
	rh := NewReviewsHandler(reviewStore, subStore, listingStore, evtPub)
	deployer := deploy.NewDeployer(m04Client, m03Client, listingStore, subStore, evtPub, m04Token)
	lh = NewListingsHandler(listingStore, subStore, evtPub, deployer)

	r := chi.NewRouter()

	// CORS, logging, request/trace ID middleware
	r.Use(middleware.SetupCORS())
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.TraceID)

	// Health endpoint (no auth required)
	r.Get("/health", healthHandler)

	// Auth middleware — applies to API sub-router only
	api := chi.NewRouter()
	authValidator := middleware.NewAuthValidator("", "")
	api.Use(middleware.JWTMiddleware(authValidator))
	api.Use(middleware.TenantMiddleware())

	// API routes
	api.Get("/v1/listings", lh.ListListings)
	api.Get("/v1/listings/{id}", lh.GetListing)
	api.Post("/v1/listings/{id}/deploy", lh.DeployListing)

	api.Post("/v1/subscriptions", sh.Subscribe)
	api.Get("/v1/subscriptions", sh.ListSubscriptions)

	api.Post("/v1/reviews", rh.CreateReview)
	api.Get("/v1/reviews", rh.ListReviews)

	r.Mount("/", api)
	return r
}

// healthHandler returns a simple health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}