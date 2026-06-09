package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ─── InMemoryBroker tests ───────────────────────────────────────────────────

func TestInMemoryBroker_Publish(t *testing.T) {
	broker := NewInMemoryBroker()
	defer broker.Close()

	payload := []byte(`{"workflow_id":"wf-1","status":"started"}`)
	err := broker.Publish(context.Background(), "operan.orchestration.workflow.started", nil, payload, nil)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	published := broker.Published()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}

	msg := published[0]
	if msg.Topic != "operan.orchestration.workflow.started" {
		t.Errorf("expected topic %q, got %q", "operan.orchestration.workflow.started", msg.Topic)
	}
	if string(msg.Value) != string(payload) {
		t.Errorf("expected value %q, got %q", payload, msg.Value)
	}
}

func TestInMemoryBroker_PublishWithHeaders(t *testing.T) {
	broker := NewInMemoryBroker()
	defer broker.Close()

	headers := map[string]string{
		"X-Correlation-ID": "corr-123",
		"Content-Type":     "application/json",
	}
	payload := []byte(`{"test": true}`)
	err := broker.Publish(context.Background(), "test.topic", nil, payload, headers)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	published := broker.Published()
	if len(published) != 1 {
		t.Fatalf("expected 1 message, got %d", len(published))
	}

	msg := published[0]
	if msg.Headers["X-Correlation-ID"] != "corr-123" {
		t.Errorf("expected header X-Correlation-ID=corr-123, got %q", msg.Headers["X-Correlation-ID"])
	}
	if msg.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected header Content-Type=application/json, got %q", msg.Headers["Content-Type"])
	}
}

func TestInMemoryBroker_PublishedByTopic(t *testing.T) {
	broker := NewInMemoryBroker()
	defer broker.Close()

	broker.Publish(context.Background(), "topic-a", nil, []byte("msg1"), nil)
	broker.Publish(context.Background(), "topic-b", nil, []byte("msg2"), nil)
	broker.Publish(context.Background(), "topic-a", nil, []byte("msg3"), nil)

	byTopicA := broker.PublishedByTopic("topic-a")
	if len(byTopicA) != 2 {
		t.Fatalf("expected 2 messages for topic-a, got %d", len(byTopicA))
	}

	byTopicB := broker.PublishedByTopic("topic-b")
	if len(byTopicB) != 1 {
		t.Fatalf("expected 1 message for topic-b, got %d", len(byTopicB))
	}
}

func TestInMemoryBroker_Subscribe(t *testing.T) {
	broker := NewInMemoryBroker()
	defer broker.Close()

	var receivedMessages []Message
	var mu sync.Mutex
	done := make(chan struct{}, 1) // buffered to avoid blocking

	err := broker.Subscribe(context.Background(), "test.topic", "group-1", func(ctx context.Context, msg Message) {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	// Publish a message — the subscriber callback should be invoked synchronously
	payload := []byte(`{"event": "test"}`)
	err = broker.Publish(context.Background(), "test.topic", nil, payload, nil)
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	// InMemoryBroker delivers synchronously, so we should have the message already
	mu.Lock()
	if len(receivedMessages) != 1 {
		t.Fatalf("expected 1 received message, got %d", len(receivedMessages))
	}
	if string(receivedMessages[0].Value) != string(payload) {
		t.Errorf("expected value %q, got %q", payload, receivedMessages[0].Value)
	}
	mu.Unlock()
}

func TestInMemoryBroker_Clear(t *testing.T) {
	broker := NewInMemoryBroker()
	defer broker.Close()

	broker.Publish(context.Background(), "topic-1", nil, []byte("msg1"), nil)
	broker.Publish(context.Background(), "topic-2", nil, []byte("msg2"), nil)

	if len(broker.Published()) != 2 {
		t.Fatalf("expected 2 messages before clear, got %d", len(broker.Published()))
	}

	broker.Clear()
	if len(broker.Published()) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(broker.Published()))
	}
}

// ─── RetryableBroker tests ──────────────────────────────────────────────────

