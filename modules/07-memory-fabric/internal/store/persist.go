package store

import "encoding/json"

// Export/Import implement the persist.Store contract for each store:
// full-state JSON snapshots used for restart persistence. Tenant indexes
// are rebuilt on import.

// ─── VectorStore ─────────────────────────────────────────────────────────────

// Export dumps all vectors as JSON.
func (s *VectorStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]MemoryVector, 0, len(s.vectors))
	for _, v := range s.vectors {
		items = append(items, *v)
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *VectorStore) Import(data []byte) error {
	var items []MemoryVector
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vectors = make(map[string]*MemoryVector, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		v := items[i]
		s.vectors[v.ID] = &v
		s.byTenant[v.TenantID] = append(s.byTenant[v.TenantID], v.ID)
	}
	return nil
}

// ─── PolicyStore ─────────────────────────────────────────────────────────────

// Export dumps all retention policies as JSON.
func (s *PolicyStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]RetentionPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		items = append(items, *p)
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *PolicyStore) Import(data []byte) error {
	var items []RetentionPolicy
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies = make(map[string]*RetentionPolicy, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		p := items[i]
		s.policies[p.ID] = &p
		s.byTenant[p.TenantID] = append(s.byTenant[p.TenantID], p.ID)
	}
	return nil
}

// ─── OperationStore ──────────────────────────────────────────────────────────

// persistedOperation carries the tenant association OperationStatus lacks.
type persistedOperation struct {
	TenantID string          `json:"tenant_id"`
	Op       OperationStatus `json:"op"`
}

// Export dumps all GC operations as JSON.
func (s *OperationStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []persistedOperation
	for tenant, ids := range s.byTenant {
		for _, id := range ids {
			if op := s.ops[id]; op != nil {
				items = append(items, persistedOperation{TenantID: tenant, Op: *op})
			}
		}
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *OperationStore) Import(data []byte) error {
	var items []persistedOperation
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ops = make(map[string]*OperationStatus, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		op := items[i].Op
		s.ops[op.ID] = &op
		s.byTenant[items[i].TenantID] = append(s.byTenant[items[i].TenantID], op.ID)
	}
	return nil
}
