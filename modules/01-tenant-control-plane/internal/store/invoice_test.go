package store

import (
	"testing"
	"time"
)

func TestBillingStore_CreateInvoice(t *testing.T) {
	store := NewBillingStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	t.Run("creates invoice with defaults", func(t *testing.T) {
		inv := &Invoice{
			TenantID:       "tenant-1",
			SubscriptionID: "sub-1",
			Amount:         495.0,
			Currency:       "USD",
			LineItems: []InvoiceLineItem{
				{Description: "SaaS Plan", Quantity: 5, UnitPrice: 99.0, Amount: 495.0},
			},
		}

		created, err := store.CreateInvoice(inv)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
		if created.Status != BillingStatusPending {
			t.Errorf("expected pending status, got %q", created.Status)
		}
		if created.DueDate.IsZero() {
			t.Error("expected due date to be set (30 days from issue)")
		}
		if created.DueDateRaw == "" {
			t.Error("expected due_date_raw to be set")
		}
	})

	t.Run("creates invoice with custom dates", func(t *testing.T) {
		issueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		dueDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

		inv := &Invoice{
			TenantID:       "tenant-2",
			SubscriptionID: "sub-2",
			IssueDate:      issueDate,
			DueDate:        dueDate,
			Amount:         99.0,
			Currency:       "USD",
		}

		created, err := store.CreateInvoice(inv)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !created.IssueDate.Equal(issueDate) {
			t.Errorf("expected issue date %v, got %v", issueDate, created.IssueDate)
		}
		if !created.DueDate.Equal(dueDate) {
			t.Errorf("expected due date %v, got %v", dueDate, created.DueDate)
		}
	})

	t.Run("nil line items becomes empty slice", func(t *testing.T) {
		inv := &Invoice{
			TenantID:       "tenant-3",
			SubscriptionID: "sub-3",
			Amount:         99.0,
			LineItems:      nil,
		}

		created, err := store.CreateInvoice(inv)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if created.LineItems == nil {
			t.Error("expected empty line items slice, got nil")
		}
	})
}

func TestBillingStore_GetByID(t *testing.T) {
	store := NewBillingStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	inv := &Invoice{
		TenantID:       "tenant-1",
		SubscriptionID: "sub-1",
		Amount:         495.0,
	}

	created, err := store.CreateInvoice(inv)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("retrieves invoice by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.TenantID != "tenant-1" {
			t.Errorf("expected tenant-1, got %q", got.TenantID)
		}
		if got.Amount != 495.0 {
			t.Errorf("expected amount 495.0, got %f", got.Amount)
		}
	})

	t.Run("returns error for non-existent invoice", func(t *testing.T) {
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
		got.Status = BillingStatusPaid
		original, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if original.Status == BillingStatusPaid {
			t.Fatal("modifying returned copy should not affect stored invoice")
		}
	})
}

func TestBillingStore_Update(t *testing.T) {
	store := NewBillingStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	inv := &Invoice{
		TenantID:       "tenant-1",
		SubscriptionID: "sub-1",
		Amount:         495.0,
		Status:         BillingStatusPending,
	}

	created, err := store.CreateInvoice(inv)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	t.Run("marks invoice as paid", func(t *testing.T) {
		updated, err := store.Update(created.ID, InvoiceUpdateRequest{
			Status: BillingStatusPaid,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Status != BillingStatusPaid {
			t.Errorf("expected paid status, got %q", updated.Status)
		}
		if updated.PaidAt == nil {
			t.Error("expected paid_at to be set")
		}
	})

	t.Run("marks invoice as overdue", func(t *testing.T) {
		updated, err := store.Update(created.ID, InvoiceUpdateRequest{
			Status: BillingStatusOverdue,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if updated.Status != BillingStatusOverdue {
			t.Errorf("expected overdue status, got %q", updated.Status)
		}
	})

	t.Run("returns error for non-existent invoice", func(t *testing.T) {
		_, err := store.Update("non-existent", InvoiceUpdateRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestBillingStore_GetByTenant(t *testing.T) {
	store := NewBillingStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	inv1 := &Invoice{
		TenantID:    "tenant-1",
		Amount:      495.0,
		IssueDate:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		LineItems:   []InvoiceLineItem{{Description: "Invoice 1"}},
	}
	inv2 := &Invoice{
		TenantID:    "tenant-1",
		Amount:      99.0,
		IssueDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		LineItems:   []InvoiceLineItem{{Description: "Invoice 2"}},
	}

	store.CreateInvoice(inv1)
	store.CreateInvoice(inv2)

	t.Run("lists invoices for tenant", func(t *testing.T) {
		items, total, _ := store.GetByTenant("tenant-1", 1, 20)
		if total != 2 {
			t.Errorf("expected 2 invoices, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		// Should be sorted by issue date descending
		if !items[0].IssueDate.Equal(inv1.IssueDate) {
			t.Error("expected first invoice to be most recent")
		}
	})

	t.Run("returns empty for tenant without invoices", func(t *testing.T) {
		items, total, hasMore := store.GetByTenant("non-existent", 1, 20)
		if total != 0 {
			t.Errorf("expected 0 invoices, got %d", total)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
		if hasMore {
			t.Error("expected hasMore false")
		}
	})

	t.Run("paginates results", func(t *testing.T) {
		// Create more invoices
		for i := 0; i < 5; i++ {
			store.CreateInvoice(&Invoice{
				TenantID:    "tenant-2",
				Amount:      float64(100 * (i + 1)),
				IssueDate:   time.Now().AddDate(0, 0, -i),
				LineItems:   []InvoiceLineItem{{Description: "Invoice"}},
			})
		}

		items, total, hasMore := store.GetByTenant("tenant-2", 1, 2)
		if total != 5 {
			t.Errorf("expected total 5, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if !hasMore {
			t.Error("expected hasMore true")
		}
	})
}

func TestBillingStore_CountTotal(t *testing.T) {
	store := NewBillingStore()
	now := time.Now()
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	if store.CountTotal() != 0 {
		t.Error("expected 0 invoices initially")
	}

	store.CreateInvoice(&Invoice{TenantID: "tenant-1", Amount: 495.0, LineItems: []InvoiceLineItem{{Description: "Inv1"}}})
	store.CreateInvoice(&Invoice{TenantID: "tenant-1", Amount: 99.0, LineItems: []InvoiceLineItem{{Description: "Inv2"}}})
	store.CreateInvoice(&Invoice{TenantID: "tenant-2", Amount: 1999.0, LineItems: []InvoiceLineItem{{Description: "Inv3"}}})

	if store.CountTotal() != 3 {
		t.Errorf("expected 3 invoices, got %d", store.CountTotal())
	}
}
