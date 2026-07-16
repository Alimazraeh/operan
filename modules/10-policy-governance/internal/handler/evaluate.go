package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/operan/policy-governance/internal/ctxkeys"
	"github.com/operan/policy-governance/internal/engine"
	"github.com/operan/policy-governance/internal/store"
)

// EvaluateHandler handles the policy evaluation endpoint.
type EvaluateHandler struct {
	engine     *engine.Engine
	auditStore *store.AuditStore
	eventPub   engine.EventPublisher
}

// NewEvaluateHandler creates a new EvaluateHandler.
func NewEvaluateHandler(engine *engine.Engine, auditStore *store.AuditStore, eventPub engine.EventPublisher) *EvaluateHandler {
	return &EvaluateHandler{
		engine:     engine,
		auditStore: auditStore,
		eventPub:   eventPub,
	}
}

// EvaluatePolicy handles POST /v1/policies/evaluate
func (h *EvaluateHandler) EvaluatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	var req struct {
		AgentID      string                 `json:"agent_id,omitempty"`
		AgentRole    string                 `json:"agent_role,omitempty"`
		DepartmentID string                 `json:"department_id,omitempty"`
		Resource     string                 `json:"resource"`
		ActionType   string                 `json:"action_type"`
		DataClass    string                 `json:"data_class,omitempty"`
		Cost         float64                `json:"cost,omitempty"`
		Metadata     map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "invalid JSON body",
		})
		return
	}

	if req.Resource == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "resource is required",
		})
		return
	}
	if req.ActionType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "bad-request",
			"message": "action_type is required",
		})
		return
	}

	result, err := h.engine.Evaluate(r.Context(), engine.EvaluateRequest{
		TenantID:     tenantID,
		AgentID:      req.AgentID,
		AgentRole:    req.AgentRole,
		DepartmentID: req.DepartmentID,
		Resource:     req.Resource,
		ActionType:   req.ActionType,
		DataClass:    req.DataClass,
		Cost:         req.Cost,
		Metadata:     req.Metadata,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	// Build response warnings
	warnings := make([]map[string]string, len(result.Warnings))
	for i, w := range result.Warnings {
		warnings[i] = map[string]string{"message": w}
	}

	// Build response rules
	rules := make([]map[string]interface{}, len(result.Rules))
	for i, rm := range result.Rules {
		rules[i] = map[string]interface{}{
			"policy_id":   rm.PolicyID,
			"policy_name": rm.PolicyName,
			"effect":      rm.Effect,
			"description": rm.Description,
		}
	}

	response := map[string]interface{}{
		"allowed":      result.Allowed,
		"action":       result.Action,
		"policy_name":  result.PolicyName,
		"reason":       result.Reason,
		"warnings":     warnings,
		"rules":        rules,
	}

	writeJSON(w, http.StatusOK, response)
}

// AuditHandler handles audit log endpoints.
type AuditHandler struct {
	auditStore *store.AuditStore
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditStore *store.AuditStore) *AuditHandler {
	return &AuditHandler{auditStore: auditStore}
}

// ListAudits handles GET /v1/audit
func (h *AuditHandler) ListAudits(w http.ResponseWriter, r *http.Request) {
	tenantID := ctxkeys.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	var agentID, result *string
	if a := r.URL.Query().Get("agent_id"); a != "" {
		agentID = &a
	}
	if r := r.URL.Query().Get("result"); r != "" {
		result = &r
	}

	audits, total, err := h.auditStore.List(r.Context(), tenantID, agentID, result, nil, nil, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal-error",
			"message": err.Error(),
		})
		return
	}

	resultList := make([]map[string]interface{}, len(audits))
	for i, a := range audits {
		resultList[i] = map[string]interface{}{
			"id":                  a.ID,
			"agent_id":            a.AgentID,
			"resource_type":       a.ResourceType,
			"resource_target":     a.ResourceTarget,
			"requested_action":    a.RequestedAction,
			"result":              a.Result,
			"matched_policy_name": a.MatchedPolicyName,
			"evaluation_ms":       a.EvaluationMS,
			"request_data":        a.RequestData,
			"created_at":          a.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"audits":  resultList,
		"page":    page,
		"page_size": pageSize,
		"total":   total,
		"has_more": (page * pageSize) < total,
	})
}