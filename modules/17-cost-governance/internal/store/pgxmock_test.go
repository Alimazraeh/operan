package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newTestStore(t *testing.T) (*BudgetStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewBudgetStore(mockPool), mockPool
}

// ─── BudgetStore ───

func TestBudgetStore_Create(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	desc := "Monthly budget"
	budget := &CostBudget{
		TenantID:     "tenant-1",
		Description:  &desc,
		BudgetAmount: 500.00,
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
	}

	mockPool.ExpectQuery("INSERT INTO cost_budgets").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg(), 500.00, "USD",
			"monthly", 80, 95, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New().String(), time.Now(), time.Now()))

	err := store.Create(ctx, budget)
	assert.NoError(t, err)
	assert.NotEmpty(t, budget.ID)
}

func TestBudgetStore_Create_Fail(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("INSERT INTO cost_budgets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	err := store.Create(ctx, &CostBudget{
		TenantID:     "tenant-1",
		BudgetAmount: 100.00,
		Period:       "daily",
	})
	assert.Error(t, err)
}

func TestBudgetStore_GetByID(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	now := time.Now()

	// Use pgtype.Text with Valid=true for non-null columns, Valid=false for null columns
	mockPool.ExpectQuery("SELECT.*FROM cost_budgets WHERE id = ").
		WithArgs("test-budget-id").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}).
			AddRow("test-budget-id", "tenant-1",
				pgtype.Text{String: "", Valid: false},
				pgtype.Text{String: "", Valid: false},
				500.0, "USD", "monthly", 80, 95, true, now, now, now, nil))

	budget, err := store.GetByID(ctx, "test-budget-id")
	assert.NoError(t, err)
	assert.Equal(t, "test-budget-id", budget.ID)
	assert.Equal(t, "tenant-1", budget.TenantID)
	assert.Nil(t, budget.AgentID)
	assert.Nil(t, budget.Description)
}

func TestBudgetStore_GetByID_NotFound(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT.*FROM cost_budgets WHERE id = ").
		WithArgs("nonexistent").
		WillReturnError(assert.AnError)

	_, err := store.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestBudgetStore_List(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*FROM cost_budgets").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mockPool.ExpectQuery("SELECT.*FROM cost_budgets.*WHERE tenant_id = ").
		WithArgs("tenant-1", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}).
			AddRow("b1", "tenant-1",
				pgtype.Text{String: "", Valid: false},
				pgtype.Text{String: "", Valid: false},
				500.0, "USD", "monthly", 80, 95, true, now, now, now, nil).
			AddRow("b2", "tenant-1",
				pgtype.Text{String: "agent-5", Valid: true},
				pgtype.Text{String: "Agent 5 budget", Valid: true},
				1000.0, "USD", "quarterly", 80, 95, true, now, now, now, nil))

	budgets, total, err := store.List(ctx, "tenant-1", "", nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, budgets, 2)
	assert.Equal(t, "b1", budgets[0].ID)
	assert.Nil(t, budgets[0].AgentID)
	assert.Nil(t, budgets[0].Description)
	assert.NotNil(t, budgets[1].AgentID)
	assert.Equal(t, "agent-5", *budgets[1].AgentID)
}

func TestBudgetStore_List_WithAgentFilter(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*FROM cost_budgets.*WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-42").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*FROM cost_budgets.*WHERE tenant_id = ").
		WithArgs("tenant-1", "agent-42", 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}).
			AddRow("b3", "tenant-1",
				pgtype.Text{String: "agent-42", Valid: true},
				pgtype.Text{String: "Agent 42 budget", Valid: true},
				250.0, "USD", "weekly", 80, 95, true, now, now, now, nil))

	budgets, total, err := store.List(ctx, "tenant-1", "agent-42", nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, budgets, 1)
}

func TestBudgetStore_List_WithActiveFilter(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	active := true

	mockPool.ExpectQuery("SELECT COUNT.*FROM cost_budgets").
		WithArgs("tenant-1", true).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))

	mockPool.ExpectQuery("SELECT.*FROM cost_budgets.*WHERE tenant_id = ").
		WithArgs("tenant-1", true, 20, 0).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}))

	budgets, total, err := store.List(ctx, "tenant-1", "", &active, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, budgets, 0)
}

func TestBudgetStore_List_PaginationReset(t *testing.T) {
	store, mockPool := newTestStore(t)
	ctx := context.Background()

	// Set up a no-op mock since List always runs a query even with invalid pagination
	mockPool.ExpectQuery("SELECT").WillReturnError(assert.AnError)

	// Invalid page → should reset to 1, then fail on query
	_, _, err := store.List(ctx, "tenant-1", "", nil, 0, 20)
	assert.Error(t, err)

	// Invalid page size → should reset to 20, then fail on query
	_, _, err = store.List(ctx, "tenant-1", "", nil, 1, 0)
	assert.Error(t, err)
}

