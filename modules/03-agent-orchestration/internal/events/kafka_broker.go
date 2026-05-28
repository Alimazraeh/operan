package events

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// KafkaBroker implements Broker using segmentio/kafka-go.
type KafkaBroker struct {
	mu          sync.RWMutex
	writers     map[string]*kafka.Writer
	readers     map[string]*kafka.Reader
	topicPrefix string
	brokers     []string
	dialer      *kafka.Dialer
}

// NewKafkaBroker creates a new KafkaBroker from the given configuration.
func NewKafkaBroker(cfg BrokerConfig) (*KafkaBroker, error) {
	if cfg.BrokerAddress == "" {
		cfg.BrokerAddress = DefaultBrokerConfig(BrokerKafka).BrokerAddress
	}
	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = DefaultBrokerConfig(BrokerKafka).TopicPrefix
	}

	dialer := &kafka.Dialer{
		Timeout:  10 * time.Second,
		DualStack: true,
	}

	// SASL/PLAIN authentication if credentials provided
	if cfg.Username != "" && cfg.Password != "" {
		dialer.SASLMechanism = plain.Mechanism{
			Username: cfg.Username,
			Password: cfg.Password,
		}
	}

	// Parse broker addresses from comma-separated string
	brokers := parseBrokerAddresses(cfg.BrokerAddress)

	return &KafkaBroker{
		writers:     make(map[string]*kafka.Writer),
		readers:     make(map[string]*kafka.Reader),
		topicPrefix: cfg.TopicPrefix,
		brokers:     brokers,
		dialer:      dialer,
	}, nil
}

// parseBrokerAddresses converts "host:port[,host:port...]" to []string.
func parseBrokerAddresses(addr string) []string {
	var result []string
	for _, b := range strings.Split(addr, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			result = append(result, b)
		}
	}
	return result
}

// resolveTopic ensures the topic has the configured prefix.
func (kb *KafkaBroker) resolveTopic(topic string) string {
	if kb.topicPrefix == "" || topic == "" {
		return topic
	}
	if strings.HasPrefix(topic, kb.topicPrefix+".") || topic == kb.topicPrefix {
		return topic
	}
	return kb.topicPrefix + "." + topic
}

// getWriter returns or creates a kafka.Writer for the given topic.
func (kb *KafkaBroker) getWriter(topic string) (*kafka.Writer, error) {
	fullTopic := kb.resolveTopic(topic)

	kb.mu.RLock()
	if w, ok := kb.writers[fullTopic]; ok {
		kb.mu.RUnlock()
		return w, nil
	}
	kb.mu.RUnlock()

	kb.mu.Lock()
	defer kb.mu.Unlock()

	// Double-check after acquiring write lock
	if w, ok := kb.writers[fullTopic]; ok {
		return w, nil
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(kb.brokers...),
		Topic:        fullTopic,
		Balancer:     &kafka.LeastBytes{},
		Compression:  kafka.Snappy,
		RequiredAcks: kafka.RequireOne,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	kb.writers[fullTopic] = w
	return w, nil
}

// Publish sends a message to the configured Kafka topic.
func (kb *KafkaBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	w, err := kb.getWriter(topic)
	if err != nil {
		return fmt.Errorf("kafka get writer for topic %q: %w", topic, err)
	}

	// Convert headers to kafka format
	kHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kHeaders = append(kHeaders, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	msg := kafka.Message{
		Key:     key,
		Value:   value,
		Headers: kHeaders,
	}

	err = w.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("kafka write to topic %q: %w", topic, err)
	}

	return nil
}

// getReader returns or creates a kafka.Reader for the given topic and consumer group.
func (kb *KafkaBroker) getReader(topic string, consumerGroup string) (*kafka.Reader, error) {
	fullTopic := kb.resolveTopic(topic)
	cacheKey := fullTopic + "|" + consumerGroup

	kb.mu.RLock()
	if r, ok := kb.readers[cacheKey]; ok {
		kb.mu.RUnlock()
		return r, nil
	}
	kb.mu.RUnlock()

	kb.mu.Lock()
	defer kb.mu.Unlock()

	if r, ok := kb.readers[cacheKey]; ok {
		return r, nil
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:         kb.brokers,
		Topic:           fullTopic,
		GroupID:         consumerGroup,
		Dialer:          kb.dialer,
		MinBytes:        10,
		MaxBytes:        10e6,
		MaxWait:         250 * time.Millisecond,
		QueueCapacity:   100,
	})

	kb.readers[cacheKey] = r
	return r, nil
}

