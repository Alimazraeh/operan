package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/operan/model-routing/internal/ctxkeys"
	"github.com/operan/model-routing/internal/store"
)

// RulesHandler handles routing rules CRUD.
type RulesHandler struct {
	store store.RuleStore
}

// NewRulesHandler creates a new rules handler.
func NewRulesHandler(s store.RuleStore) *RulesHandler {
	return &RulesHandler{store: s}
}

// List handles GET /v1/rules
func (h *RulesHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	taskType := r.URL.Query().Get("task_type")
	isActive := r.URL.Query().Get("is_active")

	var taskTypePtr *string
	if taskType != "" {
		taskTypePtr = &taskType
	}

	var isActivePtr *bool
	if isActive != "" {
		v := isActive == "true"
		isActivePtr = &v
	}

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(r.URL.Query().Get("page_size"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	rules, total, err := h.store.ListRules(tenantID, taskTypePtr, isActivePtr, page, pageSize)
	if err != nil {
		log.Printf("list rules error: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	WriteJSON(w, http.StatusOK, PaginatedResponse{
		Data:     rules,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

// Create handles POST /v1/rules
func (h *RulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	var req struct {
		RuleName         string  `json:"rule_name"`
		Description      string  `json:"description"`
		TaskType         string  `json:"task_type"`
		Priority         int     `json:"priority"`
		MinCostThreshold float64 `json:"min_cost_threshold"`
		MaxLatencyMs     int     `json:"max_latency_ms"`
		MaxTokens        int     `json:"max_tokens"`
		FailoverEnabled  *bool   `json:"failover_enabled"`
		Models           []struct {
			ModelID         string  `json:"model_id"`
			CapabilityScore float64 `json:"capability_score"`
			CostWeight      float64 `json:"cost_weight"`
			LatencyWeight   float64 `json:"latency_weight"`
			ReliabilityWeight float64 `json:"reliability_weight"`
		} `json:"models"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RuleName == "" || req.TaskType == "" {
		WriteError(w, http.StatusBadRequest, "rule_name and task_type are required")
		return
	}

	rule := &store.RoutingRule{
		TenantID:         tenantID,
		RuleName:         req.RuleName,
		Description:      req.Description,
		TaskType:         req.TaskType,
		Priority:         req.Priority,
		MinCostThreshold: req.MinCostThreshold,
		MaxLatencyMs:     req.MaxLatencyMs,
		MaxTokens:        req.MaxTokens,
		FailoverEnabled:  true,
		IsActive:         true,
	}
	if req.FailoverEnabled != nil {
		rule.FailoverEnabled = *req.FailoverEnabled
	}

	if err := h.store.CreateRule(rule); err != nil {
		log.Printf("create rule error: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	// Attach models if provided
	for _, m := range req.Models {
		if m.ModelID == "" {
			continue
		}
		model := &store.RoutingRuleModel{
			TenantID:        tenantID,
			RuleID:          rule.ID,
			ModelID:         m.ModelID,
			CapabilityScore: m.CapabilityScore,
			CostWeight:      m.CostWeight,
			LatencyWeight:   m.LatencyWeight,
			ReliabilityWeight: m.ReliabilityWeight,
		}
		if err := h.store.AddModelToRule(model); err != nil {
			log.Printf("add model to rule error: %v", err)
		}
	}

	WriteJSON(w, http.StatusCreated, rule)
}

// Get handles GET /v1/rules/{id}
func (h *RulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	id := chi.URLParam(r, "id")

	rule, err := h.store.GetRule(id, tenantID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "rule not found")
		return
	}

	WriteJSON(w, http.StatusOK, rule)
}

// Update handles PATCH /v1/rules/{id}
func (h *RulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	id := chi.URLParam(r, "id")

	existing, err := h.store.GetRule(id, tenantID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "rule not found")
		return
	}

	var req struct {
		RuleName         string `json:"rule_name"`
		Description      string `json:"description"`
		TaskType         string `json:"task_type"`
		Priority         int    `json:"priority"`
		MaxLatencyMs     int    `json:"max_latency_ms"`
		MaxTokens        int    `json:"max_tokens"`
		FailoverEnabled  *bool  `json:"failover_enabled"`
		IsActive         *bool  `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RuleName != "" {
		existing.RuleName = strings.TrimSpace(req.RuleName)
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.TaskType != "" {
		existing.TaskType = req.TaskType
	}
	if req.Priority != 0 {
		existing.Priority = req.Priority
	}
	if req.MaxLatencyMs != 0 {
		existing.MaxLatencyMs = req.MaxLatencyMs
	}
	if req.MaxTokens != 0 {
		existing.MaxTokens = req.MaxTokens
	}
	if req.FailoverEnabled != nil {
		existing.FailoverEnabled = *req.FailoverEnabled
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := h.store.UpdateRule(existing); err != nil {
		log.Printf("update rule error: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	WriteJSON(w, http.StatusOK, existing)
}

// Delete handles DELETE /v1/rules/{id}
func (h *RulesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	id := chi.URLParam(r, "id")

	if err := h.store.DeleteRule(id, tenantID); err != nil {
		log.Printf("delete rule error: %v", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}