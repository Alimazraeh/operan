package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// AlertStore handles cost_alerts CRUD.
type AlertStore struct {
	pool PgxPool
}

// NewAlertStore creates a new AlertStore.
func NewAlertStore(pool PgxPool) *AlertStore {
	return &AlertStore{pool: pool}
}

// Create inserts a cost alert.
func (s *AlertStore) Create(ctx context.Context, a *CostAlert) error {
	a.ID = uuid.New().String()
	a.CreatedAt = time.Now()

	query := `
		INSERT INTO cost_alerts
			(tenant_id, budget_id, agent_id, alert_type, current_spend,
			 budget_amount, percentage_used, severity, is_resolved)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	return s.pool.QueryRow(ctx, query,
		a.TenantID, a.BudgetID, a.AgentID, a.AlertType, a.CurrentSpend,
		a.BudgetAmount, a.PercentageUsed, a.Severity, a.IsResolved,
	).Scan(&a.ID, &a.CreatedAt)
}

// List returns paginated alerts for a tenant.
func (s *AlertStore) List(ctx context.Context, tenantID, severity, alertType string, isResolved *bool, page, pageSize int) ([]CostAlert, int, error) {
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

	if severity != "" {
		where += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, severity)
		argIdx++
	}
	if alertType != "" {
		where += fmt.Sprintf(" AND alert_type = $%d", argIdx)
		args = append(args, alertType)
		argIdx++
	}
	if isResolved != nil {
		where += fmt.Sprintf(" AND is_resolved = $%d", argIdx)
		args = append(args, *isResolved)
		argIdx++
	}

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM cost_alerts "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, budget_id, agent_id, alert_type, current_spend,
		       budget_amount, percentage_used, severity, is_resolved, resolved_at,
		       created_at
		FROM cost_alerts `+where+` ORDER BY created_at DESC LIMIT $`+fmt.Sprint(argIdx)+` OFFSET $`+fmt.Sprint(argIdx+1)+``,
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []CostAlert
	for rows.Next() {
		var a CostAlert
		var budgetID, agentID pgtype.Text
		var resolvedAt pgtype.Timestamptz
		if err := rows.Scan(
			&a.ID, &a.TenantID, &budgetID, &agentID, &a.AlertType,
			&a.CurrentSpend, &a.BudgetAmount, &a.PercentageUsed, &a.Severity,
			&a.IsResolved, &resolvedAt, &a.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if budgetID.Valid {
			a.BudgetID = &budgetID.String
		}
		if agentID.Valid {
			a.AgentID = &agentID.String
		}
		if resolvedAt.Valid {
			a.ResolvedAt = &resolvedAt.Time
		}
		items = append(items, a)
	}

	return items, total, nil
}

// UpdateResolved marks an alert as resolved.
func (s *AlertStore) UpdateResolved(ctx context.Context, id string, resolved bool) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE cost_alerts SET is_resolved = $1, resolved_at = $2 WHERE id = $3`,
		resolved, now, id,
	)
	return err
}
