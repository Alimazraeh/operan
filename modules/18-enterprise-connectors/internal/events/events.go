package events

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Publisher publishes connector events to Kafka.
type Publisher struct {
	broker string
}

// NewPublisher creates a Kafka publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{
		broker: brokerURL,
	}
}

// PublishSyncStarted publishes a connector sync started event.
func (p *Publisher) PublishSyncStarted(tenantID string, connectorID interface{}, connectorType, syncType string) {
	p.publish("operan.connector.sync_started", tenantID, map[string]interface{}{
		"tenant_id":       tenantID,
		"connector_id":    connectorID,
		"connector_type":  connectorType,
		"sync_type":       syncType,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	})
}

// PublishSyncCompleted publishes a connector sync completed event.
func (p *Publisher) PublishSyncCompleted(tenantID string, connectorID interface{}, connectorType string,
	objectsFetched, objectsUpdated, objectsFailed, durationMs int,
) {
	p.publish("operan.connector.sync_completed", tenantID, map[string]interface{}{
		"tenant_id":       tenantID,
		"connector_id":    connectorID,
		"connector_type":  connectorType,
		"objects_fetched": objectsFetched,
		"objects_updated": objectsUpdated,
		"objects_failed":  objectsFailed,
		"duration_ms":     durationMs,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	})
}

// PublishSyncFailed publishes a connector sync failed event.
func (p *Publisher) PublishSyncFailed(tenantID string, connectorID interface{}, connectorType, errorMessage string) {
	p.publish("operan.connector.sync_failed", tenantID, map[string]interface{}{
		"tenant_id":      tenantID,
		"connector_id":   connectorID,
		"connector_type": connectorType,
		"error_message":  errorMessage,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	})
}

// PublishToolsRegistered publishes a tools registered event.
func (p *Publisher) PublishToolsRegistered(tenantID string, connectorID interface{}, toolCount int) {
	p.publish("operan.connector.tools_registered", tenantID, map[string]interface{}{
		"tenant_id":    tenantID,
		"connector_id": connectorID,
		"tool_count":   toolCount,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	})
}

// PublishHealthChanged publishes a health changed event.
func (p *Publisher) PublishHealthChanged(tenantID string, connectorID interface{}, healthy bool, message string) {
	p.publish("operan.connector.health_changed", tenantID, map[string]interface{}{
		"tenant_id":    tenantID,
		"connector_id": connectorID,
		"healthy":      healthy,
		"message":      message,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (p *Publisher) publish(topic, key string, value interface{}) {
	if p.broker == "" {
		log.Printf("[connector-event] %s: %v (no broker)", topic, value)
		return
	}

	conn, err := kafka.DialLeader(context.Background(), "tcp", p.broker, topic, 0)
	if err != nil {
		log.Printf("[connector-event] dial kafka %s: %v (log-only)", topic, err)
		return
	}
	defer conn.Close()

	payload, err := json.Marshal(value)
	if err != nil {
		log.Printf("[connector-event] marshal %s: %v", topic, err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: payload,
	}

	if _, err := conn.WriteMessages(msg); err != nil {
		log.Printf("[connector-event] write %s: %v (log-only)", topic, err)
	}
}