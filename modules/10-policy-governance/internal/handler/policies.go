package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/operan/policy-governance/internal/ctxkeys"
	"github.com/operan/policy-governance/internal/store"
)

// PolicyHandler handles policy CRUD endpoints.
type PolicyHandler struct {
	policyStore *store.PolicyStore
}

// NewPolicyHandler creates a new PolicyHandler.
func NewPolicyHandler(policyStore *store.PolicyStore) *PolicyHandler {
	return &PolicyHandler{
		policyStore: policyStore,
	}
}

// CreatePolicy handles POST /v1/policies
func (h *PolicyHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req struct {
		GroupID             string                 `json:"group_id"`
		Name                string                 `json:"name"`
		Description         *string                `json:"description"`
		Action              string                 `json:"action"`
		Scope               string                 `json:"scope"`
		ResourceType        string                 `json:"resource_type"`
		ResourceTarget      *string                `json:"resource_target"`
		ConditionExpression map[string]interface{} `json:"condition_expression"`
		Effect              string                 `json:"effect"`
		Priority            *int                   `json:"priority"`
		CreatedBy           *string                `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid JSON body",
		})
		return
	}

	if req.GroupID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "group_id is required",
		})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "name is required",
		})
		return
	}
	if !isValidAction(req.Action) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "action must be allow, deny, or proxy",
		})
		return
	}
	if !isValidScope(req.Scope) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "scope must be agent, department, tenant, or global",
		})
		return
	}
	if !isValidResourceType(req.ResourceType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "resource_type must be tool, model, workflow, data, or all",
		})
		return
	}
	if !isValidEffect(req.Effect) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "effect must be enforce, warn, or log",
		})
		return
	}

	priority := 50
	if req.Priority != nil {
		priority = *req.Priority
	}

	policy := &store.Policy{
		TenantID:            tenantID,
		GroupID:             req.GroupID,
		Name:                req.Name,
		Description:         req.Description,
		Action:              req.Action,
		Scope:               req.Scope,
		ResourceType:        req.ResourceType,
		ResourceTarget:      req.ResourceTarget,
		ConditionExpression: req.ConditionExpression,
		Effect:              req.Effect,
		Priority:            priority,
		IsActive:            true,
		CreatedBy:           req.CreatedBy,
	}

	if err := h.policyStore.Create(r.Context(), policy); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                 policy.ID,
		"name":               policy.Name,
		"action":             policy.Action,
		"scope":              policy.Scope,
		"resource_type":      policy.ResourceType,
		"effect":             policy.Effect,
		"priority":           policy.Priority,
		"is_active":          policy.IsActive,
		"created_at":         policy.CreatedAt,
		"updated_at":         policy.UpdatedAt,
	})
}

// ListPolicies handles GET /v1/policies
func (h *PolicyHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var scope, resourceType *string
	if s := r.URL.Query().Get("scope"); s != "" {
		scope = &s
	}
	if rt := r.URL.Query().Get("resource_type"); rt != "" {
		resourceType = &rt
	}

	policies, total, err := h.policyStore.List(r.Context(), tenantID, scope, resourceType, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	result := make([]map[string]interface{}, len(policies))
	for i, p := range policies {
		result[i] = map[string]interface{}{
			"id":                p.ID,
			"group_id":          p.GroupID,
			"name":              p.Name,
			"action":            p.Action,
			"scope":             p.Scope,
			"resource_type":     p.ResourceType,
			"effect":            p.Effect,
			"priority":          p.Priority,
			"is_active":         p.IsActive,
			"created_at":        p.CreatedAt,
			"updated_at":        p.UpdatedAt,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"policies": result,
		"page":     page,
		"page_size": pageSize,
		"total":    total,
		"has_more": (page * pageSize) < total,
	})
}

// GetPolicy handles GET /v1/policies/{id}
func (h *PolicyHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid policy ID",
		})
		return
	}

	policy, err := h.policyStore.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}
	if policy.TenantID != tenantID {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":   "forbidden",
			"message": "tenant mismatch",
		})
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

// UpdatePolicy handles PATCH /v1/policies/{id}
func (h *PolicyHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid policy ID",
		})
		return
	}

	policy, err := h.policyStore.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}
	if policy.TenantID != tenantID {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":   "forbidden",
			"message": "tenant mismatch",
		})
		return
	}

	var req struct {
		Name                *string                `json:"name"`
		Description         *string                `json:"description"`
		Action              *string                `json:"action"`
		Scope               *string                `json:"scope"`
		ResourceType        *string                `json:"resource_type"`
		ResourceTarget      *string                `json:"resource_target"`
		ConditionExpression map[string]interface{} `json:"condition_expression"`
		Effect              *string                `json:"effect"`
		Priority            *int                   `json:"priority"`
		IsActive            *bool                  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid JSON body",
		})
		return
	}

	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Description != nil {
		policy.Description = req.Description
	}
	if req.Action != nil {
		policy.Action = *req.Action
	}
	if req.Scope != nil {
		policy.Scope = *req.Scope
	}
	if req.ResourceType != nil {
		policy.ResourceType = *req.ResourceType
	}
	if req.ResourceTarget != nil {
		policy.ResourceTarget = req.ResourceTarget
	}
	if req.ConditionExpression != nil {
		policy.ConditionExpression = req.ConditionExpression
	}
	if req.Effect != nil {
		policy.Effect = *req.Effect
	}
	if req.Priority != nil {
		policy.Priority = *req.Priority
	}
	if req.IsActive != nil {
		policy.IsActive = *req.IsActive
	}

	if err := h.policyStore.Update(r.Context(), policy); err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

// DeletePolicy handles DELETE /v1/policies/{id}
func (h *PolicyHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid policy ID",
		})
		return
	}

	if err := h.policyStore.Delete(r.Context(), id, tenantID); err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error":   "not-found",
				"message": "policy not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// isValidAction checks if an action value is valid.
func isValidAction(action string) bool {
	switch action {
	case "allow", "deny", "proxy":
		return true
	}
	return false
}

// isValidScope checks if a scope value is valid.
func isValidScope(scope string) bool {
	switch scope {
	case "agent", "department", "tenant", "global":
		return true
	}
	return false
}

// isValidResourceType checks if a resource_type value is valid.
func isValidResourceType(rt string) bool {
	switch rt {
	case "tool", "model", "workflow", "data", "all":
		return true
	}
	return false
}

// isValidEffect checks if an effect value is valid.
func isValidEffect(effect string) bool {
	switch effect {
	case "enforce", "warn", "log":
		return true
	}
	return false
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// best-effort: can't send error after headers are written
	}
}