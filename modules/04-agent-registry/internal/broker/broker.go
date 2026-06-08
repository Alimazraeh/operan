// Package broker provides Kafka and mock broker implementations for event publishing.
package broker

import (
	"log"
	"sync"
	"time"
)

// Producer is the interface for producing messages to a message broker.
// Implementations include KafkaProducer and MockProducer.
type Producer interface {
	// Produce sends a message to the specified topic with the given key and value.
	Produce(topic, key string, value []byte) error
	// Close flushes any pending messages and closes the broker connection.
	Close() error
}

// Config holds configuration for the Kafka broker.
type Config struct {
	Host     string
	Port     string
	Proto    string // "kafka", "amqp", etc.
	Timeout  time.Duration
}

// DefaultConfig returns a default broker configuration.
func DefaultConfig() Config {
	return Config{
		Host:    "localhost",
		Port:    "9092",
		Proto:   "kafka",
		Timeout: 5 * time.Second,
	}
}

// KafkaProducer is a stub implementation of the Producer interface.
// In production, this would wire to Apache Kafka or similar message broker.
// The Wave 2 integration requires wiring to a real Kafka cluster.
type KafkaProducer struct {
	config Config
	mu     sync.Mutex
	closed bool
	logger *log.Logger
}

// NewKafkaProducer creates a new KafkaProducer with the given configuration.
func NewKafkaProducer(config Config) *KafkaProducer {
	return &KafkaProducer{
		config: config,
		logger: log.New(log.Writer(), "[kafka] ", log.LstdFlags),
	}
}

// Produce sends a message to the Kafka topic.
// This is a stub implementation for Wave 2 integration.
// Production code should wire to a real Kafka client (e.g.,IBM/sarama).
func (k *KafkaProducer) Produce(topic, key string, value []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.closed {
		return nil
	}

	k.logger.Printf("produce topic=%s key=%s payload=%s", topic, key, string(value))
	return nil
}

// Close flushes pending messages and closes the Kafka connection.
func (k *KafkaProducer) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.closed = true
	k.logger.Println("kafka producer closed")
	return nil
}

// MockProducer is a test implementation that captures messages for verification.
type MockProducer struct {
	mu       sync.Mutex
	messages []Message
	closed   bool
	failNext bool
}

// Message represents a produced message.
type Message struct {
	Topic string
	Key   string
	Value []byte
}

// brokerError is an error type for broker failures.
type brokerError string

func (e brokerError) Error() string {
	return string(e)
}

// NewMockProducer creates a new MockProducer for testing.
func NewMockProducer() *MockProducer {
	return &MockProducer{
		messages: make([]Message, 0),
	}
}

// Produce captures a message for later verification.
func (m *MockProducer) Produce(topic, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	if m.failNext {
		m.failNext = false
		return brokerError("broker error")
	}

	msg := Message{Topic: topic, Key: key, Value: value}
	m.messages = append(m.messages, msg)
	return nil
}

// Close marks the mock producer as closed.
func (m *MockProducer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

// SetFailNext configures the next Produce call to fail.
func (m *MockProducer) SetFailNext() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failNext = true
}

// Messages returns all captured messages.
func (m *MockProducer) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// LastMessage returns the most recent message.
func (m *MockProducer) LastMessage() *Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return nil
	}
	last := m.messages[len(m.messages)-1]
	return &last
}

// MessageCount returns the number of captured messages.
func (m *MockProducer) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.messages)
}
