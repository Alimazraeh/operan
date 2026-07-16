package store

import (
	"errors"
	"time"
)

// ErrNoRows is returned when a query expected a row but found none.
var ErrNoRows = errors.New("no rows in result set")

// IngestionSource represents a document ingestion source.
type IngestionSource struct {
	ID              string         `db:"id" json:"id"`
	TenantID        string         `db:"tenant_id" json:"tenant_id"`
	Name            string         `db:"name" json:"name"`
	SourceType      string         `db:"source_type" json:"source_type"`
	SourceURL       string         `db:"source_url" json:"source_url"`
	FileType        string         `db:"file_type" json:"file_type,omitempty"`
	FileSizeBytes   int            `db:"file_size_bytes" json:"file_size_bytes"`
	FileHash        string         `db:"file_hash" json:"file_hash,omitempty"`
	NormalizeArabic bool           `db:"normalize_arabic" json:"normalize_arabic"`
	ChunkStrategy   string         `db:"chunk_strategy" json:"chunk_strategy"`
	ChunkSize       int            `db:"chunk_size" json:"chunk_size"`
	ChunkOverlap    int            `db:"chunk_overlap" json:"chunk_overlap"`
	Status          string         `db:"status" json:"status"`
	LastIngested    *time.Time     `db:"last_ingested" json:"last_ingested,omitempty"`
	Metadata        map[string]any `db:"metadata" json:"metadata"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updated_at"`
}

// IngestionJob represents an ingestion job.
type IngestionJob struct {
	ID             string     `db:"id" json:"id"`
	TenantID       string     `db:"tenant_id" json:"tenant_id"`
	SourceID       string     `db:"source_id" json:"source_id"`
	Status         string     `db:"status" json:"status"`
	TotalChunks    int        `db:"total_chunks" json:"total_chunks"`
	ProcessedChunks int       `db:"processed_chunks" json:"processed_chunks"`
	ErrorMessage   string     `db:"error_message" json:"error_message,omitempty"`
	StartedAt      *time.Time `db:"started_at" json:"started_at,omitempty"`
	CompletedAt    *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}

// IngestionResult represents a per-chunk ingestion result.
type IngestionResult struct {
	ID             string         `db:"id" json:"id"`
	TenantID       string         `db:"tenant_id" json:"tenant_id"`
	JobID          string         `db:"job_id" json:"job_id"`
	SourceID       string         `db:"source_id" json:"source_id"`
	ChunkIndex     int            `db:"chunk_index" json:"chunk_index"`
	ChunkHash      string         `db:"chunk_hash" json:"chunk_hash"`
	ChunkText      string         `db:"chunk_text" json:"chunk_text,omitempty"`
	ChunkMetadata  map[string]any `db:"chunk_metadata" json:"chunk_metadata"`
	EmbeddingModel string         `db:"embedding_model" json:"embedding_model,omitempty"`
	VectorDim      int            `db:"vector_dim" json:"vector_dim"`
	Status         string         `db:"status" json:"status"`
	ErrorMessage   string         `db:"error_message" json:"error_message,omitempty"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
}