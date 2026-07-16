package events

import (
	"context"
	"encoding/json"
	"log"
)

// Broker publishes messages to an event bus.
type Broker interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
	Close() error
}

// NoOpBroker is a no-op implementation for when no broker is configured.
type NoOpBroker struct{}

func (b NoOpBroker) Publish(ctx context.Context, topic string, key, value []byte) error {
	return nil
}

func (b NoOpBroker) Close() error {
	return nil
}

// Events wraps a Broker for publishing knowledge-ingestion events.
type Events struct {
	broker Broker
	logger *log.Logger
}

// NewEvents creates a new Events instance.
func NewEvents(broker Broker, logger *log.Logger) *Events {
	return &Events{broker: broker, logger: logger}
}

// IngestionStarted publishes when a new ingestion job begins.
func (e *Events) IngestionStarted(ctx context.Context, tenantID, jobID, sourceID, fileType string, fileSize int64) {
	payload := map[string]any{
		"tenant_id":     tenantID,
		"job_id":        jobID,
		"source_id":     sourceID,
		"file_type":     fileType,
		"file_size_bytes": fileSize,
	}
	e.publish(ctx, "operan.knowledge.ingestion_started", jobID, payload)
}

// ChunkCreated publishes when each chunk is extracted.
func (e *Events) ChunkCreated(ctx context.Context, tenantID, jobID, chunkHash string, chunkIndex int) {
	payload := map[string]any{
		"tenant_id":    tenantID,
		"job_id":       jobID,
		"chunk_index":  chunkIndex,
		"chunk_hash":   chunkHash,
	}
	e.publish(ctx, "operan.knowledge.chunk_created", jobID, payload)
}

// EmbeddingStored publishes when a chunk embedding is stored.
func (e *Events) EmbeddingStored(ctx context.Context, tenantID, jobID string, chunkIndex int, model string, vectorDim int) {
	payload := map[string]any{
		"tenant_id":   tenantID,
		"job_id":      jobID,
		"chunk_index": chunkIndex,
		"model":       model,
		"vector_dim":  vectorDim,
	}
	e.publish(ctx, "operan.knowledge.embedding_stored", jobID, payload)
}

// IngestionCompleted publishes when a job finishes successfully.
func (e *Events) IngestionCompleted(ctx context.Context, tenantID, jobID, sourceID string, totalChunks int) {
	payload := map[string]any{
		"tenant_id":     tenantID,
		"job_id":        jobID,
		"source_id":     sourceID,
		"total_chunks":  totalChunks,
	}
	e.publish(ctx, "operan.knowledge.ingestion_completed", jobID, payload)
}

// IngestionFailed publishes when a job fails.
func (e *Events) IngestionFailed(ctx context.Context, tenantID, jobID, sourceID, errMsg string) {
	payload := map[string]any{
		"tenant_id":    tenantID,
		"job_id":       jobID,
		"source_id":    sourceID,
		"error_message": errMsg,
	}
	e.publish(ctx, "operan.knowledge.ingestion_failed", jobID, payload)
}

func (e *Events) publish(ctx context.Context, topic, key string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		e.logger.Printf("events marshal error: %v", err)
		return
	}
	if err := e.broker.Publish(ctx, topic, []byte(key), data); err != nil {
		e.logger.Printf("events publish error (%s): %v", topic, err)
	}
}