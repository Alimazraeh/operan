package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TerminologyStore handles PostgreSQL access to the terminology glossary.
type TerminologyStore struct {
	pool PgxPool
}

// PgxPool is an interface for pgx pool operations we need.
type PgxPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NewTerminologyStore creates a new terminology store.
func NewTerminologyStore(pool PgxPool) *TerminologyStore {
	return &TerminologyStore{pool: pool}
}

// Create inserts a new glossary entry and returns it.
func (s *TerminologyStore) Create(ctx context.Context, entry *TerminologyGlossary) error {
	query := `
		INSERT INTO terminology_glossary
			(tenant_id, term_arabic, term_english, term_transliterated, category, domain,
			 status, approved_by, preferred_form, alternatives, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`

	err := s.pool.QueryRow(ctx, query,
		entry.TenantID, entry.TermArabic, entry.TermEnglish, entry.TermTransliterated,
		entry.Category, entry.Domain, entry.Status, entry.ApprovedBy,
		entry.PreferredForm, serializeStringArray(entry.Alternatives), entry.Notes,
	).Scan(&entry.ID, &entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return fmt.Errorf("terminology_store.create: %w", err)
	}
	return nil
}

// GetByID returns a single glossary entry.
func (s *TerminologyStore) GetByID(ctx context.Context, id string) (*TerminologyGlossary, error) {
	var entry TerminologyGlossary
	query := `
		SELECT id, tenant_id, term_arabic, term_english, term_transliterated,
		       category, domain, status, approved_by, preferred_form, alternatives, notes,
		       created_at, updated_at
		FROM terminology_glossary WHERE id = $1`

	err := s.pool.QueryRow(ctx, query, id).Scan(
		&entry.ID, &entry.TenantID, &entry.TermArabic, &entry.TermEnglish, &entry.TermTransliterated,
		&entry.Category, &entry.Domain, &entry.Status, &entry.ApprovedBy, &entry.PreferredForm,
		&entry.Alternatives, &entry.Notes, &entry.CreatedAt, &entry.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("terminology_store.get_by_id: %w", err)
	}
	return &entry, nil
}

