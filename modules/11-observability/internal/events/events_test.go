package events

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

type mockBroker struct {
	mu        sync.Mutex
	published []captured
	closed    bool
}

type captured struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

func (m *mockBroker) Publish(_ context.Context, topic string, key, value []byte, headers map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, captured{Topic: topic, Key: key, Value: value, Headers: headers})
	return nil
}

func (m *mockBroker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func TestPublishEnvelopeInHeaders(t *testing.T) {
	mock := &mockBroker{}
	p := NewPublisherWithBroker(mock)

	err := p.PublishMetricRecorded(MetricRecordedPayload{
		MetricID:    "m-1",
		TenantID:    "t-1",
		MetricName:  "tokens",
		MetricValue: 5,
		MetricType:  "counter",
		Labels:      map[string]interface{}{},
		SourceID:    "s-1",
		RecordedAt:  "2026-06-10T00:00:00Z",
	}, "corr-1")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg := mock.published[0]
	if msg.Topic != "operan.observability.metric.recorded" {
		t.Errorf("topic = %q", msg.Topic)
	}
	if string(msg.Key) != "t-1" {
		t.Errorf("key = %q", string(msg.Key))
	}
	// Envelope lives in headers per the AsyncAPI contract.
	for _, f := range []string{"correlationId", "tenantId", "messageId", "timestamp"} {
		if msg.Headers[f] == "" {
			t.Errorf("header %q missing", f)
		}
	}
	if msg.Headers["correlationId"] != "corr-1" || msg.Headers["tenantId"] != "t-1" {
		t.Errorf("headers = %v", msg.Headers)
	}
	// Payload matches the contract schema (no envelope duplication).
	var decoded map[string]interface{}
	json.Unmarshal(msg.Value, &decoded)
	for _, f := range []string{"metric_id", "tenant_id", "metric_name", "metric_value", "metric_type", "labels", "source_id", "recorded_at"} {
		if _, ok := decoded[f]; !ok {
			t.Errorf("payload missing %q", f)
		}
	}
}

func TestAllTopics(t *testing.T) {
	mock := &mockBroker{}
	p := NewPublisherWithBroker(mock)

	p.PublishTraceSpan(TraceSpanPayload{TenantID: "t"}, "")
	p.PublishTraceFlush(TraceFlushPayload{TenantID: "t"}, "")
	p.PublishAlertFired(AlertFiredPayload{TenantID: "t"}, "")
	p.PublishHealthStatusChange(HealthStatusChangePayload{TenantID: "t"}, "")

	want := []string{
		"operan.observability.trace.span",
		"operan.observability.trace.flush",
		"operan.observability.alert.fired",
		"operan.observability.health.status_change",
	}
	if len(mock.published) != len(want) {
		t.Fatalf("published %d, want %d", len(mock.published), len(want))
	}
	for i, topic := range want {
		if mock.published[i].Topic != topic {
			t.Errorf("topic[%d] = %q, want %q", i, mock.published[i].Topic, topic)
		}
	}
}

func TestLogBrokerAndClose(t *testing.T) {
	p := NewPublisher()
	if err := p.PublishTraceFlush(TraceFlushPayload{TenantID: "t"}, ""); err != nil {
		t.Errorf("log broker publish: %v", err)
	}
	mock := &mockBroker{}
	p2 := NewPublisherWithBroker(mock)
	p2.Close()
	if !mock.closed {
		t.Error("Close should close broker")
	}
}
