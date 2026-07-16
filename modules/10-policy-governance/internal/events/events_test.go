package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPublisher(t *testing.T) {
	p := NewPublisher("localhost:9092")
	assert.NotNil(t, p)
	assert.NotNil(t, p.writer)
}

func TestPublisher_PublishEvaluation(t *testing.T) {
	p := NewPublisher("localhost:9092")
	defer p.Close()

	event := EvaluationEvent{
		TenantID:     "tenant-1",
		RequestID:    "req-1",
		AgentID:      "agent-1",
		Resource:     "send_email",
		ActionType:   "send",
		Result:       "denied",
		PolicyName:   "deny-email",
		EvaluationMS: 100,
		DataClass:    "internal",
		Cost:         0.5,
	}

	// Since Kafka broker isn't running, this may fail silently (async, RequireNone)
	assert.NotPanics(t, func() {
		p.PublishEvaluation(context.Background(), event)
	})
}

func TestPublisher_PublishViolation(t *testing.T) {
	p := NewPublisher("localhost:9092")
	defer p.Close()

	event := ViolationEvent{
		TenantID:   "tenant-1",
		RequestID:  "req-1",
		AgentID:    "agent-1",
		Resource:   "send_email",
		ActionType: "send",
		PolicyName: "deny-email",
		Result:     "denied",
	}

	assert.NotPanics(t, func() {
		p.PublishViolation(context.Background(), event)
	})
}

func TestPublisher_PublishPolicyUpdated(t *testing.T) {
	p := NewPublisher("localhost:9092")
	defer p.Close()

	event := PolicyUpdatedEvent{
		TenantID:  "tenant-1",
		PolicyID:  "policy-1",
		Name:      "Updated Policy",
		UpdatedBy: "admin",
	}

	assert.NotPanics(t, func() {
		p.PublishPolicyUpdated(context.Background(), event)
	})
}

func TestPublisher_PublishGroupUpdated(t *testing.T) {
	p := NewPublisher("localhost:9092")
	defer p.Close()

	event := GroupUpdatedEvent{
		TenantID:  "tenant-1",
		GroupID:   "group-1",
		Name:      "Updated Group",
		UpdatedBy: "admin",
	}

	assert.NotPanics(t, func() {
		p.PublishGroupUpdated(context.Background(), event)
	})
}

func TestPublisher_PublishMalformedData(t *testing.T) {
	p := NewPublisher("localhost:9092")
	defer p.Close()

	// Even with nil context, should not panic
	assert.NotPanics(t, func() {
		p.PublishEvaluation(nil, EvaluationEvent{TenantID: "test"})
	})
}