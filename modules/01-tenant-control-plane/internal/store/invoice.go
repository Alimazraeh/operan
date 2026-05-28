package store

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BillingStatus represents invoice payment status.
type BillingStatus string

const (
	BillingStatusPending   BillingStatus = "pending"
	BillingStatusPaid      BillingStatus = "paid"
	BillingStatusOverdue   BillingStatus = "overdue"
	BillingStatusCancelled BillingStatus = "cancelled"
)

// InvoiceLineItem represents a single line item on an invoice.
type InvoiceLineItem struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
}

// Invoice represents a billing invoice for a tenant.
type Invoice struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	SubscriptionID string           `json:"subscription_id"`
	IssueDate     time.Time         `json:"issue_date"`
	DueDate       time.Time         `json:"due_date"`
	DueDateRaw    string            `json:"due_date_raw"` // Raw format per contract
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	Status        BillingStatus     `json:"status"`
	LineItems     []InvoiceLineItem `json:"line_items"`
	PaidAt        *time.Time        `json:"paid_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// InvoiceUpdateRequest for updating invoice status.
type InvoiceUpdateRequest struct {
	Status BillingStatus `json:"status"`
}

// BillingStore manages invoices and billing data.
type BillingStore struct {
	mu         sync.RWMutex
	invoices   map[string]*Invoice
	byTenant   map[string][]string // keyed by TenantID -> InvoiceIDs
	bySubscription map[string]string // SubscriptionID -> InvoiceID
}

// NewBillingStore creates a new BillingStore.
func NewBillingStore() *BillingStore {
	return &BillingStore{
		invoices:     make(map[string]*Invoice),
		byTenant:     make(map[string][]string),
		bySubscription: make(map[string]string),
	}
}

// CreateInvoice creates a new invoice for a tenant subscription.
func (s *BillingStore) CreateInvoice(inv *Invoice) (*Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = timeNow()
	}
	inv.UpdatedAt = timeNow()

	if inv.Status == "" {
		inv.Status = BillingStatusPending
	}
	if inv.Currency == "" {
		inv.Currency = "USD"
	}
	if inv.IssueDate.IsZero() {
		inv.IssueDate = timeNow()
	}

	// Set due date (30 days from issue)
	if inv.DueDate.IsZero() {
		inv.DueDate = inv.IssueDate.AddDate(0, 0, 30)
		inv.DueDateRaw = inv.DueDate.Format(time.RFC3339)
	}

	if inv.LineItems == nil {
		inv.LineItems = []InvoiceLineItem{}
	}

	s.invoices[inv.ID] = inv
	s.byTenant[inv.TenantID] = append(s.byTenant[inv.TenantID], inv.ID)
	if inv.SubscriptionID != "" {
		s.bySubscription[inv.SubscriptionID] = inv.ID
	}

	return inv, nil
}

// GetByID retrieves an invoice by ID.
func (s *BillingStore) GetByID(id string) (*Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inv, ok := s.invoices[id]
	if !ok {
		return nil, fmt.Errorf("invoice %s not found", id)
	}
	cpy := *inv
	return &cpy, nil
}

// Update updates an invoice (e.g., payment confirmation).
func (s *BillingStore) Update(id string, req InvoiceUpdateRequest) (*Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inv, ok := s.invoices[id]
	if !ok {
		return nil, fmt.Errorf("invoice %s not found", id)
	}

	inv.Status = req.Status
	if req.Status == BillingStatusPaid {
		now := timeNow()
		inv.PaidAt = &now
	}
	inv.UpdatedAt = timeNow()

	return inv, nil
}

// GetByTenant returns all invoices for a tenant, sorted by date.
func (s *BillingStore) GetByTenant(tenantID string, page, pageSize int) ([]*Invoice, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Invoice, 0, len(ids))
	for _, id := range ids {
		inv, ok := s.invoices[id]
		if !ok {
			continue
		}
		cpy := *inv
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Invoice) int {
		return b.IssueDate.Compare(a.IssueDate)
	})

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := items[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// CountTotal returns the total number of invoices.
func (s *BillingStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.invoices)
}
