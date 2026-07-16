package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvidersStore handles model_providers CRUD.
type ProvidersStore struct {
	pool *pgxpool.Pool
}

// NewProvidersStore creates a new ProvidersStore.
func NewProvidersStore(pool *pgxpool.Pool) *ProvidersStore {
	return &ProvidersStore{pool: pool}
}

// Create inserts a new model provider.
func (s *ProvidersStore) Create(ctx context.Context, p *ModelProvider) error {
	query := `
		INSERT INTO model_providers
			(tenant_id, name, description, type, base_url, api_key_secret_name,
			 is_active, priority, max_retries, timeout_ms, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`

	return s.pool.QueryRow(ctx, query,
		p.TenantID, p.Name, p.Description, p.Type, p.BaseURL,
		p.APIKeySecretName, p.IsActive, p.Priority, p.MaxRetries, p.TimeoutMs,
		p.Metadata,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

// GetByID returns a provider by ID or pgx.ErrNoRows.
func (s *ProvidersStore) GetByID(ctx context.Context, id string) (*ModelProvider, error) {
	var p ModelProvider
	query := `
		SELECT id, tenant_id, name, description, type, base_url,
		       api_key_secret_name, is_active, priority, max_retries, timeout_ms,
		       metadata, created_at, updated_at
		FROM model_providers
		WHERE id = $1`

	err := s.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Type, &p.BaseURL,
		&p.APIKeySecretName, &p.IsActive, &p.Priority, &p.MaxRetries, &p.TimeoutMs,
		&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListByTenant returns paginated providers for a tenant.
func (s *ProvidersStore) ListByTenant(ctx context.Context, tenantID string, page, pageSize int) ([]ModelProvider, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var items []ModelProvider
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, type, base_url,
		       api_key_secret_name, is_active, priority, max_retries, timeout_ms,
		       metadata, created_at, updated_at
		FROM model_providers
		WHERE tenant_id = $1
		ORDER BY priority DESC, created_at DESC
		LIMIT $2 OFFSET $3`, tenantID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var p ModelProvider
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Type, &p.BaseURL,
			&p.APIKeySecretName, &p.IsActive, &p.Priority, &p.MaxRetries, &p.TimeoutMs,
			&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM model_providers WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// SoftDelete marks a provider as inactive (soft delete).
func (s *ProvidersStore) SoftDelete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE model_providers SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// Update updates a provider's fields.
func (s *ProvidersStore) Update(ctx context.Context, p *ModelProvider) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE model_providers
		SET name = $1, description = $2, type = $3, base_url = $4,
		    api_key_secret_name = $5, is_active = $6, priority = $7,
		    max_retries = $8, timeout_ms = $9, metadata = $10, updated_at = NOW()
		WHERE id = $11`,
		p.Name, p.Description, p.Type, p.BaseURL,
		p.APIKeySecretName, p.IsActive, p.Priority, p.MaxRetries, p.TimeoutMs,
		p.Metadata, p.ID)
	return err
}

// ActiveByTenant returns all active providers for a tenant, ordered by priority.
func (s *ProvidersStore) ActiveByTenant(ctx context.Context, tenantID string) ([]ModelProvider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, type, base_url,
		       api_key_secret_name, is_active, priority, max_retries, timeout_ms,
		       metadata, created_at, updated_at
		FROM model_providers
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY priority DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ModelProvider
	for rows.Next() {
		var p ModelProvider
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Type, &p.BaseURL,
			&p.APIKeySecretName, &p.IsActive, &p.Priority, &p.MaxRetries, &p.TimeoutMs,
			&p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, nil
}