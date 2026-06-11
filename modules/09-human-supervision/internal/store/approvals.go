package store

import (
	"sync"

	"github.com/google/uuid"
)

// ApprovalStore provides tenant-isolated storage and decision logic for
// approval gates.
type ApprovalStore struct {
	mu        sync.RWMutex
	approvals map[string]*Approval
	byTenant  map[string][]string
	byRequest map[string]string // tenantID+"|"+requestID -> approvalID
}

// NewApprovalStore creates an empty ApprovalStore.
func NewApprovalStore() *ApprovalStore {
	return &ApprovalStore{
		approvals: make(map[string]*Approval),
		byTenant:  make(map[string][]string),
		byRequest: make(map[string]string),
	}
}

// Create stores a new pending approval.
func (s *ApprovalStore) Create(a *Approval) (*Approval, error) {
	if a.TenantID == "" {
		return nil, ErrTenantMismatch
	}
	if a.RequestID == "" || a.RequesterID == "" || !ValidApprovalType(a.Type) {
		return nil, ErrValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	a.ID = uuid.New().String()
	a.Status = "pending"
	a.CreatedAt = timeNow()
	a.UpdatedAt = a.CreatedAt

	s.approvals[a.ID] = a
	s.byTenant[a.TenantID] = append(s.byTenant[a.TenantID], a.ID)
	s.byRequest[a.TenantID+"|"+a.RequestID] = a.ID
	cp := *a
	return &cp, nil
}

// Get returns a tenant's approval, transitioning it to expired first when
// its deadline has passed. The second return reports whether this call
// performed the expiry transition (callers publish GateTimeout on true).
func (s *ApprovalStore) Get(id, tenantID string) (*Approval, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return nil, false, ErrNotFound
	}
	expired := s.expireLocked(a)
	cp := *a
	return &cp, expired, nil
}

// GetByRequestID returns a tenant's approval by its originating request.
func (s *ApprovalStore) GetByRequestID(requestID, tenantID string) (*Approval, error) {
	s.mu.RLock()
	id, ok := s.byRequest[tenantID+"|"+requestID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	a, _, err := s.Get(id, tenantID)
	return a, err
}

// Update applies a partial update; conflicts when the approval is terminal.
func (s *ApprovalStore) Update(id, tenantID string, upd func(*Approval)) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	s.expireLocked(a)
	if a.Terminal() {
		return nil, ErrConflict
	}
	upd(a)
	a.UpdatedAt = timeNow()
	cp := *a
	return &cp, nil
}

// Delete removes a tenant's approval.
func (s *ApprovalStore) Delete(id, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return ErrNotFound
	}
	delete(s.approvals, id)
	delete(s.byRequest, tenantID+"|"+a.RequestID)
	list := s.byTenant[tenantID]
	for i, aid := range list {
		if aid == id {
			s.byTenant[tenantID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

// Approve records an approval act and applies the decision rules:
// threshold-type approvals need min_approvals (default 1); other types
// approve on the first approval. Conflicts on terminal approvals.
func (s *ApprovalStore) Approve(id, tenantID string, act ApprovalAction) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if s.expireLocked(a) || a.Terminal() {
		return nil, ErrConflict
	}

	act.Action = "approve"
	act.CreatedAt = timeNow()
	a.Approvals = append(a.Approvals, act)
	a.CurrentStep++

	needed := 1
	if a.Type == "threshold" && a.ThresholdConfig != nil && a.ThresholdConfig.MinApprovals > 0 {
		needed = a.ThresholdConfig.MinApprovals
	}
	if len(a.Approvals) >= needed {
		now := timeNow()
		a.Status = "approved"
		a.ApprovedAt = &now
	} else {
		a.Status = "in_progress"
	}
	a.UpdatedAt = timeNow()
	cp := *a
	return &cp, nil
}

// Reject records a rejection; any rejection beyond max_rejections (default
// 0, i.e. the first) finalizes the approval as rejected.
func (s *ApprovalStore) Reject(id, tenantID string, act ApprovalAction) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if s.expireLocked(a) || a.Terminal() {
		return nil, ErrConflict
	}

	act.Action = "reject"
	act.CreatedAt = timeNow()
	a.Rejections = append(a.Rejections, act)

	allowed := 0
	if a.Type == "threshold" && a.ThresholdConfig != nil {
		allowed = a.ThresholdConfig.MaxRejections
	}
	if len(a.Rejections) > allowed {
		now := timeNow()
		a.Status = "rejected"
		a.RejectedAt = &now
	} else {
		a.Status = "in_progress"
	}
	a.UpdatedAt = timeNow()
	cp := *a
	return &cp, nil
}

// Delegate hands the approval to a new approver.
func (s *ApprovalStore) Delegate(id, tenantID string, d ApprovalDelegate) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if s.expireLocked(a) || a.Terminal() {
		return nil, ErrConflict
	}

	d.CreatedAt = timeNow()
	a.Delegates = append(a.Delegates, d)
	a.Status = "delegated"
	a.RequiredApprovers = append(a.RequiredApprovers, ApprovalTarget{UserID: d.ToUserID})
	a.UpdatedAt = timeNow()
	cp := *a
	return &cp, nil
}

// Pending returns a tenant's non-terminal approvals (for queue/dashboard).
func (s *ApprovalStore) Pending(tenantID string) []Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Approval
	for _, id := range s.byTenant[tenantID] {
		a := s.approvals[id]
		if a == nil {
			continue
		}
		s.expireLocked(a)
		if !a.Terminal() {
			out = append(out, *a)
		}
	}
	return out
}

// expireLocked transitions a past-deadline approval to expired. Returns
// true when the transition happened in this call.
func (s *ApprovalStore) expireLocked(a *Approval) bool {
	if a.Terminal() || a.ExpiresAt == nil || a.ExpiresAt.After(timeNow()) {
		return false
	}
	a.Status = "expired"
	a.UpdatedAt = timeNow()
	return true
}
