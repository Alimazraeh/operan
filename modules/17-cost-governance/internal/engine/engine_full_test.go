package engine

import (
	"context"
	"testing"
	"time"

	"github.com/operan/cost-governance/internal/store"
	"github.com/stretchr/testify/assert"
)

// ─── Mock stores for engine unit tests ───

type mockBudgetStore struct {
	budgets []store.CostBudget
}

func (m *mockBudgetStore) Create(ctx context.Context, b *store.CostBudget) error {
	return nil
}
func (m *mockBudgetStore) GetByID(ctx context.Context, id string) (*store.CostBudget, error) {
	for _, b := range m.budgets {
		if b.ID == id {
			return &b, nil
		}
	}
	return nil, store.ErrNotFound
}
func (m *mockBudgetStore) List(ctx context.Context, tenantID, agentID string, isActive *bool, page, pageSize int) ([]store.CostBudget, int, error) {
	return nil, 0, nil
}
func (m *mockBudgetStore) Update(ctx context.Context, id string, agentID *string, description *string, budgetAmount *float64, softLimitPct *int, hardLimitPct *int, isActive *bool) (*store.CostBudget, error) {
	return nil, nil
}
func (m *mockBudgetStore) Delete(ctx context.Context, id string) error { return nil }
func (m *mockBudgetStore) ListActiveByTenant(ctx context.Context, tenantID, agentID string) ([]store.CostBudget, error) {
	var out []store.CostBudget
	for _, b := range m.budgets {
		if b.TenantID == tenantID && b.IsActive && b.EndedAt == nil && b.StartedAt.Before(time.Now()) {
			if agentID == "" {
				// Tenant-wide: only return budgets where agent_id is nil
				if b.AgentID == nil {
					out = append(out, b)
				}
			} else if b.AgentID != nil && *b.AgentID == agentID {
				// Agent-specific: match
				out = append(out, b)
			} else if b.AgentID == nil && agentID != "" {
				// Also include tenant-wide when agent is requested
				out = append(out, b)
			}
		}
	}
	return out, nil
}

type mockEventStore struct {
	spendByTenant map[string]float64
	spendByAgent  map[string]float64
}

func (m *mockEventStore) Create(ctx context.Context, e *store.CostEvent) error { return nil }
func (m *mockEventStore) List(ctx context.Context, tenantID, agentID, sourceModule string, page, pageSize int) ([]store.CostEvent, int, error) {
	return nil, 0, nil
}
func (m *mockEventStore) SumCostByTenant(ctx context.Context, tenantID string, from, to time.Time) (float64, error) {
	return m.spendByTenant[tenantID], nil
}
func (m *mockEventStore) SumCostByTenantAndAgent(ctx context.Context, tenantID, agentID string, from, to time.Time) (float64, error) {
	return m.spendByAgent[tenantID+"::"+agentID], nil
}
func (m *mockEventStore) GetTotalSpent(ctx context.Context, tenantID string) (float64, error) { return 0, nil }

type mockAlertStore struct {
	created []*store.CostAlert
}

func (m *mockAlertStore) Create(ctx context.Context, a *store.CostAlert) error {
	m.created = append(m.created, a)
	return nil
}
func (m *mockAlertStore) List(ctx context.Context, tenantID, severity, alertType string, isResolved *bool, page, pageSize int) ([]store.CostAlert, int, error) {
	return nil, 0, nil
}
func (m *mockAlertStore) UpdateResolved(ctx context.Context, id string, resolved bool) error { return nil }

// newTestEngine creates an engine with mock stores
func newTestEngine() (*Engine, *mockBudgetStore, *mockEventStore, *mockAlertStore, *ThrottleManager) {
	bm := &mockBudgetStore{}
	em := &mockEventStore{
		spendByTenant: make(map[string]float64),
		spendByAgent:  make(map[string]float64),
	}
	am := &mockAlertStore{}
	tm := NewThrottleManager()
	e := NewEngine(bm, em, am, tm)
	return e, bm, em, am, tm
}

// ─── Engine CheckBudgets tests ───

func TestEngine_CheckBudgets_NoBudgets(t *testing.T) {
	e, _, _, _, _ := newTestEngine()

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "agent-1", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "none", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 0)
	assert.Contains(t, result.Messages[0], "no active budgets")
}

func TestEngine_CheckBudgets_BudgetOk(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()
	_ = em // used implicitly via e.eventStore

	budget := store.CostBudget{
		ID:           "b1",
		TenantID:     "tenant-1",
		Description:  ptrStr("Monthly budget"),
		BudgetAmount: 100.0,
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
		IsActive:     true,
		StartedAt:    time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "none", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "ok", result.Budgets[0].ThrottleState)
	assert.True(t, result.Budgets[0].PercentageUsed > 0)
}

