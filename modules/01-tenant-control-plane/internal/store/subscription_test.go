package store

import (
	"testing"
	"time"
)

func TestSubscriptionStore_Create(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	t.Run("creates subscription with defaults", func(t *testing.T) {
		sub := &Subscription{
			TenantID:   "tenant-1",
			Plan:       PlanSaaS,
			SeatCount:  5,
			PlanName:   "SaaS",
		}

		created, err := store.Create(sub)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
		if created.Status != SubStatusTrialing {
			t.Errorf("expected status trialing, got %q", created.Status)
		}
		if created.UnitPrice != 99.0 {
			t.Errorf("expected unit price 99.0, got %f", created.UnitPrice)
		}
		if created.TotalAmount != 495.0 {
			t.Errorf("expected total 495.0, got %f", created.TotalAmount)
		}
		if created.Currency != "USD" {
			t.Errorf("expected USD, got %q", created.Currency)
		}
	})

	t.Run("rejects duplicate tenant subscription", func(t *testing.T) {
		sub1 := &Subscription{
			TenantID: "tenant-2",
			Plan:     PlanEnterprise,
			PlanName: "Enterprise",
		}
		_, err := store.Create(sub1)
		if err != nil {
			t.Fatalf("expected no error on first create, got %v", err)
		}

		sub2 := &Subscription{
			TenantID: "tenant-2",
			Plan:     PlanSaaS,
			PlanName: "SaaS",
		}
		_, err = store.Create(sub2)
		if err == nil {
			t.Fatal("expected error for duplicate tenant subscription, got nil")
		}
	})

	t.Run("sovereign plan pricing", func(t *testing.T) {
		sub := &Subscription{
			TenantID:   "tenant-3",
			Plan:       PlanSovereign,
			PlanName:   "Sovereign",
			SeatCount:  2,
		}

		created, err := store.Create(sub)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.UnitPrice != 1999.0 {
			t.Errorf("expected unit price 1999.0, got %f", created.UnitPrice)
		}
	})
}

func TestSubscriptionStore_GetByID(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sub := &Subscription{
		TenantID:   "tenant-1",
		Plan:       PlanSaaS,
		PlanName:   "SaaS",
		SeatCount:  1,
	}

	created, err := store.Create(sub)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves subscription by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.TenantID != "tenant-1" {
			t.Errorf("expected tenant-1, got %q", got.TenantID)
		}
	})

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returned copy is independent", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		got.Plan = PlanEnterprise
		original, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if original.Plan == PlanEnterprise {
			t.Fatal("modifying returned copy should not affect stored subscription")
		}
	})
}

func TestSubscriptionStore_GetByTenant(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sub := &Subscription{
		TenantID:   "tenant-1",
		Plan:       PlanEnterprise,
		PlanName:   "Enterprise",
	}

	_, err := store.Create(sub)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves subscription by tenant ID", func(t *testing.T) {
		got, err := store.GetByTenant("tenant-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Plan != PlanEnterprise {
			t.Errorf("expected enterprise plan, got %q", got.Plan)
		}
	})

	t.Run("returns error for tenant without subscription", func(t *testing.T) {
		_, err := store.GetByTenant("non-existent-tenant")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSubscriptionStore_Patch(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sub := &Subscription{
		TenantID:   "tenant-1",
		Plan:       PlanSaaS,
		PlanName:   "SaaS",
		SeatCount:  2,
	}

	created, err := store.Create(sub)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("upgrades plan", func(t *testing.T) {
		updated, err := store.Patch(created.ID, SubscriptionUpdateRequest{
			Plan: PlanEnterprise,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Plan != PlanEnterprise {
			t.Errorf("expected enterprise plan, got %q", updated.Plan)
		}
		if updated.UnitPrice != 499.0 {
			t.Errorf("expected unit price 499.0, got %f", updated.UnitPrice)
		}
	})

	t.Run("updates seat count", func(t *testing.T) {
		updated, err := store.Patch(created.ID, SubscriptionUpdateRequest{
			SeatCount: intPtr(10),
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.SeatCount != 10 {
			t.Errorf("expected 10 seats, got %d", updated.SeatCount)
		}
	})

	t.Run("rejects seat count < 1", func(t *testing.T) {
		_, err := store.Patch(created.ID, SubscriptionUpdateRequest{
			SeatCount: intPtr(0),
		})
		if err == nil {
			t.Fatal("expected error for seat_count < 1, got nil")
		}
	})

	t.Run("sets custom quotas", func(t *testing.T) {
		quotas := &QuotaConfig{MaxAgents: 100}
		updated, err := store.Patch(created.ID, SubscriptionUpdateRequest{
			CustomQuotas: quotas,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.CustomQuotas == nil {
			t.Fatal("expected custom quotas to be set")
		}
	})

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		_, err := store.Patch("non-existent", SubscriptionUpdateRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func intPtr(i int) *int { return &i }

func TestSubscriptionStore_Cancel(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sub := &Subscription{
		TenantID:   "tenant-1",
		Plan:       PlanSaaS,
		PlanName:   "SaaS",
		Status:     SubStatusActive,
	}

	created, err := store.Create(sub)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("cancels at period end", func(t *testing.T) {
		updated, err := store.Cancel(created.ID, SubscriptionCancelRequest{
			CancelAtPeriodEnd: true,
			Reason:            "cost",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Status != SubStatusCancelling {
			t.Errorf("expected cancelling status, got %q", updated.Status)
		}
		if !updated.CancelAtPeriodEnd {
			t.Error("expected cancel_at_period_end true")
		}
	})

	t.Run("immediate cancellation", func(t *testing.T) {
		updated, err := store.Cancel(created.ID, SubscriptionCancelRequest{
			CancelAtPeriodEnd: false,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Status != SubStatusCancelled {
			t.Errorf("expected cancelled status, got %q", updated.Status)
		}
		if updated.CancelledAt == nil {
			t.Error("expected cancelled_at to be set")
		}
	})

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		_, err := store.Cancel("non-existent", SubscriptionCancelRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSubscriptionStore_Delete(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	sub := &Subscription{
		TenantID:   "tenant-1",
		Plan:       PlanSaaS,
		PlanName:   "SaaS",
	}

	created, err := store.Create(sub)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	err = store.Delete(created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = store.GetByID(created.ID)
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}

	_, err = store.GetByTenant("tenant-1")
	if err == nil {
		t.Fatal("expected error for tenant after subscription deletion, got nil")
	}

	t.Run("returns error for non-existent subscription", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSubscriptionStore_List(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	store.Create(&Subscription{TenantID: "tenant-b", Plan: PlanSaaS, PlanName: "SaaS"})
	store.Create(&Subscription{TenantID: "tenant-a", Plan: PlanEnterprise, PlanName: "Enterprise"})

	subs := store.List()
	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(subs))
	}
	// Should be sorted by TenantID
	if subs[0].TenantID != "tenant-a" {
		t.Errorf("expected sorted by tenant ID, first is %q", subs[0].TenantID)
	}
}

func TestSubscriptionStore_CountTotal(t *testing.T) {
	store := NewSubscriptionStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	if store.CountTotal() != 0 {
		t.Error("expected 0 subscriptions initially")
	}

	store.Create(&Subscription{TenantID: "tenant-1", Plan: PlanSaaS, PlanName: "SaaS"})
	store.Create(&Subscription{TenantID: "tenant-2", Plan: PlanEnterprise, PlanName: "Enterprise"})

	if store.CountTotal() != 2 {
		t.Errorf("expected 2 subscriptions, got %d", store.CountTotal())
	}
}
