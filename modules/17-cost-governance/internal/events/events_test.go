package events

import (
	"sync"
	"testing"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker()
	if b == nil {
		t.Fatal("expected non-nil Broker")
	}
}

func TestBroker_PublishAndConsume(t *testing.T) {
	b := NewBroker()

	var received map[string]any
	var mu sync.Mutex

	b.Subscribe("test.topic", func(payload map[string]any) {
		mu.Lock()
		defer mu.Unlock()
		received = payload
	})

	b.Publish("test.topic", map[string]any{
		"tenant_id": "t1",
		"cost":      1.5,
	})

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("expected received event")
	}
	if received["tenant_id"] != "t1" {
		t.Errorf("expected tenant_id t1, got %v", received["tenant_id"])
	}
}

func TestBroker_MultipleSubscribers(t *testing.T) {
	b := NewBroker()

	count1 := 0
	count2 := 0

	b.Subscribe("multi.topic", func(payload map[string]any) {
		count1++
	})
	b.Subscribe("multi.topic", func(payload map[string]any) {
		count2++
	})

	b.Publish("multi.topic", map[string]any{"x": 1})

	if count1 != 1 {
		t.Errorf("subscriber 1: expected 1, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("subscriber 2: expected 1, got %d", count2)
	}
}

func TestBroker_PublishToUnknownTopic(t *testing.T) {
	b := NewBroker()
	// Should not panic
	b.Publish("unknown.topic", map[string]any{"x": 1})
}

func TestBroker_PublishFailable(t *testing.T) {
	b := NewBroker()
	err := b.PublishFailable("test.topic", map[string]any{"x": 1})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}