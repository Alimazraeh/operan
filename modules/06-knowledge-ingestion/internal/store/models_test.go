package store

import (
	"testing"
)

// TestIngestionSource_StructDefaults validates the IngestionSource struct defaults.
func TestIngestionSource_StructDefaults(t *testing.T) {
	src := IngestionSource{
		ID:              "src-001",
		TenantID:        "tenant-001",
		Name:            "Test Source",
		SourceType:      "file",
		SourceURL:       "https://example.com/doc.pdf",
		FileType:        "pdf",
		ChunkStrategy:   "adaptive",
		ChunkSize:       512,
		ChunkOverlap:    50,
		Status:          "active",
		NormalizeArabic: false,
		Metadata:        map[string]any{"key": "value"},
	}

	if src.Status != "active" {
		t.Errorf("expected status active, got %s", src.Status)
	}
	if src.ChunkStrategy != "adaptive" {
		t.Errorf("expected chunk_strategy adaptive, got %s", src.ChunkStrategy)
	}
	if src.ChunkSize != 512 {
		t.Errorf("expected chunk_size 512, got %d", src.ChunkSize)
	}
	if src.NormalizeArabic {
		t.Error("expected normalize_arabic false by default")
	}
}

// TestIngestionJob_StructDefaults validates the IngestionJob struct defaults.
func TestIngestionJob_StructDefaults(t *testing.T) {
	job := IngestionJob{
		ID:              "job-001",
		TenantID:        "tenant-001",
		SourceID:        "src-001",
		Status:          "pending",
		TotalChunks:     0,
		ProcessedChunks: 0,
	}

	if job.Status != "pending" {
		t.Errorf("expected status pending, got %s", job.Status)
	}
	if job.TotalChunks != 0 {
		t.Errorf("expected total_chunks 0, got %d", job.TotalChunks)
	}
	if job.ProcessedChunks != 0 {
		t.Errorf("expected processed_chunks 0, got %d", job.ProcessedChunks)
	}
}

// TestIngestionResult_StructDefaults validates the IngestionResult struct defaults.
func TestIngestionResult_StructDefaults(t *testing.T) {
	result := IngestionResult{
		ID:              "res-001",
		TenantID:        "tenant-001",
		JobID:           "job-001",
		SourceID:        "src-001",
		ChunkIndex:      0,
		ChunkHash:       "abc123",
		ChunkText:       "test text",
		ChunkMetadata:   map[string]any{"source": "test"},
		Status:          "pending",
	}

	if result.Status != "pending" {
		t.Errorf("expected status pending, got %s", result.Status)
	}
	if result.ChunkIndex != 0 {
		t.Errorf("expected chunk_index 0, got %d", result.ChunkIndex)
	}
}

// TestErrNoRows_IsNotNil verifies the sentinel error exists.
func TestErrNoRows_IsNotNil(t *testing.T) {
	if ErrNoRows == nil {
		t.Fatal("ErrNoRows should not be nil")
	}
}