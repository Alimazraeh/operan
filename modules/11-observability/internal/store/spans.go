package store

import (
	"sort"
	"sync"
)

// SpanStore provides tenant-isolated storage for trace spans.
type SpanStore struct {
	mu       sync.RWMutex
	spans    map[string]*TraceSpan // spanID -> span
	byTenant map[string][]string   // tenantID -> []spanID
	byTrace  map[string][]string   // traceID -> []spanID
}

// NewSpanStore creates an empty SpanStore.
func NewSpanStore() *SpanStore {
	return &SpanStore{
		spans:    make(map[string]*TraceSpan),
		byTenant: make(map[string][]string),
		byTrace:  make(map[string][]string),
	}
}

// Add stores a span, validating required fields.
func (s *SpanStore) Add(sp *TraceSpan) (*TraceSpan, error) {
	if sp.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if sp.TraceID == "" || sp.SpanID == "" || sp.SpanName == "" || !ValidSpanType(string(sp.SpanType)) {
		return nil, ErrValidation
	}
	if sp.Status != "" && !ValidSpanStatus(string(sp.Status)) {
		return nil, ErrValidation
	}
	if sp.StartTime.IsZero() {
		sp.StartTime = timeNow()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.spans[sp.SpanID] = sp
	s.byTenant[sp.TenantID] = append(s.byTenant[sp.TenantID], sp.SpanID)
	s.byTrace[sp.TraceID] = append(s.byTrace[sp.TraceID], sp.SpanID)
	cp := *sp
	return &cp, nil
}

// SpanFilter narrows List results (nil = no filter).
type SpanFilter struct {
	TraceID    *string
	SpanType   *string
	WorkflowID *string
	AgentID    *string
	Status     *string
}

// List returns a tenant's spans, filtered and paginated, newest first.
func (s *SpanStore) List(tenantID string, page, pageSize int, f SpanFilter) ([]TraceSpan, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*TraceSpan
	for _, id := range s.byTenant[tenantID] {
		sp := s.spans[id]
		if sp == nil {
			continue
		}
		if f.TraceID != nil && sp.TraceID != *f.TraceID {
			continue
		}
		if f.SpanType != nil && string(sp.SpanType) != *f.SpanType {
			continue
		}
		if f.WorkflowID != nil && sp.WorkflowID != *f.WorkflowID {
			continue
		}
		if f.AgentID != nil && sp.AgentID != *f.AgentID {
			continue
		}
		if f.Status != nil && string(sp.Status) != *f.Status {
			continue
		}
		matched = append(matched, sp)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].StartTime.After(matched[j].StartTime) })

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]TraceSpan, 0, end-start)
	for _, sp := range matched[start:end] {
		out = append(out, *sp)
	}
	return out, total, end < total
}

// Trace returns all of a tenant's spans for one trace, ordered by start
// time, with the total duration. ErrNotFound when the trace has no spans
// visible to the tenant.
func (s *SpanStore) Trace(traceID, tenantID string) ([]TraceSpan, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var spans []TraceSpan
	for _, id := range s.byTrace[traceID] {
		sp := s.spans[id]
		if sp == nil || sp.TenantID != tenantID {
			continue
		}
		spans = append(spans, *sp)
	}
	if len(spans) == 0 {
		return nil, 0, ErrNotFound
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].StartTime.Before(spans[j].StartTime) })

	total := 0
	for _, sp := range spans {
		total += sp.DurationMs
	}
	return spans, total, nil
}
