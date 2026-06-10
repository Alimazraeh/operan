package consumer

import (
	"context"
	"sync"
)

// mockBroker captures published topics for assertions.
type mockBroker struct {
	mu     sync.Mutex
	topics []string
}

func (m *mockBroker) Publish(_ context.Context, topic string, _, _ []byte, _ map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.topics = append(m.topics, topic)
	return nil
}

func (m *mockBroker) Close() error { return nil }
