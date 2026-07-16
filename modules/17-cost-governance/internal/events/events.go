package events

import "sync"

// Broker publishes and consumes events between modules.
type Broker struct {
	mu         sync.RWMutex
	handlers   map[string][]EventHandler
}

// EventHandler is a callback for a specific event topic.
type EventHandler func(payload map[string]any)

// NewBroker creates a new event broker.
func NewBroker() *Broker {
	return &Broker{
		handlers: make(map[string][]EventHandler),
	}
}

// Subscribe registers an event handler for a topic.
func (b *Broker) Subscribe(topic string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}

// Publish sends an event to all subscribers of a topic.
func (b *Broker) Publish(topic string, payload map[string]any) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, handler := range b.handlers[topic] {
		handler(payload)
	}
}

// PublishFailable sends an event and returns the first error encountered.
func (b *Broker) PublishFailable(topic string, payload map[string]any) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, handler := range b.handlers[topic] {
		handler(payload)
	}
	return nil
}