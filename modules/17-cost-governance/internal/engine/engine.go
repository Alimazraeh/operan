package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/operan/cost-governance/internal/store"
)

// BudgetStoreIface is the interface that wraps budget store methods used by the engine.
type BudgetStoreIface interface {
	ListActiveByTenant(ctx context.Context, tenantID, agentID string) ([]store.CostBudget, error)
}

// EventStoreIface is the interface that wraps event store methods used by the engine.
type EventStoreIface interface {
	SumCostByTenant(ctx context.Context, tenantID string, from, to time.Time) (float64, error)
	SumCostByTenantAndAgent(ctx context.Context, tenantID, agentID string, from, to time.Time) (float64, error)
}

// AlertStoreIface is the interface that wraps alert store methods used by the engine.
type AlertStoreIface interface {
	List(ctx context.Context, tenantID, severity, alertType string, isResolved *bool, page, pageSize int) ([]store.CostAlert, int, error)
	Create(ctx context.Context, a *store.CostAlert) error
}

// BudgetCheckResult holds the outcome of a budget check.
type BudgetCheckResult struct {
	Accepted      bool               `json:"accepted"`
	ThrottleLevel string             `json:"throttle_level"` // "none", "soft", "hard"
	Messages      []string           `json:"messages"`
	Budgets       []BudgetStatus     `json:"budgets"`
}

// BudgetStatus holds per-budget status.
type BudgetStatus struct {
	BudgetID        string  `json:"budget_id"`
	AgentID         string  `json:"agent_id,omitempty"`
	TotalBudget     float64 `json:"total_budget"`
	SpentThisPeriod float64 `json:"spent_this_period"`
	Remaining       float64 `json:"remaining"`
	PercentageUsed  float64 `json:"percentage_used"`
	SoftThreshold   int     `json:"soft_threshold"`
	HardThreshold   int     `json:"hard_threshold"`
	ThrottleState   string  `json:"throttle_state"` // "ok", "soft", "hard"
}

// Engine evaluates budgets against cost events.
type Engine struct {
	budgetStore BudgetStoreIface
	eventStore  EventStoreIface
	alertStore  AlertStoreIface
	throttleMgr *ThrottleManager
}

// NewEngine creates a new budget check engine.
func NewEngine(budgetStore BudgetStoreIface, eventStore EventStoreIface, alertStore AlertStoreIface, throttleMgr *ThrottleManager) *Engine {
	return &Engine{
		budgetStore: budgetStore,
		eventStore:  eventStore,
		alertStore:  alertStore,
		throttleMgr: throttleMgr,
	}
}

