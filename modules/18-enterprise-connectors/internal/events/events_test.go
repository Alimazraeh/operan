package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublisher_PublishSyncStarted_NoBroker(t *testing.T) {
	pub := NewPublisher("") // no broker = log-only mode

	// The publish method falls back to log.Printf when broker is empty.
	// We can't easily capture log output, but we verify no panic.
	pub.PublishSyncStarted("tenant-1", "conn-1", "smtp", "full")
}

func TestPublisher_PublishSyncCompleted_NoBroker(t *testing.T) {
	pub := NewPublisher("")

	// Should not panic
	pub.PublishSyncCompleted("tenant-1", "conn-1", "smtp", 100, 50, 0, 5000)
}

func TestPublisher_PublishSyncFailed_NoBroker(t *testing.T) {
	pub := NewPublisher("")

	// Should not panic
	pub.PublishSyncFailed("tenant-1", "conn-1", "smtp", "connection timeout")
}

func TestPublisher_PublishToolsRegistered_NoBroker(t *testing.T) {
	pub := NewPublisher("")

	// Should not panic
	pub.PublishToolsRegistered("tenant-1", "conn-1", 5)
}

func TestPublisher_PublishHealthChanged_NoBroker(t *testing.T) {
	pub := NewPublisher("")

	// Should not panic
	pub.PublishHealthChanged("tenant-1", "conn-1", false, "connection lost")
}

func TestPublisher_NewPublisher(t *testing.T) {
	pub := NewPublisher("kafka://localhost:9092")
	require.NotNil(t, pub)

	// Empty broker should also work (log-only mode)
	pub2 := NewPublisher("")
	require.NotNil(t, pub2)
}

func TestPublisher_PublishSyncStarted_WithBrokerButUnavailable(t *testing.T) {
	// Use a non-routable broker address; the publisher should fall back to log
	pub := NewPublisher("kafka://127.0.0.1:9")

	// Should not panic; dial should fail and fall back to log
	pub.PublishSyncStarted("tenant-1", "conn-1", "smtp", "full")
}

func TestPublisher_PublishSyncCompleted_WithBrokerButUnavailable(t *testing.T) {
	pub := NewPublisher("kafka://127.0.0.1:9")
	pub.PublishSyncCompleted("tenant-1", "conn-1", "smtp", 100, 50, 0, 5000)
}