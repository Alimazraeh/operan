package events

import (
	"context"
	"log"
	"os"
	"testing"
)

func TestNoOpBroker_Publish(t *testing.T) {
	b := NoOpBroker{}
	err := b.Publish(context.Background(), "topic", "key", "value")
	if err != nil {
		t.Fatalf("NoOpBroker should not return errors: %v", err)
	}
}

func TestNoOpBroker_Close(t *testing.T) {
	b := NoOpBroker{}
	if err := b.Close(); err != nil {
		t.Fatalf("NoOpBroker.Close should not return errors: %v", err)
	}
}

func TestNewBroker(t *testing.T) {
	b := NewBroker(NoOpBroker{})
	if b == nil {
		t.Fatal("NewBroker should not return nil")
	}
}

func TestBroker_ModelCallPublished(t *testing.T) {
	b := NewBroker(NoOpBroker{})
	err := b.ModelCallPublished(
		context.Background(),
		"tenant-001", "agent-1", "wf-2",
		"gpt-4", "openai",
		100, 50, 500, 0.025, "success",
	)
	if err != nil {
		t.Fatalf("ModelCallPublished should not return errors: %v", err)
	}
}

func TestBroker_ModelFailoverPublished(t *testing.T) {
	b := NewBroker(NoOpBroker{})
	err := b.ModelFailoverPublished(
		context.Background(),
		"tenant-001", "gpt-4", "openai", "anthropic", "primary failed",
	)
	if err != nil {
		t.Fatalf("ModelFailoverPublished should not return errors: %v", err)
	}
}

func TestBroker_ModelCostRecorded(t *testing.T) {
	b := NewBroker(NoOpBroker{})
	err := b.ModelCostRecorded(
		context.Background(),
		"tenant-001", "agent-1", "gpt-4", 0.025, "llm-inference",
	)
	if err != nil {
		t.Fatalf("ModelCostRecorded should not return errors: %v", err)
	}
}

func TestKafkaPublisher_Publish(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	p := NewKafkaPublisher(logger)
	err := p.Publish(context.Background(), "topic", "key", "value")
	if err != nil {
		t.Fatalf("KafkaPublisher should not return errors: %v", err)
	}
}

func TestKafkaPublisher_Close(t *testing.T) {
	p := NewKafkaPublisher(nil)
	if err := p.Close(); err != nil {
		t.Fatalf("KafkaPublisher.Close should not return errors: %v", err)
	}
}

func TestNewKafkaPublisherFromURL_Empty(t *testing.T) {
	p, err := NewKafkaPublisherFromURL("", nil)
	if err != nil {
		t.Fatalf("NewKafkaPublisherFromURL with empty URL should not error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
}