func TestEngine_CheckBudgets_SoftThrottle(t *testing.T) {
	e, bm, em, am, _ := newTestEngine()

	budget := store.CostBudget{
		ID:           "b1",
		TenantID:     "tenant-1",
		Description:  ptrStr("Monthly budget"),
		BudgetAmount: 100.0,
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
		IsActive:     true,
		StartedAt:    time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	// agent_id is "agent-1" so engine calls SumCostByTenantAndAgent
	em.spendByAgent["tenant-1::agent-1"] = 85.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "agent-1", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "soft", result.Budgets[0].ThrottleState)
	assert.Len(t, am.created, 1)
	assert.Equal(t, "soft_limit", am.created[0].AlertType)
	assert.Equal(t, "warning", am.created[0].Severity)
}

func TestEngine_CheckBudgets_HardThrottle(t *testing.T) {
	e, bm, em, am, _ := newTestEngine()

	budget := store.CostBudget{
		ID:           "b1",
		TenantID:     "tenant-1",
		Description:  ptrStr("Monthly budget"),
		BudgetAmount: 100.0,
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
		IsActive:     true,
		StartedAt:    time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 96.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "hard", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "hard", result.Budgets[0].ThrottleState)
	assert.Len(t, am.created, 1)
	assert.Equal(t, "budget_exceeded", am.created[0].AlertType)
	assert.Equal(t, "fatal", am.created[0].Severity)
}

func TestEngine_CheckBudgets_ExactHardLimit(t *testing.T) {
	e, bm, em, am, _ := newTestEngine()

	budget := store.CostBudget{
		ID:           "b1",
		TenantID:     "tenant-1",
		Description:  ptrStr("Monthly budget"),
		BudgetAmount: 100.0,
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
		IsActive:     true,
		StartedAt:    time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 95.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "hard", result.ThrottleLevel)
	assert.Len(t, am.created, 1)
}

func TestEngine_CheckBudgets_AgentScoping(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID:           "b1",
		TenantID:     "tenant-1",
		AgentID:      ptrStr("agent-1"),
		Description:  ptrStr("Agent-specific budget"),
		BudgetAmount: 50.0,
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
		IsActive:     true,
		StartedAt:    time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByAgent["tenant-1::agent-1"] = 45.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "agent-1", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Len(t, result.Budgets, 1)
}

func TestEngine_CheckBudgets_MultiBudgetOk(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	b1 := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Budget 1"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	b2 := store.CostBudget{
		ID: "b2", TenantID: "tenant-1", Description: ptrStr("Budget 2"),
		BudgetAmount: 500.0, Period: "quarterly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, b1, b2)

	em.spendByTenant["tenant-1"] = 10.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "none", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 2)
	for _, bs := range result.Budgets {
		assert.Equal(t, "ok", bs.ThrottleState)
	}
}

func TestEngine_CheckBudgets_MultiBudgetMixed(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	b1 := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Budget 1 (soft)"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	b2 := store.CostBudget{
		ID: "b2", TenantID: "tenant-1", Description: ptrStr("Budget 2 (ok)"),
		BudgetAmount: 500.0, Period: "quarterly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, b1, b2)

	em.spendByTenant["tenant-1"] = 85.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 2)

	for _, bs := range result.Budgets {
		if bs.BudgetID == "b1" {
			assert.Equal(t, "soft", bs.ThrottleState)
			assert.True(t, bs.PercentageUsed >= 80)
		} else if bs.BudgetID == "b2" {
			assert.Equal(t, "ok", bs.ThrottleState)
			assert.True(t, bs.PercentageUsed < 80)
		}
	}
}

func TestEngine_CheckBudgets_MultiBudgetBothHard(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	b1 := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Budget 1"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	b2 := store.CostBudget{
		ID: "b2", TenantID: "tenant-1", Description: ptrStr("Budget 2"),
		BudgetAmount: 200.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, b1, b2)

	em.spendByTenant["tenant-1"] = 196.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "hard", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 2)
	for _, bs := range result.Budgets {
		assert.Equal(t, "hard", bs.ThrottleState)
	}
}

func TestEngine_CheckBudgets_HardThrottleRejects(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Monthly budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)
	em.spendByTenant["tenant-1"] = 50.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)

	// Now set hard throttle manually
	e.throttleMgr.SetState("tenant-1", "hard")

	result2, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.False(t, result2.Accepted)
	assert.Equal(t, "hard", result2.ThrottleLevel)
	assert.Contains(t, result2.Messages[0], "hard-throttled")
	assert.Len(t, result2.Budgets, 0)
}

func TestEngine_CheckBudgets_ExactSoftThreshold(t *testing.T) {
	e, bm, em, am, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Monthly budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 80.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
	assert.Len(t, am.created, 1)
	assert.Equal(t, "soft_limit", am.created[0].AlertType)
}

