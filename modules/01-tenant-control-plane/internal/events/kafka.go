package events

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaBroker implements Broker using segmentio/kafka-go. A single writer
// handles all topics; topics are set per message.
type KafkaBroker struct {
	writer *kafka.Writer
}

// NewKafkaBroker creates a Kafka-backed broker from a broker URL of the form
// "host:port[,host:port...]" (an optional "kafka://" prefix is stripped).
func NewKafkaBroker(brokerURL string) (*KafkaBroker, error) {
	brokers := parseBrokerAddresses(brokerURL)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka broker URL is empty")
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.LeastBytes{},
		Compression:            kafka.Snappy,
		RequiredAcks:           kafka.RequireOne,
		WriteTimeout:           10 * time.Second,
		ReadTimeout:            10 * time.Second,
		AllowAutoTopicCreation: true,
	}

	return &KafkaBroker{writer: w}, nil
}

func parseBrokerAddresses(addr string) []string {
	addr = strings.TrimPrefix(strings.TrimSpace(addr), "kafka://")
	var result []string
	for _, b := range strings.Split(addr, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			result = append(result, b)
		}
	}
	return result
}

// Publish sends a message to the given Kafka topic.
func (kb *KafkaBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	kHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kHeaders = append(kHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}

	msg := kafka.Message{
		Topic:   topic,
		Key:     key,
		Value:   value,
		Headers: kHeaders,
	}

	if err := kb.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("[WARN] kafka publish to %q failed: %v", topic, err)
		return fmt.Errorf("kafka write to topic %q: %w", topic, err)
	}
	return nil
}

// Close flushes pending messages and closes the writer.
func (kb *KafkaBroker) Close() error {
	return kb.writer.Close()
}
