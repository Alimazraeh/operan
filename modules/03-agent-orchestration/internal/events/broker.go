package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Broker is the interface that both Kafka and AMQP implementations must satisfy.
// It handles publishing events and subscribing to topic streams.
type Broker interface {
	// Publish sends a message to the given topic with the provided payload bytes.
	Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error
	// Subscribe starts consuming messages from the given topic. OnMessage is called
	// for each received message. Returns an error if the subscription cannot be established.
	Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error
	// Close gracefully shuts down the broker.
	Close() error
}

// MessageHandler is called for each message received by a subscriber.
type MessageHandler func(ctx context.Context, msg Message)

// Message represents a single received message from a broker.
type Message struct {
	Topic    string
	Key      []byte
	Value    []byte
	Headers  map[string]string
	Received time.Time
}

// RetryConfig holds configuration for retry policy on publish failures.
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns sensible defaults for broker retry behavior.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      2 * time.Second,
		BackoffFactor: 2.0,
	}
}

// RetryableBroker wraps a Broker with exponential backoff retry logic.
type RetryableBroker struct {
	broker      Broker
	retryConfig RetryConfig
	mu          sync.RWMutex
}

// NewRetryableBroker creates a broker wrapper with retry on transient failures.
func NewRetryableBroker(broker Broker, cfg RetryConfig) *RetryableBroker {
	return &RetryableBroker{broker: broker, retryConfig: cfg}
}

// Publish retries the underlying broker's Publish with exponential backoff.
func (rb *RetryableBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	var lastErr error
	delay := rb.retryConfig.InitialDelay

	for attempt := 0; attempt < rb.retryConfig.MaxAttempts; attempt++ {
		err := rb.broker.Publish(ctx, topic, key, value, headers)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < rb.retryConfig.MaxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay = time.Duration(float64(delay) * rb.retryConfig.BackoffFactor)
			if delay > rb.retryConfig.MaxDelay {
				delay = rb.retryConfig.MaxDelay
			}
		}
	}
	return fmt.Errorf("publish to %q after %d retries: %w", topic, rb.retryConfig.MaxAttempts, lastErr)
}

// Subscribe delegates to the underlying broker's Subscribe.
func (rb *RetryableBroker) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	return rb.broker.Subscribe(ctx, topic, consumerGroup, onMessage)
}

// Close delegates to the underlying broker's Close.
func (rb *RetryableBroker) Close() error {
	return rb.broker.Close()
}

// ─── InMemoryBroker (for testing) ────────────────────────────────────────────

// InMemoryBroker implements Broker in memory for unit tests.
type InMemoryBroker struct {
	mu          sync.RWMutex
	published   []*PublishedMessage
	subscribers map[string][]*subscription
}

type subscription struct {
	onMessage MessageHandler
}

// PublishedMessage records a message sent to InMemoryBroker.
type PublishedMessage struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// NewInMemoryBroker creates a new in-memory broker for testing.
func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{
		published:   make([]*PublishedMessage, 0),
		subscribers: make(map[string][]*subscription),
	}
}

// Publish stores the message in memory.
func (ib *InMemoryBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	msg := &PublishedMessage{
		Topic:   topic,
		Key:     key,
		Value:   value,
		Headers: headers,
	}
	ib.mu.Lock()
	ib.published = append(ib.published, msg)
	ib.mu.Unlock()

	// Fan-out to subscribers
	ib.mu.RLock()
	subs := ib.subscribers[topic]
	ib.mu.RUnlock()

	for _, sub := range subs {
		msg := Message{
			Topic:    topic,
			Key:      key,
			Value:    value,
			Headers:  headers,
			Received: time.Now(),
		}
		sub.onMessage(ctx, msg)
	}
	return nil
}

// Subscribe records the handler for fan-out on Publish.
func (ib *InMemoryBroker) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	_ = consumerGroup // unused in in-memory mode
	ib.mu.Lock()
	ib.subscribers[topic] = append(ib.subscribers[topic], &subscription{onMessage: onMessage})
	ib.mu.Unlock()
	return nil
}

// Close is a no-op for InMemoryBroker.
func (ib *InMemoryBroker) Close() error {
	return nil
}

// Published returns all messages published to this broker.
func (ib *InMemoryBroker) Published() []*PublishedMessage {
	ib.mu.RLock()
	defer ib.mu.RUnlock()
	result := make([]*PublishedMessage, len(ib.published))
	copy(result, ib.published)
	return result
}

// PublishedByTopic returns messages published to a specific topic.
func (ib *InMemoryBroker) PublishedByTopic(topic string) []*PublishedMessage {
	ib.mu.RLock()
	defer ib.mu.RUnlock()
	var result []*PublishedMessage
	for _, m := range ib.published {
		if m.Topic == topic {
			result = append(result, m)
		}
	}
	return result
}

// Clear resets the broker state.
func (ib *InMemoryBroker) Clear() {
	ib.mu.Lock()
	defer ib.mu.Unlock()
	ib.published = ib.published[:0]
	for k := range ib.subscribers {
		delete(ib.subscribers, k)
	}
}

// ─── BrokerFactory ───────────────────────────────────────────────────────────

// BrokerType identifies the message broker protocol.
type BrokerType string

const (
	BrokerKafka  BrokerType = "kafka"
	BrokerAMQP   BrokerType = "amqp"
	BrokerMemory BrokerType = "memory" // for tests
)

// BrokerFactory creates Broker instances from configuration.
type BrokerFactory struct{}

// NewBrokerFactory creates a new BrokerFactory.
func NewBrokerFactory() *BrokerFactory {
	return &BrokerFactory{}
}

// CreateBroker creates a Broker based on the given type and configuration.
func (f *BrokerFactory) CreateBroker(brokerType BrokerType, cfg BrokerConfig) (Broker, error) {
	switch brokerType {
	case BrokerKafka:
		return NewKafkaBroker(cfg)
	case BrokerAMQP:
		return NewAMQPBroker(cfg)
	case BrokerMemory:
		return NewInMemoryBroker(), nil
	default:
		return nil, fmt.Errorf("unknown broker type: %q", brokerType)
	}
}

// BrokerConfig holds parameters for creating a Broker.
type BrokerConfig struct {
	// BrokerAddress is the connection string (e.g., "localhost:9092" for Kafka,
	// "amqp://localhost:5672" for AMQP).
	BrokerAddress string
	// Username and Password for SASL/PLAIN auth (Kafka) or AMQP auth.
	Username string
	Password string
	// TopicPrefix is prepended to all topic names (e.g., "operan.orchestration").
	TopicPrefix string
	// EnableTLS enables TLS for broker connections.
	EnableTLS bool
	// RetryEnabled enables retryable broker wrapping.
	RetryEnabled bool
	// RetryConfig only used when RetryEnabled is true.
	RetryConfig RetryConfig
}

// DefaultBrokerConfig returns a BrokerConfig with sensible defaults.
func DefaultBrokerConfig(brokerType BrokerType) BrokerConfig {
	switch brokerType {
	case BrokerKafka:
		return BrokerConfig{
			BrokerAddress: "localhost:9092",
			TopicPrefix:   "operan.orchestration",
		}
	case BrokerAMQP:
		return BrokerConfig{
			BrokerAddress: "amqp://localhost:5672",
			TopicPrefix:   "operan.orchestration",
		}
	default:
		return BrokerConfig{}
	}
}

// ─── UUID helper (used by Broker implementations) ────────────────────────────

// generateMessageID returns a unique message ID as a string.
func generateMessageID() string {
	return uuid.New().String()
}
