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

func TestGateRaisedEnvelopeAndPayload(t *testing.T) {
	mock := &mockBroker{}
	p := NewPublisherWithBroker(mock)

	approver := "approver-1"
	err := p.PublishGateRaised(GateRaisedPayload{
		GateID:          "g-1",
		TenantID:        "t-1",
		WorkflowID:      "w-1",
		NodeID:          "n-1",
		GateType:        "human_approval",
		HumanApproverID: &approver,
		RaisedBy:        "agent-1",
		RaisedAt:        "2026-06-10T00:00:00Z",
		Priority:        "high",
	}, "corr-1")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg := mock.published[0]
	if msg.Topic != "operan.supervision.gate.raised" {
		t.Errorf("topic = %q", msg.Topic)
	}
	if string(msg.Key) != "t-1" {
		t.Errorf("key = %q", string(msg.Key))
	}
	for _, f := range []string{"correlationId", "tenantId", "messageId", "timestamp"} {
		if msg.Headers[f] == "" {
			t.Errorf("header %q missing", f)
		}
	}

	var decoded map[string]interface{}
	json.Unmarshal(msg.Value, &decoded)
	for _, f := range []string{"gate_id", "tenant_id", "workflow_id", "node_id", "gate_type", "raised_by", "raised_at"} {
		if _, ok := decoded[f]; !ok {
			t.Errorf("payload missing required field %q", f)
		}
	}
	if decoded["human_approver_id"] != "approver-1" {
		t.Errorf("human_approver_id = %v", decoded["human_approver_id"])
	}
}

func TestAllTopics(t *testing.T) {
	mock := &mockBroker{}
	p := NewPublisherWithBroker(mock)

	p.PublishGateResponded(GateRespondedPayload{TenantID: "t"}, "")
	p.PublishGateEscalated(GateEscalatedPayload{TenantID: "t"}, "")
	p.PublishGateTimeout(GateTimeoutPayload{TenantID: "t"}, "")
	p.PublishPolicyViolationDetected(PolicyViolationDetectedPayload{TenantID: "t"}, "")

	want := []string{
		"operan.supervision.gate.responded",
		"operan.supervision.gate.escalated",
		"operan.supervision.gate.timeout",
		"operan.supervision.policy.violation_detected",
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
	if err := p.PublishGateTimeout(GateTimeoutPayload{TenantID: "t"}, ""); err != nil {
		t.Errorf("log broker publish: %v", err)
	}
	mock := &mockBroker{}
	p2 := NewPublisherWithBroker(mock)
	p2.Close()
	if !mock.closed {
		t.Error("Close should close broker")
	}
}
