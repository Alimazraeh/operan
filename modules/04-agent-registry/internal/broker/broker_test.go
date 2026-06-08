// Package broker tests for the Agent Registry module.
package broker

import (
	"strings"
	"testing"
	"time"
)

func TestKafkaProducer_Produce(t *testing.T) {
	producer := NewKafkaProducer(DefaultConfig())
	defer producer.Close()

	err := producer.Produce("test-topic", "test-key", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKafkaProducer_Close(t *testing.T) {
	producer := NewKafkaProducer(DefaultConfig())
	err := producer.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Produce after close should not error
	err = producer.Produce("test-topic", "test-key", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("expected no error after close, got: %v", err)
	}
}

func TestKafkaProducer_ProduceAfterClose(t *testing.T) {
	producer := NewKafkaProducer(DefaultConfig())
	producer.Close()

	err := producer.Produce("test-topic", "test-key", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("expected no error after close, got: %v", err)
	}
}

func TestMockProducer_Produce(t *testing.T) {
	producer := NewMockProducer()
	defer producer.Close()

	err := producer.Produce("test-topic", "test-key", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if producer.MessageCount() != 1 {
		t.Errorf("expected 1 message, got %d", producer.MessageCount())
	}

	msg := producer.LastMessage()
	if msg == nil {
		t.Fatal("expected last message")
	}
	if msg.Topic != "test-topic" {
		t.Errorf("expected topic test-topic, got %q", msg.Topic)
	}
	if msg.Key != "test-key" {
		t.Errorf("expected key test-key, got %q", msg.Key)
	}
	if !strings.Contains(string(msg.Value), `"test":true`) {
		t.Errorf("expected payload to contain test:true, got %s", string(msg.Value))
	}
}

func TestMockProducer_MultipleMessages(t *testing.T) {
	producer := NewMockProducer()
	defer producer.Close()

	for i := 0; i < 5; i++ {
		err := producer.Produce("topic", "key", []byte(`{}`))
		if err != nil {
			t.Errorf("unexpected error on message %d: %v", i, err)
		}
	}

	if producer.MessageCount() != 5 {
		t.Errorf("expected 5 messages, got %d", producer.MessageCount())
	}
}

func TestMockProducer_SetFailNext(t *testing.T) {
	producer := NewMockProducer()
	defer producer.Close()

	producer.SetFailNext()
	err := producer.Produce("topic", "key", []byte(`{}`))
	if err == nil {
		t.Error("expected error from SetFailNext")
	}
	if err.Error() != "broker error" {
		t.Errorf("expected 'broker error', got %q", err.Error())
	}

	// Next produce should succeed
	err = producer.Produce("topic", "key", []byte(`{}`))
	if err != nil {
		t.Errorf("expected no error after fail, got: %v", err)
	}
}

func TestMockProducer_Close(t *testing.T) {
	producer := NewMockProducer()
	err := producer.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Produce after close should not error
	err = producer.Produce("topic", "key", []byte(`{}`))
	if err != nil {
		t.Errorf("expected no error after close, got: %v", err)
	}
}

func TestMockProducer_MessagesAfterClose(t *testing.T) {
	producer := NewMockProducer()

	err := producer.Produce("topic", "key", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	producer.Close()

	// Should still be able to read messages
	msgs := producer.Messages()
	if len(msgs) != 1 {
		t.Errorf("expected 1 message after close, got %d", len(msgs))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Host != "localhost" {
		t.Errorf("expected host localhost, got %q", cfg.Host)
	}
	if cfg.Port != "9092" {
		t.Errorf("expected port 9092, got %q", cfg.Port)
	}
	if cfg.Proto != "kafka" {
		t.Errorf("expected proto kafka, got %q", cfg.Proto)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", cfg.Timeout)
	}
}
