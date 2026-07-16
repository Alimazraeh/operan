package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SourcesStore handles ingestion_sources CRUD.
type SourcesStore struct {
	pool *pgxpool.Pool
}

// NewSourcesStore creates a new SourcesStore.
func NewSourcesStore(pool *pgxpool.Pool) *SourcesStore {
	return &SourcesStore{pool: pool}
}

// Create inserts a new ingestion source.
func (s *SourcesStore) Create(ctx context.Context, src *IngestionSource) error {
	metaBytes, err := json.Marshal(src.Metadata)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO ingestion_sources
			(tenant_id, name, source_type, source_url, file_type,
			 file_size_bytes, file_hash, normalize_arabic,
			 chunk_strategy, chunk_size, chunk_overlap, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at`

	return s.pool.QueryRow(ctx, query,
		src.TenantID, src.Name, src.SourceType, src.SourceURL,
		src.FileType, src.FileSizeBytes, src.FileHash,
		src.NormalizeArabic, src.ChunkStrategy, src.ChunkSize,
		src.ChunkOverlap, src.Status, metaBytes,
	).Scan(&src.ID, &src.CreatedAt, &src.UpdatedAt)
}

// GetByID returns a source by ID.
func (s *SourcesStore) GetByID(ctx context.Context, id string) (*IngestionSource, error) {
	var src IngestionSource
	var metaBytes []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, source_type, source_url, file_type,
		       file_size_bytes, file_hash, normalize_arabic, chunk_strategy,
		       chunk_size, chunk_overlap, status, last_ingested, metadata,
		       created_at, updated_at
		FROM ingestion_sources WHERE id = $1`, id).Scan(
		&src.ID, &src.TenantID, &src.Name, &src.SourceType, &src.SourceURL,
		&src.FileType, &src.FileSizeBytes, &src.FileHash, &src.NormalizeArabic,
		&src.ChunkStrategy, &src.ChunkSize, &src.ChunkOverlap, &src.Status,
		&src.LastIngested, &metaBytes, &src.CreatedAt, &src.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(metaBytes, &src.Metadata)
	return &src, nil
}

// ListByTenant returns paginated sources with optional filters.
func (s *SourcesStore) ListByTenant(ctx context.Context, tenantID string, sourceType *string, page, pageSize int) ([]IngestionSource, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int
	countSQL := `SELECT COUNT(*) FROM ingestion_sources WHERE tenant_id = $1`
	countArgs := []any{tenantID}
	idx := 2

	if sourceType != nil && *sourceType != "" {
		countSQL += fmt.Sprintf(" AND source_type = $%d", idx)
		countArgs = append(countArgs, *sourceType)
		idx++
	}

	if err := s.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, tenant_id, name, source_type, source_url, file_type,
		       file_size_bytes, file_hash, normalize_arabic, chunk_strategy,
		       chunk_size, chunk_overlap, status, last_ingested, metadata,
		       created_at, updated_at
		FROM ingestion_sources WHERE tenant_id = $1`
	args := []any{tenantID}
	idx = 2

	if sourceType != nil && *sourceType != "" {
		query += fmt.Sprintf(" AND source_type = $%d", idx)
		args = append(args, *sourceType)
		idx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []IngestionSource
	for rows.Next() {
		var item IngestionSource
		var metaBytes []byte
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.Name, &item.SourceType, &item.SourceURL,
			&item.FileType, &item.FileSizeBytes, &item.FileHash, &item.NormalizeArabic,
			&item.ChunkStrategy, &item.ChunkSize, &item.ChunkOverlap, &item.Status,
			&item.LastIngested, &metaBytes, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(metaBytes, &item.Metadata)
		items = append(items, item)
	}
	return items, total, nil
}

// Update updates an existing source's configuration.
func (s *SourcesStore) Update(ctx context.Context, src *IngestionSource) error {
	metaBytes, err := json.Marshal(src.Metadata)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE ingestion_sources
		SET name = $1, source_type = $2, source_url = $3, file_type = $4,
		    file_size_bytes = $5, file_hash = $6, normalize_arabic = $7,
		    chunk_strategy = $8, chunk_size = $9, chunk_overlap = $10,
		    status = $11, metadata = $12, updated_at = NOW()
		WHERE id = $13`,
		src.Name, src.SourceType, src.SourceURL, src.FileType,
		src.FileSizeBytes, src.FileHash, src.NormalizeArabic,
		src.ChunkStrategy, src.ChunkSize, src.ChunkOverlap,
		src.Status, metaBytes, src.ID,
	)
	return err
}

// Delete removes a source by ID.
func (s *SourcesStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM ingestion_sources WHERE id = $1", id)
	return err
}

// GetByHash returns a source with the given file hash for deduplication.
func (s *SourcesStore) GetByHash(ctx context.Context, tenantID, fileHash string) (*IngestionSource, error) {
	var src IngestionSource
	var metaBytes []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, source_type, source_url, file_type,
		       file_size_bytes, file_hash, normalize_arabic, chunk_strategy,
		       chunk_size, chunk_overlap, status, last_ingested, metadata,
		       created_at, updated_at
		FROM ingestion_sources WHERE tenant_id = $1 AND file_hash = $2 LIMIT 1`,
		tenantID, fileHash).Scan(
		&src.ID, &src.TenantID, &src.Name, &src.SourceType, &src.SourceURL,
		&src.FileType, &src.FileSizeBytes, &src.FileHash, &src.NormalizeArabic,
		&src.ChunkStrategy, &src.ChunkSize, &src.ChunkOverlap, &src.Status,
		&src.LastIngested, &metaBytes, &src.CreatedAt, &src.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(metaBytes, &src.Metadata)
	return &src, nil
}