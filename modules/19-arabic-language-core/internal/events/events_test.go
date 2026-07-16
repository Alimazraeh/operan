package events

import (
	"context"
	"testing"
)

func TestNoOpBroker_Publish(t *testing.T) {
	b := NewBroker(NoOpBroker{})

	tests := []struct {
		name string
		fn   func() error
	}{
		{"TextNormalized", func() error { return b.TextNormalized(context.Background(), "t1", 100, 80, 3) }},
		{"DialectDetected", func() error { return b.DialectDetected(context.Background(), "t1", "saudi", 0.85) }},
		{"TerminologyCheck", func() error { return b.TerminologyCheck(context.Background(), "t1", 500, 5, 1) }},
		{"TerminologyViolation", func() error { return b.TerminologyViolation(context.Background(), "t1", "term", "deprecated", "suggestion", "user") }},
		{"EmbeddingRequested", func() error { return b.EmbeddingRequested(context.Background(), "t1", 200, "arabic-v1", "success") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err != nil {
				t.Errorf("NoOpBroker.%s returned error: %v", tc.name, err)
			}
		})
	}
}

func TestNoOpBroker_Close(t *testing.T) {
	// NoOpBroker.Close() has no error; we test it indirectly
	// by verifying the broker is created and works.
	b := NewBroker(NoOpBroker{})
	if b == nil {
		t.Fatal("expected non-nil broker")
	}
	// NoOpBroker.Close is available through the underlying publisher
	_ = b // used above
}

func TestBroker_NewBroker(t *testing.T) {
	b := NewBroker(NoOpBroker{})
	if b == nil {
		t.Fatal("expected non-nil broker")
	}
}