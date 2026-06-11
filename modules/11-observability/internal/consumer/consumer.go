// Package consumer ingests platform Kafka events into the observability
// stores: every consumed event becomes a trace span, increments a counter
// metric, refreshes component health, and fires an alert on error events.
package consumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/operan/modules/11-observability/internal/events"
	"github.com/operan/modules/11-observability/internal/store"
)

// Ingestor converts platform events into observability records. It is
// transport-agnostic: HandleMessage can be fed from Kafka or from tests.
type Ingestor struct {
	Spans     *store.SpanStore
	Metrics   *store.MetricStore
	Alerts    *store.AlertStore
	Health    *store.HealthStore
	Publisher *events.Publisher
}

// NewIngestor constructs an Ingestor over the given stores.
func NewIngestor(sp *store.SpanStore, m *store.MetricStore, a *store.AlertStore, h *store.HealthStore, p *events.Publisher) *Ingestor {
	return &Ingestor{Spans: sp, Metrics: m, Alerts: a, Health: h, Publisher: p}
}

// envelopeFields are the common identifiers found in platform event
// payloads (modules embed them in the payload; module 11 puts them in
// headers — both are checked).
type envelopeFields struct {
	TenantID      string `json:"tenantId"`
	TenantIDSnake string `json:"tenant_id"`
	CorrelationID string `json:"correlationId"`
	Timestamp     string `json:"timestamp"`
}

// HandleMessage ingests one consumed event. Unparseable or tenant-less
// messages are counted and dropped (observability must never crash on
// foreign input).
func (in *Ingestor) HandleMessage(topic string, key, value []byte, headers map[string]string) {
	var env envelopeFields
	_ = json.Unmarshal(value, &env) // best effort; fields stay empty on error

	tenantID := env.TenantID
	if tenantID == "" {
		tenantID = env.TenantIDSnake
	}
	if tenantID == "" {
		tenantID = headers["tenantId"]
	}
	if tenantID == "" && len(key) > 0 {
		tenantID = string(key)
	}
	if tenantID == "" {
		log.Printf("[CONSUMER] dropping %s message without tenant id", topic)
		return
	}

	correlationID := env.CorrelationID
	if correlationID == "" {
		correlationID = headers["correlationId"]
	}

	traceID := correlationID
	if traceID == "" {
		traceID = uuid.New().String()
	}

	start := time.Now().UTC()
	if env.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, env.Timestamp); err == nil {
			start = t
		}
	}

	status := store.SpanOK
	if strings.HasSuffix(topic, ".failed") || strings.HasSuffix(topic, ".deployment_failed") {
		status = store.SpanError
	}

	span := &store.TraceSpan{
		TraceID:   traceID,
		SpanID:    uuid.New().String(),
		TenantID:  tenantID,
		SpanName:  topic,
		SpanType:  SpanTypeForTopic(topic),
		StartTime: start,
		Status:    status,
		Tags:      map[string]interface{}{"topic": topic},
	}
	if added, err := in.Spans.Add(span); err == nil {
		in.Publisher.PublishTraceSpan(events.TraceSpanPayload{
			TraceID:    added.TraceID,
			SpanID:     added.SpanID,
			TenantID:   added.TenantID,
			SpanName:   added.SpanName,
			SpanType:   string(added.SpanType),
			StartTime:  added.StartTime.Format(time.RFC3339),
			EndTime:    added.StartTime.Format(time.RFC3339),
			DurationMs: added.DurationMs,
			Status:     string(added.Status),
			Tags:       added.Tags,
		}, correlationID)
	}

	in.Metrics.Record(&store.Metric{
		TenantID:    tenantID,
		MetricName:  "operan.events.consumed",
		MetricValue: 1,
		MetricType:  store.MetricCounter,
		Labels:      map[string]interface{}{"topic": topic},
	})

	componentID := ComponentForTopic(topic)
	healthState := store.Healthy
	reason := "event flow observed"
	if status == store.SpanError {
		healthState = store.Degraded
		reason = "error event observed: " + topic
	}
	if hs, changed := in.Health.Upsert(tenantID, componentID, ComponentTypeForTopic(topic), healthState, reason); changed {
		in.Publisher.PublishHealthStatusChange(events.HealthStatusChangePayload{
			TenantID:       hs.TenantID,
			ComponentID:    hs.ComponentID,
			ComponentType:  hs.ComponentType,
			PreviousStatus: string(hs.PreviousStatus),
			NewStatus:      string(hs.NewStatus),
			Reason:         hs.Reason,
			ChangedAt:      hs.ChangedAt.Format(time.RFC3339),
		}, correlationID)
	}

	if status == store.SpanError {
		if alert, err := in.Alerts.Fire(&store.Alert{
			TenantID:             tenantID,
			AlertName:            "event_error:" + topic,
			Severity:             store.SeverityWarning,
			ConditionDescription: "error event consumed from " + topic,
			CurrentValue:         1,
			Threshold:            0,
		}); err == nil {
			in.Publisher.PublishAlertFired(events.AlertFiredPayload{
				AlertID:              alert.ID,
				TenantID:             alert.TenantID,
				AlertName:            alert.AlertName,
				Severity:             string(alert.Severity),
				ConditionDescription: alert.ConditionDescription,
				CurrentValue:         alert.CurrentValue,
				Threshold:            alert.Threshold,
				TriggeredAt:          alert.TriggeredAt.Format(time.RFC3339),
			}, correlationID)
		}
	}
}

