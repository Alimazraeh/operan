package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/agent-marketplace/internal/ctxkeys"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/store"
)

// CreateReviewRequest is the body for POST /v1/reviews.
type CreateReviewRequest struct {
	ListingID        string `json:"listing_id"`
	Rating           int    `json:"rating"`
	Title            string `json:"title"`
	ReviewText       string `json:"review_text"`
}

// ReviewsHandler handles review operations.
type ReviewsHandler struct {
	store       *store.ReviewStore
	subStore    *store.SubscriptionStore
	listingStore *store.ListingStore
	evtPub      *events.Publisher
}

// NewReviewsHandler creates a new ReviewsHandler.
func NewReviewsHandler(s *store.ReviewStore, subStore *store.SubscriptionStore, ls *store.ListingStore, evtPub *events.Publisher) *ReviewsHandler {
	return &ReviewsHandler{store: s, subStore: subStore, listingStore: ls, evtPub: evtPub}
}

// CreateReview handles POST /v1/reviews.
func (h *ReviewsHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "")
		return
	}
	if req.ListingID == "" {
		writeError(w, http.StatusBadRequest, "invalid request body", "listing_id is required")
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "rating out of range", "rating must be between 1 and 5")
		return
	}

	// Verify listing exists
	_, err := h.listingStore.GetByID(r.Context(), req.ListingID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "listing not found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get listing", err.Error())
		return
	}

	// Verify tenant is subscribed (or has purchased)
	active, err := h.subStore.IsActive(r.Context(), tenantID, req.ListingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "subscription check failed", err.Error())
		return
	}
	if !active {
		writeError(w, http.StatusForbidden, "not subscribed", "You must subscribe to this listing before reviewing")
		return
	}

	// Check for existing review (one review per tenant per listing)
	existing, err := h.store.GetByTenantAndListing(r.Context(), tenantID, req.ListingID)
	if err == nil && existing != nil {
		writeError(w, http.StatusConflict, "review already exists", "You already have a review for this listing")
		return
	}

	// Create review
	review := &store.Review{
		TenantID:         tenantID,
		ListingID:        req.ListingID,
		Rating:           req.Rating,
		Title:            req.Title,
		ReviewText:       req.ReviewText,
		VerifiedPurchase: true,
		Status:           "active",
	}
	review.ID = uuid.New().String()

	if err := h.store.Create(r.Context(), review); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create review", err.Error())
		return
	}

	// Update listing rating
	if err := h.listingStore.UpdateRating(r.Context(), req.ListingID); err != nil {
		// Non-critical, just log
	}

	// Publish event
	h.evtPub.PublishReviewCreated(r.Context(), tenantID, req.ListingID, req.Rating)

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"review": map[string]interface{}{
			"id":                 review.ID,
			"listing_id":         review.ListingID,
			"rating":             review.Rating,
			"title":              review.Title,
			"review_text":        review.ReviewText,
			"verified_purchase":  review.VerifiedPurchase,
			"created_at":         review.CreatedAt,
		},
	})
}

// ListReviews handles GET /v1/reviews.
func (h *ReviewsHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	listingID := chi.URLParam(r, "listing_id")
	if listingID == "" {
		listingID = r.URL.Query().Get("listing_id")
	}
	if listingID == "" {
		writeError(w, http.StatusBadRequest, "missing listing_id", "listing_id is required")
		return
	}

	page := 1
	pageSize := 20
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if ps, err := strconv.Atoi(psStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	pag, err := h.store.ListByListing(r.Context(), listingID, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reviews", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, pag)
}