// Package events tests for the Agent Registry module.
package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/config"
	"github.com/operan/modules/04-agent-registry/internal/store"
)

// mockBroker captures published events for testing.
type mockBroker struct {
	messages []mockMessage
	err      error
}

type mockMessage struct {
	topic string
	key   string
	value []byte
}

func (m *mockBroker) Produce(topic, key string, value []byte) error {
	if m.err != nil {
		return m.err
	}
	m.messages = append(m.messages, mockMessage{topic: topic, key: key, value: value})
	return nil
}

func (m *mockBroker) Close() error {
	return nil
}

func (m *mockBroker) LastMessage() *mockMessage {
	if len(m.messages) == 0 {
		return nil
	}
	return &m.messages[len(m.messages)-1]
}

func newMockBroker() *mockBroker {
	return &mockBroker{messages: []mockMessage{}}
}

func TestNewPublisher(t *testing.T) {
	p := NewPublisher()
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
	if p.broker == nil {
		t.Error("expected broker to be set")
	}
}

func TestNewPublisherWithBroker(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
	if p.broker != b {
		t.Error("expected custom broker to be set")
	}
}

func TestSetBroker(t *testing.T) {
	p := NewPublisher()
	b := newMockBroker()
	p.SetBroker(b)
	if p.broker != b {
		t.Error("expected broker to be replaced")
	}
}

