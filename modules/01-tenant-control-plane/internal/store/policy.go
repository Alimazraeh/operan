package store

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Policy types ────────────────────────────────────────────────────────────

// PolicyScope represents where a policy applies.
type PolicyScope string

const (
	PolicyScopeTenant   PolicyScope = "tenant"
	PolicyScopeNamespace PolicyScope = "namespace"
	PolicyScopeResource PolicyScope = "resource"
	PolicyScopeAgent    PolicyScope = "agent"
)

// PolicyAction represents what a policy permits, denies, or restricts.
type PolicyAction string

const (
	PolicyActionPermit  PolicyAction = "permit"
	PolicyActionDeny    PolicyAction = "deny"
	PolicyActionRequire PolicyAction = "require"
	PolicyActionAudit   PolicyAction = "audit"
)

// PolicyPriority determines evaluation order.
type PolicyPriority string

const (
	PolicyPriorityCritical PolicyPriority = "critical"
	PolicyPriorityHigh     PolicyPriority = "high"
	PolicyPriorityMedium   PolicyPriority = "medium"
	PolicyPriorityLow      PolicyPriority = "low"
)

// Policy represents a tenant governance policy.
type Policy struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Scope       PolicyScope     `json:"scope"`
	Action      PolicyAction    `json:"action"`
	Rules       []byte          `json:"rules"` // JSON-encoded rules
	Priority    PolicyPriority  `json:"priority"`
	Enabled     bool            `json:"enabled"`
	Effect      string          `json:"effect,omitempty"` // Result of last evaluation
	LastEvalAt  *time.Time      `json:"last_eval_at,omitempty"`
	CreatedBy   string          `json:"created_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// PolicyPatchRequest for partial updates.
type PolicyPatchRequest struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Scope       PolicyScope    `json:"scope,omitempty"`
	Action      PolicyAction   `json:"action,omitempty"`
	Rules       json.RawMessage `json:"rules,omitempty"`
	Priority    PolicyPriority `json:"priority,omitempty"`
	Enabled     *bool          `json:"enabled,omitempty"`
}

// PolicyEvaluationRequest for evaluating a policy against a resource.
type PolicyEvaluationRequest struct {
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Action       string                 `json:"action"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// PolicyEvaluationResult represents the outcome of a policy evaluation.
type PolicyEvaluationResult struct {
	PolicyID    string                 `json:"policy_id"`
	PolicyName  string                 `json:"policy_name"`
	Action      PolicyAction           `json:"action"`
	Matched     bool                   `json:"matched"`
	Reason      string                 `json:"reason,omitempty"`
	Resources   []string               `json:"resources,omitempty"`
	EvaluatedAt time.Time              `json:"evaluated_at"`
}

// ─── PolicyStore ─────────────────────────────────────────────────────────────

// PolicyStore manages tenant policies.
type PolicyStore struct {
	mu       sync.RWMutex
	policies map[string]*Policy
	byTenant map[string][]string // keyed by TenantID -> PolicyIDs
}

// NewPolicyStore creates a new PolicyStore.
func NewPolicyStore() *PolicyStore {
	return &PolicyStore{
		policies: make(map[string]*Policy),
		byTenant: make(map[string][]string),
	}
}

// Create adds a new policy.
func (s *PolicyStore) Create(p *Policy) (*Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Priority == "" {
		p.Priority = PolicyPriorityMedium
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = timeNow()
	}
	p.UpdatedAt = timeNow()

	s.policies[p.ID] = p
	s.byTenant[p.TenantID] = append(s.byTenant[p.TenantID], p.ID)

	return p, nil
}

// GetByID retrieves a policy by ID.
func (s *PolicyStore) GetByID(id string) (*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	cpy := *p
	return &cpy, nil
}

// Patch updates a policy.
func (s *PolicyStore) Patch(id string, req PolicyPatchRequest) (*Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}

	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Scope != "" {
		p.Scope = req.Scope
	}
	if req.Action != "" {
		p.Action = req.Action
	}
	if req.Priority != "" {
		p.Priority = req.Priority
	}
	if req.Rules != nil {
		p.Rules = req.Rules
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}

	p.UpdatedAt = timeNow()

	return p, nil
}

// Delete removes a policy.
func (s *PolicyStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.policies[id]
	if !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	// Remove from byTenant
	tenantIDs, ok := s.byTenant[p.TenantID]
	if ok {
		idx := slices.Index(tenantIDs, id)
		if idx >= 0 {
			s.byTenant[p.TenantID] = append(tenantIDs[:idx], tenantIDs[idx+1:]...)
		}
	}

	delete(s.policies, id)

	return nil
}

// ListByTenant returns all policies for a tenant.
func (s *PolicyStore) ListByTenant(tenantID string, page, pageSize int) ([]*Policy, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, 0, false
	}

	items := make([]*Policy, 0, len(ids))
	for _, id := range ids {
		p, ok := s.policies[id]
		if !ok {
			continue
		}
		cpy := *p
		items = append(items, &cpy)
	}

	total := len(items)
	slices.SortFunc(items, func(a, b *Policy) int {
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

// CountTotal returns the total number of policies.
func (s *PolicyStore) CountTotal() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.policies)
}

// Evaluate evaluates all enabled policies for a tenant against a request.
func (s *PolicyStore) Evaluate(tenantID string, req PolicyEvaluationRequest) ([]*PolicyEvaluationResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byTenant[tenantID]
	if !ok {
		return nil, nil
	}

	var results []*PolicyEvaluationResult
	now := timeNow()

	for _, id := range ids {
		p, ok := s.policies[id]
		if !ok || !p.Enabled {
			continue
		}

		// Simple rule matching: if policy scope matches request type
		matched := matchesPolicy(p, req)
		if matched {
			result := &PolicyEvaluationResult{
				PolicyID:    p.ID,
				PolicyName:  p.Name,
				Action:      p.Action,
				Matched:     true,
				Reason:      p.Description,
				EvaluatedAt: now,
			}

			// Update policy last eval
			p.LastEvalAt = &now
			p.Effect = fmt.Sprintf("%s: %s", p.Action, result.Reason)

			results = append(results, result)
		}
	}

	return results, nil
}

// matchesPolicy checks if a policy matches a given request.
func matchesPolicy(p *Policy, req PolicyEvaluationRequest) bool {
	// Scope matching
	switch p.Scope {
	case PolicyScopeTenant:
		return true
	case PolicyScopeNamespace:
		return req.ResourceType == "namespace"
	case PolicyScopeResource:
		return true // Broad resource scope
	case PolicyScopeAgent:
		return req.ResourceType == "agent"
	default:
		return false
	}
}
