package store

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PaymentMethodType represents the type of payment method.
type PaymentMethodType string

const (
	PaymentMethodCreditCard PaymentMethodType = "credit_card"
	PaymentMethodBankTransfer PaymentMethodType = "bank_transfer"
	PaymentMethodWire       PaymentMethodType = "wire"
)

// PaymentMethod represents a tenant's payment method.
type PaymentMethod struct {
	ID             string              `json:"id"`
	TenantID       string              `json:"tenant_id"`
	Type           PaymentMethodType   `json:"type"`
	LastFour       string              `json:"last_four"`
	ExpiryMonth    int                 `json:"expiry_month,omitempty"`
	ExpiryYear     int                 `json:"expiry_year,omitempty"`
	BillingAddress string              `json:"billing_address,omitempty"`
	IsDefault      bool                `json:"is_default"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

// PaymentMethodStore manages payment methods.
type PaymentMethodStore struct {
	mu         sync.RWMutex
	methods    map[string]*PaymentMethod
	byTenant   map[string][]string // TenantID -> PaymentMethodIDs
}

// NewPaymentMethodStore creates a new PaymentMethodStore.
func NewPaymentMethodStore() *PaymentMethodStore {
	return &PaymentMethodStore{
		methods:  make(map[string]*PaymentMethod),
		byTenant: make(map[string][]string),
	}
}

// Create adds a new payment method.
func (s *PaymentMethodStore) Create(pm *PaymentMethod) (*PaymentMethod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pm.ID == "" {
		pm.ID = uuid.New().String()
	}
	if pm.CreatedAt.IsZero() {
		pm.CreatedAt = timeNow()
	}
	pm.UpdatedAt = timeNow()

	if pm.TenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}

	// If this is set as default, clear other defaults for this tenant
	if pm.IsDefault {
		for _, existing := range s.methods {
			if existing.TenantID == pm.TenantID {
				existing.IsDefault = false
				existing.UpdatedAt = timeNow()
			}
		}
	}

	s.methods[pm.ID] = pm
	s.byTenant[pm.TenantID] = append(s.byTenant[pm.TenantID], pm.ID)

	return pm, nil
}

// GetByID retrieves a payment method by ID (no tenant check — for admin use only).
func (s *PaymentMethodStore) GetByID(id string) (*PaymentMethod, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pm, ok := s.methods[id]
	if !ok {
		return nil, fmt.Errorf("payment method %s not found", id)
	}
	cpy := *pm
	return &cpy, nil
}

// GetByIDAndTenant retrieves a payment method by ID and verifies the TenantID matches.
func (s *PaymentMethodStore) GetByIDAndTenant(id, tenantID string) (*PaymentMethod, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pm, ok := s.methods[id]
	if !ok {
		return nil, fmt.Errorf("payment method %s not found", id)
	}
	if pm.TenantID != tenantID {
		return nil, fmt.Errorf("permission denied: payment method %s does not belong to tenant %s", id, tenantID)
	}
	cpy := *pm
	return &cpy, nil
}

// Update updates a payment method.
func (s *PaymentMethodStore) Update(pm *PaymentMethod) (*PaymentMethod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.methods[pm.ID]
	if !ok {
		return nil, fmt.Errorf("payment method %s not found", pm.ID)
	}

	pm.UpdatedAt = timeNow()
	s.methods[pm.ID] = pm

	cpy := *pm
	return &cpy, nil
}

// List returns a paginated list of payment methods for all tenants.
func (s *PaymentMethodStore) List(page, pageSize int) ([]*PaymentMethod, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*PaymentMethod, 0, len(s.methods))
	for _, pm := range s.methods {
		cpy := *pm
		all = append(all, &cpy)
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	slices.SortFunc(all, func(a, b *PaymentMethod) int {
		if a.TenantID < b.TenantID {
			return -1
		}
		if a.TenantID > b.TenantID {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pageItems := all[start:end]
	hasMore := end < total

	return pageItems, total, hasMore
}

// GetByTenant returns all payment methods for a tenant.
func (s *PaymentMethodStore) GetByTenant(tenantID string) ([]*PaymentMethod, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, nil
	}

	result := make([]*PaymentMethod, 0, len(ids))
	for _, id := range ids {
		pm, ok := s.methods[id]
		if !ok {
			continue
		}
		cpy := *pm
		result = append(result, &cpy)
	}

	return result, nil
}

// Delete removes a payment method.
func (s *PaymentMethodStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pm, ok := s.methods[id]
	if !ok {
		return fmt.Errorf("payment method %s not found", id)
	}

	delete(s.methods, id)
	for i, existingID := range s.byTenant[pm.TenantID] {
		if existingID == id {
			s.byTenant[pm.TenantID] = append(s.byTenant[pm.TenantID][:i], s.byTenant[pm.TenantID][i+1:]...)
			break
		}
	}

	return nil
}

// CountTotal returns the total number of payment methods.
func (s *PaymentMethodStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.methods)
}
