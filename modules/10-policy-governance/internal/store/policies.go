package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgxPool is the interface wrapping pgxpool.Pool for testability.
type PgxPool interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Close()
}

// ErrNotFound is returned when a record is not found.
var ErrNotFound = fmt.Errorf("record not found")

// PolicyStore handles policy CRUD operations against PostgreSQL.
type PolicyStore struct {
	Pool PgxPool
}

// NewPolicyStore creates a new PolicyStore.
func NewPolicyStore(pool PgxPool) *PolicyStore {
	return &PolicyStore{Pool: pool}
}

// Create inserts a new policy and sets its ID and timestamps.
func (s *PolicyStore) Create(ctx context.Context, p *Policy) error {
	query := `
		INSERT INTO policies (tenant_id, group_id, name, description, action, scope,
			resource_type, resource_target, condition_expression, effect, priority,
			is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at`

	var condExpr []byte
	if p.ConditionExpression != nil {
		condExpr, _ = json.Marshal(p.ConditionExpression)
	}

	var id string
	var createdAt, updatedAt time.Time
	err := s.Pool.QueryRow(ctx, query,
		p.TenantID, p.GroupID, p.Name, p.Description, p.Action,
		p.Scope, p.ResourceType, p.ResourceTarget, condExpr,
		p.Effect, p.Priority, p.IsActive, p.CreatedBy,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	p.ID = id
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return nil
}

// GetByID returns a policy by its UUID.
func (s *PolicyStore) GetByID(ctx context.Context, id uuid.UUID) (*Policy, error) {
	var p Policy
	var desc, resTarget, createdBY *string
	var condExpr *[]byte

	err := s.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, group_id, name, description, action, scope,
			resource_type, resource_target, condition_expression, effect,
			priority, is_active, created_by, created_at, updated_at
		FROM policies WHERE id = $1`, id).Scan(
		&p.ID, &p.TenantID, &p.GroupID, &p.Name, &desc, &p.Action,
		&p.Scope, &p.ResourceType, &resTarget, &condExpr, &p.Effect,
		&p.Priority, &p.IsActive, &createdBY, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	p.Description = desc
	p.ResourceTarget = resTarget
	p.CreatedBy = createdBY
	if condExpr != nil && len(*condExpr) > 0 {
		p.ConditionExpression = jsonUnmarshal(*condExpr)
	}

	return &p, nil
}

// List returns paginated policies for a tenant with optional filters.
func (s *PolicyStore) List(ctx context.Context, tenantID string, scope, resourceType *string,
	page, pageSize int,
) ([]Policy, int, error) {
	offset := (page - 1) * pageSize

	query := `
		SELECT id, tenant_id, group_id, name, description, action, scope,
			resource_type, resource_target, condition_expression, effect,
			priority, is_active, created_by, created_at, updated_at
		FROM policies WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2

	if scope != nil {
		query += fmt.Sprintf(" AND scope = $%d", argIdx)
		args = append(args, *scope)
		argIdx++
	}
	if resourceType != nil {
		query += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, *resourceType)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY priority DESC, created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		p, err := scanPolicyRow(rows)
		if err != nil {
			return nil, 0, err
		}
		policies = append(policies, *p)
	}

	// Count query
	countQuery := `SELECT COUNT(*) FROM policies WHERE tenant_id = $1`
	countArgs := []interface{}{tenantID}
	ci := 2
	if scope != nil {
		countQuery += fmt.Sprintf(" AND scope = $%d", ci)
		countArgs = append(countArgs, *scope)
		ci++
	}
	if resourceType != nil {
		countQuery += fmt.Sprintf(" AND resource_type = $%d", ci)
		countArgs = append(countArgs, *resourceType)
	}

	var total int
	err = s.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return policies, total, nil
}

// Update partially updates a policy.
func (s *PolicyStore) Update(ctx context.Context, p *Policy) error {
	query := `
		UPDATE policies
		SET name = $1, description = $2, action = $3, scope = $4,
			resource_type = $5, resource_target = $6, effect = $7,
			priority = $8, is_active = $9, updated_at = NOW()
		WHERE id = $10 AND tenant_id = $11 AND is_active = true
		RETURNING updated_at`

	var updatedAt time.Time
	err := s.Pool.QueryRow(ctx, query,
		p.Name, p.Description, p.Action, p.Scope,
		p.ResourceType, p.ResourceTarget, p.Effect,
		p.Priority, p.IsActive, p.ID, p.TenantID,
	).Scan(&updatedAt)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	p.UpdatedAt = updatedAt
	return nil
}

// Delete marks a policy as inactive (soft delete).
func (s *PolicyStore) Delete(ctx context.Context, id uuid.UUID, tenantID string) error {
	result, err := s.Pool.Exec(ctx,
		"UPDATE policies SET is_active = false, updated_at = NOW() WHERE id = $1 AND tenant_id = $2",
		id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByGroup returns all active policies for a group ordered by priority.
func (s *PolicyStore) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Policy, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, tenant_id, group_id, name, description, action, scope,
			resource_type, resource_target, condition_expression, effect,
			priority, is_active, created_by, created_at, updated_at
		FROM policies WHERE group_id = $1 AND is_active = true
		ORDER BY priority DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		p, err := scanPolicyRow(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *p)
	}
	return policies, nil
}

// ListActiveForTenant returns all active policies for a tenant ordered by priority.
func (s *PolicyStore) ListActiveForTenant(ctx context.Context, tenantID string) ([]Policy, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, tenant_id, group_id, name, description, action, scope,
			resource_type, resource_target, condition_expression, effect,
			priority, is_active, created_by, created_at, updated_at
		FROM policies WHERE tenant_id = $1 AND is_active = true
		ORDER BY priority DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		p, err := scanPolicyRow(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *p)
	}
	return policies, nil
}

// scanPolicyRow scans a single policy row from pgx.Rows.
// Uses *any scans for nullable columns to work with pgxmock.
func scanPolicyRow(rows interface{ Scan(...interface{}) error }) (*Policy, error) {
	p := &Policy{}
	var desc, resTarget, createdBY *string
	var condExpr *[]byte

	err := rows.Scan(
		&p.ID, &p.TenantID, &p.GroupID, &p.Name, &desc, &p.Action,
		&p.Scope, &p.ResourceType, &resTarget, &condExpr, &p.Effect,
		&p.Priority, &p.IsActive, &createdBY, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.Description = desc
	p.ResourceTarget = resTarget
	p.CreatedBy = createdBY
	if condExpr != nil && len(*condExpr) > 0 {
		p.ConditionExpression = jsonUnmarshal(*condExpr)
	}

	return p, nil
}

// jsonUnmarshal safely decodes JSON bytes to map[string]interface{}.
func jsonUnmarshal(b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}