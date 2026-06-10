// Package store provides in-memory, tenant-isolated storage for Module 07
// (Memory Fabric): memory vectors, retention policies, and GC operations.
package store

import (
	"errors"
	"time"
)

// Sentinel errors shared by all stores.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrTenantMismatch = errors.New("tenant_id is required or does not match")
	ErrValidation     = errors.New("validation failed")
)

// timeNow returns the current UTC time (indirection eases testing).
var timeNow = func() time.Time { return time.Now().UTC() }

// EmbeddingType is the scope of a memory vector.
type EmbeddingType string

const (
	EmbeddingAgentPersonal  EmbeddingType = "agent_personal"
	EmbeddingAgentEphemeral EmbeddingType = "agent_ephemeral"
	EmbeddingDepartment     EmbeddingType = "department"
	EmbeddingPlatform       EmbeddingType = "platform"
)

// ValidEmbeddingType reports whether s is a known embedding type.
func ValidEmbeddingType(s string) bool {
	switch EmbeddingType(s) {
	case EmbeddingAgentPersonal, EmbeddingAgentEphemeral, EmbeddingDepartment, EmbeddingPlatform:
		return true
	}
	return false
}

// SegmentType categorizes the source segment of a vector.
type SegmentType string

const (
	SegmentFact       SegmentType = "fact"
	SegmentContext    SegmentType = "context"
	SegmentMemory     SegmentType = "memory"
	SegmentToolOutput SegmentType = "tool_output"
	SegmentPolicy     SegmentType = "policy"
)

// ValidSegmentType reports whether s is a known segment type.
func ValidSegmentType(s string) bool {
	switch SegmentType(s) {
	case SegmentFact, SegmentContext, SegmentMemory, SegmentToolOutput, SegmentPolicy:
		return true
	}
	return false
}

// MemoryType is the scope used by retention policies and GC.
type MemoryType string

const (
	MemoryPersonal   MemoryType = "personal"
	MemoryEphemeral  MemoryType = "ephemeral"
	MemoryDepartment MemoryType = "department"
	MemoryPlatform   MemoryType = "platform"
)

// ValidMemoryType reports whether s is a known memory type.
func ValidMemoryType(s string) bool {
	switch MemoryType(s) {
	case MemoryPersonal, MemoryEphemeral, MemoryDepartment, MemoryPlatform:
		return true
	}
	return false
}

// EmbeddingTypeForMemoryType maps a retention/GC memory type to the
// embedding type it governs.
func EmbeddingTypeForMemoryType(m MemoryType) EmbeddingType {
	switch m {
	case MemoryPersonal:
		return EmbeddingAgentPersonal
	case MemoryEphemeral:
		return EmbeddingAgentEphemeral
	case MemoryDepartment:
		return EmbeddingDepartment
	default:
		return EmbeddingPlatform
	}
}

// MemoryVector matches the OpenAPI MemoryVector schema.
type MemoryVector struct {
	ID              string                 `json:"id"`
	TenantID        string                 `json:"tenant_id"`
	DocumentID      string                 `json:"document_id"`
	EmbeddingType   EmbeddingType          `json:"embedding_type"`
	SemanticContent string                 `json:"semantic_content"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       time.Time              `json:"created_at"`
	EmbeddingModel  string                 `json:"embedding_model,omitempty"`
	ChunkID         string                 `json:"chunk_id,omitempty"`
	SegmentType     SegmentType            `json:"segment_type,omitempty"`
	EmbeddingVector []float64              `json:"embedding_vector,omitempty"`
	VectorHash      string                 `json:"vector_hash,omitempty"`
	TTL             *time.Time             `json:"ttl"`
}

// RetentionPolicy matches the OpenAPI RetentionPolicy schema.
type RetentionPolicy struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	MemoryType     MemoryType `json:"memory_type"`
	MaxAgeDays     int        `json:"max_age_days,omitempty"`
	MaxMemoryCount int        `json:"max_memory_count,omitempty"`
	TTLSeconds     int        `json:"ttl_seconds,omitempty"`
	AutoGCEnabled  bool       `json:"auto_gc_enabled,omitempty"`
	CreationDate   time.Time  `json:"creation_date,omitempty"`
}

// OperationStatus matches the OpenAPI OperationStatus schema (GC jobs).
type OperationStatus struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"` // processing | completed | failed
	BatchSize    int        `json:"batch_size,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// AgentMemory matches the OpenAPI AgentMemory schema.
type AgentMemory struct {
	AgentID          string           `json:"agent_id"`
	TenantID         string           `json:"tenant_id"`
	PersonalMemories []string         `json:"personal_memories"`
	EphemeralWindow  *EphemeralWindow `json:"ephemeral_window,omitempty"`
	LastUpdated      *time.Time       `json:"last_updated,omitempty"`
	Status           string           `json:"status,omitempty"` // active | suspended
}

// EphemeralWindow matches the OpenAPI EphemeralWindow schema.
type EphemeralWindow struct {
	MaxTokens  int `json:"max_tokens,omitempty"`
	TTLSeconds int `json:"ttl_seconds,omitempty"`
}
