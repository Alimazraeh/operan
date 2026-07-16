package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoOpBroker_Publish(t *testing.T) {
	b := NoOpBroker{}
	err := b.Publish("topic", []byte("value"))
	assert.NoError(t, err)
}

func TestNoOpBroker_Close(t *testing.T) {
	b := NoOpBroker{}
	err := b.Close()
	assert.NoError(t, err)
}

func TestLogBroker_Publish(t *testing.T) {
	b := &LogBroker{}
	err := b.Publish("test.topic", []byte(`{"key":"val"}`))
	assert.NoError(t, err)
	assert.Len(t, b.Events, 1)
	assert.Equal(t, `{"key":"val"}`, b.Events[0])
}

func TestLogBroker_MultiplePublish(t *testing.T) {
	b := &LogBroker{}
	b.Publish("t1", []byte("v1"))
	b.Publish("t2", []byte("v2"))
	assert.Len(t, b.Events, 2)
	assert.Equal(t, "v1", b.Events[0])
	assert.Equal(t, "v2", b.Events[1])
}

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "operan.model.route.resolved", EventRouteResolved)
	assert.Equal(t, "operan.model.route.fallback_triggered", EventFallbackTriggered)
	assert.Equal(t, "operan.model.route.performance_recorded", EventPerformanceRecorded)
}