func TestRetryableBroker_PublishSucceeds(t *testing.T) {
	broker := NewInMemoryBroker()
	retryBroker := NewRetryableBroker(broker, DefaultRetryConfig())
	defer broker.Close()

	err := retryBroker.Publish(context.Background(), "test.topic", nil, []byte("test"), nil)
	if err != nil {
		t.Fatalf("RetryableBroker.Publish returned error: %v", err)
	}

	published := broker.Published()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}
}

func TestRetryableBroker_PublishWithRetries(t *testing.T) {
	// Create a failing in-memory broker that fails twice then succeeds
	actualFailures := 0
	var mu sync.Mutex
	failingBroker := &failingBroker2{
		failUntil: 2,
		inner:     NewInMemoryBroker(),
		onFail: func() {
			mu.Lock()
			actualFailures++
			mu.Unlock()
		},
	}

	retryBroker := NewRetryableBroker(failingBroker, RetryConfig{
		MaxAttempts:   4,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})
	defer failingBroker.inner.Close()

	err := retryBroker.Publish(context.Background(), "test.topic", nil, []byte("test"), nil)
	if err != nil {
		t.Fatalf("RetryableBroker.Publish returned error after retries: %v", err)
	}

	mu.Lock()
	if actualFailures != 2 {
		t.Errorf("expected 2 actual failures before success, got %d", actualFailures)
	}
	mu.Unlock()
}

