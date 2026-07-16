package events

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPublisher_NoBroker(t *testing.T) {
	pub := NewPublisher("")
	assert.NotNil(t, pub)
	assert.Equal(t, "", pub.broker)
}

func TestNewPublisher_WithBroker(t *testing.T) {
	pub := NewPublisher("localhost:9092")
	assert.NotNil(t, pub)
	assert.Equal(t, "localhost:9092", pub.broker)
}

func TestPublisher_Publish_NoBroker(t *testing.T) {
	pub := NewPublisher("")
	
	// Should log but not panic
	pub.Publish("test-topic", "test-key", map[string]interface{}{"foo": "bar"})
}

func TestPublisher_Publish_EventMessageSent(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventMessageSent("tenant-1", "ch-1", "msg-1", "agent-1", "text")
}

func TestPublisher_Publish_EventHandoffCreated(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventHandoffCreated("tenant-1", "ho-1", "agent-1", "agent-2", "high", "ch-1")
}

func TestPublisher_Publish_EventHandoffAccepted(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventHandoffAccepted("tenant-1", "ho-1", "agent-2")
}

func TestPublisher_Publish_EventHandoffCompleted(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventHandoffCompleted("tenant-1", "ho-1", "agent-2", 100)
}

func TestPublisher_Publish_EventHandoffExpired(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventHandoffExpired("tenant-1", "ho-1", time.Now())
}

func TestPublisher_Publish_EventPresenceUpdated(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventPresenceUpdated("tenant-1", "agent-1", "online", time.Now())
}

func TestPublisher_Publish_EventChannelJoined(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventChannelJoined("tenant-1", "ch-1", "agent-1")
}

func TestPublisher_Publish_EventChannelLeft(t *testing.T) {
	pub := NewPublisher("")
	
	// Should not panic
	pub.EventChannelLeft("tenant-1", "ch-1", "agent-1")
}

func TestPublisher_NoOpBroker_NoPanic(t *testing.T) {
	// Test that all event types work without a broker
	pub := NewPublisher("")
	
	// Capture logs to avoid cluttering test output
	oldLogger := log.Writer()
	_, w, _ := os.Pipe()
	log.SetOutput(w)
	defer func() {
		log.SetOutput(oldLogger)
		w.Close()
	}()
	
	// Trigger all event types
	pub.EventMessageSent("t1", "c1", "m1", "a1", "text")
	pub.EventHandoffCreated("t1", "h1", "a1", "a2", "high", "c1")
	pub.EventHandoffAccepted("t1", "h1", "a2")
	pub.EventHandoffCompleted("t1", "h1", "a2", 50)
	pub.EventHandoffExpired("t1", "h1", time.Now())
	pub.EventPresenceUpdated("t1", "a1", "online", time.Now())
	pub.EventChannelJoined("t1", "c1", "a1")
	pub.EventChannelLeft("t1", "c1", "a1")
	
	// Verify no panics occurred
	assert.True(t, true)
}