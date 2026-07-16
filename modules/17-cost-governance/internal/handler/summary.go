package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/operan/cost-governance/internal/ctxkeys"
	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/store"
)

// SummaryHandler handles cost summary endpoints.
type SummaryHandler struct {
	eventStore  *store.CostEventStore
	budgetStore *store.BudgetStore
	alertStore  *store.AlertStore
	throttleMgr *engine.ThrottleManager
}

// NewSummaryHandler creates a new SummaryHandler.
func NewSummaryHandler(eventStore *store.CostEventStore, budgetStore *store.BudgetStore, alertStore *store.AlertStore, throttleMgr *engine.ThrottleManager) *SummaryHandler {
	return &SummaryHandler{eventStore: eventStore, budgetStore: budgetStore, alertStore: alertStore, throttleMgr: throttleMgr}
}

// CostSummary handles GET /v1/summary
func (h *SummaryHandler) CostSummary(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	period := r.URL.Query().Get("period")
	agentID := r.URL.Query().Get("agent_id")
	if period == "" {
		period = "monthly"
	}

	now := time.Now()
	from := periodStart(period, now)

	// Calculate total spent
	totalSpent := 0.0
	var err error
	if agentID != "" {
		totalSpent, err = h.eventStore.SumCostByTenantAndAgent(r.Context(), tenantID, agentID, from, now)
	} else {
		totalSpent, err = h.eventStore.SumCostByTenant(r.Context(), tenantID, from, now)
	}
	if err != nil {
		http.Error(w, `{"error":"failed to calculate summary"}`, http.StatusInternalServerError)
		return
	}

	// Get active budgets
	budgets, _ := h.budgetStore.ListActiveByTenant(r.Context(), tenantID, agentID)

	var budgetStatuses []map[string]any
	for _, b := range budgets {
		spent := 0.0
		if b.AgentID != nil {
			spent, _ = h.eventStore.SumCostByTenantAndAgent(r.Context(), tenantID, *b.AgentID, from, now)
		} else {
			spent, _ = h.eventStore.SumCostByTenant(r.Context(), tenantID, from, now)
		}
		remaining := b.BudgetAmount - spent
		pct := 0.0
		if b.BudgetAmount > 0 {
			pct = (spent / b.BudgetAmount) * 100
		}

		budgetStatuses = append(budgetStatuses, map[string]any{
			"name":         b.Description,
			"amount":       b.BudgetAmount,
			"spent":        spent,
			"remaining":    remaining,
			"percentage":   pct,
			"period":       b.Period,
			"soft_limit":   b.SoftLimitPct,
			"hard_limit":   b.HardLimitPct,
		})
	}

	// Get throttle state
	throttleState, _ := h.throttleMgr.GetThrottleInfo(tenantID)

	// Get recent alerts
	alerts, _, _ := h.alertStore.List(r.Context(), tenantID, "", "", nil, 1, 10)
	var alertSummaries []map[string]any
	for _, a := range alerts {
		alertSummaries = append(alertSummaries, map[string]any{
			"type":       a.AlertType,
			"severity":   a.Severity,
			"message":    a.AlertType + " — " + a.Severity + " (" + strconv.FormatFloat(a.PercentageUsed, 'f', 1, 64) + "% used)",
		})
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"total_spent":     totalSpent,
		"period":          period,
		"from":            from,
		"budgets":         budgetStatuses,
		"throttle_state":  throttleState,
		"alerts":          alertSummaries,
	})
}