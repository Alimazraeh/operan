package store

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Namespace types ─────────────────────────────────────────────────────────

// NamespaceStatus represents a namespace lifecycle state.
type NamespaceStatus string

const (
	NamespaceStatusProvisioning NamespaceStatus = "provisioning"
	NamespaceStatusActive       NamespaceStatus = "active"
	NamespaceStatusError        NamespaceStatus = "error"
	NamespaceStatusDeprovisioned NamespaceStatus = "deprovisioned"
)

// Namespace represents an isolated namespace within a tenant.
type Namespace struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Status       NamespaceStatus   `json:"status"`
	Config       NamespaceConfig   `json:"config"`
	ResourceQuota *NamespaceQuota  `json:"resource_quota,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// NamespaceConfig defines isolation settings for a namespace.
type NamespaceConfig struct {
	NetworkPolicy  string                 `json:"network_policy"`
	IsolationLevel IsolationLevel         `json:"isolation_level"`
	Tags           map[string]string      `json:"tags,omitempty"`
	ExtraConfig    map[string]interface{} `json:"extra_config,omitempty"`
}

// NamespaceQuota defines resource limits for a namespace.
type NamespaceQuota struct {
	MaxAgents            int `json:"max_agents"`
	MaxStorageGB         int `json:"max_storage_gb"`
	MaxConcurrentWorkflows int `json:"max_concurrent_workflows"`
}

// NamespacePatchRequest for partial updates.
type NamespacePatchRequest struct {
	Description  string            `json:"description,omitempty"`
	Config       NamespaceConfig   `json:"config,omitempty"`
	ResourceQuota *NamespaceQuota  `json:"resource_quota,omitempty"`
	Status       NamespaceStatus   `json:"status,omitempty"`
}

// ─── NamespaceStore ──────────────────────────────────────────────────────────

// NamespaceStore manages tenant namespaces.
type NamespaceStore struct {
	mu         sync.RWMutex
	namespaces map[string]*Namespace
	byTenant   map[string][]string // keyed by TenantID -> NamespaceIDs
	byName     map[string]string   // keyed by "tenant_id::name"
}

// NewNamespaceStore creates a new NamespaceStore.
func NewNamespaceStore() *NamespaceStore {
	return &NamespaceStore{
		namespaces: make(map[string]*Namespace),
		byTenant:   make(map[string][]string),
		byName:     make(map[string]string),
	}
}

// Create adds a new namespace.
func (s *NamespaceStore) Create(ns *Namespace) (*Namespace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ns.ID == "" {
		ns.ID = uuid.New().String()
	}
	if ns.Status == "" {
		ns.Status = NamespaceStatusProvisioning
	}
	if ns.CreatedAt.IsZero() {
		ns.CreatedAt = timeNow()
	}
	ns.UpdatedAt = timeNow()

	if ns.Config.IsolationLevel == "" {
		ns.Config.IsolationLevel = IsolationNamespace
	}
	if ns.Config.NetworkPolicy == "" {
		ns.Config.NetworkPolicy = "deny-all"
	}

	key := fmt.Sprintf("%s::%s", ns.TenantID, ns.Name)
	if _, exists := s.byName[key]; exists {
		return nil, fmt.Errorf("namespace %q already exists for tenant %s", ns.Name, ns.TenantID)
	}

	s.namespaces[ns.ID] = ns
	s.byTenant[ns.TenantID] = append(s.byTenant[ns.TenantID], ns.ID)
	s.byName[key] = ns.ID

	return ns, nil
}

// GetByID retrieves a namespace by ID (no tenant check — for admin use only).
func (s *NamespaceStore) GetByID(id string) (*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.namespaces[id]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", id)
	}
	cpy := *ns
	return &cpy, nil
}

// GetByIDAndTenant retrieves a namespace by ID and verifies the TenantID matches.
func (s *NamespaceStore) GetByIDAndTenant(id, tenantID string) (*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.namespaces[id]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", id)
	}
	if ns.TenantID != tenantID {
		return nil, fmt.Errorf("permission denied: namespace %s does not belong to tenant %s", id, tenantID)
	}
	cpy := *ns
	return &cpy, nil
}

// GetByName retrieves a namespace by name within a tenant.
func (s *NamespaceStore) GetByName(tenantID, name string) (*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s::%s", tenantID, name)
	id, ok := s.byName[key]
	if !ok {
		return nil, fmt.Errorf("namespace %q not found for tenant %s", name, tenantID)
	}

	ns, ok := s.namespaces[id]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", id)
	}
	cpy := *ns
	return &cpy, nil
}

// Patch updates a namespace.
func (s *NamespaceStore) Patch(id string, req NamespacePatchRequest) (*Namespace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.namespaces[id]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found", id)
	}

	if req.Description != "" {
		ns.Description = req.Description
	}
	if req.Status != "" {
		ns.Status = req.Status
	}
	if req.Config.NetworkPolicy != "" {
		ns.Config.NetworkPolicy = req.Config.NetworkPolicy
	}
	if req.Config.IsolationLevel != "" {
		ns.Config.IsolationLevel = req.Config.IsolationLevel
	}
	if req.Config.Tags != nil {
		if ns.Config.Tags == nil {
			ns.Config.Tags = make(map[string]string)
		}
		for k, v := range req.Config.Tags {
			ns.Config.Tags[k] = v
		}
	}
	if req.Config.ExtraConfig != nil {
		if ns.Config.ExtraConfig == nil {
			ns.Config.ExtraConfig = make(map[string]interface{})
		}
		for k, v := range req.Config.ExtraConfig {
			ns.Config.ExtraConfig[k] = v
		}
	}
	if req.ResourceQuota != nil {
		if req.ResourceQuota.MaxAgents > 0 {
			ns.ResourceQuota.MaxAgents = req.ResourceQuota.MaxAgents
		}
		if req.ResourceQuota.MaxStorageGB > 0 {
			ns.ResourceQuota.MaxStorageGB = req.ResourceQuota.MaxStorageGB
		}
		if req.ResourceQuota.MaxConcurrentWorkflows > 0 {
			ns.ResourceQuota.MaxConcurrentWorkflows = req.ResourceQuota.MaxConcurrentWorkflows
		}
	}

	ns.UpdatedAt = timeNow()

	return ns, nil
}

// Delete removes a namespace.
func (s *NamespaceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.namespaces[id]
	if !ok {
		return fmt.Errorf("namespace %s not found", id)
	}

	// Remove from byTenant
	tenantIDs, ok := s.byTenant[ns.TenantID]
	if ok {
		idx := slices.Index(tenantIDs, id)
		if idx >= 0 {
			s.byTenant[ns.TenantID] = append(tenantIDs[:idx], tenantIDs[idx+1:]...)
		}
	}

	// Remove from byName
	key := fmt.Sprintf("%s::%s", ns.TenantID, ns.Name)
	delete(s.byName, key)

	delete(s.namespaces, id)

	return nil
}

// ListByTenant returns all namespaces for a tenant.
func (s *NamespaceStore) ListByTenant(tenantID string, page, pageSize int) ([]*Namespace, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Namespace, 0, len(ids))
	for _, id := range ids {
		ns, ok := s.namespaces[id]
		if !ok {
			continue
		}
		cpy := *ns
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Namespace) int {
		return strings.Compare(a.Name, b.Name)
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

// CountTotal returns the total number of namespaces.
func (s *NamespaceStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.namespaces)
}

// StatusTransitions returns allowed status transitions for a namespace.
func StatusTransitions(from NamespaceStatus) []NamespaceStatus {
	switch from {
	case NamespaceStatusProvisioning:
		return []NamespaceStatus{NamespaceStatusActive, NamespaceStatusError}
	case NamespaceStatusActive:
		return []NamespaceStatus{NamespaceStatusDeprovisioned, NamespaceStatusError}
	case NamespaceStatusError:
		return []NamespaceStatus{NamespaceStatusProvisioning, NamespaceStatusDeprovisioned}
	case NamespaceStatusDeprovisioned:
		return nil
	default:
		return nil
	}
}
