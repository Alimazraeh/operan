// Package store implements the in-memory data store for tenant-control-plane module.
// All operations are protected with sync.RWMutex for thread safety.
// This is a reference implementation; production deployments should replace
// with a persistent backing store (e.g., PostgreSQL).
package store

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Domain types ────────────────────────────────────────────────────────────

// TenantStatus represents the lifecycle state of a tenant.
type TenantStatus string

const (
	TenantStatusProvisioning   TenantStatus = "provisioning"
	TenantStatusActive         TenantStatus = "active"
	TenantStatusSuspended      TenantStatus = "suspended"
	TenantStatusDeprovisioning TenantStatus = "deprovisioning"
	TenantStatusDeprovisioned  TenantStatus = "deprovisioned"
)

// Plan represents a subscription plan tier.
type Plan string

const (
	PlanSaaS      Plan = "saas"
	PlanEnterprise Plan = "enterprise"
	PlanSovereign  Plan = "sovereign"
)

// Region represents a cloud deployment region.
type Region string

const (
	RegionMEAST1 Region = "me-east-1"
	RegionEUWest1 Region = "eu-west-1"
	RegionUSEAST1 Region = "us-east-1"
	RegionAPS1   Region = "ap-south-1"
	RegionOnPrem Region = "on-prem"
)

// IsolationLevel represents tenant isolation mode.
type IsolationLevel string

const (
	IsolationNamespace  IsolationLevel = "namespace"
	IsolationEncryption IsolationLevel = "encryption"
	IsolationNetwork    IsolationLevel = "network_policy"
)

// QuotaConfig defines resource limits for a tenant.
type QuotaConfig struct {
	MaxAgents             int `json:"max_agents" validate:"gte=0"`
	MaxWorkflowsPerDay    int `json:"max_workflows_per_day" validate:"gte=0"`
	MaxStorageGB          int `json:"max_storage_gb" validate:"gte=0"`
	MaxMonthlyTokens      int `json:"max_monthly_tokens" validate:"gte=0"`
	MaxConcurrentWorkflows int `json:"max_concurrent_workflows" validate:"gte=0"`
}

