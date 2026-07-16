package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GroupStore handles policy group CRUD operations against PostgreSQL.
type GroupStore struct {
	Pool PgxPool
}

// NewGroupStore creates a new GroupStore.
func NewGroupStore(pool PgxPool) *GroupStore {
	return &GroupStore{Pool: pool}
}

// Create inserts a new policy group.
func (s *GroupStore) Create(ctx context.Context, g *PolicyGroup) error {
	query := `
		INSERT INTO policy_groups (tenant_id, name, description, priority, is_active, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	var id string
	err := s.Pool.QueryRow(ctx, query,
		g.TenantID, g.Name, g.Description, g.Priority, g.IsActive, g.Metadata,
	).Scan(&id, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return err
	}
	g.ID = id
	return nil
}

// GetByID returns a policy group by its UUID.
func (s *GroupStore) GetByID(ctx context.Context, id uuid.UUID) (*PolicyGroup, error) {
	var g PolicyGroup
	var desc, meta interface{}

	err := s.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, priority, is_active, metadata, created_at, updated_at
		FROM policy_groups WHERE id = $1`, id).Scan(
		&g.ID, &g.TenantID, &g.Name, &desc, &g.Priority, &g.IsActive, &meta,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if desc != nil {
		if s, ok := desc.(string); ok {
			g.Description = &s
		}
	}
	if meta != nil {
		if m, ok := meta.(map[string]interface{}); ok {
			g.Metadata = m
		}
	}

	return &g, nil
}

// GetByName returns a policy group by name within a tenant.
func (s *GroupStore) GetByName(ctx context.Context, tenantID, name string) (*PolicyGroup, error) {
	var g PolicyGroup
	var desc, meta interface{}

	err := s.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, priority, is_active, metadata, created_at, updated_at
		FROM policy_groups WHERE tenant_id = $1 AND name = $2`,
		tenantID, name).Scan(
		&g.ID, &g.TenantID, &g.Name, &desc, &g.Priority, &g.IsActive, &meta,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if desc != nil {
		if s, ok := desc.(string); ok {
			g.Description = &s
		}
	}
	if meta != nil {
		if m, ok := meta.(map[string]interface{}); ok {
			g.Metadata = m
		}
	}

	return &g, nil
}

// List returns paginated policy groups for a tenant.
func (s *GroupStore) List(ctx context.Context, tenantID string, page, pageSize int) ([]PolicyGroup, int, error) {
	offset := (page - 1) * pageSize

	rows, err := s.Pool.Query(ctx, `
		SELECT id, tenant_id, name, description, priority, is_active, metadata, created_at, updated_at
		FROM policy_groups WHERE tenant_id = $1
		ORDER BY priority DESC, created_at DESC
		LIMIT $2 OFFSET $3`, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var groups []PolicyGroup
	for rows.Next() {
		var g PolicyGroup
		var desc, meta interface{}
		err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &desc, &g.Priority, &g.IsActive, &meta,
			&g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		if desc != nil {
			if s, ok := desc.(string); ok {
				g.Description = &s
			}
		}
		if meta != nil {
			if m, ok := meta.(map[string]interface{}); ok {
				g.Metadata = m
			}
		}
		groups = append(groups, g)
	}

	// Count
	var total int
	err = s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM policy_groups WHERE tenant_id = $1`, tenantID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

// Update partially updates a policy group.
func (s *GroupStore) Update(ctx context.Context, g *PolicyGroup) error {
	setParts := []string{}
	args := []interface{}{}
	argIdx := 1

	if g.Name != "" {
		setParts = append(setParts, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, g.Name)
		argIdx++
	}
	if g.Description != nil {
		setParts = append(setParts, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *g.Description)
		argIdx++
	}
	if g.Priority != 0 {
		setParts = append(setParts, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, g.Priority)
		argIdx++
	}
	if g.IsActive {
		setParts = append(setParts, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, g.IsActive)
		argIdx++
	}
	if g.Metadata != nil {
		setParts = append(setParts, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, g.Metadata)
		argIdx++
	}

	setParts = append(setParts, "updated_at = NOW()")
	args = append(args, g.ID, g.TenantID)

	// Join set parts
	setStr := ""
	for i, p := range setParts {
		if i > 0 {
			setStr += ", "
		}
		setStr += p
	}

	query := fmt.Sprintf("UPDATE policy_groups SET %s WHERE id = $%d AND tenant_id = $%d",
		setStr, len(args), len(args)+1)

	_, err := s.Pool.Exec(ctx, query, args...)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	return err
}

// Delete marks a policy group as inactive (soft delete).
func (s *GroupStore) Delete(ctx context.Context, id uuid.UUID, tenantID string) error {
	result, err := s.Pool.Exec(ctx,
		"UPDATE policy_groups SET is_active = false, updated_at = NOW() WHERE id = $1 AND tenant_id = $2",
		id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}