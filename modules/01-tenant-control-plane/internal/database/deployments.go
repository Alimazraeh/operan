package database

import (
	"context"
	"fmt"
	"time"
)

// DeploymentRow is the durable form of a deployment. The manifest and state
// objects are JSONB: they are read and written whole by the API.
type DeploymentRow struct {
	ID            string
	TenantID      string
	Name          string
	Version       string
	Status        string
	Strategy      string
	Manifest      []byte
	DesiredState  []byte
	CurrentState  []byte
	Error         string
	ResourceRefs  []byte
	NamespaceID   string
	PreviousID    *string
	CreatedBy     string
	Notes         string
	DeployedAt    *time.Time
	DeprecatedAt  *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertDeployment writes a deployment.
func (s *ControlPlaneStore) UpsertDeployment(ctx context.Context, d DeploymentRow) error {
	manifest, desired, current, refs := d.Manifest, d.DesiredState, d.CurrentState, d.ResourceRefs
	if len(manifest) == 0 {
		manifest = []byte("{}")
	}
	if len(desired) == 0 {
		desired = []byte("{}")
	}
	if len(current) == 0 {
		current = []byte("{}")
	}
	if len(refs) == 0 {
		refs = []byte("[]")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_deployments
			(id, tenant_id, name, version, status, strategy, manifest,
			 desired_state, current_state, error, resource_refs, namespace_id,
			 previous_id, created_by, notes, deployed_at, deprecated_at,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			version = EXCLUDED.version,
			status = EXCLUDED.status,
			strategy = EXCLUDED.strategy,
			manifest = EXCLUDED.manifest,
			desired_state = EXCLUDED.desired_state,
			current_state = EXCLUDED.current_state,
			error = EXCLUDED.error,
			resource_refs = EXCLUDED.resource_refs,
			namespace_id = EXCLUDED.namespace_id,
			previous_id = EXCLUDED.previous_id,
			created_by = EXCLUDED.created_by,
			notes = EXCLUDED.notes,
			deployed_at = EXCLUDED.deployed_at,
			deprecated_at = EXCLUDED.deprecated_at,
			updated_at = EXCLUDED.updated_at
	`, d.ID, d.TenantID, d.Name, d.Version, d.Status, d.Strategy, manifest,
		desired, current, d.Error, refs, d.NamespaceID, d.PreviousID, d.CreatedBy,
		d.Notes, d.DeployedAt, d.DeprecatedAt, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert deployment %s: %w", d.ID, err)
	}
	return nil
}

// DeleteDeployment removes a deployment.
func (s *ControlPlaneStore) DeleteDeployment(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_deployments WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete deployment %s: %w", id, err)
	}
	return nil
}

// LoadDeployments returns every deployment, for rehydration at boot.
func (s *ControlPlaneStore) LoadDeployments(ctx context.Context) ([]DeploymentRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, version, status, strategy, manifest,
		       desired_state, current_state, error, resource_refs, namespace_id,
		       previous_id, created_by, notes, deployed_at, deprecated_at,
		       created_at, updated_at
		FROM tctl_deployments ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load deployments: %w", err)
	}
	defer rows.Close()

	var out []DeploymentRow
	for rows.Next() {
		var d DeploymentRow
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Version, &d.Status,
			&d.Strategy, &d.Manifest, &d.DesiredState, &d.CurrentState, &d.Error,
			&d.ResourceRefs, &d.NamespaceID, &d.PreviousID, &d.CreatedBy, &d.Notes,
			&d.DeployedAt, &d.DeprecatedAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