func TestBudgetStore_ListActiveByTenant(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*cost_budgets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}).
			AddRow("b1", "tenant-1",
				pgtype.Text{String: "", Valid: false},
				pgtype.Text{String: "", Valid: false},
				500.0, "USD", "monthly", 80, 95, true, now, now, now, nil).
			AddRow("b2", "tenant-1",
				pgtype.Text{String: "agent-7", Valid: true},
				pgtype.Text{String: "Agent 7 budget", Valid: true},
				1000.0, "USD", "quarterly", 80, 95, true, now, now, now, nil))

	budgets, err := store.ListActiveByTenant(ctx, "tenant-1", "")
	assert.NoError(t, err)
	assert.Len(t, budgets, 2)
}

func TestBudgetStore_ListActiveByTenant_WithAgent(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*cost_budgets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}).
			AddRow("b3", "tenant-1",
				pgtype.Text{String: "agent-42", Valid: true},
				pgtype.Text{String: "Agent 42 budget", Valid: true},
				250.0, "USD", "daily", 80, 95, true, now, now, now, nil))

	budgets, err := store.ListActiveByTenant(ctx, "tenant-1", "agent-42")
	assert.NoError(t, err)
	assert.Len(t, budgets, 1)
}

func TestBudgetStore_Update(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	now := time.Now()
	newDesc := "Updated budget"
	newAmt := 750.00
	newSoft := 75
	newHard := 90
	newActive := false
	newAgent := "agent-99"

	// First GetByID call - use pgtype.Text for nullable columns
	mockPool.ExpectQuery("SELECT.*FROM cost_budgets WHERE id = ").
		WithArgs("test-budget-id").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "description", "budget_amount",
			"currency", "period", "soft_limit_pct", "hard_limit_pct",
			"is_active", "created_at", "updated_at", "started_at", "ended_at",
		}).
			AddRow("test-budget-id", "tenant-1",
				pgtype.Text{String: "", Valid: false},
				pgtype.Text{String: "", Valid: false},
				500.0, "USD", "monthly", 80, 95, true, now, now, now, nil))

	// Then UPDATE
	mockPool.ExpectExec("UPDATE cost_budgets").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), "test-budget-id").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	budget, err := store.Update(ctx, "test-budget-id", &newAgent, &newDesc, &newAmt, &newSoft, &newHard, &newActive)
	assert.NoError(t, err)
	assert.Equal(t, "test-budget-id", budget.ID)
}

func TestBudgetStore_Update_NotFound(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()
	newDesc := "Updated"

	mockPool.ExpectQuery("SELECT.*FROM cost_budgets WHERE id = ").
		WithArgs("nonexistent").
		WillReturnError(assert.AnError)

	_, err := store.Update(ctx, "nonexistent", nil, &newDesc, nil, nil, nil, nil)
	assert.Error(t, err)
}

func TestBudgetStore_Delete(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM cost_budgets").
		WithArgs("test-budget-id").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := store.Delete(ctx, "test-budget-id")
	assert.NoError(t, err)
}

func TestBudgetStore_Delete_NotFound(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM cost_budgets").
		WithArgs("nonexistent").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err := store.Delete(ctx, "nonexistent")
	assert.Equal(t, ErrNotFound, err)
}

func TestBudgetStore_Delete_Error(t *testing.T) {
	store, mockPool := newTestStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("DELETE FROM cost_budgets").
		WithArgs("test-budget-id").
		WillReturnError(assert.AnError)

	err := store.Delete(ctx, "test-budget-id")
	assert.Error(t, err)
}

// ─── CostEventStore ───

func newEventStore(t *testing.T) (*CostEventStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewCostEventStore(mockPool), mockPool
}

func TestCostEventStore_Create(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	model := "gpt-4"
	ts := time.Now()
	sourceID := "call-123"

	event := &CostEvent{
		TenantID:         "tenant-1",
		AgentID:          ptr("agent-1"),
		SourceModule:     "m12",
		SourceID:         &sourceID,
		ModelName:        &model,
		CostUSD:          0.05,
		PromptTokens:     100,
		CompletionTokens: 50,
		EventType:        "model_call",
		EventTimestamp:   ts,
	}

	mockPool.ExpectQuery("INSERT INTO cost_events").
		WithArgs("tenant-1", pgxmock.AnyArg(), "m12", pgxmock.AnyArg(), pgxmock.AnyArg(),
			0.05, 100, 50, "model_call", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "recorded_at"}).
			AddRow(uuid.New().String(), time.Now()))

	err := store.Create(ctx, event)
	assert.NoError(t, err)
	assert.NotEmpty(t, event.ID)
}

