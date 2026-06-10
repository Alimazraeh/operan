// Package events publishes AsyncAPI events for Module 07 (Memory Fabric).
// Topics follow the platform standard: operan.memory.vector.{event}.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

const topicPrefix = "operan.memory.vector."

// Broker is the interface for event publishing.
type Broker interface {
	Publish(ctx context.Context, topic string, key, value []byte, headers map[string]string) error
	Close() error
}

// logBroker is the default no-op broker that logs events instead of publishing.
type logBroker struct{}

func (l *logBroker) Publish(_ context.Context, topic string, _, value []byte, _ map[string]string) error {
	log.Printf("[EVENT] %s: %s", topic, string(value))
	return nil
}

func (l *logBroker) Close() error { return nil }

// Publisher publishes memory-fabric lifecycle events.
type Publisher struct {
	broker Broker
}

// NewPublisher creates a publisher with a log-only broker.
func NewPublisher() *Publisher { return &Publisher{broker: &logBroker{}} }

// NewPublisherWithBroker creates a publisher backed by a real broker.
func NewPublisherWithBroker(b Broker) *Publisher { return &Publisher{broker: b} }

// SetBroker replaces the underlying broker.
func (p *Publisher) SetBroker(b Broker) { p.broker = b }

// Close shuts down the broker.
func (p *Publisher) Close() error {
	if p.broker != nil {
		return p.broker.Close()
	}
	return nil
}

// envelope holds the message fields required on every AsyncAPI payload.
type envelope struct {
	CorrelationID string `json:"correlationId"`
	TenantID      string `json:"tenantId"`
	MessageID     string `json:"messageId"`
	Timestamp     string `json:"timestamp"`
}

func newEnvelope(tenantID, correlationID string) envelope {
	if correlationID == "" {
		correlationID = uuid.New().String()
	}
	return envelope{
		CorrelationID: correlationID,
		TenantID:      tenantID,
		MessageID:     uuid.New().String(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
}

func (p *Publisher) publish(topic, tenantID string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", topic, err)
	}
	if err := p.broker.Publish(context.Background(), topicPrefix+topic, []byte(tenantID), data, map[string]string{
		"content-type": "application/json",
		"tenant_id":    tenantID,
	}); err != nil {
		log.Printf("[WARN] publish %s%s failed: %v", topicPrefix, topic, err)
		return err
	}
	return nil
}

// ─── Payloads (match asyncapi-07-memory-fabric.yaml) ─────────────────────────

// MemoryVectorIngestedPayload matches the MemoryVectorIngested message.
type MemoryVectorIngestedPayload struct {
	VectorID            string `json:"vector_id"`
	TenantID            string `json:"tenant_id"`
	DocumentID          string `json:"document_id"`
	ChunkID             string `json:"chunk_id"`
	EmbeddingModel      string `json:"embedding_model"`
	EmbeddingDimensions int    `json:"embedding_dimensions"`
	SegmentType         string `json:"segment_type"`
	CreatedAt           string `json:"created_at"`
	envelope
}

// MemoryVectorSearchedPayload matches the MemoryVectorSearched message.
type MemoryVectorSearchedPayload struct {
	QueryID        string `json:"query_id"`
	TenantID       string `json:"tenant_id"`
	QueryType      string `json:"query_type"`
	TopN           int    `json:"top_n"`
	ResultsCount   int    `json:"results_count"`
	ResponseTimeMs int    `json:"response_time_ms"`
	envelope
}

// MemoryVectorUpdatedPayload matches the MemoryVectorUpdated message.
type MemoryVectorUpdatedPayload struct {
	VectorID   string `json:"vector_id"`
	TenantID   string `json:"tenant_id"`
	DocumentID string `json:"document_id"`
	UpdateType string `json:"update_type"` // metadata_refresh | content_update | embedding_refresh
	UpdatedBy  string `json:"updated_by"`
	UpdatedAt  string `json:"updated_at"`
	envelope
}

// MemoryVectorDeletedPayload matches the MemoryVectorDeleted message.
type MemoryVectorDeletedPayload struct {
	VectorIDs []string `json:"vector_ids"`
	TenantID  string   `json:"tenant_id"`
	Reason    string   `json:"reason"` // document_deleted | policy_retention | data_subject_request | tenant_deprovisioned
	DeletedBy string   `json:"deleted_by"`
	DeletedAt string   `json:"deleted_at"`
	envelope
}

// MemoryVectorGarbageCollectedPayload matches the MemoryVectorGarbageCollected message.
type MemoryVectorGarbageCollectedPayload struct {
	BatchSize   int    `json:"batch_size"`
	DeletedAt   string `json:"deleted_at"`
	Reason      string `json:"reason"` // retention_policy | tenant_deprovisioned | cleanup_job
	TriggeredBy string `json:"triggered_by"`
	envelope
}

// ─── Typed publish methods ───────────────────────────────────────────────────

// PublishVectorIngested emits operan.memory.vector.ingested.
func (p *Publisher) PublishVectorIngested(pl MemoryVectorIngestedPayload, correlationID string) error {
	pl.envelope = newEnvelope(pl.TenantID, correlationID)
	return p.publish("ingested", pl.TenantID, pl)
}

// PublishVectorSearched emits operan.memory.vector.searched.
func (p *Publisher) PublishVectorSearched(pl MemoryVectorSearchedPayload, correlationID string) error {
	pl.envelope = newEnvelope(pl.TenantID, correlationID)
	return p.publish("searched", pl.TenantID, pl)
}

// PublishVectorUpdated emits operan.memory.vector.updated.
func (p *Publisher) PublishVectorUpdated(pl MemoryVectorUpdatedPayload, correlationID string) error {
	pl.envelope = newEnvelope(pl.TenantID, correlationID)
	return p.publish("updated", pl.TenantID, pl)
}

// PublishVectorDeleted emits operan.memory.vector.deleted.
func (p *Publisher) PublishVectorDeleted(pl MemoryVectorDeletedPayload, correlationID string) error {
	pl.envelope = newEnvelope(pl.TenantID, correlationID)
	return p.publish("deleted", pl.TenantID, pl)
}

// PublishVectorGarbageCollected emits operan.memory.vector.garbage_collected.
func (p *Publisher) PublishVectorGarbageCollected(pl MemoryVectorGarbageCollectedPayload, tenantID, correlationID string) error {
	pl.envelope = newEnvelope(tenantID, correlationID)
	return p.publish("garbage_collected", tenantID, pl)
}
