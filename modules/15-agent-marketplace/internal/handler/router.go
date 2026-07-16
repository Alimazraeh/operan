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

	// Authenticated routes — middleware must come before any route definitions
	authValidator := middleware.NewAuthValidator("", "")
	r.Use(middleware.JWTMiddleware(authValidator))
	r.Use(middleware.TenantMiddleware())

	// Health endpoint (no auth required)
	r.Get("/health", healthHandler)

	// API routes
	r.Get("/v1/listings", lh.ListListings)
	r.Get("/v1/listings/{id}", lh.GetListing)
	r.Post("/v1/listings/{id}/deploy", lh.DeployListing)

	r.Post("/v1/subscriptions", sh.Subscribe)
	r.Get("/v1/subscriptions", sh.ListSubscriptions)

	r.Post("/v1/reviews", rh.CreateReview)
	r.Get("/v1/reviews", rh.ListReviews)

	return r
}

// healthHandler returns a simple health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}