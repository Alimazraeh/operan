package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/operan/cost-governance/internal/ctxkeys"
	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/store"

	"github.com/go-chi/chi/v5"
)

// BudgetHandler handles budget CRUD endpoints.
type BudgetHandler struct {
	budgetStore *store.BudgetStore
	eventStore  *store.CostEventStore
	engine      *engine.Engine
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(budgetStore *store.BudgetStore, eventStore *store.CostEventStore, engine *engine.Engine) *BudgetHandler {
	return &BudgetHandler{budgetStore: budgetStore, eventStore: eventStore, engine: engine}
}

// CreateBudget handles POST /v1/budgets
func (h *BudgetHandler) CreateBudget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID       *string `json:"agent_id,omitempty"`
		Description   string  `json:"description"`
		BudgetAmount  float64 `json:"budget_amount"`
		Period        string  `json:"period"`
		SoftLimitPct  *int    `json:"soft_limit_pct"`
		HardLimitPct  *int    `json:"hard_limit_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.BudgetAmount <= 0 {
		http.Error(w, `{"error":"budget_amount must be positive"}`, http.StatusBadRequest)
		return
	}
	if req.Period == "" {
		http.Error(w, `{"error":"period is required"}`, http.StatusBadRequest)
		return
	}

	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	softLimit := 80
	hardLimit := 95
	if req.SoftLimitPct != nil {
		softLimit = *req.SoftLimitPct
	}
	if req.HardLimitPct != nil {
		hardLimit = *req.HardLimitPct
	}

	budget := &store.CostBudget{
		TenantID:     tenantID,
		AgentID:      req.AgentID,
		Description:  &req.Description,
		BudgetAmount: req.BudgetAmount,
		Period:       req.Period,
		SoftLimitPct: softLimit,
		HardLimitPct: hardLimit,
		IsActive:     true,
	}

	if err := h.budgetStore.Create(r.Context(), budget); err != nil {
		http.Error(w, `{"error":"failed to create budget"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{"budget": budget})
}

// ListBudgets handles GET /v1/budgets
func (h *BudgetHandler) ListBudgets(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	agentID := r.URL.Query().Get("agent_id")
	isActiveStr := r.URL.Query().Get("is_active")

	var isActive *bool
	if isActiveStr != "" {
		v, _ := strconv.ParseBool(isActiveStr)
		isActive = &v
	}

	budgets, total, err := h.budgetStore.List(r.Context(), tenantID, agentID, isActive, page, pageSize)
	if err != nil {
		http.Error(w, `{"error":"failed to list budgets"}`, http.StatusInternalServerError)
		return
	}

	// Enrich with spent_this_period
	now := time.Now()
	for i := range budgets {
		spent := 0.0
		from := periodStart(budgets[i].Period, now)
		if budgets[i].AgentID != nil {
			spent, _ = h.eventStore.SumCostByTenantAndAgent(r.Context(), tenantID, *budgets[i].AgentID, from, now)
		} else {
			spent, _ = h.eventStore.SumCostByTenant(r.Context(), tenantID, from, now)
		}
		budgets[i].SpentThisPeriod = &spent
		if budgets[i].BudgetAmount > 0 {
			pct := (spent / budgets[i].BudgetAmount) * 100
			budgets[i].PercentageUsed = &pct
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"budgets":   budgets,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetBudget handles GET /v1/budgets/{id}
func (h *BudgetHandler) GetBudget(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	id := chi.URLParam(r, "id")

	budget, err := h.budgetStore.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"budget not found"}`, http.StatusNotFound)
		return
	}

	if budget.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"budget": budget})
}

// UpdateBudget handles PATCH /v1/budgets/{id}
func (h *BudgetHandler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	id := chi.URLParam(r, "id")

	budget, err := h.budgetStore.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"budget not found"}`, http.StatusNotFound)
		return
	}
	if budget.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var agentID *string
	if v, ok := req["agent_id"].(string); ok {
		agentID = &v
	}
	var description *string
	if v, ok := req["description"].(string); ok {
		description = &v
	}
	var budgetAmount *float64
	if v, ok := req["budget_amount"].(float64); ok {
		budgetAmount = &v
	}
	var softLimitPct *int
	if v, ok := req["soft_limit_pct"].(float64); ok {
		vi := int(v)
		softLimitPct = &vi
	}
	var hardLimitPct *int
	if v, ok := req["hard_limit_pct"].(float64); ok {
		hv := int(v)
		hardLimitPct = &hv
	}
	var isActive *bool
	if v, ok := req["is_active"].(bool); ok {
		isActive = &v
	}

	updated, err := h.budgetStore.Update(r.Context(), id, agentID, description, budgetAmount, softLimitPct, hardLimitPct, isActive)
	if err != nil {
		http.Error(w, `{"error":"failed to update budget"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"budget": updated})
}

// DeleteBudget handles DELETE /v1/budgets/{id}
func (h *BudgetHandler) DeleteBudget(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	id := chi.URLParam(r, "id")

	budget, err := h.budgetStore.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"budget not found"}`, http.StatusNotFound)
		return
	}
	if budget.TenantID != tenantID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	if err := h.budgetStore.Delete(r.Context(), id); err != nil {
		http.Error(w, `{"error":"failed to delete budget"}`, http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// periodStart returns the start of the period for a given budget period string.
func periodStart(period string, now time.Time) time.Time {
	switch period {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		weekday := now.Weekday()
		if weekday == 0 {
			weekday = 7
		}
		return now.AddDate(0, 0, -(int(weekday) - 1)).Truncate(24 * time.Hour)
	case "monthly":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "quarterly":
		quarter := (now.Month() - 1) / 3
		return time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, now.Location())
	default:
		return now
	}
}