package store

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MetricStore provides tenant-isolated storage for metrics.
type MetricStore struct {
	mu       sync.RWMutex
	metrics  map[string]*Metric
	byTenant map[string][]string
}

// NewMetricStore creates an empty MetricStore.
func NewMetricStore() *MetricStore {
	return &MetricStore{
		metrics:  make(map[string]*Metric),
		byTenant: make(map[string][]string),
	}
}

// Record stores a new metric, generating ID and timestamp.
func (s *MetricStore) Record(m *Metric) (*Metric, error) {
	if m.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if m.MetricName == "" || !ValidMetricType(string(m.MetricType)) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	m.RecordedAt = timeNow()

	s.metrics[m.ID] = m
	s.byTenant[m.TenantID] = append(s.byTenant[m.TenantID], m.ID)
	cp := *m
	return &cp, nil
}

// MetricFilter narrows List results (nil/zero = no filter).
type MetricFilter struct {
	MetricType *string
	MetricName *string
	SourceID   *string
	Start      *time.Time
	End        *time.Time
}

// List returns a tenant's metrics, filtered and paginated, newest first.
func (s *MetricStore) List(tenantID string, page, pageSize int, f MetricFilter) ([]Metric, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*Metric
	for _, id := range s.byTenant[tenantID] {
		m := s.metrics[id]
		if m == nil {
			continue
		}
		if f.MetricType != nil && string(m.MetricType) != *f.MetricType {
			continue
		}
		if f.MetricName != nil && m.MetricName != *f.MetricName {
			continue
		}
		if f.SourceID != nil && m.SourceID != *f.SourceID {
			continue
		}
		if f.Start != nil && m.RecordedAt.Before(*f.Start) {
			continue
		}
		if f.End != nil && m.RecordedAt.After(*f.End) {
			continue
		}
		matched = append(matched, m)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].RecordedAt.After(matched[j].RecordedAt) })

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]Metric, 0, end-start)
	for _, m := range matched[start:end] {
		out = append(out, *m)
	}
	return out, total, end < total
}
