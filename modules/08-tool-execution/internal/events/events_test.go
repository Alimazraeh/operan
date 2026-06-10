package events

import (
	"context"
	"sync"
	"testing"
)

// capturingBroker records published topics for assertions.
type capturingBroker struct {
	mu     sync.Mutex
	topics []string
}

func (b *capturingBroker) Publish(_ context.Context, topic string, _, _ []byte, _ map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.topics = append(b.topics, topic)
	return nil
}
func (b *capturingBroker) Close() error { return nil }

func TestPublisher_AllEvents(t *testing.T) {
	b := &capturingBroker{}
	p := NewPublisherWithBroker(b)

	if err := p.PublishToolRegistered(ToolRegisteredPayload{ToolID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := p.PublishToolVersionChanged(ToolVersionChangedPayload{ToolID: "t1"}); err != nil {
		t.Fatal(err)
	}
	for _, fn := range []func(ExecutionPayload) error{
		p.PublishExecutionRequested, p.PublishExecutionStarted,
		p.PublishExecutionCompleted, p.PublishExecutionFailed,
	} {
		if err := fn(ExecutionPayload{ExecutionID: "e1", Tool: "calc"}); err != nil {
			t.Fatal(err)
		}
	}

	want := []string{
		"operan.tools.tool_registered",
		"operan.tools.tool_version_changed",
		"operan.tools.execution.requested",
		"operan.tools.execution.started",
		"operan.tools.execution.completed",
		"operan.tools.execution.failed",
	}
	if len(b.topics) != len(want) {
		t.Fatalf("published %d topics, want %d: %v", len(b.topics), len(want), b.topics)
	}
	for i, w := range want {
		if b.topics[i] != w {
			t.Errorf("topic[%d] = %q, want %q", i, b.topics[i], w)
		}
	}

	if err := p.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestNewPublisher_LogBroker(t *testing.T) {
	p := NewPublisher()
	if err := p.PublishToolRegistered(ToolRegisteredPayload{ToolID: "x"}); err != nil {
		t.Errorf("log-broker publish should not error, got %v", err)
	}
	p.SetBroker(&capturingBroker{})
	_ = p.Close()
}