func TestCostEventStore_Create_Fail(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("INSERT INTO cost_events").
		WillReturnError(assert.AnError)

	err := store.Create(ctx, &CostEvent{
		TenantID:       "tenant-1",
		SourceModule:   "m12",
		CostUSD:        1.0,
		EventTimestamp: ts,
	})
	assert.Error(t, err)
}

func TestCostEventStore_SumCostByTenant(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)

	mockPool.ExpectQuery("SELECT COALESCE\\(SUM\\(cost_usd\\), 0\\).*FROM cost_events.*WHERE tenant_id = \\$1 AND event_timestamp >= \\$2 AND event_timestamp < \\$3").
		WithArgs("tenant-1", from, to).
		WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(42.50))

	total, err := store.SumCostByTenant(ctx, "tenant-1", from, to)
	assert.NoError(t, err)
	assert.Equal(t, 42.50, total)
}

func TestCostEventStore_SumCostByTenantZero(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)

	mockPool.ExpectQuery("SELECT COALESCE\\(SUM\\(cost_usd\\), 0\\).*FROM cost_events.*WHERE tenant_id = \\$1 AND event_timestamp >= \\$2 AND event_timestamp < \\$3").
		WithArgs("tenant-1", from, to).
		WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(0.00))

	total, err := store.SumCostByTenant(ctx, "tenant-1", from, to)
	assert.NoError(t, err)
	assert.Equal(t, 0.00, total)
}

func TestCostEventStore_SumCostByTenantAndAgent(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)

	mockPool.ExpectQuery("SELECT COALESCE\\(SUM\\(cost_usd\\), 0\\).*FROM cost_events.*WHERE tenant_id = \\$1 AND agent_id = \\$2 AND event_timestamp >= \\$3 AND event_timestamp < \\$4").
		WithArgs("tenant-1", "agent-1", from, to).
		WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(15.75))

	total, err := store.SumCostByTenantAndAgent(ctx, "tenant-1", "agent-1", from, to)
	assert.NoError(t, err)
	assert.Equal(t, 15.75, total)
}

func TestCostEventStore_GetTotalSpent(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()

	mockPool.ExpectQuery("SELECT COALESCE\\(SUM\\(cost_usd\\), 0\\).*FROM cost_events WHERE tenant_id = \\$1").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(125.30))

	total, err := store.GetTotalSpent(ctx, "tenant-1")
	assert.NoError(t, err)
	assert.Equal(t, 125.30, total)
}

func TestCostEventStore_List(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_events.*tenant_id").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))

	mockPool.ExpectQuery("SELECT.*cost_events.*event_timestamp.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "source_module", "source_id", "model_name",
			"cost_usd", "prompt_tokens", "completion_tokens", "event_type",
			"billing_tag", "event_timestamp", "recorded_at",
		}).
			AddRow("e1", "tenant-1",
				pgtype.Text{String: "agent-1", Valid: true},
				"m12",
				pgtype.Text{String: "call-1", Valid: true},
				pgtype.Text{String: "gpt-4", Valid: true},
				0.05, 100, 50, "model_call", pgtype.Text{String: "", Valid: false}, ts, ts).
			AddRow("e2", "tenant-1",
				pgtype.Text{String: "agent-2", Valid: true},
				"m08",
				pgtype.Text{String: "tool-1", Valid: true},
				pgtype.Text{String: "calculator", Valid: true},
				0.01, 0, 0, "tool_execution", pgtype.Text{String: "", Valid: false}, ts, ts).
			AddRow("e3", "tenant-1",
				pgtype.Text{String: "", Valid: false},
				"manual",
				pgtype.Text{String: "", Valid: false},
				pgtype.Text{String: "", Valid: false},
				10.00, 0, 0, "manual_adjustment",
				pgtype.Text{String: "prod", Valid: true}, ts, ts))

	events, total, err := store.List(ctx, "tenant-1", "", "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, events, 3)
	assert.Equal(t, "e1", events[0].ID)
}

