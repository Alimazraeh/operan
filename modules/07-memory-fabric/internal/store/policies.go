package store

import (
	"sort"
	"sync"

	"github.com/google/uuid"
)

// PolicyStore provides tenant-isolated storage for retention policies.
type PolicyStore struct {
	mu       sync.RWMutex
	policies map[string]*RetentionPolicy
	byTenant map[string][]string
}

// NewPolicyStore creates an empty PolicyStore.
func NewPolicyStore() *PolicyStore {
	return &PolicyStore{
		policies: make(map[string]*RetentionPolicy),
		byTenant: make(map[string][]string),
	}
}

// Create stores a new retention policy, generating ID and creation date.
func (s *PolicyStore) Create(p *RetentionPolicy) (*RetentionPolicy, error) {
	if p.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if !ValidMemoryType(string(p.MemoryType)) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	p.CreationDate = timeNow()

	s.policies[p.ID] = p
	s.byTenant[p.TenantID] = append(s.byTenant[p.TenantID], p.ID)
	cp := *p
	return &cp, nil
}

// List returns a tenant's policies, paginated, oldest first.
func (s *PolicyStore) List(tenantID string, page, pageSize int) ([]RetentionPolicy, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.byTenant[tenantID]
	var matched []*RetentionPolicy
	for _, id := range ids {
		if p := s.policies[id]; p != nil {
			matched = append(matched, p)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].CreationDate.Before(matched[j].CreationDate) })

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	out := make([]RetentionPolicy, 0, end-start)
	for _, p := range matched[start:end] {
		out = append(out, *p)
	}
	return out, total, end < total
}
