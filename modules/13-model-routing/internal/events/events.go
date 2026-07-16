package events

import (
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

// Broker abstracts Kafka event publishing.
type Broker interface {
	Publish(topic string, value []byte) error
	Close() error
}

// NoOpBroker is a do-nothing broker for testing.
type NoOpBroker struct{}

func (NoOpBroker) Publish(_ string, _ []byte) error  { return nil }
func (NoOpBroker) Close() error                      { return nil }

// KafkaBroker publishes events to a Kafka topic.
type KafkaBroker struct {
	writer *kafka.Writer
}

// NewKafkaBroker creates a Kafka broker connected to the given broker URLs.
func NewKafkaBroker(brokerURLs []string) (*KafkaBroker, error) {
	if len(brokerURLs) == 0 {
		return nil, fmt.Errorf("kafka: no broker URLs provided")
	}
	kw := &kafka.Writer{
		Addr:         kafka.TCP(brokerURLs...),
		Topic:        "operan.model-routing",
		Async:        true,
		BatchSize:    128,
	}
	return &KafkaBroker{writer: kw}, nil
}

// Publish sends an event to the specified Kafka topic.
func (k *KafkaBroker) Publish(topic string, value []byte) error {
	msg := kafka.Message{
		Key:   []byte(topic),
		Value: value,
	}
	if err := k.writer.WriteMessages(nil, msg); err != nil {
		return fmt.Errorf("kafka publish: %w", err)
	}
	return nil
}

// Close flushes and closes the Kafka writer.
func (k *KafkaBroker) Close() error {
	return k.writer.Close()
}

// Event types published to Kafka.
const (
	EventRouteResolved       = "operan.model.route.resolved"
	EventFallbackTriggered   = "operan.model.route.fallback_triggered"
	EventPerformanceRecorded = "operan.model.route.performance_recorded"
)

// LogBroker logs events to stdout instead of publishing. Used in tests.
type LogBroker struct {
	Events []string
}

func (l *LogBroker) Publish(_ string, value []byte) error {
	l.Events = append(l.Events, string(value))
	return nil
}
func (l *LogBroker) Close() error {
	log.Println("LogBroker closed")
	return nil
}