// Subscribe starts consuming messages from a Kafka topic.
func (kb *KafkaBroker) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	r, err := kb.getReader(topic, consumerGroup)
	if err != nil {
		return fmt.Errorf("kafka get reader for topic %q: %w", topic, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				r.Close()
				return
			default:
			}

			m, err := r.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[KAFKA] read error on topic %q: %v", topic, err)
				continue
			}

			headers := make(map[string]string, len(m.Headers))
			for _, h := range m.Headers {
				headers[h.Key] = string(h.Value)
			}

			msg := Message{
				Topic:    kb.resolveTopic(topic),
				Key:      m.Key,
				Value:    m.Value,
				Headers:  headers,
				Received: time.Now(),
			}

			onMessage(ctx, msg)
		}
	}()

	return nil
}

// Close releases all resources held by the broker.
func (kb *KafkaBroker) Close() error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	for key, r := range kb.readers {
		if err := r.Close(); err != nil {
			log.Printf("[KAFKA] error closing reader %q: %v", key, err)
		}
	}

	for key, w := range kb.writers {
		if err := w.Close(); err != nil {
			log.Printf("[KAFKA] error closing writer %q: %v", key, err)
		}
	}

	kb.writers = make(map[string]*kafka.Writer)
	kb.readers = make(map[string]*kafka.Reader)
	return nil
}

// ─── AMQP Broker (RabbitMQ) ─────────────────────────────────────────────────

// AMQPBroker implements Broker using streadway/amqp (RabbitMQ).
// This is a skeleton implementation — actual RabbitMQ wiring requires
// amqp.Dial and channel management.
type AMQPBroker struct {
	topicPrefix string
	exchange    string
}

// NewAMQPBroker creates a new AMQPBroker from the given configuration.
func NewAMQPBroker(cfg BrokerConfig) (*AMQPBroker, error) {
	if cfg.BrokerAddress == "" {
		cfg.BrokerAddress = DefaultBrokerConfig(BrokerAMQP).BrokerAddress
	}
	if cfg.TopicPrefix == "" {
		cfg.TopicPrefix = DefaultBrokerConfig(BrokerAMQP).TopicPrefix
	}

	log.Printf("[AMQP] broker initialized (skeleton) for %q with topic prefix %q",
		cfg.BrokerAddress, cfg.TopicPrefix)

	return &AMQPBroker{
		topicPrefix: cfg.TopicPrefix,
		exchange:    "operan-orchestration",
	}, nil
}

// Publish sends a message to the configured AMQP exchange.
func (ab *AMQPBroker) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	// TODO: Implement actual AMQP publishing using streadway/amqp
	// 1. amqp.Dial(cfg.BrokerAddress)
	// 2. ch, err := conn.Channel()
	// 3. ch.ExchangeDeclare(exchange, "topic", true, false, ...)
	// 4. ch.Publish(exchange, routingKey, mandatory, immediate, publishing)
	_ = topic
	_ = key
	_ = value
	_ = headers
	_ = ctx
	return nil
}

// Subscribe starts consuming messages from the AMQP exchange.
func (ab *AMQPBroker) Subscribe(ctx context.Context, topic string, consumerGroup string, onMessage MessageHandler) error {
	// TODO: Implement actual AMQP subscription
	_ = topic
	_ = consumerGroup
	_ = onMessage
	return nil
}

// Close releases the AMQP connection.
func (ab *AMQPBroker) Close() error {
	return nil
}
