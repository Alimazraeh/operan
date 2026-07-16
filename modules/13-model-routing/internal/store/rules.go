package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxPool is an interface for pgx pool operations.
// Both *pgxpool.Pool and pgxmock.PgxPoolIface satisfy this.
type PgxPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PGRuleStore implements RuleStore against PostgreSQL.
type PGRuleStore struct {
	pool PgxPool
}

// NewPGRuleStore creates a new PostgreSQL-backed rule store.
func NewPGRuleStore(pool PgxPool) *PGRuleStore {
	return &PGRuleStore{pool: pool}
}

func (s *PGRuleStore) CreateRule(rule *RoutingRule) error {
	rule.ID = uuid.New().String()
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	// Apply schema defaults for zero values (otherwise explicit 0/false overwrites the DEFAULT)
	if rule.MaxTokens == 0 {
		rule.MaxTokens = 4096
	}
	if !rule.FailoverEnabled {
		rule.FailoverEnabled = true
	}
	if !rule.IsActive {
		rule.IsActive = true
	}

	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO routing_rules (id, tenant_id, rule_name, description, task_type, priority,
		 min_cost_threshold, max_latency_ms, max_tokens, failover_enabled, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		rule.ID, rule.TenantID, rule.RuleName, rule.Description, rule.TaskType,
		rule.Priority, rule.MinCostThreshold, rule.MaxLatencyMs, rule.MaxTokens,
		rule.FailoverEnabled, rule.IsActive,
	)
	if err != nil {
		return fmt.Errorf("create rule: %w", err)
	}
	return nil
}

func (s *PGRuleStore) GetRule(id, tenantID string) (*RoutingRule, error) {
	var rule RoutingRule
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, tenant_id, rule_name, description, task_type, priority,
		 min_cost_threshold, max_latency_ms, max_tokens, failover_enabled, is_active,
		 created_at, updated_at FROM routing_rules WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(
		&rule.ID, &rule.TenantID, &rule.RuleName, &rule.Description, &rule.TaskType,
		&rule.Priority, &rule.MinCostThreshold, &rule.MaxLatencyMs, &rule.MaxTokens,
		&rule.FailoverEnabled, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("rule not found: %w", err)
		}
		return nil, fmt.Errorf("get rule: %w", err)
	}
	return &rule, nil
}

