package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RegistryStore handles model_registry CRUD.
type RegistryStore struct {
	pool *pgxpool.Pool
}

// NewRegistryStore creates a new RegistryStore.
func NewRegistryStore(pool *pgxpool.Pool) *RegistryStore {
	return &RegistryStore{pool: pool}
}

// Create inserts a new model registry entry.
func (s *RegistryStore) Create(ctx context.Context, m *ModelRegistry) error {
	costBytes, err := json.Marshal(m.CostPerToken)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO model_registry
			(tenant_id, model_name, provider_id, provider_model_name,
			 supports_chat, supports_embed, max_tokens, cost_per_token,
			 is_default, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`

	return s.pool.QueryRow(ctx, query,
		m.TenantID, m.ModelName, m.ProviderID, m.ProviderModelName,
		m.SupportsChat, m.SupportsEmbed, m.MaxTokens, costBytes,
		m.IsDefault, m.IsActive,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
}

// GetByID returns a model registry entry by ID.
func (s *RegistryStore) GetByID(ctx context.Context, id string) (*ModelRegistry, error) {
	var m ModelRegistry
	var costBytes []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, model_name, provider_id, provider_model_name,
		       supports_chat, supports_embed, max_tokens, cost_per_token,
		       is_default, is_active, created_at, updated_at
		FROM model_registry
		WHERE id = $1`, id).Scan(
		&m.ID, &m.TenantID, &m.ModelName, &m.ProviderID, &m.ProviderModelName,
		&m.SupportsChat, &m.SupportsEmbed, &m.MaxTokens, &costBytes,
		&m.IsDefault, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(costBytes, &m.CostPerToken)
	return &m, nil
}

// GetByName returns a model registry entry by tenant + model_name (active only).
func (s *RegistryStore) GetByName(ctx context.Context, tenantID, modelName string) (*ModelRegistry, error) {
	return s.GetByNameWithActive(ctx, tenantID, modelName, true)
}

// GetByNameWithActive optionally filters by is_active.
func (s *RegistryStore) GetByNameWithActive(ctx context.Context, tenantID, modelName string, activeOnly bool) (*ModelRegistry, error) {
	var m ModelRegistry
	var costBytes []byte

	query := `
		SELECT id, tenant_id, model_name, provider_id, provider_model_name,
		       supports_chat, supports_embed, max_tokens, cost_per_token,
		       is_default, is_active, created_at, updated_at
		FROM model_registry
		WHERE tenant_id = $1 AND model_name = $2`

	if activeOnly {
		query += " AND is_active = true"
	}

	query += " LIMIT 1"

	err := s.pool.QueryRow(ctx, query, tenantID, modelName).Scan(
		&m.ID, &m.TenantID, &m.ModelName, &m.ProviderID, &m.ProviderModelName,
		&m.SupportsChat, &m.SupportsEmbed, &m.MaxTokens, &costBytes,
		&m.IsDefault, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(costBytes, &m.CostPerToken)
	return &m, nil
}

// ListByTenant returns paginated model registry entries with optional provider filter.
func (s *RegistryStore) ListByTenant(ctx context.Context, tenantID string, providerID *string, page, pageSize int) ([]ModelRegistry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Count total first.
	var total int
	countArgs := []any{tenantID}
	countSQL := `SELECT COUNT(*) FROM model_registry WHERE tenant_id = $1`
	if providerID != nil && *providerID != "" {
		countSQL += " AND provider_id = $2"
		countArgs = append(countArgs, *providerID)
	}
	if err := s.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch rows.
	query := `
		SELECT id, tenant_id, model_name, provider_id, provider_model_name,
		       supports_chat, supports_embed, max_tokens, cost_per_token,
		       is_default, is_active, created_at, updated_at
		FROM model_registry
		WHERE tenant_id = $1`
	args := []any{tenantID}
	idx := 2

	if providerID != nil && *providerID != "" {
		query += fmt.Sprintf(" AND provider_id = $%d", idx)
		args = append(args, *providerID)
		idx++
	}

	query += fmt.Sprintf(" ORDER BY model_name ASC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []ModelRegistry
	for rows.Next() {
		var m ModelRegistry
		var costBytes []byte
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ModelName, &m.ProviderID, &m.ProviderModelName,
			&m.SupportsChat, &m.SupportsEmbed, &m.MaxTokens, &costBytes,
			&m.IsDefault, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(costBytes, &m.CostPerToken)
		items = append(items, m)
	}
	return items, total, nil
}

// Update updates a model registry entry.
func (s *RegistryStore) Update(ctx context.Context, m *ModelRegistry) error {
	costBytes, err := json.Marshal(m.CostPerToken)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE model_registry
		SET model_name = $1, provider_id = $2, provider_model_name = $3,
		    supports_chat = $4, supports_embed = $5, max_tokens = $6,
		    cost_per_token = $7, is_default = $8, is_active = $9, updated_at = NOW()
		WHERE id = $10`,
		m.ModelName, m.ProviderID, m.ProviderModelName,
		m.SupportsChat, m.SupportsEmbed, m.MaxTokens, costBytes,
		m.IsDefault, m.IsActive, m.ID)
	return err
}

// SetDefault marks the only default for a tenant by clearing others first.
func (s *RegistryStore) SetDefault(ctx context.Context, tenantID, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE model_registry SET is_default = false WHERE tenant_id = $1 AND is_default = true`, tenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE model_registry SET is_default = true, updated_at = NOW() WHERE id = $1`, modelID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetByProviderID returns all active models registered for a specific provider.
func (s *RegistryStore) GetByProviderID(ctx context.Context, providerID string) ([]ModelRegistry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, model_name, provider_id, provider_model_name,
		       supports_chat, supports_embed, max_tokens, cost_per_token,
		       is_default, is_active, created_at, updated_at
		FROM model_registry
		WHERE provider_id = $1 AND is_active = true`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ModelRegistry
	for rows.Next() {
		var m ModelRegistry
		var costBytes []byte
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ModelName, &m.ProviderID, &m.ProviderModelName,
			&m.SupportsChat, &m.SupportsEmbed, &m.MaxTokens, &costBytes,
			&m.IsDefault, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(costBytes, &m.CostPerToken)
		items = append(items, m)
	}
	return items, nil
}

// GetAllActive returns every active model in the registry.
func (s *RegistryStore) GetAllActive(ctx context.Context) ([]ModelRegistry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, model_name, provider_id, provider_model_name,
		       supports_chat, supports_embed, max_tokens, cost_per_token,
		       is_default, is_active, created_at, updated_at
		FROM model_registry
		WHERE is_active = true
		ORDER BY tenant_id, model_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ModelRegistry
	for rows.Next() {
		var m ModelRegistry
		var costBytes []byte
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ModelName, &m.ProviderID, &m.ProviderModelName,
			&m.SupportsChat, &m.SupportsEmbed, &m.MaxTokens, &costBytes,
			&m.IsDefault, &m.IsActive, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(costBytes, &m.CostPerToken)
		items = append(items, m)
	}
	return items, nil
}

// Helper: format a PostgreSQL positional placeholder (e.g., "$1").
func formatPlaceholder(n int) string {
	return fmt.Sprintf("$%d", n)
}