// List returns paginated glossary entries with optional filters.
func (s *TerminologyStore) List(ctx context.Context, tenantID, category, domain, status string, page, pageSize int) ([]TerminologyGlossary, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var whereParts []string
	var args []interface{}
	argIdx := 1

	whereParts = append(whereParts, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, tenantID)
	argIdx++

	if category != "" {
		whereParts = append(whereParts, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, category)
		argIdx++
	}
	if domain != "" {
		whereParts = append(whereParts, fmt.Sprintf("domain = $%d", argIdx))
		args = append(args, domain)
		argIdx++
	}
	if status != "" {
		whereParts = append(whereParts, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = "WHERE " + strings.Join(whereParts, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, term_arabic, term_english, term_transliterated,
		       category, domain, status, approved_by, preferred_form, alternatives, notes,
		       created_at, updated_at
		FROM terminology_glossary %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("terminology_store.list: %w", err)
	}
	defer rows.Close()

	var entries []TerminologyGlossary
	for rows.Next() {
		var e TerminologyGlossary
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.TermArabic, &e.TermEnglish, &e.TermTransliterated,
			&e.Category, &e.Domain, &e.Status, &e.ApprovedBy, &e.PreferredForm,
			&e.Alternatives, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}

	// Count total.
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM terminology_glossary %s`, whereClause)
	var total int
	err = s.pool.QueryRow(ctx, countQuery, args[:len(args)-2]...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("terminology_store.list count: %w", err)
	}

	return entries, total, nil
}

// Update modifies an existing glossary entry.
func (s *TerminologyStore) Update(ctx context.Context, entry *TerminologyGlossary) error {
	query := `
		UPDATE terminology_glossary
		SET term_english = COALESCE($2, term_english),
			term_transliterated = COALESCE($3, term_transliterated),
			term_arabic = COALESCE($4, term_arabic),
			category = COALESCE($5, category),
			domain = COALESCE($6, domain),
			status = COALESCE($7, status),
			approved_by = COALESCE($8, approved_by),
			preferred_form = COALESCE($9, preferred_form),
			alternatives = COALESCE($10, alternatives),
			notes = COALESCE($11, notes),
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $12
		RETURNING updated_at`

	err := s.pool.QueryRow(ctx, query,
		entry.ID, entry.TermEnglish, entry.TermTransliterated, entry.TermArabic,
		entry.Category, entry.Domain, entry.Status, entry.ApprovedBy,
		entry.PreferredForm, serializeStringArray(entry.Alternatives), entry.Notes,
		entry.TenantID,
	).Scan(&entry.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoRows
		}
		return fmt.Errorf("terminology_store.update: %w", err)
	}
	return nil
}

// Delete removes a glossary entry.
func (s *TerminologyStore) Delete(ctx context.Context, id, tenantID string) error {
	result, err := s.pool.Exec(ctx,
		"DELETE FROM terminology_glossary WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return fmt.Errorf("terminology_store.delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

// CheckTerms searches for terms matching tokens in source text.
func (s *TerminologyStore) CheckTerms(ctx context.Context, tenantID, text string, category, domain string) ([]TerminologyGlossary, error) {
	query := `
		SELECT id, tenant_id, term_arabic, term_english, term_transliterated,
		       category, domain, status, approved_by, preferred_form, alternatives, notes,
		       created_at, updated_at
		FROM terminology_glossary
		WHERE tenant_id = $1
		  AND status = 'active'`

	var args []interface{}
	argIdx := 2

	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}
	if domain != "" {
		query += fmt.Sprintf(" AND domain = $%d", argIdx)
		args = append(args, domain)
		argIdx++
	}

	query += " ORDER BY length(term_arabic) DESC" // longest match first

	rows, err := s.pool.Query(ctx, query, append(args, tenantID)...)
	if err != nil {
		return nil, fmt.Errorf("terminology_store.check_terms: %w", err)
	}
	defer rows.Close()

	var terms []TerminologyGlossary
	for rows.Next() {
		var t TerminologyGlossary
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.TermArabic, &t.TermEnglish, &t.TermTransliterated,
			&t.Category, &t.Domain, &t.Status, &t.ApprovedBy, &t.PreferredForm,
			&t.Alternatives, &t.Notes, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		terms = append(terms, t)
	}
	return terms, nil
}

// LogUsage records a terminology check for audit.
func (s *TerminologyStore) LogUsage(ctx context.Context, log *TerminologyUsageLog) error {
	query := `
		INSERT INTO terminology_usage_log
			(tenant_id, source_text, matched_terms, flagged_terms, checked_by)
		VALUES ($1, $2, $3, $4, $5)`

	matchedJSON, _ := json.Marshal(log.MatchedTerms)
	flaggedJSON, _ := json.Marshal(log.FlaggedTerms)

	_, err := s.pool.Exec(ctx, query,
		log.TenantID, log.SourceText, matchedJSON, flaggedJSON, log.CheckedBy)
	if err != nil {
		return fmt.Errorf("terminology_store.log_usage: %w", err)
	}
	return nil
}

// LogEmbeddingRequest records an embedding request for monitoring.
func (s *TerminologyStore) LogEmbeddingRequest(ctx context.Context, req *ArabicEmbeddingRequest) error {
	query := `
		INSERT INTO arabic_embedding_requests
			(tenant_id, text_length, embedding_model, vector_dim, status, error_message, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.pool.Exec(ctx, query,
		req.TenantID, req.TextLength, req.EmbeddingModel, req.VectorDim,
		req.Status, req.ErrorMessage, req.DurationMs)
	if err != nil {
		return fmt.Errorf("terminology_store.log_embedding: %w", err)
	}
	return nil
}

// Helper to serialize string slices for PostgreSQL text[]
func serializeStringArray(arr []string) interface{} {
	if len(arr) == 0 {
		return []string(nil)
	}
	return arr
}