package database

import (
	"context"
	"fmt"
	"time"
)

// ResourceRow is the durable form of a managed resource.
type ResourceRow struct {
	ID        string
	TenantID  string
	Name      string
	Type      string
	Region    string
	Spec      []byte
	Status    string
	Endpoint  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpsertResource writes a resource.
func (s *ControlPlaneStore) UpsertResource(ctx context.Context, r ResourceRow) error {
	spec := r.Spec
	if len(spec) == 0 {
		spec = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_resources
			(id, tenant_id, name, type, region, spec, status, endpoint,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			region = EXCLUDED.region,
			spec = EXCLUDED.spec,
			status = EXCLUDED.status,
			endpoint = EXCLUDED.endpoint,
			updated_at = EXCLUDED.updated_at
	`, r.ID, r.TenantID, r.Name, r.Type, r.Region, spec, r.Status, r.Endpoint,
		r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert resource %s: %w", r.ID, err)
	}
	return nil
}

// DeleteResource removes a resource.
func (s *ControlPlaneStore) DeleteResource(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_resources WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete resource %s: %w", id, err)
	}
	return nil
}

// LoadResources returns every resource, for rehydration at boot.
func (s *ControlPlaneStore) LoadResources(ctx context.Context) ([]ResourceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, type, region, spec, status, endpoint,
		       created_at, updated_at
		FROM tctl_resources ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load resources: %w", err)
	}
	defer rows.Close()

	var out []ResourceRow
	for rows.Next() {
		var r ResourceRow
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Type, &r.Region,
			&r.Spec, &r.Status, &r.Endpoint, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan resource: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
