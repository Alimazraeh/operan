package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// JobsStore handles ingestion_jobs CRUD.
type JobsStore struct {
	pool *pgxpool.Pool
}

// NewJobsStore creates a new JobsStore.
func NewJobsStore(pool *pgxpool.Pool) *JobsStore {
	return &JobsStore{pool: pool}
}

// Create inserts a new ingestion job.
func (s *JobsStore) Create(ctx context.Context, job *IngestionJob) error {
	query := `
		INSERT INTO ingestion_jobs
			(tenant_id, source_id, status, total_chunks, processed_chunks)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	return s.pool.QueryRow(ctx, query,
		job.TenantID, job.SourceID, job.Status, job.TotalChunks, job.ProcessedChunks,
	).Scan(&job.ID, &job.CreatedAt)
}

// GetByID returns a job by ID.
func (s *JobsStore) GetByID(ctx context.Context, id string) (*IngestionJob, error) {
	var job IngestionJob
	var jobErrMsg *string

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, source_id, status, total_chunks, processed_chunks,
		       error_message, started_at, completed_at, created_at
		FROM ingestion_jobs WHERE id = $1`, id).Scan(
		&job.ID, &job.TenantID, &job.SourceID, &job.Status,
		&job.TotalChunks, &job.ProcessedChunks, &jobErrMsg,
		&job.StartedAt, &job.CompletedAt, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if jobErrMsg != nil {
		job.ErrorMessage = *jobErrMsg
	}
	return &job, nil
}

// ListByTenant returns paginated jobs with optional filters.
func (s *JobsStore) ListByTenant(ctx context.Context, tenantID string, statusFilter *string, sourceID *string, page, pageSize int) ([]IngestionJob, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	countSQL := `SELECT COUNT(*) FROM ingestion_jobs WHERE tenant_id = $1`
	args := []any{tenantID}
	idx := 2

	if statusFilter != nil && *statusFilter != "" {
		countSQL += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, *statusFilter)
		idx++
	}
	if sourceID != nil && *sourceID != "" {
		countSQL += fmt.Sprintf(" AND source_id = $%d", idx)
		args = append(args, *sourceID)
		idx++
	}

	var total int
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, tenant_id, source_id, status, total_chunks, processed_chunks,
		       error_message, started_at, completed_at, created_at
		FROM ingestion_jobs WHERE tenant_id = $1`
	args = []any{tenantID}
	idx = 2

	if statusFilter != nil && *statusFilter != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, *statusFilter)
		idx++
	}
	if sourceID != nil && *sourceID != "" {
		query += fmt.Sprintf(" AND source_id = $%d", idx)
		args = append(args, *sourceID)
		idx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []IngestionJob
	for rows.Next() {
		var item IngestionJob
		var errMsg *string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.SourceID, &item.Status,
			&item.TotalChunks, &item.ProcessedChunks, &errMsg,
			&item.StartedAt, &item.CompletedAt, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if errMsg != nil {
			item.ErrorMessage = *errMsg
		}
		items = append(items, item)
	}
	return items, total, nil
}

// UpdateStatus updates a job's status and optionally processed_chunks, error_message, and completed_at.
func (s *JobsStore) UpdateStatus(ctx context.Context, id string, status string, updates map[string]any) error {
	if updates == nil {
		updates = map[string]any{}
	}
	if updates["status"] == nil {
		updates["status"] = status
	}

	// Build the dynamic update: status always, optional columns only when
	// the caller provided them.
	setClauses := []string{"status = $1", "updated_at = NOW()"}
	args := []any{status}
	idx := 2

	for _, col := range []string{"processed_chunks", "total_chunks", "error_message", "started_at", "completed_at"} {
		if v, ok := updates[col]; ok {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, idx))
			args = append(args, v)
			idx++
		}
	}

	args = append(args, id)

	_, err := s.pool.Exec(ctx,
		fmt.Sprintf("UPDATE ingestion_jobs SET %s WHERE id = $%d",
			strings.Join(setClauses, ", "), idx),
		args...)
	return err
}

// ListPending returns jobs that should be recovered (pending, extracting, embedding).
func (s *JobsStore) ListPending(ctx context.Context) ([]*IngestionJob, error) {
	query := `
		SELECT id, tenant_id, source_id, status, total_chunks, processed_chunks,
		       error_message, started_at, completed_at, created_at
		FROM ingestion_jobs
		WHERE status IN ('pending', 'extracting', 'embedding')
		ORDER BY created_at ASC`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*IngestionJob
	for rows.Next() {
		var item IngestionJob
		var itemErrMsg *string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.SourceID, &item.Status,
			&item.TotalChunks, &item.ProcessedChunks, &itemErrMsg,
			&item.StartedAt, &item.CompletedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if itemErrMsg != nil {
			item.ErrorMessage = *itemErrMsg
		}
		items = append(items, &item)
	}
	return items, nil
}

// ListStuck returns jobs stuck in a non-terminal state for >30 minutes.
func (s *JobsStore) ListStuck(ctx context.Context) ([]*IngestionJob, error) {
	query := `
		SELECT id, tenant_id, source_id, status, total_chunks, processed_chunks,
		       error_message, started_at, completed_at, created_at
		FROM ingestion_jobs
		WHERE status IN ('extracting', 'chunking', 'embedding', 'storing')
		  AND created_at < NOW() - INTERVAL '30 minutes'
		ORDER BY created_at ASC`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*IngestionJob
	for rows.Next() {
		var item IngestionJob
		var itemErrMsg *string
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.SourceID, &item.Status,
			&item.TotalChunks, &item.ProcessedChunks, &itemErrMsg,
			&item.StartedAt, &item.CompletedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if itemErrMsg != nil {
			item.ErrorMessage = *itemErrMsg
		}
		items = append(items, &item)
	}
	return items, nil
}
