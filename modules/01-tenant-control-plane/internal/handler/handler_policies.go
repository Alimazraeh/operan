// Package handler implements HTTP request handlers for tenant-control-plane.
// Each handler function processes a request, calls the appropriate store method,
// and returns a JSON response matching the OpenAPI spec.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/operan/modules/01-tenant-control-plane/internal/middleware"
	"github.com/operan/modules/01-tenant-control-plane/internal/store"
)

// ─── Policy request/response types ───────────────────────────────────────────

// PolicyCreateRequest for creating a new policy.
type PolicyCreateRequest struct {
	Name        string            `json:"name" validate:"required"`
	Description string            `json:"description,omitempty"`
	Scope       string            `json:"scope"`
	Action      string            `json:"action"`
	Rules       json.RawMessage   `json:"rules,omitempty"`
	Priority    string            `json:"priority,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	CreatedBy   string            `json:"created_by,omitempty"`
}

// PolicyPatchRequest for partial policy updates.
type PolicyPatchRequest struct {
	Name        string                   `json:"name,omitempty"`
	Description string                   `json:"description,omitempty"`
	Scope       store.PolicyScope        `json:"scope,omitempty"`
	Action      store.PolicyAction       `json:"action,omitempty"`
	Rules       json.RawMessage          `json:"rules,omitempty"`
	Priority    store.PolicyPriority     `json:"priority,omitempty"`
	Enabled     *bool                    `json:"enabled,omitempty"`
}

// PolicyResponse represents a policy in API responses.
type PolicyResponse struct {
	ID          string             `json:"id"`
	TenantID    string             `json:"tenant_id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Scope       string             `json:"scope"`
	Action      string             `json:"action"`
	Rules       json.RawMessage    `json:"rules,omitempty"`
	Priority    string             `json:"priority"`
	Enabled     bool               `json:"enabled"`
	Effect      string             `json:"effect,omitempty"`
	LastEvalAt  *string            `json:"last_eval_at,omitempty"`
	CreatedBy   string             `json:"created_by,omitempty"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
}

// PolicyEvaluationRequest for evaluating policies against a resource.
type PolicyEvaluationRequest struct {
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id,omitempty"`
	Action       string            `json:"action"`
	Attributes   map[string]any    `json:"attributes,omitempty"`
}

// PolicyEvaluationResponse for evaluation results.
type PolicyEvaluationResponse struct {
	Results     []PolicyResult `json:"results"`
	Passed      bool           `json:"passed"`
	EvaluatedAt string          `json:"evaluated_at"`
}

// PolicyResult represents the outcome of evaluating a single policy.
type PolicyResult struct {
	PolicyID   string `json:"policy_id"`
	PolicyName string `json:"policy_name"`
	Action     string `json:"action"`
	Matched    bool   `json:"matched"`
	Reason     string `json:"reason,omitempty"`
	Resources  []string `json:"resources,omitempty"`
}

// PolicyStatsResponse for policy statistics.
type PolicyStatsResponse struct {
	TotalPolicies    int            `json:"total_policies"`
	EnabledPolicies  int            `json:"enabled_policies"`
	DisabledPolicies int            `json:"disabled_policies"`
	ByScope          map[string]int `json:"by_scope,omitempty"`
	ByAction         map[string]int `json:"by_action,omitempty"`
}

// ─── Policy handlers ─────────────────────────────────────────────────────────

// ListPolicies handles GET /v1/tenants/{id}/policies.
func ListPolicies(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		page, pageSize := paginationFrom(r)
		policies, total, hasMore := h.PolicyStore.ListByTenant(tenantID, page, pageSize)

		items := make([]*PolicyResponse, 0, len(policies))
		for _, p := range policies {
			items = append(items, policyToResponse(p))
		}

		h.WriteJSON(w, http.StatusOK, PolicyListResponse{
			Items:    items,
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			HasMore:  hasMore,
		})
	}
}

// CreatePolicy handles POST /v1/tenants/{id}/policies.
func CreatePolicy(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		var req PolicyCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "Invalid request body")
			return
		}

		if req.Name == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "Name is required")
			return
		}
		if req.Scope == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "Scope is required")
			return
		}
		if req.Action == "" {
			h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "Action is required")
			return
		}

		p := &store.Policy{
			TenantID:    tenantID,
			Name:        req.Name,
			Description: req.Description,
			Scope:       store.PolicyScope(req.Scope),
			Action:      store.PolicyAction(req.Action),
			Rules:       req.Rules,
			Priority:    store.PolicyPriority(req.Priority),
			Enabled:     true,
			CreatedBy:   req.CreatedBy,
		}
		if req.Enabled != nil {
			p.Enabled = *req.Enabled
		}

		created, err := h.PolicyStore.Create(p)
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "internal error", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusCreated, policyToResponse(created))
	}
}

