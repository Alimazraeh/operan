package store

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ServiceRequest is a unit of work submitted to a department against one of
// its service offerings — the department's front door. Its lifecycle is
// driven by the dispatcher/poller (internal/workloop): open → in_progress →
// awaiting_approval → completed | rejected | failed | cancelled.
type ServiceRequest struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	DepartmentID string    `json:"department_id"`
	ServiceID    string    `json:"service_id"`
	ServiceName  string    `json:"service_name,omitempty"`
	Title        string    `json:"title"`
	Body         string    `json:"body,omitempty"`
	Priority     string    `json:"priority,omitempty"` // P1..P4 / normal
	Requester    Requester `json:"requester"`

	Status string `json:"status"` // open, in_progress, awaiting_approval, completed, rejected, failed, cancelled

	SLAResponseDue   *time.Time `json:"sla_response_due,omitempty"`
	SLAResolutionDue *time.Time `json:"sla_resolution_due,omitempty"`
	FirstResponseAt  *time.Time `json:"first_response_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`

	// WorkflowRunRef is the per-request M03 workflow id executing this
	// request (workflow-per-run model).
	WorkflowRunRef string         `json:"workflow_run_ref,omitempty"`
	Output         string         `json:"output,omitempty"`
	TokensUsed     int            `json:"tokens_used,omitempty"`
	Timeline       []RequestEvent `json:"timeline,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Requester identifies who filed the request.
type Requester struct {
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
}

// RequestEvent is one entry of a request's visible timeline.
type RequestEvent struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"` // created, dispatched, agent_output, gate_raised, gate_responded, completed, rejected, failed, cancelled, note
	Detail string    `json:"detail,omitempty"`
	Node   string    `json:"node,omitempty"`
}

// TerminalRequestStatus reports whether s is a terminal request state.
func TerminalRequestStatus(s string) bool {
	switch s {
	case "completed", "rejected", "failed", "cancelled":
		return true
	}
	return false
}

// RequestStore is the tenant-scoped in-memory store for service requests,
// snapshot-persisted like the other Module 05 stores.
type RequestStore struct {
	mu       sync.RWMutex
	requests map[string]*ServiceRequest
}

func NewRequestStore() *RequestStore {
	return &RequestStore{requests: map[string]*ServiceRequest{}}
}

// Create assigns id/timestamps and stores the request.
func (s *RequestStore) Create(r *ServiceRequest) (*ServiceRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	if r.Status == "" {
		r.Status = "open"
	}
	r.Timeline = append(r.Timeline, RequestEvent{At: now, Kind: "created",
		Detail: "request submitted"})
	s.requests[r.ID] = r
	cp := *r
	return &cp, nil
}

// GetByIDAndTenant returns a copy of the request.
func (s *RequestStore) GetByIDAndTenant(id, tenantID string) (*ServiceRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.requests[id]
	if !ok || r.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

// ListByDepartment returns the department's requests, newest first.
func (s *RequestStore) ListByDepartment(tenantID, departmentID string, statusFilter *string, page, pageSize int) ([]ServiceRequest, int, bool) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	s.mu.RLock()
	var all []ServiceRequest
	for _, r := range s.requests {
		if r.TenantID != tenantID || r.DepartmentID != departmentID {
			continue
		}
		if statusFilter != nil && *statusFilter != "" && r.Status != *statusFilter {
			continue
		}
		all = append(all, *r)
	}
	s.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return []ServiceRequest{}, total, false
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, end < total
}

// ListNonTerminal returns every request still in flight (any tenant) — the
// poller's work list.
func (s *RequestStore) ListNonTerminal() []ServiceRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ServiceRequest
	for _, r := range s.requests {
		if !TerminalRequestStatus(r.Status) {
			out = append(out, *r)
		}
	}
	return out
}

// Mutate applies fn to the stored request under lock. Returns ErrNotFound if
// the id is unknown.
func (s *RequestStore) Mutate(id string, fn func(*ServiceRequest)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return ErrNotFound
	}
	fn(r)
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// AppendEvent adds a timeline entry (and bumps updated_at).
func (s *RequestStore) AppendEvent(id string, ev RequestEvent) error {
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	return s.Mutate(id, func(r *ServiceRequest) {
		r.Timeline = append(r.Timeline, ev)
	})
}

// Export/Import implement the persist.File contract.
func (s *RequestStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.requests)
}

func (s *RequestStore) Import(data []byte) error {
	m := map[string]*ServiceRequest{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	s.mu.Lock()
	s.requests = m
	s.mu.Unlock()
	return nil
}