// SpanTypeForTopic maps a platform topic to the TraceSpan span_type enum.
func SpanTypeForTopic(topic string) store.SpanType {
	switch {
	case strings.HasPrefix(topic, "operan.memory."):
		return store.SpanMemory
	case strings.HasPrefix(topic, "operan.tools."):
		return store.SpanTool
	case strings.HasPrefix(topic, "operan.iam."):
		return store.SpanPolicy
	case strings.HasPrefix(topic, "operan.supervision."):
		return store.SpanHumanGate
	case strings.HasPrefix(topic, "operan.orchestration."):
		return store.SpanOrchestration
	default: // tenant, registry, templates, unknown
		return store.SpanOrchestration
	}
}

// ComponentForTopic derives a component ID from a topic's module segment,
// e.g. "operan.memory.vector.ingested" -> "memory".
func ComponentForTopic(topic string) string {
	parts := strings.Split(topic, ".")
	if len(parts) >= 2 && parts[0] == "operan" {
		return parts[1]
	}
	return "platform"
}

// ComponentTypeForTopic maps a topic to the HealthStatus component_type enum.
func ComponentTypeForTopic(topic string) string {
	switch ComponentForTopic(topic) {
	case "memory":
		return "memory"
	case "tools":
		return "tool"
	case "iam":
		return "policy"
	case "supervision":
		return "gateway"
	case "registry":
		return "agent"
	case "orchestration", "templates":
		return "workflow"
	default:
		return "gateway"
	}
}

// ─── Kafka wiring ────────────────────────────────────────────────────────────

// Run starts one Kafka reader per topic and feeds messages into the
// Ingestor until ctx is cancelled. Errors are logged and retried by the
// underlying reader; Run never panics on foreign input.
func (in *Ingestor) Run(ctx context.Context, brokerURL, consumerGroup string, topics []string) {
	brokers := parseBrokerAddresses(brokerURL)
	if len(brokers) == 0 {
		log.Printf("[CONSUMER] no broker addresses; consumer not started")
		return
	}
	for _, topic := range topics {
		go in.consumeTopic(ctx, brokers, consumerGroup, topic)
	}
	log.Printf("[CONSUMER] consuming %d topics as group %q", len(topics), consumerGroup)
}

func (in *Ingestor) consumeTopic(ctx context.Context, brokers []string, group, topic string) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  group,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})
	defer r.Close()

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[CONSUMER] read error on %s: %v", topic, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		headers := make(map[string]string, len(m.Headers))
		for _, h := range m.Headers {
			headers[h.Key] = string(h.Value)
		}
		in.HandleMessage(m.Topic, m.Key, m.Value, headers)
	}
}

func parseBrokerAddresses(addr string) []string {
	addr = strings.TrimPrefix(strings.TrimSpace(addr), "kafka://")
	var result []string
	for _, b := range strings.Split(addr, ",") {
		if b = strings.TrimSpace(b); b != "" {
			result = append(result, b)
		}
	}
	return result
}
