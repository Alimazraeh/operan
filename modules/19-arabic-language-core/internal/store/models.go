package store

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrNoRows = errors.New("no rows in result set")

// TerminologyGlossary represents an entry in the terminology glossary.
type TerminologyGlossary struct {
	ID                 string         `db:"id" json:"id"`
	TenantID           string         `db:"tenant_id" json:"tenant_id"`
	TermArabic         string         `db:"term_arabic" json:"term_arabic"`
	TermEnglish        string         `db:"term_english" json:"term_english,omitempty"`
	TermTransliterated string         `db:"term_transliterated" json:"term_transliterated,omitempty"`
	Category           string         `db:"category" json:"category"`
	Domain             string         `db:"domain" json:"domain"`
	Status             string         `db:"status" json:"status"`
	ApprovedBy         string         `db:"approved_by" json:"approved_by,omitempty"`
	PreferredForm      string         `db:"preferred_form" json:"preferred_form,omitempty"`
	Alternatives       []string       `db:"alternatives" json:"alternatives"`
	Notes              string         `db:"notes" json:"notes,omitempty"`
	CreatedAt          time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at" json:"updated_at"`
}

// TerminologyUsageLog records every terminology check.
type TerminologyUsageLog struct {
	ID           string     `db:"id" json:"id"`
	TenantID     string     `db:"tenant_id" json:"tenant_id"`
	SourceText   string     `db:"source_text" json:"source_text"`
	MatchedTerms jsonByte   `db:"matched_terms" json:"matched_terms"`
	FlaggedTerms jsonByte   `db:"flagged_terms" json:"flagged_terms"`
	CheckedBy    string     `db:"checked_by" json:"checked_by,omitempty"`
	Timestamp    time.Time  `db:"timestamp" json:"timestamp"`
}

// ArabicEmbeddingRequest logs embedding calls to M12.
type ArabicEmbeddingRequest struct {
	ID             string    `db:"id" json:"id"`
	TenantID       string    `db:"tenant_id" json:"tenant_id"`
	TextLength     int       `db:"text_length" json:"text_length"`
	EmbeddingModel string    `db:"embedding_model" json:"embedding_model"`
	VectorDim      int       `db:"vector_dim" json:"vector_dim"`
	Status         string    `db:"status" json:"status"`
	ErrorMessage   string    `db:"error_message" json:"error_message,omitempty"`
	DurationMs     int       `db:"duration_ms" json:"duration_ms"`
	Timestamp      time.Time `db:"timestamp" json:"timestamp"`
}

// SliceToJSONByte converts a slice to json.RawMessage bytes for JSONB storage.
func SliceToJSONByte(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

// jsonByte is a raw JSON byte slice for JSONB columns.
type jsonByte []byte