func (s *PGRuleStore) ListRules(tenantID string, taskType *string, isActive *bool, page, pageSize int) ([]RoutingRule, int, error) {
	where := "tenant_id = $1"
	args := []interface{}{tenantID}
	argIdx := 2

	if taskType != nil && *taskType != "" {
		where += fmt.Sprintf(" AND task_type = $%d", argIdx)
		args = append(args, *taskType)
		argIdx++
	}
	if isActive != nil {
		where += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *isActive)
		argIdx++
	}

	// Count total
	var total int
	err := s.pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT COUNT(*) FROM routing_rules WHERE %s", where), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("list rules count: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := s.pool.Query(context.Background(),
		fmt.Sprintf(`SELECT id, tenant_id, rule_name, description, task_type, priority,
		 min_cost_threshold, max_latency_ms, max_tokens, failover_enabled, is_active,
		 created_at, updated_at FROM routing_rules WHERE %s
		 ORDER BY priority DESC, created_at DESC
		 OFFSET $%d LIMIT $%d`, where, argIdx, argIdx+1),
		append(args, offset, pageSize)...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	var rules []RoutingRule
	for rows.Next() {
		var r RoutingRule
		if err := rows.Scan(
			&r.ID, &r.TenantID, &r.RuleName, &r.Description, &r.TaskType,
			&r.Priority, &r.MinCostThreshold, &r.MaxLatencyMs, &r.MaxTokens,
			&r.FailoverEnabled, &r.IsActive, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, total, nil
}

func (s *PGRuleStore) UpdateRule(rule *RoutingRule) error {
	rule.UpdatedAt = time.Now()
	_, err := s.pool.Exec(context.Background(),
		`UPDATE routing_rules SET rule_name=$1, description=$2, task_type=$3, priority=$4,
		 min_cost_threshold=$5, max_latency_ms=$6, max_tokens=$7, failover_enabled=$8,
		 is_active=$9, updated_at=$10
		 WHERE id=$11 AND tenant_id=$12`,
		rule.RuleName, rule.Description, rule.TaskType, rule.Priority,
		rule.MinCostThreshold, rule.MaxLatencyMs, rule.MaxTokens,
		rule.FailoverEnabled, rule.IsActive, rule.UpdatedAt, rule.ID, rule.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update rule: %w", err)
	}
	return nil
}

func (s *PGRuleStore) DeleteRule(id, tenantID string) error {
	_, err := s.pool.Exec(context.Background(),
		`DELETE FROM routing_rules WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

func (s *PGRuleStore) AddModelToRule(model *RoutingRuleModel) error {
	model.ID = uuid.New().String()
	// Apply schema defaults for zero values
	if model.LatencyWeight == 0 {
		model.LatencyWeight = 50
	}
	if model.ReliabilityWeight == 0 {
		model.ReliabilityWeight = 50
	}
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO routing_rule_models (id, tenant_id, rule_id, model_id,
		 capability_score, cost_weight, latency_weight, reliability_weight)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		model.ID, model.TenantID, model.RuleID, model.ModelID,
		model.CapabilityScore, model.CostWeight, model.LatencyWeight, model.ReliabilityWeight,
	)
	if err != nil {
		return fmt.Errorf("add model to rule: %w", err)
	}
	return nil
}

func (s *PGRuleStore) GetModelsForRule(ruleID string) ([]RoutingRuleModel, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, tenant_id, rule_id, model_id, capability_score, cost_weight,
		 latency_weight, reliability_weight, created_at
		 FROM routing_rule_models WHERE rule_id = $1`, ruleID,
	)
	if err != nil {
		return nil, fmt.Errorf("get models for rule: %w", err)
	}
	defer rows.Close()

	var models []RoutingRuleModel
	for rows.Next() {
		var m RoutingRuleModel
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.RuleID, &m.ModelID, &m.CapabilityScore,
			&m.CostWeight, &m.LatencyWeight, &m.ReliabilityWeight, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		models = append(models, m)
	}
	return models, nil
}

// RuleWithModels is a rule paired with its model candidates for the engine.
type RuleWithModels struct {
	RuleID           string
	RuleName         string
	TaskType         string
	Priority         int
	MaxLatencyMs     int
	MaxTokens        int
	FailoverEnabled  bool
	MinCostThreshold float64
	Models           []RuleModelCandidate
}

// RuleModelCandidate is a lightweight model entry for engine use.
type RuleModelCandidate struct {
	ModelID           string
	CapabilityScore   float64
	CostWeight        float64
	LatencyWeight     float64
	ReliabilityWeight float64
}

// ListActiveRulesByTask returns active rules matching the task type with their models.
func (s *PGRuleStore) ListActiveRulesByTask(tenantID, taskType string) ([]RuleWithModels, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, rule_name, task_type, priority, max_latency_ms, max_tokens,
		 failover_enabled, min_cost_threshold
		 FROM routing_rules
		 WHERE tenant_id = $1 AND task_type = $2 AND is_active = true
		 ORDER BY priority DESC`,
		tenantID, taskType,
	)
	if err != nil {
		return nil, fmt.Errorf("list active rules: %w", err)
	}
	defer rows.Close()

	var rules []RuleWithModels
	for rows.Next() {
		var rw RuleWithModels
		if err := rows.Scan(
			&rw.RuleID, &rw.RuleName, &rw.TaskType, &rw.Priority,
			&rw.MaxLatencyMs, &rw.MaxTokens, &rw.FailoverEnabled, &rw.MinCostThreshold,
		); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}

		// Fetch models for this rule
		modelRows, err := s.pool.Query(context.Background(),
			`SELECT model_id, capability_score, cost_weight, latency_weight, reliability_weight
			 FROM routing_rule_models WHERE rule_id = $1`, rw.RuleID,
		)
		if err != nil {
			return nil, fmt.Errorf("list rule models: %w", err)
		}

		for modelRows.Next() {
			var mc RuleModelCandidate
			if err := modelRows.Scan(
				&mc.ModelID, &mc.CapabilityScore, &mc.CostWeight,
				&mc.LatencyWeight, &mc.ReliabilityWeight,
			); err != nil {
				modelRows.Close()
				return nil, fmt.Errorf("scan model candidate: %w", err)
			}
			rw.Models = append(rw.Models, mc)
		}
		modelRows.Close()

		rules = append(rules, rw)
	}
	return rules, nil
}