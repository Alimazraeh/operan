package events

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// mockBroker captures published messages for assertions.
type mockBroker struct {
	mu        sync.Mutex
	published []capturedMessage
	closed    bool
}

type capturedMessage struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

func (m *mockBroker) Publish(_ context.Context, topic string, key, value []byte, headers map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, capturedMessage{Topic: topic, Key: key, Value: value, Headers: headers})
	return nil
}

func (m *mockBroker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockBroker) last(t *testing.T) capturedMessage {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.published) == 0 {
		t.Fatal("no messages published")
	}
	return m.published[len(m.published)-1]
}

func TestPublishVectorIngested(t *testing.T) {
	mock := &mockBroker{}
	p := NewPublisherWithBroker(mock)

	err := p.PublishVectorIngested(MemoryVectorIngestedPayload{
		VectorID:            "v-1",
		TenantID:            "t-1",
		DocumentID:          "d-1",
		ChunkID:             "c-1",
		EmbeddingModel:      "stub",
		EmbeddingDimensions: 3,
		SegmentType:         "fact",
		CreatedAt:           "2026-06-10T00:00:00Z",
	}, "corr-1")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg := mock.last(t)
	if msg.Topic != "operan.memory.vector.ingested" {
		t.Errorf("topic = %q", msg.Topic)
	}
	if string(msg.Key) != "t-1" {
		t.Errorf("key = %q, want tenant id", string(msg.Key))
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(msg.Value, &decoded); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	for _, field := range []string{"vector_id", "tenant_id", "document_id", "chunk_id", "embedding_model", "embedding_dimensions", "segment_type", "created_at", "correlationId", "tenantId", "messageId", "timestamp"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("payload missing required field %q", field)
		}
	}
	if decoded["correlationId"] != "corr-1" {
		t.Errorf("correlationId = %v", decoded["correlationId"])
	}
}

func TestPublishAllEventTopics(t *testing.T) {
	mock := &mockBroker{}
	p := NewPublisherWithBroker(mock)

	p.PublishVectorSearched(MemoryVectorSearchedPayload{QueryID: "q", TenantID: "t-1", QueryType: "department", TopN: 5, ResultsCount: 2, ResponseTimeMs: 7}, "")
	p.PublishVectorUpdated(MemoryVectorUpdatedPayload{VectorID: "v", TenantID: "t-1", DocumentID: "d", UpdateType: "content_update", UpdatedBy: "u", UpdatedAt: "2026-06-10T00:00:00Z"}, "")
	p.PublishVectorDeleted(MemoryVectorDeletedPayload{VectorIDs: []string{"v"}, TenantID: "t-1", Reason: "document_deleted", DeletedBy: "u", DeletedAt: "2026-06-10T00:00:00Z"}, "")
	p.PublishVectorGarbageCollected(MemoryVectorGarbageCollectedPayload{BatchSize: 3, DeletedAt: "2026-06-10T00:00:00Z", Reason: "cleanup_job", TriggeredBy: "u"}, "t-1", "")

	want := []string{
		"operan.memory.vector.searched",
		"operan.memory.vector.updated",
		"operan.memory.vector.deleted",
		"operan.memory.vector.garbage_collected",
	}
	if len(mock.published) != len(want) {
		t.Fatalf("published %d messages, want %d", len(mock.published), len(want))
	}
	for i, topic := range want {
		if mock.published[i].Topic != topic {
			t.Errorf("message %d topic = %q, want %q", i, mock.published[i].Topic, topic)
		}
		var decoded map[string]interface{}
		json.Unmarshal(mock.published[i].Value, &decoded)
		if decoded["messageId"] == "" || decoded["correlationId"] == "" {
			t.Errorf("message %d missing envelope ids", i)
		}
	}
}

func TestEnvelopeGeneratesCorrelationID(t *testing.T) {
	env := newEnvelope("t-1", "")
	if env.CorrelationID == "" || env.MessageID == "" || env.Timestamp == "" {
		t.Errorf("envelope incomplete: %+v", env)
	}
	if env.TenantID != "t-1" {
		t.Errorf("tenantId = %q", env.TenantID)
	}
}

func TestLogBrokerDefaultAndClose(t *testing.T) {
	p := NewPublisher()
	if err := p.PublishVectorIngested(MemoryVectorIngestedPayload{TenantID: "t-1"}, ""); err != nil {
		t.Errorf("log broker publish should not error: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	mock := &mockBroker{}
	p2 := NewPublisherWithBroker(mock)
	p2.Close()
	if !mock.closed {
		t.Error("Close should close the broker")
	}
}

func TestNewKafkaBrokerValidation(t *testing.T) {
	if _, err := NewKafkaBroker(""); err == nil {
		t.Error("empty URL should error")
	}
	if b, err := NewKafkaBroker("kafka://b1:9092,b2:9092"); err != nil || b == nil {
		t.Errorf("valid URL should construct: %v", err)
	}
}
