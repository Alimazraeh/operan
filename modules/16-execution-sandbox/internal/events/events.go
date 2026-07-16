package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
)

// Publisher publishes sandbox execution events to Kafka.
type Publisher struct {
	broker   string
	conn     *kafka.Conn
	topics   bool
}

// NewPublisher creates a Kafka publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{
		broker: brokerURL,
		topics: brokerURL != "",
	}
}

// Publish sends an event to Kafka. If the broker is unavailable, it logs a warning.
func (p *Publisher) Publish(topic, key string, value interface{}) {
	if !p.topics {
		log.Printf("[sandbox-event] %s: %v (no broker)", topic, value)
		return
	}

	conn, err := kafka.DialLeader(context.Background(), "tcp", p.broker, topic, 0)
	if err != nil {
		log.Printf("[sandbox-event] dial kafka %s: %v (log-only)", topic, err)
		return
	}
	defer conn.Close()

	payload, err := json.Marshal(value)
	if err != nil {
		log.Printf("[sandbox-event] marshal %s: %v", topic, err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: payload,
	}

	if _, err := conn.WriteMessages(msg); err != nil {
		log.Printf("[sandbox-event] write %s: %v (log-only)", topic, err)
	}
}