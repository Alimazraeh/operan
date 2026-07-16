package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/agent-marketplace/internal/ctxkeys"
	"github.com/operan/agent-marketplace/internal/events"
	"github.com/operan/agent-marketplace/internal/store"
)

// SubscribeRequest is the body for POST /v1/subscriptions.
type SubscribeRequest struct {
	ListingID string `json:"listing_id"`
}

// SubscriptionsHandler handles subscription operations.
type SubscriptionsHandler struct {
	store      *store.SubscriptionStore
	listingStore *store.ListingStore
	evtPub     *events.Publisher
}

// NewSubscriptionsHandler creates a new SubscriptionsHandler.
func NewSubscriptionsHandler(s *store.SubscriptionStore, ls *store.ListingStore, evtPub *events.Publisher) *SubscriptionsHandler {
	return &SubscriptionsHandler{store: s, listingStore: ls, evtPub: evtPub}
}

// Subscribe handles POST /v1/subscriptions.
func (h *SubscriptionsHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "listing_id is required")
		return
	}
	if req.ListingID == "" {
		writeError(w, http.StatusBadRequest, "invalid request body", "listing_id is required")
		return
	}

	// Verify listing exists and is approved
	listing, err := h.listingStore.GetByID(r.Context(), req.ListingID)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "listing not found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get listing", err.Error())
		return
	}
	if listing.Status != "approved" {
		writeError(w, http.StatusBadRequest, "listing not approved", "Can only subscribe to approved listings")
		return
	}
	if listing.Status == "deactivated" {
		writeError(w, http.StatusBadRequest, "listing deactivated", "Cannot subscribe to deactivated listings")
		return
	}

	// Check existing subscription
	existing, err := h.store.GetByTenantAndListing(r.Context(), tenantID, req.ListingID)
	if err == nil && existing != nil {
		// Already subscribed
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"subscription_id":    existing.ID,
			"listing_name":       listing.Name,
			"subscription_tier":  existing.SubscriptionTier,
			"status":             existing.Status,
			"expires_at":         existing.ExpiresAt,
			"trial_days_remaining": calcTrialDaysRemaining(existing),
			"already_subscribed":  true,
		})
		return
	}

	// Create subscription
	sub := &store.TenantSubscription{
		TenantID:       tenantID,
		ListingID:      req.ListingID,
		Status:         "trial",
		SubscriptionTier: listing.SubscriptionTier,
		AutoRenew:      true,
		TrialUsed:      false,
	}

	if !listing.RequiresSubscription || listing.SubscriptionTier == "free" {
		sub.Status = "active"
		sub.ExpiresAt = nil
	} else {
		// Trial period for paid listings
		trialDays := listing.TrialDays
		if trialDays == 0 {
			trialDays = 14 // default 14-day trial
		}
		exp := time.Now().AddDate(0, 0, trialDays)
		sub.ExpiresAt = &exp
	}

	if err := h.store.Create(r.Context(), sub); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription", err.Error())
		return
	}

	// Publish event
	h.evtPub.PublishSubscriptionCreated(r.Context(), tenantID, req.ListingID, listing.SubscriptionTier)

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"subscription_id":        sub.ID,
		"listing_name":           listing.Name,
		"subscription_tier":      listing.SubscriptionTier,
		"status":                 sub.Status,
		"expires_at":             sub.ExpiresAt,
		"trial_days_remaining":   calcTrialDaysRemaining(sub),
	})
}

// ListSubscriptions handles GET /v1/subscriptions.
func (h *SubscriptionsHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	page := 1
	pageSize := 20
	status := ""

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
	status = r.URL.Query().Get("status")

	pag, err := h.store.ListByTenant(r.Context(), tenantID, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list subscriptions", err.Error())
		return
	}

	if status != "" {
		filtered := make([]store.TenantSubscription, 0)
		for _, s := range pag.Subscriptions {
			if s.Status == status {
				filtered = append(filtered, s)
			}
		}
		pag.Subscriptions = filtered
		// Recalculate total would need a separate count query
		pag.Total = len(filtered)
	}

	WriteJSON(w, http.StatusOK, pag)
}

func calcTrialDaysRemaining(sub *store.TenantSubscription) int {
	if sub.Status != "trial" || sub.ExpiresAt == nil {
		return 0
	}
	remaining := int(time.Until(*sub.ExpiresAt).Hours() / 24)
	if remaining < 0 {
		return 0
	}
	return remaining
}