func TestClose(t *testing.T) {
	p := NewPublisher()
	if err := p.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	p = NewPublisherWithBroker(newMockBroker())
	if err := p.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClose_NilBroker(t *testing.T) {
	p := &Publisher{broker: nil}
	if err := p.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPublishAgentRegistered(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	capabilities := []string{"nlp", "translation"}
	tools := []string{"search-tool"}
	escalation := []string{"escalate-to-human"}
	governance := []string{"data-privacy"}
	languages := []string{"en", "ar"}
	objectives := []store.Objective{
		{Description: "accuracy", Metric: "f1-score", Weight: 0.8, Tier: "P0"},
	}
	budget := &store.ExecutionBudget{
		DailyTokenLimit:   100000,
		MaxRunSeconds:     300,
		MonthlyExecutionCap: 1000,
		MonthlyBudgetUSD:  50.0,
	}
	deptID := strPtr("dept-1")
	memAccess := &store.MemoryAccess{
		Scope:            "tenant",
		IsolatedStores:   []string{"store-1"},
		AllowedTypes:     []string{"vector", "key-value"},
		IsolationLevel:   "strict",
	}
	ts := time.Now()

	err := p.PublishAgentRegistered(
		"agent-1", "tenant-1", "TestAgent", "analyst",
		"v1.0", "active", "user-1",
		objectives, capabilities, tools, escalation, governance, languages,
		budget, deptID, memAccess, ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	if msg == nil {
		t.Fatal("expected message to be published")
	}
	if msg.topic != TopicAgentRegistered {
		t.Errorf("expected topic %q, got %q", TopicAgentRegistered, msg.topic)
	}
	if msg.key != "agent-1" {
		t.Errorf("expected key agent-1, got %q", msg.key)
	}

	var payload AgentRegisteredPayload
	if err := json.Unmarshal(msg.value, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %q", payload.AgentID)
	}
	if payload.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %q", payload.TenantID)
	}
	if payload.Name != "TestAgent" {
		t.Errorf("expected TestAgent, got %q", payload.Name)
	}
	if payload.Role != "analyst" {
		t.Errorf("expected analyst, got %q", payload.Role)
	}
	if len(payload.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(payload.Capabilities))
	}
	if payload.RegisteredBy != "user-1" {
		t.Errorf("expected user-1, got %q", payload.RegisteredBy)
	}
	if !payload.RegisteredAt.Equal(ts) {
		t.Errorf("expected registered at %v, got %v", ts, payload.RegisteredAt)
	}
	if payload.ExecutionBudget.DailyTokenLimit != 100000 {
		t.Errorf("expected daily token limit 100000, got %d", payload.ExecutionBudget.DailyTokenLimit)
	}
	if payload.MemoryAccess.Scope != "tenant" {
		t.Errorf("expected memory scope tenant, got %q", payload.MemoryAccess.Scope)
	}
}

func TestPublishAgentCapabilitiesUpdated(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	err := p.PublishAgentCapabilitiesUpdated(
		"agent-1", "tenant-1", "user-1",
		[]string{"nlp"}, []string{"nlp", "translation"}, ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	if msg == nil {
		t.Fatal("expected message to be published")
	}
	if msg.topic != TopicAgentCapabilitiesUpdated {
		t.Errorf("expected topic %q, got %q", TopicAgentCapabilitiesUpdated, msg.topic)
	}

	var payload AgentCapabilitiesUpdatedPayload
	if err := json.Unmarshal(msg.value, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payload.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %q", payload.AgentID)
	}
	if len(payload.PreviousCapabilities) != 1 {
		t.Errorf("expected 1 previous capability, got %d", len(payload.PreviousCapabilities))
	}
	if payload.PreviousCapabilities[0] != "nlp" {
		t.Errorf("expected previous capability nlp, got %q", payload.PreviousCapabilities[0])
	}
	if len(payload.NewCapabilities) != 2 {
		t.Errorf("expected 2 new capabilities, got %d", len(payload.NewCapabilities))
	}
}

func TestPublishAgentVersionCreated(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	diff := strPtr("added new capability")
	err := p.PublishAgentVersionCreated(
		"agent-1", "v2.0", "v1.0", "Added translation",
		"active", "user-1", diff, ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	var payload AgentVersionCreatedPayload
	json.Unmarshal(msg.value, &payload)

	if payload.NewVersion != "v2.0" {
		t.Errorf("expected new version v2.0, got %q", payload.NewVersion)
	}
	if payload.PreviousVersion != "v1.0" {
		t.Errorf("expected previous version v1.0, got %q", payload.PreviousVersion)
	}
	if payload.DiffFromPrevious == nil || *payload.DiffFromPrevious != "added new capability" {
		t.Errorf("expected diff 'added new capability', got %v", payload.DiffFromPrevious)
	}
}

func TestPublishAgentPromoted(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	err := p.PublishAgentPromoted(
		"agent-1", "v1.0", "user-1",
		"staging", "production", ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	var payload AgentPromotedPayload
	json.Unmarshal(msg.value, &payload)

	if payload.PromotedVersion != "v1.0" {
		t.Errorf("expected promoted version v1.0, got %q", payload.PromotedVersion)
	}
	if payload.FromEnvironment != "staging" {
		t.Errorf("expected from staging, got %q", payload.FromEnvironment)
	}
	if payload.ToEnvironment != "production" {
		t.Errorf("expected to production, got %q", payload.ToEnvironment)
	}
}

func TestPublishAgentDeprecated(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	replacementID := strPtr("agent-2")
	sunsetDate := ts.Add(30 * 24 * time.Hour)

	err := p.PublishAgentDeprecated(
		"agent-1", "user-1", "Replaced by agent-2",
		"deprecated", replacementID, &sunsetDate, ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	var payload AgentDeprecatedPayload
	json.Unmarshal(msg.value, &payload)

	if payload.Reason != "Replaced by agent-2" {
		t.Errorf("expected reason 'Replaced by agent-2', got %q", payload.Reason)
	}
	if payload.ReplacementAgentID == nil || *payload.ReplacementAgentID != "agent-2" {
		t.Errorf("expected replacement agent-2, got %v", payload.ReplacementAgentID)
	}
}

func TestPublishAgentArchived(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	err := p.PublishAgentArchived(
		"agent-1", "user-1", "Inactivated", ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	var payload AgentArchivedPayload
	json.Unmarshal(msg.value, &payload)

	if payload.ArchiveReason != "Inactivated" {
		t.Errorf("expected reason 'Inactivated', got %q", payload.ArchiveReason)
	}
}

func TestPublishDependencyAdded(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	err := p.PublishDependencyAdded(
		"agent-1", "dep-1", "agent", ">=1.0.0", "user-1", ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	var payload DependencyAddedPayload
	json.Unmarshal(msg.value, &payload)

	if payload.DependencyType != "agent" {
		t.Errorf("expected type agent, got %q", payload.DependencyType)
	}
	if payload.VersionConstraint == nil || *payload.VersionConstraint != ">=1.0.0" {
		t.Errorf("expected constraint >=1.0.0, got %v", payload.VersionConstraint)
	}
}

func TestPublishDependencyRemoved(t *testing.T) {
	b := newMockBroker()
	p := NewPublisherWithBroker(b)

	ts := time.Now()
	err := p.PublishDependencyRemoved(
		"agent-1", "dep-1", "user-1", "No longer needed", ts,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := b.LastMessage()
	var payload DependencyRemovedPayload
	json.Unmarshal(msg.value, &payload)

	if payload.Reason != "No longer needed" {
		t.Errorf("expected reason 'No longer needed', got %q", payload.Reason)
	}
}

func TestBrokerError(t *testing.T) {
	b := &mockBroker{err: assertError("broker error")}
	p := NewPublisherWithBroker(b)

	err := p.PublishAgentRegistered(
		"agent-1", "tenant-1", "Test", "role", "v1.0", "active", "user-1",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, time.Now(),
	)
	if err == nil {
		t.Error("expected error from broker")
	}
	if err.Error() != "broker error" {
		t.Errorf("expected 'broker error', got %q", err.Error())
	}
}

func TestToEventObjectives(t *testing.T) {
	input := []store.Objective{
		{Description: "d1", Metric: "m1", Weight: 0.5, Tier: "P0"},
	}
	result := toEventObjectives(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 objective, got %d", len(result))
	}
	if result[0].Description != "d1" {
		t.Errorf("expected d1, got %q", result[0].Description)
	}
	if result[0].Weight != 0.5 {
		t.Errorf("expected weight 0.5, got %f", result[0].Weight)
	}
}

func TestToEventObjectives_Nil(t *testing.T) {
	result := toEventObjectives(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToEventMemoryAccess(t *testing.T) {
	input := &store.MemoryAccess{
		Scope:            "tenant",
		IsolatedStores:   []string{"s1"},
		AllowedTypes:     []string{"vector"},
		IsolationLevel:   "strict",
	}
	result := toEventMemoryAccess(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Scope != "tenant" {
		t.Errorf("expected scope tenant, got %q", result.Scope)
	}
}

func TestToEventMemoryAccess_Nil(t *testing.T) {
	result := toEventMemoryAccess(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToEventExecutionBudget(t *testing.T) {
	input := &store.ExecutionBudget{
		DailyTokenLimit:   100000,
		MaxRunSeconds:     300,
		MonthlyExecutionCap: 1000,
		MonthlyBudgetUSD:  50.0,
	}
	result := toEventExecutionBudget(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.DailyTokenLimit != 100000 {
		t.Errorf("expected daily limit 100000, got %d", result.DailyTokenLimit)
	}
}

func TestToEventExecutionBudget_Nil(t *testing.T) {
	result := toEventExecutionBudget(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestLogBroker(t *testing.T) {
	b := &logBroker{}
	err := b.Produce("topic", "key", []byte(`{"test":true}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestSerialize(t *testing.T) {
	type testPayload struct {
		Name string `json:"name"`
	}
	payload := testPayload{Name: "test"}
	value, err := serialize(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(value), `"name":"test"`) {
		t.Errorf("expected serialized payload to contain name, got %s", string(value))
	}
}

func strPtr(s string) *string {
	return &s
}

func assertError(msg string) error {
	return assertErr(msg)
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}

func TestNewPublisherWithConfig(t *testing.T) {
	cfg := config.Config{
		EventBusHost:  "localhost",
		EventBusPort:  "9092",
		EventBusProto: "kafka",
	}
	p := NewPublisherWithConfig(cfg)
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
	if p.broker == nil {
		t.Error("expected non-nil broker")
	}
}