// GetPolicy handles GET /v1/tenants/{id}/policies/{policy_id}.
func GetPolicy(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		policyID, ok := extractPathParam(r, "policy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "policy_id is required")
			return
		}

		p, err := h.PolicyStore.GetByID(policyID)
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "policy not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, policyToResponse(p))
	}
}

// PatchPolicy handles PATCH /v1/tenants/{id}/policies/{policy_id}.
func PatchPolicy(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		policyID, ok := extractPathParam(r, "policy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "policy_id is required")
			return
		}

		var req PolicyPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "Invalid request body")
			return
		}

		p, err := h.PolicyStore.Patch(policyID, store.PolicyPatchRequest{
			Name:        req.Name,
			Description: req.Description,
			Scope:       req.Scope,
			Action:      req.Action,
			Rules:       req.Rules,
			Priority:    req.Priority,
			Enabled:     req.Enabled,
		})
		if err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "policy not found", err.Error())
			return
		}

		h.WriteJSON(w, http.StatusOK, policyToResponse(p))
	}
}

// DeletePolicy handles DELETE /v1/tenants/{id}/policies/{policy_id}.
func DeletePolicy(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		policyID, ok := extractPathParam(r, "policy_id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "policy_id is required")
			return
		}

		if err := h.PolicyStore.Delete(policyID); err != nil {
			h.WriteError(w, http.StatusNotFound, 404, "policy not found", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// EvaluatePolicies handles POST /v1/tenants/{id}/policies/evaluate.
func EvaluatePolicies(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		var req PolicyEvaluationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "Invalid request body")
			return
		}

		storeReq := store.PolicyEvaluationRequest{
			ResourceType: req.ResourceType,
			ResourceID:   req.ResourceID,
			Action:       req.Action,
			Attributes:   req.Attributes,
		}

		results, err := h.PolicyStore.Evaluate(tenantID, storeReq)
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "internal error", err.Error())
			return
		}

		resp := evaluationToResponse(results)
		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// CheckPolicyCompliance handles GET /v1/tenants/{id}/policies/check-compliance.
func CheckPolicyCompliance(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		// Reuse Evaluate with a broad request to check all policies
		storeReq := store.PolicyEvaluationRequest{
			ResourceType: "*",
			Action:       "*",
		}

		results, err := h.PolicyStore.Evaluate(tenantID, storeReq)
		if err != nil {
			h.WriteError(w, http.StatusInternalServerError, 500, "internal error", err.Error())
			return
		}

		resp := evaluationToResponse(results)
		h.WriteJSON(w, http.StatusOK, resp)
	}
}

// GetPolicyStats handles GET /v1/tenants/{id}/policies/stats.
func GetPolicyStats(h *middleware.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := extractPathParam(r, "id")
		if !ok {
			h.WriteError(w, http.StatusBadRequest, 400, "invalid request", "tenant id is required")
			return
		}

		// Get all policies for the tenant to compute stats
		policies, _, _ := h.PolicyStore.ListByTenant(tenantID, 1, 10000)

		total := len(policies)
		enabled := 0
		disabled := 0
		byScope := make(map[string]int)
		byAction := make(map[string]int)

		for _, p := range policies {
			if p.Enabled {
				enabled++
			} else {
				disabled++
			}
			byScope[string(p.Scope)]++
			byAction[string(p.Action)]++
		}

		h.WriteJSON(w, http.StatusOK, PolicyStatsResponse{
			TotalPolicies:    total,
			EnabledPolicies:  enabled,
			DisabledPolicies: disabled,
			ByScope:          byScope,
			ByAction:         byAction,
		})
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// policyToResponse converts a store.Policy to API response.
func policyToResponse(p *store.Policy) *PolicyResponse {
	resp := &PolicyResponse{
		ID:          p.ID,
		TenantID:    p.TenantID,
		Name:        p.Name,
		Description: p.Description,
		Scope:       string(p.Scope),
		Action:      string(p.Action),
		Rules:       p.Rules,
		Priority:    string(p.Priority),
		Enabled:     p.Enabled,
		Effect:      p.Effect,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if p.LastEvalAt != nil {
		s := p.LastEvalAt.Format("2006-01-02T15:04:05Z")
		resp.LastEvalAt = &s
	}
	return resp
}

// evaluationToResponse converts store.PolicyEvaluationResult slice to API response.
func evaluationToResponse(results []*store.PolicyEvaluationResult) *PolicyEvaluationResponse {
	if results == nil {
		results = []*store.PolicyEvaluationResult{}
	}

	respResults := make([]PolicyResult, 0, len(results))
	for _, r := range results {
		respResults = append(respResults, PolicyResult{
			PolicyID:   r.PolicyID,
			PolicyName: r.PolicyName,
			Action:     string(r.Action),
			Matched:    r.Matched,
			Reason:     r.Reason,
			Resources:  r.Resources,
		})
	}

	return &PolicyEvaluationResponse{
		Results:     respResults,
		Passed:      len(results) == 0,
		EvaluatedAt: time.Now().Format("2006-01-02T15:04:05Z"),
	}
}
