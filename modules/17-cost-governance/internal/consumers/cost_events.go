package consumers

import (
	"context"
	"time"

	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/events"
	"github.com/operan/cost-governance/internal/store"
)

// CostEventConsumer ingests cost events from M12/M08 via the event broker.
type CostEventConsumer struct {
	broker  *events.Broker
	store   *store.CostEventStore
	engine  *engine.Engine
}

// NewCostEventConsumer creates a new consumer.
func NewCostEventConsumer(broker *events.Broker, store *store.CostEventStore, engine *engine.Engine) *CostEventConsumer {
	return &CostEventConsumer{broker: broker, store: store, engine: engine}
}

// Register subscribes to cost event topics from M12 and M08.
func (c *CostEventConsumer) Register() {
	c.broker.Subscribe("operan.model.model_cost_recorded", c.handleCostEvent)
	c.broker.Subscribe("operan.tool-execution.cost_recorded", c.handleCostEvent)
}

func (c *CostEventConsumer) handleCostEvent(payload map[string]any) {
	tenantID, _ := payload["tenant_id"].(string)
	if tenantID == "" {
		return
	}

	agentID, _ := payload["agent_id"].(string)
	modelName, _ := payload["model_name"].(string)
	costUSD, _ := payload["cost_usd"].(float64)
	promptTokens, _ := payload["prompt_tokens"].(float64)
	completionTokens, _ := payload["completion_tokens"].(float64)
	sourceModule, _ := payload["source_module"].(string)
	sourceID, _ := payload["source_id"].(string)
	eventType, _ := payload["event_type"].(string)
	eventTS, _ := payload["event_timestamp"].(time.Time)

	if costUSD <= 0 {
		return
	}

	event := &store.CostEvent{
		TenantID:         tenantID,
		AgentID:          nil,
		SourceModule:     sourceModule,
		SourceID:         &sourceID,
		ModelName:        &modelName,
		CostUSD:          costUSD,
		PromptTokens:     int(promptTokens),
		CompletionTokens: int(completionTokens),
		EventType:        eventType,
		EventTimestamp:   eventTS,
	}

	if agentID != "" {
		event.AgentID = &agentID
	}

	// Store the event
	if err := c.store.Create(context.Background(), event); err != nil {
		return
	}

	// Check budgets
	result, err := c.engine.CheckBudgets(context.Background(), tenantID, agentID, costUSD)
	if err != nil {
		return
	}

	// Publish throttle events
	if result.ThrottleLevel != "none" {
		c.broker.Publish("operan.cost.throttle_triggered", map[string]any{
			"tenant_id":      tenantID,
			"agent_id":       agentID,
			"throttle_level": result.ThrottleLevel,
			"message":        result.Messages[0],
		})
	}

	_ = result // result contains budget statuses for downstream consumers
}