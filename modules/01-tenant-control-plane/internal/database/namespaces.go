package database

import (
	"context"
	"fmt"
	"time"
)

// NamespaceRow is the durable form of a namespace.
type NamespaceRow struct {
	ID            string
	TenantID      string
	Name          string
	Description   string
	Status        string
	Config        []byte
	ResourceQuota []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertNamespace writes a namespace.
func (s *ControlPlaneStore) UpsertNamespace(ctx context.Context, n NamespaceRow) error {
	cfg, quota := n.Config, n.ResourceQuota
	if len(cfg) == 0 {
		cfg = []byte("{}")
	}
	if len(quota) == 0 {
		quota = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_namespaces
			(id, tenant_id, name, description, status, config, resource_quota,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			config = EXCLUDED.config,
			resource_quota = EXCLUDED.resource_quota,
			updated_at = EXCLUDED.updated_at
	`, n.ID, n.TenantID, n.Name, n.Description, n.Status, cfg, quota,
		n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert namespace %s: %w", n.ID, err)
	}
	return nil
}

// DeleteNamespace removes a namespace.
func (s *ControlPlaneStore) DeleteNamespace(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_namespaces WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete namespace %s: %w", id, err)
	}
	return nil
}

// LoadNamespaces returns every namespace, for rehydration at boot.
func (s *ControlPlaneStore) LoadNamespaces(ctx context.Context) ([]NamespaceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, status, config, resource_quota,
		       created_at, updated_at
		FROM tctl_namespaces ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load namespaces: %w", err)
	}
	defer rows.Close()

	var out []NamespaceRow
	for rows.Next() {
		var n NamespaceRow
		if err := rows.Scan(&n.ID, &n.TenantID, &n.Name, &n.Description, &n.Status,
			&n.Config, &n.ResourceQuota, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan namespace: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
