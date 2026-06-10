// Package broker provides Kafka and mock broker implementations for event publishing.
package broker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
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

// KafkaProducer produces messages to Apache Kafka via segmentio/kafka-go.
// Writes are asynchronous: Produce enqueues immediately and delivery failures
// are logged via the writer's completion callback, so event publishing never
// blocks or fails API request handling.
type KafkaProducer struct {
	config Config
	writer *kafka.Writer
	mu     sync.Mutex
	closed bool
	logger *log.Logger
}

// NewKafkaProducer creates a new KafkaProducer with the given configuration.
func NewKafkaProducer(config Config) *KafkaProducer {
	logger := log.New(log.Writer(), "[kafka] ", log.LstdFlags)
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = DefaultConfig().Timeout
	}
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(config.Host + ":" + config.Port),
		Balancer:               &kafka.LeastBytes{},
		Compression:            kafka.Snappy,
		RequiredAcks:           kafka.RequireOne,
		WriteTimeout:           timeout,
		AllowAutoTopicCreation: true,
		Async:                  true,
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				logger.Printf("async publish failed (%d message(s)): %v", len(messages), err)
			}
		},
	}
	return &KafkaProducer{
		config: config,
		writer: writer,
		logger: logger,
	}
}

// Produce enqueues a message for asynchronous delivery to the Kafka topic.
func (k *KafkaProducer) Produce(topic, key string, value []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.closed {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), k.writer.WriteTimeout)
	defer cancel()
	return k.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	})
}

// Close flushes pending messages and closes the Kafka connection.
func (k *KafkaProducer) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.closed {
		return nil
	}
	k.closed = true
	return k.writer.Close()
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