func TestEngine_CheckBudgets_UnderSoftThreshold(t *testing.T) {
	e, bm, em, am, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Monthly budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 77.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "none", result.ThrottleLevel)
	assert.Len(t, am.created, 0)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "ok", result.Budgets[0].ThrottleState)
}

func TestEngine_CheckBudgets_ExceedsBudget(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Monthly budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 110.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.False(t, result.Accepted)
	assert.Equal(t, "hard", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "hard", result.Budgets[0].ThrottleState)
	assert.True(t, result.Budgets[0].PercentageUsed > 100)
	assert.True(t, result.Budgets[0].Remaining < 0)
}

func TestEngine_CheckBudgets_BudgetStatusFields(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Test budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 40.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)

	bs := result.Budgets[0]
	assert.Equal(t, "b1", bs.BudgetID)
	assert.Equal(t, 100.0, bs.TotalBudget)
	assert.Equal(t, 41.0, bs.SpentThisPeriod)
	assert.Equal(t, 59.0, bs.Remaining)
	assert.Equal(t, 80, bs.SoftThreshold)
	assert.Equal(t, 95, bs.HardThreshold)
}

func TestEngine_CheckBudgets_ZeroBudget(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Zero budget"),
		BudgetAmount: 0.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 10.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	// Zero-budget: engine skips percentage calc (division by zero guard)
	assert.True(t, result.Accepted)
	assert.Equal(t, "none", result.ThrottleLevel)
}

func TestEngine_GetSpentForBudget(t *testing.T) {
	e, _, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", BudgetAmount: 100.0,
		Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		StartedAt: time.Now().Add(-30 * time.Hour),
	}

	em.spendByTenant["tenant-1"] = 25.0

	spent, err := e.getSpentForBudget(context.Background(), &budget, "")
	assert.NoError(t, err)
	assert.Equal(t, 25.0, spent)
}

func TestEngine_GetSpentForBudget_Agent(t *testing.T) {
	e, _, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", BudgetAmount: 100.0,
		Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		StartedAt: time.Now().Add(-30 * time.Hour),
	}

	em.spendByAgent["tenant-1::agent-1"] = 15.0

	spent, err := e.getSpentForBudget(context.Background(), &budget, "agent-1")
	assert.NoError(t, err)
	assert.Equal(t, 15.0, spent)
}

func TestEngine_CheckBudgets_InactiveBudgetIgnored(t *testing.T) {
	e, bm, _, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Inactive budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: false, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Len(t, result.Budgets, 0)
}

func TestEngine_CheckBudgets_FutureBudgetIgnored(t *testing.T) {
	e, bm, _, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Future budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Len(t, result.Budgets, 0)
}

func TestEngine_CheckBudgets_EndedBudgetIgnored(t *testing.T) {
	e, bm, _, _, _ := newTestEngine()

	ended := time.Now().Add(-1 * time.Hour)
	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Ended budget"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-48 * time.Hour), EndedAt: &ended,
	}
	bm.budgets = append(bm.budgets, budget)

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Len(t, result.Budgets, 0)
}

func TestEngine_CheckBudgets_WeeklyPeriod(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Weekly budget"),
		BudgetAmount: 50.0, Period: "weekly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-7 * 24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 45.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
}

func TestEngine_CheckBudgets_DailyPeriod(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Daily budget"),
		BudgetAmount: 20.0, Period: "daily", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-2 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 16.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
}

func TestEngine_CheckBudgets_QuarterlyPeriod(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Quarterly budget"),
		BudgetAmount: 1000.0, Period: "quarterly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-60 * 24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 850.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
}

func TestEngine_CheckBudgets_CustomThresholds(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Custom thresholds"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 50, HardLimitPct: 70,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 60.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)

	em.spendByTenant["tenant-1"] = 75.0
	result2, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.False(t, result2.Accepted)
	assert.Equal(t, "hard", result2.ThrottleLevel)
}

func TestEngine_CheckBudgets_TightThresholds(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Tight thresholds"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 90, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 91.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Equal(t, "soft", result.ThrottleLevel)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "soft", result.Budgets[0].ThrottleState)
}

func TestEngine_CheckBudgets_NilAgentIDBudgets(t *testing.T) {
	e, bm, em, _, _ := newTestEngine()

	budget := store.CostBudget{
		ID: "b1", TenantID: "tenant-1", Description: ptrStr("Tenant-only"),
		BudgetAmount: 100.0, Period: "monthly", SoftLimitPct: 80, HardLimitPct: 95,
		IsActive: true, StartedAt: time.Now().Add(-24 * time.Hour),
	}
	bm.budgets = append(bm.budgets, budget)

	em.spendByTenant["tenant-1"] = 20.0

	result, err := e.CheckBudgets(context.Background(), "tenant-1", "", 1.0)
	assert.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.Len(t, result.Budgets, 1)
	assert.Equal(t, "", result.Budgets[0].AgentID)
}

func ptrStr(s string) *string { return &s }