package store

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newTestRuleStore(t *testing.T) (*PGRuleStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewPGRuleStore(mockPool), mockPool
}

// ─── PGRuleStore ───

func TestPGRuleStore_CreateRule(t *testing.T) {
	store, mockPool := newTestRuleStore(t)
	rule := &RoutingRule{
		TenantID:   "tenant-1",
		RuleName:   "test-rule",
		TaskType:   "chat",
		Priority:   50,
		MaxLatencyMs: 5000,
	}

	// INSERT has 11 args: id, tenant_id, rule_name, description, task_type,
	// priority, min_cost_threshold, max_latency_ms, max_tokens,
	// failover_enabled, is_active
	mockPool.ExpectExec("INSERT INTO routing_rules").
		WithArgs(pgxmock.AnyArg(), "tenant-1", "test-rule", "", "chat", 50,
			0.0, 5000, 4096, true, true).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.CreateRule(rule)
	assert.NoError(t, err)
	assert.NotEmpty(t, rule.ID)
}

func TestPGRuleStore_CreateRule_Fail(t *testing.T) {
	store, mockPool := newTestRuleStore(t)
	rule := &RoutingRule{
		TenantID: "tenant-1", RuleName: "test", TaskType: "chat",
	}

	mockPool.ExpectExec("INSERT INTO routing_rules").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	err := store.CreateRule(rule)
	assert.Error(t, err)
}

func TestPGRuleStore_GetRule(t *testing.T) {
	store, mockPool := newTestRuleStore(t)
	now := time.Now()

	mockPool.ExpectQuery("SELECT.*FROM routing_rules WHERE id = ").
		WithArgs("rule-id-1", "tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "rule_name", "description", "task_type",
			"priority", "min_cost_threshold", "max_latency_ms", "max_tokens",
			"failover_enabled", "is_active", "created_at", "updated_at",
		}).
			AddRow("rule-id-1", "tenant-1", "test-rule", "desc", "chat",
				50, 0.0, 5000, 4096, true, true, now, now))

	rule, err := store.GetRule("rule-id-1", "tenant-1")
	assert.NoError(t, err)
	assert.Equal(t, "rule-id-1", rule.ID)
	assert.Equal(t, "chat", rule.TaskType)
}

func TestPGRuleStore_GetRule_NotFound(t *testing.T) {
	store, mockPool := newTestRuleStore(t)

	mockPool.ExpectQuery("SELECT.*FROM routing_rules WHERE id = ").
		WithArgs("nonexistent", "tenant-1").
		WillReturnError(pgx.ErrNoRows)

	_, err := store.GetRule("nonexistent", "tenant-1")
	assert.Error(t, err)
}

func TestPGRuleStore_ListRules(t *testing.T) {
	store, mockPool := newTestRuleStore(t)
	now := time.Now()

	// Count query
	mockPool.ExpectQuery("SELECT COUNT.*FROM routing_rules WHERE tenant_id = ").
		WithArgs("tenant-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))

	// Data query
	mockPool.ExpectQuery("SELECT.*FROM routing_rules WHERE tenant_id = ").
		WithArgs("tenant-1", 0, 20).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "rule_name", "description", "task_type",
			"priority", "min_cost_threshold", "max_latency_ms", "max_tokens",
			"failover_enabled", "is_active", "created_at", "updated_at",
		}).
			AddRow("r1", "tenant-1", "rule-1", "", "chat", 90, 0, 5000, 4096, true, true, now, now).
			AddRow("r2", "tenant-1", "rule-2", "", "chat", 50, 0, 3000, 2048, true, true, now, now))

	rules, total, err := store.ListRules("tenant-1", nil, nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, rules, 2)
	assert.Equal(t, "r1", rules[0].ID)
}

func TestPGRuleStore_ListRules_WithTaskTypeFilter(t *testing.T) {
	store, mockPool := newTestRuleStore(t)

	// Count query
	mockPool.ExpectQuery("SELECT COUNT.*FROM routing_rules.*task_type = ").
		WithArgs("tenant-1", "summarize").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	// Data query
	mockPool.ExpectQuery("SELECT.*FROM routing_rules.*task_type = ").
		WithArgs("tenant-1", "summarize", 0, 20).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "rule_name", "description", "task_type",
			"priority", "min_cost_threshold", "max_latency_ms", "max_tokens",
			"failover_enabled", "is_active", "created_at", "updated_at",
		}).
			AddRow("r3", "tenant-1", "sum-rule", "", "summarize", 50, 0, 5000, 4096, true, true, time.Now(), time.Now()))

	taskType := "summarize"
	rules, total, err := store.ListRules("tenant-1", &taskType, nil, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rules, 1)
	assert.Equal(t, "summarize", rules[0].TaskType)
}

func TestPGRuleStore_UpdateRule(t *testing.T) {
	store, mockPool := newTestRuleStore(t)

	// UPDATE has 12 args: rule_name, description, task_type, priority,
	// min_cost_threshold, max_latency_ms, max_tokens, failover_enabled,
	// is_active, updated_at, id, tenant_id
	mockPool.ExpectExec("UPDATE routing_rules").
		WithArgs("updated-name", "", "classify", 75, 0.0, 5000, 4096, true, true,
			pgxmock.AnyArg(), "rule-1", "tenant-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := store.UpdateRule(&RoutingRule{
		ID:              "rule-1", TenantID: "tenant-1",
		RuleName:        "updated-name", TaskType: "classify", Priority: 75,
		MaxLatencyMs:    5000, MaxTokens: 4096,
		FailoverEnabled: true,
		IsActive:        true,
	})
	assert.NoError(t, err)
}

func TestPGRuleStore_DeleteRule(t *testing.T) {
	store, mockPool := newTestRuleStore(t)

	// DELETE has 2 args: id, tenant_id
	mockPool.ExpectExec("DELETE FROM routing_rules").
		WithArgs("rule-1", "tenant-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err := store.DeleteRule("rule-1", "tenant-1")
	assert.NoError(t, err)
}

func TestPGRuleStore_AddModelToRule(t *testing.T) {
	store, mockPool := newTestRuleStore(t)
	model := &RoutingRuleModel{
		TenantID: "tenant-1", RuleID: "rule-1",
		ModelID: "gpt-4o", CapabilityScore: 95, CostWeight: 30,
	}

	// INSERT has 8 args: id, tenant_id, rule_id, model_id,
	// capability_score, cost_weight, latency_weight, reliability_weight
	mockPool.ExpectExec("INSERT INTO routing_rule_models").
		WithArgs(pgxmock.AnyArg(), "tenant-1", "rule-1", "gpt-4o", 95.0, 30.0, 50.0,
			pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := store.AddModelToRule(model)
	assert.NoError(t, err)
	assert.NotEmpty(t, model.ID)
}

func TestPGRuleStore_GetModelsForRule(t *testing.T) {
	store, mockPool := newTestRuleStore(t)

	// SELECT has 1 arg: rule_id
	mockPool.ExpectQuery("SELECT.*FROM routing_rule_models WHERE rule_id = ").
		WithArgs("rule-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "tenant_id", "rule_id", "model_id", "capability_score",
			"cost_weight", "latency_weight", "reliability_weight", "created_at",
		}).
			AddRow("m1", "tenant-1", "rule-1", "gpt-4o", 95.0, 30.0, 40.0, 90.0, time.Now()).
			AddRow("m2", "tenant-1", "rule-1", "gpt-3.5", 70.0, 10.0, 20.0, 80.0, time.Now()))

	models, err := store.GetModelsForRule("rule-1")
	assert.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "gpt-4o", models[0].ModelID)
}