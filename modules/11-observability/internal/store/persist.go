package store

import "encoding/json"

// Export/Import implement the persist.Store contract: full-state JSON
// snapshots used for restart persistence. Indexes are rebuilt on import.

// ─── MetricStore ─────────────────────────────────────────────────────────────

// Export dumps all metrics as JSON.
func (s *MetricStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Metric, 0, len(s.metrics))
	for _, m := range s.metrics {
		items = append(items, *m)
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *MetricStore) Import(data []byte) error {
	var items []Metric
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = make(map[string]*Metric, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		m := items[i]
		s.metrics[m.ID] = &m
		s.byTenant[m.TenantID] = append(s.byTenant[m.TenantID], m.ID)
	}
	return nil
}

// ─── SpanStore ───────────────────────────────────────────────────────────────

// Export dumps all spans as JSON.
func (s *SpanStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]TraceSpan, 0, len(s.spans))
	for _, sp := range s.spans {
		items = append(items, *sp)
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *SpanStore) Import(data []byte) error {
	var items []TraceSpan
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans = make(map[string]*TraceSpan, len(items))
	s.byTenant = make(map[string][]string)
	s.byTrace = make(map[string][]string)
	for i := range items {
		sp := items[i]
		s.spans[sp.SpanID] = &sp
		s.byTenant[sp.TenantID] = append(s.byTenant[sp.TenantID], sp.SpanID)
		s.byTrace[sp.TraceID] = append(s.byTrace[sp.TraceID], sp.SpanID)
	}
	return nil
}

// ─── AlertStore ──────────────────────────────────────────────────────────────

// Export dumps all alerts as JSON.
func (s *AlertStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		items = append(items, *a)
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *AlertStore) Import(data []byte) error {
	var items []Alert
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = make(map[string]*Alert, len(items))
	s.byTenant = make(map[string][]string)
	for i := range items {
		a := items[i]
		s.alerts[a.ID] = &a
		s.byTenant[a.TenantID] = append(s.byTenant[a.TenantID], a.ID)
	}
	return nil
}

// ─── HealthStore ─────────────────────────────────────────────────────────────

// Export dumps all component health statuses as JSON.
func (s *HealthStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []HealthStatus
	for _, components := range s.components {
		for _, hs := range components {
			items = append(items, *hs)
		}
	}
	return json.Marshal(items)
}

// Import replaces the store contents from a JSON snapshot.
func (s *HealthStore) Import(data []byte) error {
	var items []HealthStatus
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.components = make(map[string]map[string]*HealthStatus)
	for i := range items {
		hs := items[i]
		if s.components[hs.TenantID] == nil {
			s.components[hs.TenantID] = make(map[string]*HealthStatus)
		}
		s.components[hs.TenantID][hs.ComponentID] = &hs
	}
	return nil
}
