package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResultsStore handles ingestion_results CRUD.
type ResultsStore struct {
	pool *pgxpool.Pool
}

// NewResultsStore creates a new ResultsStore.
func NewResultsStore(pool *pgxpool.Pool) *ResultsStore {
	return &ResultsStore{pool: pool}
}

// Create inserts a single ingestion result (chunk).
func (s *ResultsStore) Create(ctx context.Context, result *IngestionResult) error {
	metaBytes, err := json.Marshal(result.ChunkMetadata)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO ingestion_results
			(tenant_id, job_id, source_id, chunk_index, chunk_hash,
			 chunk_text, chunk_metadata, embedding_model, vector_dim, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`

	// Truncate chunk_text to 64k chars.
	chunkText := result.ChunkText
	if len(chunkText) > 65536 {
		chunkText = chunkText[:65536]
	}

	return s.pool.QueryRow(ctx, query,
		result.TenantID, result.JobID, result.SourceID,
		result.ChunkIndex, result.ChunkHash, chunkText, metaBytes,
		result.EmbeddingModel, result.VectorDim, result.Status,
	).Scan(&result.ID, &result.CreatedAt)
}

// CreateBulk inserts multiple results in a single transaction.
func (s *ResultsStore) CreateBulk(ctx context.Context, results []IngestionResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO ingestion_results
			(tenant_id, job_id, source_id, chunk_index, chunk_hash,
			 chunk_text, chunk_metadata, embedding_model, vector_dim, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	for _, r := range results {
		metaBytes, err := json.Marshal(r.ChunkMetadata)
		if err != nil {
			return err
		}

		chunkText := r.ChunkText
		if len(chunkText) > 65536 {
			chunkText = chunkText[:65536]
		}

		_, err = tx.Exec(ctx, query,
			r.TenantID, r.JobID, r.SourceID,
			r.ChunkIndex, r.ChunkHash, chunkText, metaBytes,
			r.EmbeddingModel, r.VectorDim, r.Status,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// UpdateStatus updates the status of a specific chunk result.
func (s *ResultsStore) UpdateStatus(ctx context.Context, resultID, status string, errMsg *string) error {
	if errMsg != nil {
		_, err := s.pool.Exec(ctx,
			"UPDATE ingestion_results SET status = $1, error_message = $2 WHERE id = $3",
			status, *errMsg, resultID)
		return err
	}
	_, err := s.pool.Exec(ctx,
		"UPDATE ingestion_results SET status = $1 WHERE id = $2",
		status, resultID)
	return err
}

// GetByJobID returns all results for a job.
func (s *ResultsStore) GetByJobID(ctx context.Context, jobID string) ([]IngestionResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, job_id, source_id, chunk_index, chunk_hash,
		       chunk_text, chunk_metadata, embedding_model, vector_dim,
		       status, error_message, created_at
		FROM ingestion_results WHERE job_id = $1 ORDER BY chunk_index ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []IngestionResult
	for rows.Next() {
		var item IngestionResult
		var metaBytes []byte
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.JobID, &item.SourceID,
			&item.ChunkIndex, &item.ChunkHash, &item.ChunkText, &metaBytes,
			&item.EmbeddingModel, &item.VectorDim, &item.Status,
			&item.ErrorMessage, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaBytes, &item.ChunkMetadata)
		items = append(items, item)
	}
	return items, nil
}

// ExistsByHash checks if a chunk with the given hash already exists for a tenant.
func (s *ResultsStore) ExistsByHash(ctx context.Context, tenantID, chunkHash string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM ingestion_results WHERE tenant_id = $1 AND chunk_hash = $2",
		tenantID, chunkHash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}