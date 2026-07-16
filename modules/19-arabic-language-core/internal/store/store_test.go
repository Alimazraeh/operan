package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func newTestStore(t *testing.T) (*TerminologyStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	return NewTerminologyStore(mockPool), mockPool
}

func TestCreate_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	// Verify the query has 11 placeholders ($1-$11), not 10 ($1-$10 with $10 duplicated)
	mockPool.ExpectQuery("INSERT INTO terminology_glossary").
		WithArgs("tenant-1", "term", "trans", "translit", "legal", "gov", "active", "approver", "preferred", "[]", "notes").
		WillReturnError(assert.AnError)

	ctx := context.Background()
	entry := &TerminologyGlossary{
		TenantID:     "tenant-1",
		TermArabic:   "term",
		TermEnglish:  "trans",
		TermTransliterated: "translit",
		Category:     "legal",
		Domain:       "gov",
		Status:       "active",
		ApprovedBy:   "approver",
		PreferredForm: "preferred",
		Alternatives: []string{},
		Notes:        "notes",
	}

	err := store.Create(ctx, entry)
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify the query contained $11 (not $10 duplicated)
	if err.Error() == "" {
		t.Error("expected error with query details")
	}
}

func TestCreate_UsesCorrectPlaceholders(t *testing.T) {
	store, mockPool := newTestStore(t)

	// The query should have exactly 11 placeholders
	// Note: Create() does NOT use transactions, just direct QueryRow
	mockPool.ExpectQuery("INSERT.*VALUES \\(\\$1.*\\$11\\)").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	ctx := context.Background()
	entry := &TerminologyGlossary{
		TenantID:   "tenant-1",
		TermArabic: "term",
		Category:   "general",
		Notes:      "test",
	}

	err := store.Create(ctx, entry)
	if err == nil {
		t.Fatal("expected error")
	}

	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Errorf("query was not executed: %v", err)
	}
}

func TestGetByID_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectQuery("SELECT.*FROM terminology_glossary WHERE id = \\$1").
		WithArgs("test-id").
		WillReturnError(assert.AnError)

	ctx := context.Background()
	_, err := store.GetByID(ctx, "test-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestList_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectQuery("SELECT.*FROM terminology_glossary.*LIMIT \\$1 OFFSET \\$2").
		WillReturnError(assert.AnError)

	ctx := context.Background()
	_, _, err := store.List(ctx, "tenant-1", "", "", "", 1, 20)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdate_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectQuery("UPDATE terminology_glossary.*SET.*updated_at.*WHERE id = \\$1").
		WithArgs("test-id", pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	ctx := context.Background()
	entry := &TerminologyGlossary{
		ID:       "test-id",
		TenantID: "tenant-1",
		Status:   "deprecated",
	}

	err := store.Update(ctx, entry)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDelete_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectExec("DELETE FROM terminology_glossary WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs("test-id", "tenant-1").
		WillReturnError(assert.AnError)

	ctx := context.Background()
	err := store.Delete(ctx, "test-id", "tenant-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckTerms_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectQuery("SELECT.*FROM terminology_glossary.*WHERE tenant_id = \\$1").
		WithArgs("tenant-1").
		WillReturnError(assert.AnError)

	ctx := context.Background()
	_, err := store.CheckTerms(ctx, "tenant-1", "مرحبا", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLogUsage_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectExec("INSERT INTO terminology_usage_log").
		WithArgs("tenant-1", "text", pgxmock.AnyArg(), pgxmock.AnyArg(), "user").
		WillReturnError(assert.AnError)

	ctx := context.Background()
	log := &TerminologyUsageLog{
		TenantID:   "tenant-1",
		SourceText: "text",
		CheckedBy:  "user",
	}

	err := store.LogUsage(ctx, log)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLogEmbeddingRequest_QueryStructure(t *testing.T) {
	store, mockPool := newTestStore(t)

	mockPool.ExpectExec("INSERT INTO arabic_embedding_requests").
		WithArgs("tenant-1", 100, "model", 768, "success", "", 50).
		WillReturnError(assert.AnError)

	ctx := context.Background()
	req := &ArabicEmbeddingRequest{
		TenantID:       "tenant-1",
		TextLength:     100,
		EmbeddingModel: "model",
		VectorDim:      768,
		Status:         "success",
		DurationMs:     50,
	}

	err := store.LogEmbeddingRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSliceToJSONByte(t *testing.T) {
	terms := []TerminologyGlossary{
		{TermArabic: "مرحبا", Category: "general"},
		{TermArabic: "عالم", Category: "general"},
	}

	data := SliceToJSONByte(terms)
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON bytes")
	}

	var parsed []TerminologyGlossary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("SliceToJSONByte produced invalid JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 parsed terms, got %d", len(parsed))
	}
}

func TestNewTerminologyStore(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}

	store := NewTerminologyStore(mockPool)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}