func TestCostEventStore_List_WithAgentFilter(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_events.*tenant_id.*agent_id").
		WithArgs("tenant-1", "agent-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*cost_events.*tenant_id.*agent_id.*event_timestamp.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "source_module", "source_id", "model_name",
			"cost_usd", "prompt_tokens", "completion_tokens", "event_type",
			"billing_tag", "event_timestamp", "recorded_at",
		}).
			AddRow("e1", "tenant-1",
				pgtype.Text{String: "agent-1", Valid: true},
				"m12",
				pgtype.Text{String: "call-1", Valid: true},
				pgtype.Text{String: "gpt-4", Valid: true},
				0.05, 100, 50, "model_call", pgtype.Text{String: "", Valid: false}, ts, ts))

	events, total, err := store.List(ctx, "tenant-1", "agent-1", "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, events, 1)
}

func TestCostEventStore_List_WithSourceModuleFilter(t *testing.T) {
	store, mockPool := newEventStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_events.*tenant_id.*source_module").
		WithArgs("tenant-1", "m12").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*cost_events.*tenant_id.*source_module.*event_timestamp.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "agent_id", "source_module", "source_id", "model_name",
			"cost_usd", "prompt_tokens", "completion_tokens", "event_type",
			"billing_tag", "event_timestamp", "recorded_at",
		}).
			AddRow("e1", "tenant-1",
				pgtype.Text{String: "agent-1", Valid: true},
				"m12",
				pgtype.Text{String: "call-1", Valid: true},
				pgtype.Text{String: "gpt-4", Valid: true},
				0.05, 100, 50, "model_call", pgtype.Text{String: "", Valid: false}, ts, ts))

	events, total, err := store.List(ctx, "tenant-1", "", "m12", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, events, 1)
}

// ─── AlertStore ───

func newAlertStore(t *testing.T) (*AlertStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewAlertStore(mockPool), mockPool
}

func TestAlertStore_Create(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	budgetID := "budget-123"

	alert := &CostAlert{
		TenantID:       "tenant-1",
		BudgetID:       &budgetID,
		AlertType:      "soft_limit",
		CurrentSpend:   80.0,
		BudgetAmount:   100.0,
		PercentageUsed: 80.0,
		Severity:       "warning",
	}

	mockPool.ExpectQuery("INSERT INTO cost_alerts").
		WithArgs("tenant-1", pgxmock.AnyArg(), pgxmock.AnyArg(), "soft_limit",
			80.0, 100.0, 80.0, "warning", false).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(uuid.New().String(), time.Now()))

	err := store.Create(ctx, alert)
	assert.NoError(t, err)
	assert.NotEmpty(t, alert.ID)
}

func TestAlertStore_Create_Fail(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	budgetID := "budget-123"

	mockPool.ExpectQuery("INSERT INTO cost_alerts").
		WillReturnError(assert.AnError)

	err := store.Create(ctx, &CostAlert{
		TenantID:       "tenant-1",
		BudgetID:       &budgetID,
		AlertType:      "soft_limit",
		CurrentSpend:   80.0,
		BudgetAmount:   100.0,
		PercentageUsed: 80.0,
		Severity:       "warning",
	})
	assert.Error(t, err)
}

func TestAlertStore_List(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_alerts.*tenant_id").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	mockPool.ExpectQuery("SELECT.*cost_alerts.*tenant_id.*created_at.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "budget_id", "agent_id", "alert_type",
			"current_spend", "budget_amount", "percentage_used", "severity",
			"is_resolved", "resolved_at", "created_at",
		}).
			AddRow("a1", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "", Valid: false},
				"soft_limit",
				80.0, 100.0, 80.0, "warning", false, pgtype.Timestamptz{Valid: false}, ts).
			AddRow("a2", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "agent-1", Valid: true},
				"hard_limit",
				95.0, 100.0, 95.0, "critical", false, pgtype.Timestamptz{Valid: false}, ts))

	alerts, total, err := store.List(ctx, "tenant-1", "", "", nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, alerts, 2)
	assert.Equal(t, "a1", alerts[0].ID)
	assert.Equal(t, "soft_limit", alerts[0].AlertType)
}

func TestAlertStore_List_WithSeverityFilter(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_alerts.*tenant_id.*severity").
		WithArgs("tenant-1", "critical").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*cost_alerts.*tenant_id.*severity.*created_at.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "budget_id", "agent_id", "alert_type",
			"current_spend", "budget_amount", "percentage_used", "severity",
			"is_resolved", "resolved_at", "created_at",
		}).
			AddRow("a2", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "agent-1", Valid: true},
				"hard_limit",
				95.0, 100.0, 95.0, "critical", false, pgtype.Timestamptz{Valid: false}, ts))

	alerts, total, err := store.List(ctx, "tenant-1", "critical", "", nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "critical", alerts[0].Severity)
}

