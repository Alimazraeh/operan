package database

import (
	"context"
	"fmt"
	"time"
)

// PolicyRow is the durable form of a governance policy. Rules are JSONB: they
// are opaque documents evaluated by the policy engine, never queried.
type PolicyRow struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Scope       string
	Action      string
	Rules       []byte
	Priority    string
	Enabled     bool
	Effect      string
	LastEvalAt  *time.Time
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UpsertPolicy writes a policy.
func (s *ControlPlaneStore) UpsertPolicy(ctx context.Context, p PolicyRow) error {
	rules := p.Rules
	if len(rules) == 0 {
		rules = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_policies
			(id, tenant_id, name, description, scope, action, rules, priority,
			 enabled, effect, last_eval_at, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			scope = EXCLUDED.scope,
			action = EXCLUDED.action,
			rules = EXCLUDED.rules,
			priority = EXCLUDED.priority,
			enabled = EXCLUDED.enabled,
			effect = EXCLUDED.effect,
			last_eval_at = EXCLUDED.last_eval_at,
			created_by = EXCLUDED.created_by,
			updated_at = EXCLUDED.updated_at
	`, p.ID, p.TenantID, p.Name, p.Description, p.Scope, p.Action, rules,
		p.Priority, p.Enabled, p.Effect, p.LastEvalAt, p.CreatedBy,
		p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert policy %s: %w", p.ID, err)
	}
	return nil
}

// DeletePolicy removes a policy.
func (s *ControlPlaneStore) DeletePolicy(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_policies WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete policy %s: %w", id, err)
	}
	return nil
}

// LoadPolicies returns every policy, for rehydration at boot.
func (s *ControlPlaneStore) LoadPolicies(ctx context.Context) ([]PolicyRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, scope, action, rules, priority,
		       enabled, effect, last_eval_at, created_by, created_at, updated_at
		FROM tctl_policies ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load policies: %w", err)
	}
	defer rows.Close()

	var out []PolicyRow
	for rows.Next() {
		var p PolicyRow
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Scope,
			&p.Action, &p.Rules, &p.Priority, &p.Enabled, &p.Effect, &p.LastEvalAt,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