// CheckBudgets evaluates all active budgets for a tenant/agent after a cost event.
func (e *Engine) CheckBudgets(ctx context.Context, tenantID, agentID string, costUSD float64) (*BudgetCheckResult, error) {
	budgets, err := e.budgetStore.ListActiveByTenant(ctx, tenantID, agentID)
	if err != nil {
		return nil, fmt.Errorf("listing active budgets: %w", err)
	}

	if len(budgets) == 0 {
		return &BudgetCheckResult{
			Accepted:      true,
			ThrottleLevel: "none",
			Messages:      []string{"no active budgets configured"},
		}, nil
	}

	// Check for manual hard throttle
	throttleState := e.throttleMgr.GetState(tenantID)
	if throttleState == "hard" {
		return &BudgetCheckResult{
			Accepted:      false,
			ThrottleLevel: "hard",
			Messages:      []string{"spending is hard-throttled by manual override"},
		}, nil
	}

	var result = &BudgetCheckResult{
		Accepted:      true,
		ThrottleLevel: "none",
		Messages:      []string{},
		Budgets:       []BudgetStatus{},
	}

	for _, b := range budgets {
		spent, err := e.getSpentForBudget(ctx, &b, agentID)
		if err != nil {
			result.Messages = append(result.Messages, fmt.Sprintf("budget %s: error calculating spend: %v", b.ID, err))
			continue
		}

		// Add the new cost
		spent += costUSD
		percentageUsed := 0.0
		if b.BudgetAmount > 0 {
			percentageUsed = (spent / b.BudgetAmount) * 100
		}
		remaining := b.BudgetAmount - spent

		status := BudgetStatus{
			BudgetID:        b.ID,
			AgentID:         ptrToString(b.AgentID),
			TotalBudget:     b.BudgetAmount,
			SpentThisPeriod: spent,
			Remaining:       remaining,
			PercentageUsed:  percentageUsed,
			SoftThreshold:   b.SoftLimitPct,
			HardThreshold:   b.HardLimitPct,
			ThrottleState:   "ok",
		}

		// Evaluate throttle rules
		if percentageUsed >= float64(b.HardLimitPct) {
			status.ThrottleState = "hard"
			result.Accepted = false
			result.ThrottleLevel = "hard"
			result.Messages = append(result.Messages,
				fmt.Sprintf("budget %s: HARD THROTTLE — %.1f%% of $%.2f spent (%.2f remaining)", b.ID, percentageUsed, b.BudgetAmount, remaining))

			// Create alert if not already created this period
			e.createAlert(ctx, &b, agentID, "budget_exceeded", spent, b.BudgetAmount, percentageUsed, "fatal")
		} else if percentageUsed >= float64(b.SoftLimitPct) {
			if result.ThrottleLevel == "none" {
				result.ThrottleLevel = "soft"
			}
			status.ThrottleState = "soft"
			result.Messages = append(result.Messages,
				fmt.Sprintf("budget %s: SOFT THROTTLE — %.1f%% of $%.2f spent", b.ID, percentageUsed, b.BudgetAmount))

			e.createAlert(ctx, &b, agentID, "soft_limit", spent, b.BudgetAmount, percentageUsed, "warning")
		}

		result.Budgets = append(result.Budgets, status)
	}

	// If soft throttle is active but not hard, budget is still accepted
	if result.ThrottleLevel == "soft" {
		result.Accepted = true
	}

	return result, nil
}

// getSpentForBudget calculates total spent for a budget within its period.
func (e *Engine) getSpentForBudget(ctx context.Context, b *store.CostBudget, agentID string) (float64, error) {
	now := time.Now()
	var from time.Time

	switch b.Period {
	case "daily":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		// ISO week: go back to Monday
		weekday := now.Weekday()
		if weekday == 0 {
			weekday = 7
		}
		from = now.AddDate(0, 0, -(int(weekday) - 1)).Truncate(24 * time.Hour)
	case "monthly":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "quarterly":
		quarter := (now.Month() - 1) / 3
		from = time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, now.Location())
	default:
		from = b.StartedAt
	}

	if agentID != "" && agentID != "null" {
		return e.eventStore.SumCostByTenantAndAgent(ctx, b.TenantID, agentID, from, now)
	}
	return e.eventStore.SumCostByTenant(ctx, b.TenantID, from, now)
}

// createAlert creates an alert if one doesn't already exist for this budget+type today.
func (e *Engine) createAlert(ctx context.Context, b *store.CostBudget, agentID string, alertType string, currentSpend, budgetAmount, percentageUsed float64, severity string) {
	// Check if we already created this alert today
	alerts, _, err := e.alertStore.List(ctx, b.TenantID, "", alertType, nil, 1, 10)
	if err != nil {
		return
	}

	for _, a := range alerts {
		if a.AlertType == alertType && a.PercentageUsed >= percentageUsed-0.1 {
			// Similar alert already exists today
			return
		}
	}

	alert := &store.CostAlert{
		TenantID:     b.TenantID,
		BudgetID:     &b.ID,
		AgentID:      nil,
		AlertType:    alertType,
		CurrentSpend: currentSpend,
		BudgetAmount: budgetAmount,
		PercentageUsed: percentageUsed,
		Severity:     severity,
		IsResolved:   false,
	}
	_ = e.alertStore.Create(ctx, alert)
}

// ptrToString safely converts a *string to string.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}