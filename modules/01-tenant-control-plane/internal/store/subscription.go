package store

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubscriptionStatus represents subscription lifecycle.
type SubscriptionStatus string

const (
	SubStatusActive         SubscriptionStatus = "active"
	SubStatusTrialing       SubscriptionStatus = "trialing"
	SubStatusPastDue        SubscriptionStatus = "past_due"
	SubStatusCancelled      SubscriptionStatus = "cancelled"
	SubStatusCancelling     SubscriptionStatus = "cancelling"
	SubStatusExpired        SubscriptionStatus = "expired"
)

// BillingCycle represents subscription billing frequency.
type BillingCycle string

const (
	BillingMonthly   BillingCycle = "monthly"
	BillingAnnual    BillingCycle = "annual"
	BillingQuarterly BillingCycle = "quarterly"
)

// Subscription represents a tenant subscription.
type Subscription struct {
	ID                 string         `json:"id"`
	TenantID           string         `json:"tenant_id"`
	Plan               Plan           `json:"plan"`
	PlanName           string         `json:"plan_name"`
	Status             SubscriptionStatus `json:"status"`
	BillingCycle       BillingCycle   `json:"billing_cycle"`
	SeatCount          int            `json:"seat_count"`
	UnitPrice          float64        `json:"unit_price"`
	TotalAmount        float64        `json:"total_amount"`
	Currency           string         `json:"currency"`
	CurrentPeriodStart time.Time      `json:"current_period_start"`
	CurrentPeriodEnd   time.Time      `json:"current_period_end"`
	NextBillingDate    time.Time      `json:"next_billing_date"`
	CancelAtPeriodEnd  bool           `json:"cancel_at_period_end"`
	CancelledAt        *time.Time     `json:"cancelled_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	CustomQuotas       *QuotaConfig   `json:"custom_quotas,omitempty"`
}

// SubscriptionUpdateRequest for PATCH /subscriptions/{id}.
type SubscriptionUpdateRequest struct {
	Plan         Plan        `json:"plan,omitempty"`
	BillingCycle BillingCycle `json:"billing_cycle,omitempty"`
	SeatCount    *int        `json:"seat_count,omitempty"`
	CustomQuotas *QuotaConfig `json:"custom_quotas,omitempty"`
}

// SubscriptionCancelRequest for cancel operations.
type SubscriptionCancelRequest struct {
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	Reason            string `json:"reason,omitempty"`
}

// SubscriptionUpgradeRequest for plan upgrades.
type SubscriptionUpgradeRequest struct {
	TargetPlan Plan `json:"target_plan"`
}

// Plan pricing constants.
var planPricing = map[Plan]float64{
	PlanSaaS:      99.0,
	PlanEnterprise: 499.0,
	PlanSovereign:  1999.0,
}

var planNames = map[Plan]string{
	PlanSaaS:      "SaaS",
	PlanEnterprise: "Enterprise",
	PlanSovereign:  "Sovereign",
}

// SubscriptionStore manages tenant subscriptions.
type SubscriptionStore struct {
	mu        sync.RWMutex
	subscriptions map[string]*Subscription
	byTenant    map[string]string // keyed by TenantID -> SubscriptionID
}

// NewSubscriptionStore creates a new SubscriptionStore.
func NewSubscriptionStore() *SubscriptionStore {
	return &SubscriptionStore{
		subscriptions: make(map[string]*Subscription),
		byTenant:      make(map[string]string),
	}
}

// Create adds a new subscription for a tenant.
func (s *SubscriptionStore) Create(sub *Subscription) (*Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = timeNow()
	}
	sub.UpdatedAt = timeNow()

	if sub.Status == "" {
		sub.Status = SubStatusTrialing
	}
	if sub.PlanName == "" {
		sub.PlanName = planNames[sub.Plan]
	}
	if sub.UnitPrice == 0 {
		sub.UnitPrice = planPricing[sub.Plan]
	}
	if sub.Currency == "" {
		sub.Currency = "USD"
	}
	if sub.BillingCycle == "" {
		sub.BillingCycle = BillingMonthly
	}
	if sub.SeatCount < 1 {
		sub.SeatCount = 1
	}
	if sub.TotalAmount == 0 {
		sub.TotalAmount = sub.UnitPrice * float64(sub.SeatCount)
	}
	if sub.CurrentPeriodStart.IsZero() {
		sub.CurrentPeriodStart = timeNow()
	}
	if sub.CurrentPeriodEnd.IsZero() {
		sub.CurrentPeriodEnd = timeNow().AddDate(0, 1, 0)
	}
	if sub.NextBillingDate.IsZero() {
		sub.NextBillingDate = sub.CurrentPeriodEnd
	}

	// Check for existing active subscription
	if existingID, exists := s.byTenant[sub.TenantID]; exists {
		return nil, fmt.Errorf("tenant %s already has a subscription: %s", sub.TenantID, existingID)
	}

	s.subscriptions[sub.ID] = sub
	s.byTenant[sub.TenantID] = sub.ID

	return sub, nil
}

// GetByID retrieves a subscription by ID (no tenant check — for admin use only).
func (s *SubscriptionStore) GetByID(id string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	cpy := *sub
	return &cpy, nil
}

// GetByIDAndTenant retrieves a subscription by ID and verifies the TenantID matches.
func (s *SubscriptionStore) GetByIDAndTenant(id, tenantID string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	if sub.TenantID != tenantID {
		return nil, fmt.Errorf("permission denied: subscription %s does not belong to tenant %s", id, tenantID)
	}
	cpy := *sub
	return &cpy, nil
}

// GetByTenant retrieves a subscription by tenant ID.
func (s *SubscriptionStore) GetByTenant(tenantID string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subID, ok := s.byTenant[tenantID]
	if !ok {
		return nil, fmt.Errorf("no subscription for tenant %s", tenantID)
	}
	sub, ok := s.subscriptions[subID]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found for tenant %s", subID, tenantID)
	}
	cpy := *sub
	return &cpy, nil
}

// Patch updates a subscription.
func (s *SubscriptionStore) Patch(id string, req SubscriptionUpdateRequest) (*Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}

	if req.Plan != "" {
		sub.Plan = req.Plan
		sub.PlanName = planNames[req.Plan]
		sub.UnitPrice = planPricing[req.Plan]
		sub.TotalAmount = sub.UnitPrice * float64(sub.SeatCount)
	}
	if req.BillingCycle != "" {
		sub.BillingCycle = req.BillingCycle
	}
	if req.SeatCount != nil {
		if *req.SeatCount < 1 {
			return nil, fmt.Errorf("seat_count must be >= 1")
		}
		sub.SeatCount = *req.SeatCount
		sub.TotalAmount = sub.UnitPrice * float64(sub.SeatCount)
	}
	if req.CustomQuotas != nil {
		sub.CustomQuotas = req.CustomQuotas
	}

	sub.UpdatedAt = timeNow()

	return sub, nil
}

// Cancel marks a subscription for cancellation.
func (s *SubscriptionStore) Cancel(id string, req SubscriptionCancelRequest) (*Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}

	if req.CancelAtPeriodEnd {
		sub.Status = SubStatusCancelling
		sub.CancelAtPeriodEnd = true
	} else {
		sub.Status = SubStatusCancelled
		now := timeNow()
		sub.CancelledAt = &now
	}

	sub.UpdatedAt = timeNow()

	return sub, nil
}

// Delete removes a subscription entirely.
func (s *SubscriptionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[id]
	if !ok {
		return fmt.Errorf("subscription %s not found", id)
	}

	delete(s.byTenant, sub.TenantID)
	delete(s.subscriptions, id)

	return nil
}

// List returns all subscriptions.
func (s *SubscriptionStore) List() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		cpy := *sub
		result = append(result, &cpy)
	}

	slices.SortFunc(result, func(a, b *Subscription) int {
		if a.TenantID < b.TenantID {
			return -1
		}
		if a.TenantID > b.TenantID {
			return 1
		}
		return 0
	})

	return result
}

// CountTotal returns the total number of subscriptions.
func (s *SubscriptionStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscriptions)
}
