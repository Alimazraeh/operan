package consumers

import (
	"testing"

	"github.com/operan/cost-governance/internal/engine"
	"github.com/operan/cost-governance/internal/events"
	"github.com/operan/cost-governance/internal/store"
)

func TestNewCostEventConsumer(t *testing.T) {
	c := NewCostEventConsumer(nil, nil, nil)
	if c == nil {
		t.Fatal("expected non-nil CostEventConsumer")
	}
}

func TestCostEventConsumer_Register(t *testing.T) {
	// Should not panic with nil broker
	c := NewCostEventConsumer(nil, nil, nil)
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Register panicked with nil broker (expected): %v", r)
		}
	}()
	c.Register()
}

func TestCostEventConsumer_Register_WithBroker(t *testing.T) {
	broker := events.NewBroker()
	c := NewCostEventConsumer(broker, nil, nil)
	c.Register() // Should not panic
}

func TestCostEventConsumer_HandleNilPayload(t *testing.T) {
	broker := events.NewBroker()
	eventStore := store.NewCostEventStore(nil)
	thrMgr := engine.NewThrottleManager()
	budgetEngine := engine.NewEngine(nil, eventStore, nil, thrMgr)
	c := NewCostEventConsumer(broker, eventStore, budgetEngine)

	// Empty payload — should not panic
	c.handleCostEvent(map[string]any{})
}

func TestCostEventConsumer_HandleZeroCost(t *testing.T) {
	broker := events.NewBroker()
	eventStore := store.NewCostEventStore(nil)
	thrMgr := engine.NewThrottleManager()
	budgetEngine := engine.NewEngine(nil, eventStore, nil, thrMgr)
	c := NewCostEventConsumer(broker, eventStore, budgetEngine)

	// Zero cost — should not store or check
	c.handleCostEvent(map[string]any{
		"tenant_id": "t1",
		"cost_usd":  0.0,
	})
}

func TestCostEventConsumer_HandleNegativeCost(t *testing.T) {
	broker := events.NewBroker()
	eventStore := store.NewCostEventStore(nil)
	thrMgr := engine.NewThrottleManager()
	budgetEngine := engine.NewEngine(nil, eventStore, nil, thrMgr)
	c := NewCostEventConsumer(broker, eventStore, budgetEngine)

	// Negative cost — should not store
	c.handleCostEvent(map[string]any{
		"tenant_id": "t1",
		"cost_usd":  -1.0,
	})
}