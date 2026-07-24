package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// PgxPool is an interface for pgx pool operations.
type PgxPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// BudgetStore handles cost_budgets CRUD.
type BudgetStore struct {
	pool PgxPool
}

// NewBudgetStore creates a new BudgetStore.
func NewBudgetStore(pool PgxPool) *BudgetStore {
	return &BudgetStore{pool: pool}
}

// Create inserts a new budget and returns it with the generated ID.
func (s *BudgetStore) Create(ctx context.Context, b *CostBudget) error {
	b.ID = uuid.New().String()
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now
	b.StartedAt = now
	b.Currency = "USD"

	query := `
		INSERT INTO cost_budgets
			(tenant_id, agent_id, description, budget_amount, currency,
			 period, soft_limit_pct, hard_limit_pct, is_active, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`

	return s.pool.QueryRow(ctx, query,
		b.TenantID, b.AgentID, b.Description, b.BudgetAmount, b.Currency,
		b.Period, b.SoftLimitPct, b.HardLimitPct, b.IsActive, b.StartedAt,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
}

// GetByID returns a budget by ID.
func (s *BudgetStore) GetByID(ctx context.Context, id string) (*CostBudget, error) {
	var b CostBudget
	var agentID, description pgtype.Text
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, agent_id, description, budget_amount, currency,
		       period, soft_limit_pct, hard_limit_pct, is_active, created_at,
		       updated_at, started_at, ended_at
		FROM cost_budgets
		WHERE id = $1`, id).Scan(
		&b.ID, &b.TenantID, &agentID, &description, &b.BudgetAmount,
		&b.Currency, &b.Period, &b.SoftLimitPct, &b.HardLimitPct, &b.IsActive,
		&b.CreatedAt, &b.UpdatedAt, &b.StartedAt, &b.EndedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if agentID.Valid {
		b.AgentID = &agentID.String
	}
	if description.Valid {
		b.Description = &description.String
	}
	return &b, nil
}

// List returns paginated budgets with optional filters.
func (s *BudgetStore) List(ctx context.Context, tenantID, agentID string, isActive *bool, page, pageSize int) ([]CostBudget, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var where string
	var args []interface{}
	argIdx := 1
	where += "WHERE tenant_id = $1"
	args = append(args, tenantID)
	argIdx++

	if agentID != "" {
		where += fmt.Sprintf(" AND agent_id = $%d", argIdx)
		args = append(args, agentID)
		argIdx++
	}
	if isActive != nil {
		where += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *isActive)
		argIdx++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM cost_budgets " + where
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch page
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, agent_id, description, budget_amount, currency,
		       period, soft_limit_pct, hard_limit_pct, is_active, created_at,
		       updated_at, started_at, ended_at
		FROM cost_budgets `+where+` ORDER BY created_at DESC LIMIT $`+fmt.Sprint(argIdx)+` OFFSET $`+fmt.Sprint(argIdx+1)+``,
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []CostBudget
	for rows.Next() {
		var b CostBudget
		var agentID, description pgtype.Text
		if err := rows.Scan(
			&b.ID, &b.TenantID, &agentID, &description, &b.BudgetAmount,
			&b.Currency, &b.Period, &b.SoftLimitPct, &b.HardLimitPct, &b.IsActive,
			&b.CreatedAt, &b.UpdatedAt, &b.StartedAt, &b.EndedAt,
		); err != nil {
			return nil, 0, err
		}
		if agentID.Valid {
			b.AgentID = &agentID.String
		}
		if description.Valid {
			b.Description = &description.String
		}
		items = append(items, b)
	}

	return items, total, nil
}

// Update modifies a budget's fields.
func (s *BudgetStore) Update(ctx context.Context, id string, agentID *string, description *string, budgetAmount *float64, softLimitPct *int, hardLimitPct *int, isActive *bool) (*CostBudget, error) {
	b, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if agentID != nil {
		b.AgentID = agentID
	}
	if description != nil {
		b.Description = description
	}
	if budgetAmount != nil {
		b.BudgetAmount = *budgetAmount
	}
	if softLimitPct != nil {
		b.SoftLimitPct = *softLimitPct
	}
	if hardLimitPct != nil {
		b.HardLimitPct = *hardLimitPct
	}
	if isActive != nil {
		b.IsActive = *isActive
	}
	b.UpdatedAt = time.Now()

	_, err = s.pool.Exec(ctx, `
		UPDATE cost_budgets
		SET agent_id = $1, description = $2, budget_amount = $3, soft_limit_pct = $4, hard_limit_pct = $5, is_active = $6, updated_at = $7
		WHERE id = $8`,
		b.AgentID, b.Description, b.BudgetAmount, b.SoftLimitPct, b.HardLimitPct, b.IsActive, b.UpdatedAt, id,
	)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Delete removes a budget.
func (s *BudgetStore) Delete(ctx context.Context, id string) error {
	result, err := s.pool.Exec(ctx, "DELETE FROM cost_budgets WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListActiveByTenant returns all active budgets (tenant-wide and per-agent) for a tenant.
func (s *BudgetStore) ListActiveByTenant(ctx context.Context, tenantID, agentID string) ([]CostBudget, error) {
	var query string
	var args []interface{}

	if agentID != "" {
		query = `
			SELECT id, tenant_id, agent_id, description, budget_amount, currency,
			       period, soft_limit_pct, hard_limit_pct, is_active, created_at,
			       updated_at, started_at, ended_at
			FROM cost_budgets
			WHERE tenant_id = $1 AND is_active = $2
			  AND (agent_id IS NULL OR agent_id = $3)
			  AND ended_at IS NULL
			  AND started_at <= $4`
		args = []interface{}{tenantID, true, agentID, time.Now()}
	} else {
		query = `
			SELECT id, tenant_id, agent_id, description, budget_amount, currency,
			       period, soft_limit_pct, hard_limit_pct, is_active, created_at,
			       updated_at, started_at, ended_at
			FROM cost_budgets
			WHERE tenant_id = $1 AND is_active = $2
			  AND agent_id IS NULL
			  AND ended_at IS NULL
			  AND started_at <= $3`
		args = []interface{}{tenantID, true, time.Now()}
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CostBudget
	for rows.Next() {
		var b CostBudget
		var agentID, description pgtype.Text
		if err := rows.Scan(
			&b.ID, &b.TenantID, &agentID, &description, &b.BudgetAmount,
			&b.Currency, &b.Period, &b.SoftLimitPct, &b.HardLimitPct, &b.IsActive,
			&b.CreatedAt, &b.UpdatedAt, &b.StartedAt, &b.EndedAt,
		); err != nil {
			return nil, err
		}
		if agentID.Valid {
			b.AgentID = &agentID.String
		}
		if description.Valid {
			b.Description = &description.String
		}
		items = append(items, b)
	}

	return items, nil
}