func TestAlertStore_List_WithAlertTypeFilter(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_alerts.*tenant_id.*alert_type").
		WithArgs("tenant-1", "budget_exceeded").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*cost_alerts.*tenant_id.*alert_type.*created_at.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "budget_id", "agent_id", "alert_type",
			"current_spend", "budget_amount", "percentage_used", "severity",
			"is_resolved", "resolved_at", "created_at",
		}).
			AddRow("a3", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "", Valid: false},
				"budget_exceeded",
				100.0, 100.0, 100.0, "fatal", false, pgtype.Timestamptz{Valid: false}, ts))

	alerts, total, err := store.List(ctx, "tenant-1", "", "budget_exceeded", nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "budget_exceeded", alerts[0].AlertType)
}

func TestAlertStore_List_WithResolvedFilter(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	ts := time.Now()
	resolved := true

	mockPool.ExpectQuery("SELECT COUNT.*cost_alerts.*tenant_id.*is_resolved").
		WithArgs("tenant-1", true).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*cost_alerts.*tenant_id.*is_resolved.*created_at.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "budget_id", "agent_id", "alert_type",
			"current_spend", "budget_amount", "percentage_used", "severity",
			"is_resolved", "resolved_at", "created_at",
		}).
			AddRow("a4", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "", Valid: false},
				"budget_reset",
				0.0, 100.0, 0.0, "info", true, pgtype.Timestamptz{Time: ts, Valid: true}, ts))

	alerts, total, err := store.List(ctx, "tenant-1", "", "", &resolved, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, alerts, 1)
	assert.True(t, alerts[0].IsResolved)
}

func TestAlertStore_UpdateResolved(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE cost_alerts SET is_resolved = \\$1, resolved_at = \\$2 WHERE id = \\$3").
		WithArgs(true, pgxmock.AnyArg(), "alert-123").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpdateResolved(ctx, "alert-123", true)
	assert.NoError(t, err)
}

func TestAlertStore_UpdateResolved_Error(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()

	mockPool.ExpectExec("UPDATE cost_alerts SET is_resolved = \\$1, resolved_at = \\$2 WHERE id = \\$3").
		WithArgs(true, pgxmock.AnyArg(), "alert-123").
		WillReturnError(assert.AnError)

	err := store.UpdateResolved(ctx, "alert-123", true)
	assert.Error(t, err)
}

func TestAlertStore_List_WithCombinedFilters(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	ts := time.Now()
	resolved := false

	mockPool.ExpectQuery("SELECT COUNT.*cost_alerts.*tenant_id.*severity.*alert_type.*is_resolved").
		WithArgs("tenant-1", "warning", "soft_limit", false).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	mockPool.ExpectQuery("SELECT.*cost_alerts.*tenant_id.*severity.*alert_type.*is_resolved.*created_at.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "budget_id", "agent_id", "alert_type",
			"current_spend", "budget_amount", "percentage_used", "severity",
			"is_resolved", "resolved_at", "created_at",
		}).
			AddRow("a1", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "", Valid: false},
				"soft_limit",
				80.0, 100.0, 80.0, "warning", false, pgtype.Timestamptz{Valid: false}, ts))

	alerts, total, err := store.List(ctx, "tenant-1", "warning", "soft_limit", &resolved, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "soft_limit", alerts[0].AlertType)
	assert.Equal(t, "warning", alerts[0].Severity)
}

func TestAlertStore_List_WithPagination(t *testing.T) {
	store, mockPool := newAlertStore(t)

	ctx := context.Background()
	ts := time.Now()

	mockPool.ExpectQuery("SELECT COUNT.*cost_alerts.*tenant_id").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(50))

	mockPool.ExpectQuery("SELECT.*cost_alerts.*tenant_id.*created_at.*DESC").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "budget_id", "agent_id", "alert_type",
			"current_spend", "budget_amount", "percentage_used", "severity",
			"is_resolved", "resolved_at", "created_at",
		}).
			AddRow("a11", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "", Valid: false},
				"soft_limit",
				80.0, 100.0, 80.0, "warning", false, pgtype.Timestamptz{Valid: false}, ts).
			AddRow("a12", "tenant-1",
				pgtype.Text{String: "budget-1", Valid: true},
				pgtype.Text{String: "", Valid: false},
				"soft_limit",
				80.0, 100.0, 80.0, "warning", false, pgtype.Timestamptz{Valid: false}, ts))

	alerts, total, err := store.List(ctx, "tenant-1", "", "", nil, 5, 10)
	assert.NoError(t, err)
	assert.Equal(t, 50, total)
	assert.Len(t, alerts, 2)
	assert.Equal(t, "a11", alerts[0].ID)
}