package database

import (
	"context"
	"fmt"
	"time"
)

// SecretRow is the durable form of a secret. Only the encrypted value is
// stored — the plaintext lives in memory for the process lifetime, matching
// how the store already treats it.
type SecretRow struct {
	ID             string
	TenantID       string
	Key            string
	EncryptedValue string
	Description    string
	Tags           []byte
	Version        int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UpsertSecret writes a secret.
func (s *ControlPlaneStore) UpsertSecret(ctx context.Context, r SecretRow) error {
	tags := r.Tags
	if len(tags) == 0 {
		tags = []byte("[]")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tctl_secrets
			(id, tenant_id, key, encrypted_value, description, tags, version,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			key = EXCLUDED.key,
			encrypted_value = EXCLUDED.encrypted_value,
			description = EXCLUDED.description,
			tags = EXCLUDED.tags,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at
	`, r.ID, r.TenantID, r.Key, r.EncryptedValue, r.Description, tags, r.Version,
		r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert secret %s: %w", r.ID, err)
	}
	return nil
}

// DeleteSecret removes a secret.
func (s *ControlPlaneStore) DeleteSecret(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx,
		`DELETE FROM tctl_secrets WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete secret %s: %w", id, err)
	}
	return nil
}

// LoadSecrets returns every secret, for rehydration at boot.
func (s *ControlPlaneStore) LoadSecrets(ctx context.Context) ([]SecretRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, key, encrypted_value, description, tags, version,
		       created_at, updated_at
		FROM tctl_secrets ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("load secrets: %w", err)
	}
	defer rows.Close()

	var out []SecretRow
	for rows.Next() {
		var r SecretRow
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Key, &r.EncryptedValue,
			&r.Description, &r.Tags, &r.Version, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan secret: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