func TestRetryableBroker_PublishExhaustsRetries(t *testing.T) {
	inner := NewInMemoryBroker()
	defer inner.Close()

	// Broker that always fails
	alwaysFail := &alwaysFailBroker{inner: inner}

	retryBroker := NewRetryableBroker(alwaysFail, RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      5 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	err := retryBroker.Publish(context.Background(), "test.topic", nil, []byte("test"), nil)
	if err == nil {
		t.Fatal("expected error when all retries exhausted, got nil")
	}

	if len(inner.Published()) != 0 {
		t.Error("expected inner broker to have 0 messages after exhausted retries")
	}

	expectedErrPrefix := `publish to "test.topic" after 3 retries:`
	if !contains(err.Error(), expectedErrPrefix) {
		t.Errorf("expected error to contain %q, got: %v", expectedErrPrefix, err)
	}
}

func TestRetryableBroker_PublishContextCanceled(t *testing.T) {
	inner := NewInMemoryBroker()
	defer inner.Close()

	// Broker that always fails
	alwaysFail := &alwaysFailBroker{inner: inner}

	retryBroker := NewRetryableBroker(alwaysFail, RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond, // long delay so context cancels first
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before Publish

	err := retryBroker.Publish(ctx, "test.topic", nil, []byte("test"), nil)
	if err == nil {
		t.Fatal("expected context.Canceled error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type failingBroker2 struct {
	failUntil    int
	currentError int
	inner        *InMemoryBroker
	onFail       func()
}

func (f *failingBroker2) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	if f.currentError < f.failUntil {
		f.currentError++
		f.onFail()
		return fmt.Errorf("transient error (error #%d of %d)", f.currentError, f.failUntil)
	}
	return f.inner.Publish(ctx, topic, key, value, headers)
}

func (f *failingBroker2) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	return f.inner.Subscribe(ctx, topic, consumerGroup, onMessage)
}

func (f *failingBroker2) Close() error {
	return f.inner.Close()
}

type alwaysFailBroker struct {
	inner *InMemoryBroker
}

func (a *alwaysFailBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	return fmt.Errorf("always fails: connection refused")
}

func (a *alwaysFailBroker) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	return a.inner.Subscribe(ctx, topic, consumerGroup, onMessage)
}

func (a *alwaysFailBroker) Close() error {
	return a.inner.Close()
}

// ─── BrokerFactory tests ────────────────────────────────────────────────────

func TestBrokerFactory_CreateMemoryBroker(t *testing.T) {
	factory := NewBrokerFactory()
	broker, err := factory.CreateBroker(BrokerMemory, BrokerConfig{})
	if err != nil {
		t.Fatalf("CreateBroker(Memory) returned error: %v", err)
	}
	defer broker.Close()

	_, ok := broker.(*InMemoryBroker)
	if !ok {
		t.Error("expected *InMemoryBroker, got different type")
	}
}

func TestBrokerFactory_CreateUnknownBroker(t *testing.T) {
	factory := NewBrokerFactory()
	_, err := factory.CreateBroker(BrokerType("unknown"), BrokerConfig{})
	if err == nil {
		t.Fatal("expected error for unknown broker type, got nil")
	}
}

func TestBrokerFactory_CreateKafkaBroker(t *testing.T) {
	factory := NewBrokerFactory()
	// NewKafkaBroker does not actually connect to Kafka during construction —
	// it only stores config. Publish() is where connection happens.
	// So this should succeed with valid-looking config.
	broker, err := factory.CreateBroker(BrokerKafka, BrokerConfig{
		BrokerAddress: "localhost:9092",
		TopicPrefix:   "test-prefix",
	})
	if err != nil {
		t.Fatalf("CreateBroker(Kafka) returned error: %v", err)
	}
	defer broker.Close()

	kb, ok := broker.(*KafkaBroker)
	if !ok {
		t.Error("expected *KafkaBroker, got different type")
	} else {
		if kb.topicPrefix != "test-prefix" {
			t.Errorf("expected topic prefix test-prefix, got %q", kb.topicPrefix)
		}
	}
}

// ─── Publisher with InMemoryBroker tests ────────────────────────────────────

func TestPublisher_WithInMemoryBroker(t *testing.T) {
	broker := NewInMemoryBroker()
	pub := NewPublisherWithBroker(broker)
	defer broker.Close()

	err := pub.PublishWorkflowStarted(StackLangGraph, WorkflowStartedPayload{
		WorkflowID:  "wf-1",
		StartedBy:   "user-1",
		StartedAt:   time.Now(),
		InitialNodes: []string{"node-1", "node-2"},
	})
	if err != nil {
		t.Fatalf("PublishWorkflowStarted returned error: %v", err)
	}

	published := broker.Published()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}

	// Verify topic name
	expectedTopic := "operan.orchestration.langgraph.workflow.started"
	if published[0].Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, published[0].Topic)
	}

	// Verify payload structure
	var payload map[string]interface{}
	if err := json.Unmarshal(published[0].Value, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload["workflow_id"] != "wf-1" {
		t.Errorf("expected workflow_id wf-1, got %v", payload["workflow_id"])
	}
}

func TestPublisher_StackHealth(t *testing.T) {
	broker := NewInMemoryBroker()
	pub := NewPublisherWithBroker(broker)
	defer broker.Close()

	err := pub.PublishStackHealth(StackHealthPayload{
		StackType: "langgraph",
		StackName: "langgraph-prod",
		TenantID:  "tenant-1",
		Status:    "healthy",
		UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("PublishStackHealth returned error: %v", err)
	}

	published := broker.Published()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}

	expectedTopic := "operan.orchestration.stack.health"
	if published[0].Topic != expectedTopic {
		t.Errorf("expected topic %q, got %q", expectedTopic, published[0].Topic)
	}
}

func TestPublisher_SetBroker(t *testing.T) {
	broker1 := NewInMemoryBroker()
	pub := NewPublisherWithBroker(broker1)

	broker2 := NewInMemoryBroker()
	pub.SetBroker(broker2)

	// Publish should use broker2
	err := pub.PublishWorkflowStarted(StackTemporal, WorkflowStartedPayload{
		WorkflowID: "wf-2",
	})
	if err != nil {
		t.Fatalf("PublishWorkflowStarted returned error: %v", err)
	}

	// broker1 should have no messages
	if len(broker1.Published()) != 0 {
		t.Error("expected broker1 to have 0 messages")
	}

	// broker2 should have 1 message
	if len(broker2.Published()) != 1 {
		t.Errorf("expected broker2 to have 1 message, got %d", len(broker2.Published()))
	}

	broker1.Close()
	broker2.Close()
}

