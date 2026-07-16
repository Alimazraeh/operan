package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ConnectorStore handles connector CRUD operations.
type ConnectorStore struct {
	pool PgxPool
}

// NewConnectorStore creates a new ConnectorStore.
func NewConnectorStore(pool PgxPool) *ConnectorStore {
	return &ConnectorStore{pool: pool}
}

// Create inserts a new connector definition.
func (s *ConnectorStore) Create(ctx context.Context, c *Connector) error {
	configJSON, err := json.Marshal(c.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	credsJSON, err := json.Marshal(c.Credentials)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	var metadataJSON []byte = []byte("{}")
	if c.Metadata != nil {
		metadataJSON, err = json.Marshal(c.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO connector_definitions (tenant_id, name, description, connector_type,
			auth_method, config, credentials, sync_frequency, tools_registered, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, status, last_sync_at, last_sync_status, last_error,
			created_at, updated_at`

	var id uuid.UUID
	var status string
	var lastSyncAt *time.Time
	var lastSyncStatus *string
	var lastError *string
	var createdAt, updatedAt time.Time

	err = s.pool.QueryRow(ctx, query,
		c.TenantID, c.Name, c.Description, c.ConnectorType,
		c.AuthMethod, configJSON, credsJSON, c.SyncFrequency,
		c.ToolsRegistered, metadataJSON,
	).Scan(&id, &status, &lastSyncAt, &lastSyncStatus, &lastError,
		&createdAt, &updatedAt)
	if err != nil {
		return err
	}

	c.ID = id
	c.Status = status
	c.LastSyncAt = lastSyncAt
	c.LastSyncStatus = lastSyncStatus
	c.LastError = lastError
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	return nil
}

// GetByID retrieves a connector by ID.
func (s *ConnectorStore) GetByID(ctx context.Context, id uuid.UUID, tenantID string) (*Connector, error) {
	c := &Connector{}
	var configJSON, credsJSON, metadataJSON []byte

	query := `
		SELECT id, tenant_id, name, description, connector_type, auth_method,
			config, credentials, sync_frequency, last_sync_at, last_sync_status,
			last_error, tools_registered, metadata, created_at, updated_at
		FROM connector_definitions WHERE id = $1 AND tenant_id = $2`

	err := s.pool.QueryRow(ctx, query, id, tenantID).Scan(
		&c.ID, &c.TenantID, &c.Name, &c.Description, &c.ConnectorType,
		&c.AuthMethod, &configJSON, &credsJSON, &c.SyncFrequency,
		&c.LastSyncAt, &c.LastSyncStatus, &c.LastError,
		&c.ToolsRegistered, &metadataJSON, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &c.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := json.Unmarshal(credsJSON, &c.Credentials); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &c.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return c, nil
}

// List returns paginated connectors for a tenant with optional filters.
func (s *ConnectorStore) List(ctx context.Context, tenantID, connectorType, status string,
	page, pageSize int,
) ([]Connector, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	baseQuery := `
		SELECT id, tenant_id, name, description, connector_type, auth_method,
			config, credentials, sync_frequency, last_sync_at, last_sync_status,
			last_error, tools_registered, metadata, created_at, updated_at
		FROM connector_definitions WHERE tenant_id = $1`

	args := []interface{}{tenantID}
	argIdx := 2

	if connectorType != "" {
		baseQuery += fmt.Sprintf(" AND connector_type = $%d", argIdx)
		args = append(args, connectorType)
		argIdx++
	}
	if status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	baseQuery += " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", argIdx) + " OFFSET $" + fmt.Sprintf("%d", argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var connectors []Connector
	for rows.Next() {
		var c Connector
		var configJSON, credsJSON, metadataJSON []byte
		err := rows.Scan(&c.ID, &c.TenantID, &c.Name, &c.Description, &c.ConnectorType,
			&c.AuthMethod, &configJSON, &credsJSON, &c.SyncFrequency,
			&c.LastSyncAt, &c.LastSyncStatus, &c.LastError,
			&c.ToolsRegistered, &metadataJSON, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan connector: %w", err)
		}
		if err := json.Unmarshal(configJSON, &c.Config); err != nil {
			return nil, 0, fmt.Errorf("unmarshal config: %w", err)
		}
		if err := json.Unmarshal(credsJSON, &c.Credentials); err != nil {
			return nil, 0, fmt.Errorf("unmarshal credentials: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &c.Metadata); err != nil {
			return nil, 0, fmt.Errorf("unmarshal metadata: %w", err)
		}
		connectors = append(connectors, c)
	}

	// Count query
	countQuery := `
		SELECT COUNT(*) FROM connector_definitions WHERE tenant_id = $1`
	countArgs := []interface{}{tenantID}
	countArgIdx := 2
	if connectorType != "" {
		countQuery += fmt.Sprintf(" AND connector_type = $%d", countArgIdx)
		countArgs = append(countArgs, connectorType)
		countArgIdx++
	}
	if status != "" {
		countQuery += fmt.Sprintf(" AND status = $%d", countArgIdx)
		countArgs = append(countArgs, status)
		countArgIdx++
	}

	var total int
	err = s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return connectors, total, nil
}

// UpdateStatus updates the status and optional fields of a connector.
func (s *ConnectorStore) UpdateStatus(ctx context.Context, id uuid.UUID, tenantID string,
	status string, lastSyncAt *time.Time, lastSyncStatus *string, lastError *string,
) error {
	query := `
		UPDATE connector_definitions SET status = $1, last_sync_at = $2,
			last_sync_status = $3, last_error = $4, updated_at = NOW()
		WHERE id = $5 AND tenant_id = $6`

	_, err := s.pool.Exec(ctx, query, status, lastSyncAt, lastSyncStatus, lastError, id, tenantID)
	return err
}

// UpdateToolsRegistered updates the tools_registered flag.
func (s *ConnectorStore) UpdateToolsRegistered(ctx context.Context, id uuid.UUID, tenantID string, registered bool) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE connector_definitions SET tools_registered = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3",
		registered, id, tenantID)
	return err
}

// Delete removes a connector by ID.
func (s *ConnectorStore) Delete(ctx context.Context, id uuid.UUID, tenantID string) error {
	_, err := s.pool.Exec(ctx,
		"DELETE FROM connector_definitions WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return err
	}
	return nil
}