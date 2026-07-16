package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrNotFound is the sentinel error for record not found.
var ErrNotFound = errors.New("record not found")

// SyncStore handles connector sync history operations.
type SyncStore struct {
	pool PgxPool
}

// NewSyncStore creates a new SyncStore.
func NewSyncStore(pool PgxPool) *SyncStore {
	return &SyncStore{pool: pool}
}

// Create inserts a new sync history record.
func (s *SyncStore) Create(ctx context.Context, sh *ConnectorSyncHistory) error {
	query := `
		INSERT INTO connector_sync_history (tenant_id, connector_id, sync_type, status,
			objects_fetched, objects_updated, objects_failed, error_message, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, query,
		sh.TenantID, sh.ConnectorID, sh.SyncType, sh.Status,
		sh.ObjectsFetched, sh.ObjectsUpdated, sh.ObjectsFailed,
		sh.ErrorMessage, sh.StartedAt,
	).Scan(&id)
	if err != nil {
		return err
	}
	sh.ID = id
	return nil
}

// UpdateComplete updates a sync record with completion details.
func (s *SyncStore) UpdateComplete(ctx context.Context, id uuid.UUID,
	objectsFetched, objectsUpdated, objectsFailed, durationMs int, completedAt time.Time,
) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE connector_sync_history SET status = 'completed',
			objects_fetched = $1, objects_updated = $2, objects_failed = $3,
			duration_ms = $4, completed_at = $5
		WHERE id = $6`,
		objectsFetched, objectsUpdated, objectsFailed, durationMs, completedAt, id)
	return err
}

// UpdateError updates a sync record with error details.
func (s *SyncStore) UpdateError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE connector_sync_history SET status = 'error', error_message = $1
		WHERE id = $2`,
		errorMessage, id)
	return err
}

// List returns paginated sync history with optional filters.
func (s *SyncStore) List(ctx context.Context, tenantID, connectorID, status string,
	page, pageSize int,
) ([]ConnectorSyncHistory, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	baseQuery := `
		SELECT id, tenant_id, connector_id, sync_type, status,
			objects_fetched, objects_updated, objects_failed, error_message,
			started_at, completed_at, duration_ms
		FROM connector_sync_history WHERE tenant_id = $1`

	args := []interface{}{tenantID}
	argIdx := 2

	if connectorID != "" {
		id, parseErr := uuid.Parse(connectorID)
		if parseErr != nil {
			return nil, 0, fmt.Errorf("invalid connector_id: %w", parseErr)
		}
		baseQuery += fmt.Sprintf(" AND connector_id = $%d", argIdx)
		args = append(args, id)
		argIdx++
	}
	if status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	baseQuery += " ORDER BY started_at DESC LIMIT $" + fmt.Sprintf("%d", argIdx) + " OFFSET $" + fmt.Sprintf("%d", argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var history []ConnectorSyncHistory
	for rows.Next() {
		var sh ConnectorSyncHistory
		var errMsg *string
		var completedAt *time.Time
		var durMs *int
		var durationMsAny any
		err := rows.Scan(&sh.ID, &sh.TenantID, &sh.ConnectorID, &sh.SyncType, &sh.Status,
			&sh.ObjectsFetched, &sh.ObjectsUpdated, &sh.ObjectsFailed, &errMsg,
			&sh.StartedAt, &completedAt, &durationMsAny)
		if err != nil {
			return nil, 0, fmt.Errorf("scan sync history: %w", err)
		}
		sh.ErrorMessage = errMsg
		sh.CompletedAt = completedAt
		if durationMsAny != nil {
			switch v := durationMsAny.(type) {
			case int64:
				v32 := int(v)
				durMs = &v32
			case int:
				durMs = &v
			}
		}
		sh.DurationMs = durMs
		history = append(history, sh)
	}

	// Count query
	countQuery := `
		SELECT COUNT(*) FROM connector_sync_history WHERE tenant_id = $1`
	countArgs := []interface{}{tenantID}
	countArgIdx := 2
	if connectorID != "" {
		countQuery += fmt.Sprintf(" AND connector_id = $%d", countArgIdx)
		countArgs = append(countArgs, connectorID)
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

	return history, total, nil
}

// GetByID retrieves a sync history record.
func (s *SyncStore) GetByID(ctx context.Context, id uuid.UUID, tenantID string) (*ConnectorSyncHistory, error) {
	sh := &ConnectorSyncHistory{}
	var errMsg *string
	var completedAt *time.Time
	var durMs *int
	var durationMsAny any

	query := `
		SELECT id, tenant_id, connector_id, sync_type, status,
			objects_fetched, objects_updated, objects_failed, error_message,
			started_at, completed_at, duration_ms
		FROM connector_sync_history WHERE id = $1 AND tenant_id = $2`

	err := s.pool.QueryRow(ctx, query, id, tenantID).Scan(
		&sh.ID, &sh.TenantID, &sh.ConnectorID, &sh.SyncType, &sh.Status,
		&sh.ObjectsFetched, &sh.ObjectsUpdated, &sh.ObjectsFailed, &errMsg,
		&sh.StartedAt, &completedAt, &durationMsAny)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sh.ErrorMessage = errMsg
	sh.CompletedAt = completedAt
	if durationMsAny != nil {
		switch v := durationMsAny.(type) {
		case int64:
			v32 := int(v)
			durMs = &v32
		case int:
			durMs = &v
		}
	}
	sh.DurationMs = durMs
	return sh, nil
}