func TestPublisher_Close(t *testing.T) {
	broker := NewInMemoryBroker()
	pub := NewPublisherWithBroker(broker)

	// Close should succeed even if broker is already closed
	err := pub.Close()
	if err != nil {
		t.Fatalf("Publisher.Close returned error: %v", err)
	}
}

// ─── LogBroker tests ────────────────────────────────────────────────────────

func TestLogBroker_Publish(t *testing.T) {
	broker := &logBroker{}
	err := broker.Publish(context.Background(), "test.topic", nil, []byte(`{"test":true}`), nil)
	if err != nil {
		t.Fatalf("logBroker.Publish returned error: %v", err)
	}
}

func TestLogBroker_Subscribe(t *testing.T) {
	broker := &logBroker{}
	err := broker.Subscribe(context.Background(), "test.topic", "group-1", func(ctx context.Context, msg Message) {})
	if err != nil {
		t.Fatalf("logBroker.Subscribe returned error: %v", err)
	}
}

// ─── DefaultBrokerConfig tests ─────────────────────────────────────────────

func TestDefaultBrokerConfig_Kafka(t *testing.T) {
	cfg := DefaultBrokerConfig(BrokerKafka)
	if cfg.BrokerAddress != "localhost:9092" {
		t.Errorf("expected default Kafka address localhost:9092, got %q", cfg.BrokerAddress)
	}
	if cfg.TopicPrefix != "operan.orchestration" {
		t.Errorf("expected default topic prefix operan.orchestration, got %q", cfg.TopicPrefix)
	}
}

func TestDefaultBrokerConfig_AMQP(t *testing.T) {
	cfg := DefaultBrokerConfig(BrokerAMQP)
	if cfg.BrokerAddress != "amqp://localhost:5672" {
		t.Errorf("expected default AMQP address amqp://localhost:5672, got %q", cfg.BrokerAddress)
	}
}

// ─── Publisher error path tests ─────────────────────────────────────────────

func TestPublisher_PublishNilBroker(t *testing.T) {
	// NewPublisher uses logBroker, which is not nil. Create a publisher with nil broker.
	pubNil := &Publisher{broker: nil}

	err := pubNil.PublishWorkflowStarted(StackLangGraph, WorkflowStartedPayload{
		WorkflowID: "wf-1",
	})
	if err == nil {
		t.Fatal("expected error when broker is nil, got nil")
	}
	if !contains(err.Error(), "broker not set") {
		t.Errorf("expected error to contain %q, got: %v", "broker not set", err)
	}
}

func TestPublisher_MarshalAndPublish_MarshalError(t *testing.T) {
	broker := NewInMemoryBroker()
	pub := NewPublisherWithBroker(broker)
	defer broker.Close()

	// Test with valid payload to ensure marshalAndPublish flow works correctly
	err := pub.PublishWorkflowCreated(StackLangGraph, WorkflowCreatedPayload{
		WorkflowID: "wf-1",
		TenantID:   "tenant-1",
		Name:       "test",
		Version:    "v1",
		CreatedAt:  time.Now(),
		CreatedBy:  "user-1",
	})
	if err != nil {
		t.Fatalf("expected success for valid payload, got: %v", err)
	}
}

func TestPublisher_CloseNil(t *testing.T) {
	pub := &Publisher{}
	err := pub.Close()
	if err != nil {
		t.Errorf("expected nil error when closing publisher with nil broker, got: %v", err)
	}
}

func TestNewPublisher_DefaultBroker(t *testing.T) {
	pub := NewPublisher()

	// NewPublisher uses logBroker which logs but doesn't error
	err := pub.PublishWorkflowStarted(StackLangGraph, WorkflowStartedPayload{
		WorkflowID: "wf-1",
	})
	if err != nil {
		t.Fatalf("expected success with default log broker, got: %v", err)
	}
}