// Tenant represents a tenant in the system.
type Tenant struct {
	ID              string        `json:"id"`
	Name            string        `json:"name" validate:"required,min=1,max=128"`
	DisplayName     string        `json:"display_name,omitempty"`
	Plan            Plan          `json:"plan" validate:"required"`
	Region          Region        `json:"region" validate:"required"`
	IsolationLevel  IsolationLevel `json:"isolation_level" validate:"required"`
	Status          TenantStatus  `json:"status"`
	Quota           QuotaConfig   `json:"quota"`
	ContactEmail    string        `json:"contact_email,omitempty"`
	CustomMetadata  map[string]interface{} `json:"custom_metadata,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// TenantPatchRequest represents a partial update to a tenant.
type TenantPatchRequest struct {
	Name            string              `json:"name,omitempty" validate:"omitempty,min=1,max=128"`
	DisplayName     string              `json:"display_name,omitempty"`
	Status          TenantStatus        `json:"status,omitempty"`
	Plan            Plan                `json:"plan,omitempty"`
	Region          Region              `json:"region,omitempty"`
	IsolationLevel  IsolationLevel      `json:"isolation_level,omitempty"`
	ContactEmail    string              `json:"contact_email,omitempty" validate:"omitempty,email"`
	CustomMetadata  map[string]interface{} `json:"custom_metadata,omitempty"`
	Quota           *QuotaConfig        `json:"quota,omitempty"`
}

// ─── Valid transitions ───────────────────────────────────────────────────────

var validTransitions = map[TenantStatus][]TenantStatus{
	TenantStatusProvisioning:   {TenantStatusActive, TenantStatusDeprovisioning},
	TenantStatusActive:         {TenantStatusSuspended, TenantStatusDeprovisioning},
	TenantStatusSuspended:      {TenantStatusActive, TenantStatusDeprovisioning},
	TenantStatusDeprovisioning: {TenantStatusDeprovisioned},
	TenantStatusDeprovisioned:  {},
}

// AllowedTransitions returns the list of valid next statuses for the given status.
func AllowedTransitions(from TenantStatus) []TenantStatus {
	return validTransitions[from]
}

func canTransition(from, to TenantStatus) bool {
	targets := validTransitions[from]
	return slices.Contains(targets, to)
}

// PlanDefaults returns default quotas for a plan tier.
func PlanDefaults(p Plan) QuotaConfig {
	switch p {
	case PlanSaaS:
		return QuotaConfig{MaxAgents: 5, MaxWorkflowsPerDay: 1000, MaxStorageGB: 10, MaxMonthlyTokens: 10000000, MaxConcurrentWorkflows: 2}
	case PlanEnterprise:
		return QuotaConfig{MaxAgents: 50, MaxWorkflowsPerDay: 10000, MaxStorageGB: 500, MaxMonthlyTokens: 100000000, MaxConcurrentWorkflows: 20}
	case PlanSovereign:
		return QuotaConfig{MaxAgents: 200, MaxWorkflowsPerDay: 100000, MaxStorageGB: 2000, MaxMonthlyTokens: 1000000000, MaxConcurrentWorkflows: 100}
	default:
		return QuotaConfig{MaxAgents: 5, MaxWorkflowsPerDay: 1000, MaxStorageGB: 10, MaxMonthlyTokens: 10000000, MaxConcurrentWorkflows: 2}
	}
}

// ─── TenantStore ─────────────────────────────────────────────────────────────

// TenantStore provides CRUD operations on tenant data.
type TenantStore struct {
	mu      sync.RWMutex
	tenants map[string]*Tenant // keyed by ID
	byName  map[string]string  // keyed by Name (DNS-safe lowercase)
}

// NewTenantStore creates a new TenantStore.
func NewTenantStore() *TenantStore {
	return &TenantStore{
		tenants: make(map[string]*Tenant),
		byName:  make(map[string]string),
	}
}

// Create adds a new tenant. Returns the created tenant or an error.
func (s *TenantStore) Create(t *Tenant) (*Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Status == "" {
		t.Status = TenantStatusProvisioning
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = timeNow()
	}
	t.UpdatedAt = timeNow()

	if _, exists := s.tenants[t.ID]; exists {
		return nil, fmt.Errorf("tenant %s already exists", t.ID)
	}
	if t.Name != "" {
		key := normalizeName(t.Name)
		if _, byNameExists := s.byName[key]; byNameExists {
			return nil, fmt.Errorf("tenant name %q already exists", t.Name)
		}
		s.byName[key] = t.ID
	}

	s.tenants[t.ID] = t
	return t, nil
}

// GetByID retrieves a tenant by its ID (admin lookup, no tenant check).
func (s *TenantStore) GetByID(id string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", id)
	}
	cpy := *t
	return &cpy, nil
}

// GetByIDAndTenant retrieves a tenant by its ID.
// Tenants are root-level entities, so tenantID is accepted but not validated.
func (s *TenantStore) GetByIDAndTenant(id, tenantID string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", id)
	}
	// Return a copy
	cpy := *t
	return &cpy, nil
}

// Patch updates fields of an existing tenant.
func (s *TenantStore) Patch(id string, req TenantPatchRequest) (*Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", id)
	}

	if req.Name != "" {
		t.Name = req.Name
	}
	if req.DisplayName != "" {
		t.DisplayName = req.DisplayName
	}
	if req.Status != "" {
		if !canTransition(t.Status, req.Status) {
			return nil, fmt.Errorf("invalid status transition from %s to %s", t.Status, req.Status)
		}
		t.Status = req.Status
	}
	if req.Plan != "" {
		t.Plan = req.Plan
	}
	if req.Region != "" {
		t.Region = req.Region
	}
	if req.IsolationLevel != "" {
		t.IsolationLevel = req.IsolationLevel
	}
	if req.ContactEmail != "" {
		t.ContactEmail = req.ContactEmail
	}
	if req.CustomMetadata != nil {
		t.CustomMetadata = req.CustomMetadata
	}
	if req.Quota != nil {
		if req.Quota.MaxAgents > 0 {
			t.Quota.MaxAgents = req.Quota.MaxAgents
		}
		if req.Quota.MaxWorkflowsPerDay > 0 {
			t.Quota.MaxWorkflowsPerDay = req.Quota.MaxWorkflowsPerDay
		}
		if req.Quota.MaxStorageGB > 0 {
			t.Quota.MaxStorageGB = req.Quota.MaxStorageGB
		}
		if req.Quota.MaxMonthlyTokens > 0 {
			t.Quota.MaxMonthlyTokens = req.Quota.MaxMonthlyTokens
		}
		if req.Quota.MaxConcurrentWorkflows > 0 {
			t.Quota.MaxConcurrentWorkflows = req.Quota.MaxConcurrentWorkflows
		}
	}
	t.UpdatedAt = timeNow()

	return t, nil
}

// Delete removes a tenant. Returns nil on success or error if not found.
func (s *TenantStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.tenants[id]
	if !ok {
		return fmt.Errorf("tenant %s not found", id)
	}
	delete(s.tenants, id)
	// Also clean up byName lookup
	for name, tid := range s.byName {
		if tid == id {
			delete(s.byName, name)
			break
		}
	}
	return nil
}

// List returns a paginated list of tenants, optionally filtered by status.
func (s *TenantStore) List(page, pageSize int, status *string) ([]*Tenant, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		if status != nil && t.Status != TenantStatus(*status) {
			continue
		}
		cpy := *t
		all = append(all, &cpy)
	}

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	slices.SortFunc(all, func(a, b *Tenant) int {
		return a.NameCompare(b)
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

// CountTotal returns the total number of tenants (ignoring filters).
func (s *TenantStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tenants)
}

// NameCompare is a helper to sort tenants alphabetically by name.
func (t *Tenant) NameCompare(other *Tenant) int {
	if t.Name < other.Name {
		return -1
	}
	if t.Name > other.Name {
		return 1
	}
	return 0
}

func normalizeName(name string) string {
	return name
}

// timeNow returns the current UTC time. Overridable for testing.
var timeNow = time.Now
