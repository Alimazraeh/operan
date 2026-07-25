package store

import (
	"sort"
	"sync"
	"time"
)

// Provider is a system that can perform capabilities for a tenant: an MCP
// server, a native connector, a declarative HTTP target — or the simulated
// provider, which answers realistically without touching anything and is
// flagged as such on every record it produces.
type Provider struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	// Kind: simulated | mcp | native | http. Only simulated executes today;
	// the others are registered shapes for the integration roadmap.
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint,omitempty"`
	// CredentialRef names a secret held elsewhere. Credentials never live on
	// this record, never appear in responses, and the simulated provider needs
	// none.
	CredentialRef string    `json:"credential_ref,omitempty"`
	Status        string    `json:"status"` // active | disabled
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CapabilityBinding is the customer-specific join: which provider performs a
// capability for this tenant (or, overriding that, for one department). The
// SOP stays portable; this row is what changes between customers — and
// swapping simulated for live is a binding change, not a code change.
type CapabilityBinding struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	DepartmentID string `json:"department_id,omitempty"` // empty = tenant default
	CapabilityID string `json:"capability_id"`
	ProviderID   string `json:"provider_id"`
	// ProviderTool names the provider-side operation. For the simulated
	// provider it equals the capability id.
	ProviderTool string    `json:"provider_tool"`
	Enabled      bool      `json:"enabled"`
	Simulated    bool      `json:"simulated"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ─── Stores ─────────────────────────────────────────────────────────────────

type ProviderStore struct {
	mu   sync.RWMutex
	byID map[string]*Provider
	sink *durabilitySink
}

func NewProviderStore() *ProviderStore { return &ProviderStore{byID: map[string]*Provider{}} }

func (s *ProviderStore) Put(p *Provider) {
	s.mu.Lock()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = timeNow()
	}
	p.UpdatedAt = timeNow()
	s.byID[p.ID] = p
	s.mu.Unlock()
	s.saveProvider(p)
}

func (s *ProviderStore) Get(tenantID, id string) (*Provider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.byID[id]
	if !ok || p.TenantID != tenantID {
		return nil, false
	}
	return p, true
}

func (s *ProviderStore) List(tenantID string) []*Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Provider
	for _, p := range s.byID {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type BindingStore struct {
	mu   sync.RWMutex
	byID map[string]*CapabilityBinding
	sink *durabilitySink
}

func NewBindingStore() *BindingStore { return &BindingStore{byID: map[string]*CapabilityBinding{}} }

func (s *BindingStore) Put(b *CapabilityBinding) {
	s.mu.Lock()
	if b.CreatedAt.IsZero() {
		b.CreatedAt = timeNow()
	}
	b.UpdatedAt = timeNow()
	s.byID[b.ID] = b
	s.mu.Unlock()
	s.saveBinding(b)
}

func (s *BindingStore) Get(tenantID, id string) (*CapabilityBinding, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.byID[id]
	if !ok || b.TenantID != tenantID {
		return nil, false
	}
	return b, true
}

func (s *BindingStore) List(tenantID string) []*CapabilityBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*CapabilityBinding
	for _, b := range s.byID {
		if b.TenantID == tenantID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CapabilityID != out[j].CapabilityID {
			return out[i].CapabilityID < out[j].CapabilityID
		}
		return out[i].DepartmentID < out[j].DepartmentID
	})
	return out
}

// Resolve finds the binding in force for a capability: the department's own
// binding wins over the tenant default; disabled bindings are invisible. A nil
// return is the honest "unbound" answer the funnel turns into
// blocked_no_binding — never a silent pass-through, never an invented
// provider.
func (s *BindingStore) Resolve(tenantID, departmentID, capabilityID string) *CapabilityBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var tenantDefault *CapabilityBinding
	for _, b := range s.byID {
		if b.TenantID != tenantID || b.CapabilityID != capabilityID || !b.Enabled {
			continue
		}
		if b.DepartmentID != "" {
			if departmentID != "" && b.DepartmentID == departmentID {
				return b
			}
			continue
		}
		tenantDefault = b
	}
	return tenantDefault
}
