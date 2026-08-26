package database

import (
	"context"
	"fmt"
	"time"
)

// TenantRow is the durable form of a tenant.
type TenantRow struct {
	ID             string
	TenantID       string
	Name           string
	DisplayName    string
	Plan           string
	Region         string
	IsolationLevel string
	Status         string
	Quota          []byte
	ContactEmail   string
	CustomMetadata []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertTenant writes a tenant. Create and update share one statement because
// the in-memory store is the arbiter of whether an id already exists.
func (s *ControlPlaneStore) UpsertTenant(ctx context.Context, t TenantRow) error {
	quota, meta := t.Quota, t.CustomMetadata
	if len(quota) == 0 {
		quota = []byte("{}")
	}
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_tenants
			(id, tenant_id, name, display_name, plan, region, isolation_level,
			 status, quota, contact_email, custom_metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			name = EXCLUDED.name,
			display_name = EXCLUDED.display_name,
			plan = EXCLUDED.plan,
			region = EXCLUDED.region,
			isolation_level = EXCLUDED.isolation_level,
			status = EXCLUDED.status,
			quota = EXCLUDED.quota,
			contact_email = EXCLUDED.contact_email,
			custom_metadata = EXCLUDED.custom_metadata,
			updated_at = EXCLUDED.updated_at
	`, t.ID, t.TenantID, t.Name, t.DisplayName, t.Plan, t.Region, t.IsolationLevel,
		t.Status, quota, t.ContactEmail, meta, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert tenant %s: %w", t.ID, err)
	}
	return nil
}

// DeleteTenant removes a tenant.
func (s *ControlPlaneStore) DeleteTenant(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_tenants WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete tenant %s: %w", id, err)
	}
	return nil
}

// LoadTenants returns every tenant, for rehydration at boot.
func (s *ControlPlaneStore) LoadTenants(ctx context.Context) ([]TenantRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, display_name, plan, region, isolation_level,
		       status, quota, contact_email, custom_metadata, created_at, updated_at
		FROM tctl_tenants ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load tenants: %w", err)
	}
	defer rows.Close()

	var out []TenantRow
	for rows.Next() {
		var t TenantRow
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.DisplayName, &t.Plan,
			&t.Region, &t.IsolationLevel, &t.Status, &t.Quota, &t.ContactEmail,
			&t.CustomMetadata, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
