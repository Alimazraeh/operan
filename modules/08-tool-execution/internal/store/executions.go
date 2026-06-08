package store

import (
	"sort"
	"sync"

	"github.com/google/uuid"
)

// ExecutionStore tracks tool execution records with tenant isolation.
type ExecutionStore struct {
	mu         sync.RWMutex
	executions map[string]*ToolExecution // id -> execution
	byTenant   map[string][]string       // tenantID -> []executionID
}

// NewExecutionStore creates an empty ExecutionStore.
func NewExecutionStore() *ExecutionStore {
	return &ExecutionStore{
		executions: make(map[string]*ToolExecution),
		byTenant:   make(map[string][]string),
	}
}

// Create records a new execution, defaulting status to queued.
func (s *ExecutionStore) Create(e *ToolExecution) (*ToolExecution, error) {
	if e.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if e.AgentID == "" || e.Tool == "" {
		return nil, ErrValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.Status == "" {
		e.Status = ExecQueued
	}
	e.CreatedAt = timeNow()
	e.UpdatedAt = e.CreatedAt
	s.executions[e.ID] = e
	s.byTenant[e.TenantID] = append(s.byTenant[e.TenantID], e.ID)
	cp := *e
	return &cp, nil
}

// GetByIDAndTenant returns an execution scoped to a tenant.
func (s *ExecutionStore) GetByIDAndTenant(id, tenantID string) (*ToolExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.executions[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

// Update applies a mutation to a tenant-owned execution.
func (s *ExecutionStore) Update(id, tenantID string, apply func(*ToolExecution)) (*ToolExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.executions[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrNotFound
	}
	apply(e)
	e.UpdatedAt = timeNow()
	cp := *e
	return &cp, nil
}

// List returns a tenant's executions, optionally filtered by tool/status, paginated.
func (s *ExecutionStore) List(tenantID string, page, pageSize int, tool, status *string) ([]ToolExecution, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*ToolExecution
	for _, id := range s.byTenant[tenantID] {
		e, ok := s.executions[id]
		if !ok {
			continue
		}
		if tool != nil && e.Tool != *tool {
			continue
		}
		if status != nil && string(e.Status) != *status {
			continue
		}
		all = append(all, e)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]ToolExecution, 0, end-start)
	for _, e := range all[start:end] {
		out = append(out, *e)
	}
	return out, total, end < total
}

// CostSummary aggregates cost across a tenant's executions, optionally for one tool.
type CostSummary struct {
	Tool       string
	TotalCalls int
	TotalCost  float64
	Currency   string
}

// AggregateCost computes total/average cost for a tenant (optionally per tool).
func (s *ExecutionStore) AggregateCost(tenantID string, tool *string) CostSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sum := CostSummary{Currency: "USD"}
	if tool != nil {
		sum.Tool = *tool
	}
	for _, id := range s.byTenant[tenantID] {
		e, ok := s.executions[id]
		if !ok {
			continue
		}
		if tool != nil && e.Tool != *tool {
			continue
		}
		sum.TotalCalls++
		if e.Cost != nil {
			sum.TotalCost += e.Cost.Amount
			if e.Cost.Currency != "" {
				sum.Currency = e.Cost.Currency
			}
		}
	}
	return sum
}
