package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/operan/agent-marketplace/internal/ctxkeys"
	"github.com/operan/agent-marketplace/internal/deploy"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/store"
)

// ListingsHandler handles listing browse/search and detail operations.
type ListingsHandler struct {
	store      *store.ListingStore
	subStore   *store.SubscriptionStore
	evtPub     *events.Publisher
	deployer   *deploy.Deployer
}

// NewListingsHandler creates a new ListingsHandler.
func NewListingsHandler(s *store.ListingStore, subStore *store.SubscriptionStore, evtPub *events.Publisher, d *deploy.Deployer) *ListingsHandler {
	return &ListingsHandler{store: s, subStore: subStore, evtPub: evtPub, deployer: d}
}

// ListListings handles GET /v1/listings — search & browse marketplace.
func (h *ListingsHandler) ListListings(w http.ResponseWriter, r *http.Request) {
	f := &store.ListingFilter{
		Page:     1,
		PageSize: 20,
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			f.Page = p
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil && ps > 0 {
			f.PageSize = ps
		}
	}
	f.Category = r.URL.Query().Get("category")
	f.ListingType = r.URL.Query().Get("listing_type")
	f.Status = r.URL.Query().Get("status")
	f.Capability = r.URL.Query().Get("capability")
	f.Language = r.URL.Query().Get("language")
	f.Search = r.URL.Query().Get("search")
	f.SubscriptionTier = r.URL.Query().Get("subscription_tier")

	if reqSub := r.URL.Query().Get("requires_subscription"); reqSub != "" {
		v := reqSub == "true"
		f.RequiresSubscription = &v
	}
	if priceMin := r.URL.Query().Get("price_min"); priceMin != "" {
		if v, err := strconv.ParseFloat(priceMin, 64); err == nil {
			f.PriceMin = &v
		}
	}
	if priceMax := r.URL.Query().Get("price_max"); priceMax != "" {
		if v, err := strconv.ParseFloat(priceMax, 64); err == nil {
			f.PriceMax = &v
		}
	}
	if ratingMin := r.URL.Query().Get("rating_min"); ratingMin != "" {
		if v, err := strconv.ParseFloat(ratingMin, 64); err == nil {
			f.RatingMin = &v
		}
	}

	pag, err := h.store.List(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list listings", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, pag)
}

// GetListing handles GET /v1/listings/{id}.
func (h *ListingsHandler) GetListing(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	listing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "listing not found", "The requested listing does not exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get listing", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, listing)
}

// DeployListing handles POST /v1/listings/{id}/deploy.
func (h *ListingsHandler) DeployListing(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tenantID := ctxkeys.GetTenantID(r.Context())

	if h.deployer == nil {
		writeError(w, http.StatusServiceUnavailable, "deployer not available", "")
		return
	}

	result, err := h.deployer.Deploy(r.Context(), tenantID, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "deployment failed", result.Errors[0])
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// createListingRequest represents a POST /v1/listings body.
type createListingRequest struct {
	VendorID              string   `json:"vendor_id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	Category              string   `json:"category"`
	ListingType           string   `json:"listing_type"`
	Version               string   `json:"version"`
	Capabilities          []string `json:"capabilities"`
	SupportedLanguages    []string `json:"supported_languages"`
	RequiresSubscription  bool     `json:"requires_subscription"`
	SubscriptionTier      string   `json:"subscription_tier"`
	TrialDays             int      `json:"trial_days"`
	PriceUSD              float64  `json:"price_usd"`
	CompatibilityVersions interface{} `json:"compatibility_versions"`
	Metadata              interface{} `json:"metadata"`
}