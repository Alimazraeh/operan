package database

import (
	"context"
	"fmt"
	"time"
)

// EnvironmentRow is the durable form of an environment.
type EnvironmentRow struct {
	ID              string
	TenantID        string
	Name            string
	Type            string
	State           string
	IsolationLevel  string
	IsolationConfig []byte
	Resources       []byte
	NetworkConfig   []byte
	CreatedBy       string
	Notes           string
	ActivatedAt     *time.Time
	DeactivatedAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpsertEnvironment writes an environment.
func (s *ControlPlaneStore) UpsertEnvironment(ctx context.Context, e EnvironmentRow) error {
	iso, resources, network := e.IsolationConfig, e.Resources, e.NetworkConfig
	if len(iso) == 0 {
		iso = []byte("{}")
	}
	if len(resources) == 0 {
		resources = []byte("[]")
	}
	if len(network) == 0 {
		network = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_environments
			(id, tenant_id, name, type, state, isolation_level, isolation_config,
			 resources, network_config, created_by, notes, activated_at,
			 deactivated_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			state = EXCLUDED.state,
			isolation_level = EXCLUDED.isolation_level,
			isolation_config = EXCLUDED.isolation_config,
			resources = EXCLUDED.resources,
			network_config = EXCLUDED.network_config,
			created_by = EXCLUDED.created_by,
			notes = EXCLUDED.notes,
			activated_at = EXCLUDED.activated_at,
			deactivated_at = EXCLUDED.deactivated_at,
			updated_at = EXCLUDED.updated_at
	`, e.ID, e.TenantID, e.Name, e.Type, e.State, e.IsolationLevel, iso,
		resources, network, e.CreatedBy, e.Notes, e.ActivatedAt, e.DeactivatedAt,
		e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert environment %s: %w", e.ID, err)
	}
	return nil
}

// DeleteEnvironment removes an environment.
func (s *ControlPlaneStore) DeleteEnvironment(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_environments WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete environment %s: %w", id, err)
	}
	return nil
}

// LoadEnvironments returns every environment, for rehydration at boot.
func (s *ControlPlaneStore) LoadEnvironments(ctx context.Context) ([]EnvironmentRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, type, state, isolation_level, isolation_config,
		       resources, network_config, created_by, notes, activated_at,
		       deactivated_at, created_at, updated_at
		FROM tctl_environments ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load environments: %w", err)
	}
	defer rows.Close()

	var out []EnvironmentRow
	for rows.Next() {
		var e EnvironmentRow
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Name, &e.Type, &e.State,
			&e.IsolationLevel, &e.IsolationConfig, &e.Resources, &e.NetworkConfig,
			&e.CreatedBy, &e.Notes, &e.ActivatedAt, &e.DeactivatedAt,
			&e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan environment: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
