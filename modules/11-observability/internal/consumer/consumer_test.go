package consumer

import (
	"encoding/json"
	"testing"

	"github.com/operan/modules/11-observability/internal/events"
	"github.com/operan/modules/11-observability/internal/store"
)

func newIngestor() (*Ingestor, *store.SpanStore, *store.MetricStore, *store.AlertStore, *store.HealthStore) {
	sp := store.NewSpanStore()
	m := store.NewMetricStore()
	a := store.NewAlertStore()
	h := store.NewHealthStore()
	in := NewIngestor(sp, m, a, h, events.NewPublisher())
	return in, sp, m, a, h
}

func payload(t *testing.T, v map[string]interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHandleMessageCreatesSpanMetricHealth(t *testing.T) {
	in, sp, m, _, h := newIngestor()

	in.HandleMessage("operan.memory.vector.ingested", []byte("t1"), payload(t, map[string]interface{}{
		"tenantId":      "t1",
		"correlationId": "corr-1",
		"timestamp":     "2026-06-10T12:00:00Z",
	}), nil)

	spans, total, _ := sp.List("t1", 1, 10, store.SpanFilter{})
	if total != 1 {
		t.Fatalf("spans = %d, want 1", total)
	}
	s := spans[0]
	if s.TraceID != "corr-1" || s.SpanType != store.SpanMemory || s.Status != store.SpanOK || s.SpanName != "operan.memory.vector.ingested" {
		t.Errorf("span = %+v", s)
	}

	metrics, mTotal, _ := m.List("t1", 1, 10, store.MetricFilter{})
	if mTotal != 1 || metrics[0].MetricName != "operan.events.consumed" {
		t.Errorf("metrics = %d %+v", mTotal, metrics)
	}

	hs, err := h.Get("t1", "memory")
	if err != nil || hs.NewStatus != store.Healthy || hs.ComponentType != "memory" {
		t.Errorf("health = %+v err=%v", hs, err)
	}
}

func TestHandleMessageErrorEventFiresAlertAndDegrades(t *testing.T) {
	in, _, _, a, h := newIngestor()

	in.HandleMessage("operan.tools.execution.failed", nil, payload(t, map[string]interface{}{
		"tenant_id": "t1",
	}), nil)

	alerts, total, _ := a.List("t1", 1, 10, nil, nil)
	if total != 1 {
		t.Fatalf("alerts = %d, want 1", total)
	}
	if alerts[0].Severity != store.SeverityWarning || alerts[0].ResolvedAt != nil {
		t.Errorf("alert = %+v", alerts[0])
	}

	hs, err := h.Get("t1", "tools")
	if err != nil || hs.NewStatus != store.Degraded {
		t.Errorf("health = %+v err=%v", hs, err)
	}
}

func TestHandleMessageTenantFallbacks(t *testing.T) {
	in, sp, _, _, _ := newIngestor()

	// Tenant only in headers.
	in.HandleMessage("operan.iam.user.created", nil, []byte(`{}`), map[string]string{"tenantId": "t-h"})
	if _, total, _ := sp.List("t-h", 1, 10, store.SpanFilter{}); total != 1 {
		t.Errorf("header tenant: spans = %d", total)
	}

	// Tenant only in key.
	in.HandleMessage("operan.iam.user.created", []byte("t-k"), []byte(`{}`), nil)
	if _, total, _ := sp.List("t-k", 1, 10, store.SpanFilter{}); total != 1 {
		t.Errorf("key tenant: spans = %d", total)
	}

	// No tenant anywhere — dropped, no panic.
	in.HandleMessage("operan.iam.user.created", nil, []byte(`{}`), nil)
	// Garbage payload with key — still ingested.
	in.HandleMessage("operan.iam.user.created", []byte("t-g"), []byte(`{{{not json`), nil)
	if _, total, _ := sp.List("t-g", 1, 10, store.SpanFilter{}); total != 1 {
		t.Errorf("garbage payload: spans = %d", total)
	}
}

func TestTopicMappings(t *testing.T) {
	cases := []struct {
		topic     string
		spanType  store.SpanType
		component string
		compType  string
	}{
		{"operan.memory.vector.ingested", store.SpanMemory, "memory", "memory"},
		{"operan.tools.execution.completed", store.SpanTool, "tools", "tool"},
		{"operan.iam.user.created", store.SpanPolicy, "iam", "policy"},
		{"operan.orchestration.langgraph.workflow.started", store.SpanOrchestration, "orchestration", "workflow"},
		{"operan.registry.agent.registered", store.SpanOrchestration, "registry", "agent"},
		{"operan.templates.template.deployed", store.SpanOrchestration, "templates", "workflow"},
		{"operan.tenant.provisioned", store.SpanOrchestration, "tenant", "gateway"},
		{"weird.topic", store.SpanOrchestration, "platform", "gateway"},
	}
	for _, c := range cases {
		if got := SpanTypeForTopic(c.topic); got != c.spanType {
			t.Errorf("SpanTypeForTopic(%s) = %s, want %s", c.topic, got, c.spanType)
		}
		if got := ComponentForTopic(c.topic); got != c.component {
			t.Errorf("ComponentForTopic(%s) = %s, want %s", c.topic, got, c.component)
		}
		if got := ComponentTypeForTopic(c.topic); got != c.compType {
			t.Errorf("ComponentTypeForTopic(%s) = %s, want %s", c.topic, got, c.compType)
		}
	}
}

func TestHandleMessagePublishesObservabilityEvents(t *testing.T) {
	sp := store.NewSpanStore()
	m := store.NewMetricStore()
	a := store.NewAlertStore()
	h := store.NewHealthStore()
	mock := &mockBroker{}
	in := NewIngestor(sp, m, a, h, events.NewPublisherWithBroker(mock))

	in.HandleMessage("operan.tools.execution.failed", []byte("t1"), []byte(`{}`), nil)

	want := map[string]bool{
		"operan.observability.trace.span":           false,
		"operan.observability.health.status_change": false,
		"operan.observability.alert.fired":          false,
	}
	for _, topic := range mock.topics {
		if _, ok := want[topic]; ok {
			want[topic] = true
		}
	}
	for topic, seen := range want {
		if !seen {
			t.Errorf("expected %s to be published; got %v", topic, mock.topics)
		}
	}
}
