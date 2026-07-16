package events

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Publisher publishes collaboration events to Kafka.
type Publisher struct {
	broker string
}

// NewPublisher creates a Kafka publisher.
func NewPublisher(brokerURL string) *Publisher {
	return &Publisher{broker: brokerURL}
}

// Publish sends an event to Kafka. If the broker is unavailable, it logs a warning.
func (p *Publisher) Publish(topic, key string, value interface{}) {
	if p.broker == "" {
		log.Printf("[collab-event] %s: %v (no broker)", topic, value)
		return
	}

	conn, err := kafka.DialLeader(context.Background(), "tcp", p.broker, topic, 0)
	if err != nil {
		log.Printf("[collab-event] dial kafka %s: %v (log-only)", topic, err)
		return
	}
	defer conn.Close()

	payload, err := json.Marshal(value)
	if err != nil {
		log.Printf("[collab-event] marshal %s: %v", topic, err)
		return
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: payload,
	}

	if _, err := conn.WriteMessages(msg); err != nil {
		log.Printf("[collab-event] write %s: %v (log-only)", topic, err)
	}
}

// EventMessageSent is published when a message is posted.
func (p *Publisher) EventMessageSent(tenantID, channelID, messageID, senderID, messageType string) {
	p.Publish("operan.collaboration.message_sent", tenantID, map[string]interface{}{
		"tenant_id":    tenantID,
		"channel_id":   channelID,
		"message_id":   messageID,
		"sender_id":    senderID,
		"message_type": messageType,
		"timestamp":    time.Now().Unix(),
	})
}

// EventHandoffCreated is published when a handoff is created.
func (p *Publisher) EventHandoffCreated(tenantID, handoffID, fromAgent, toAgent, priority, channelID string) {
	p.Publish("operan.collaboration.handoff_created", tenantID, map[string]interface{}{
		"tenant_id":  tenantID,
		"handoff_id": handoffID,
		"from_agent": fromAgent,
		"to_agent":   toAgent,
		"priority":   priority,
		"channel_id": channelID,
	})
}

// EventHandoffAccepted is published when a handoff is accepted.
func (p *Publisher) EventHandoffAccepted(tenantID, handoffID, acceptedBy string) {
	p.Publish("operan.collaboration.handoff_accepted", tenantID, map[string]interface{}{
		"tenant_id":    tenantID,
		"handoff_id":   handoffID,
		"accepted_by":  acceptedBy,
		"timestamp":    time.Now().Unix(),
	})
}

// EventHandoffCompleted is published when a handoff is completed.
func (p *Publisher) EventHandoffCompleted(tenantID, handoffID, completedBy string, responseLength int) {
	p.Publish("operan.collaboration.handoff_completed", tenantID, map[string]interface{}{
		"tenant_id":      tenantID,
		"handoff_id":     handoffID,
		"completed_by":   completedBy,
		"response_length": responseLength,
		"timestamp":      time.Now().Unix(),
	})
}

// EventHandoffExpired is published when a handoff expires.
func (p *Publisher) EventHandoffExpired(tenantID, handoffID string, expiresAt time.Time) {
	p.Publish("operan.collaboration.handoff_expired", tenantID, map[string]interface{}{
		"tenant_id":  tenantID,
		"handoff_id": handoffID,
		"expires_at": expiresAt.Unix(),
		"timestamp":  time.Now().Unix(),
	})
}

// EventPresenceUpdated is published when presence changes.
func (p *Publisher) EventPresenceUpdated(tenantID, agentID, status string, lastHeartbeat time.Time) {
	p.Publish("operan.collaboration.presence_updated", tenantID, map[string]interface{}{
		"tenant_id":       tenantID,
		"agent_id":        agentID,
		"status":          status,
		"last_heartbeat":  lastHeartbeat.Unix(),
	})
}

// EventChannelJoined is published when an agent joins a channel.
func (p *Publisher) EventChannelJoined(tenantID, channelID, agentID string) {
	p.Publish("operan.collaboration.channel_joined", tenantID, map[string]interface{}{
		"tenant_id":  tenantID,
		"channel_id": channelID,
		"agent_id":   agentID,
		"timestamp":  time.Now().Unix(),
	})
}

// EventChannelLeft is published when an agent leaves a channel.
func (p *Publisher) EventChannelLeft(tenantID, channelID, agentID string) {
	p.Publish("operan.collaboration.channel_left", tenantID, map[string]interface{}{
		"tenant_id":  tenantID,
		"channel_id": channelID,
		"agent_id":   agentID,
		"timestamp":  time.Now().Unix(),
	})
}