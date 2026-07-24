package store

import (
	"testing"
	"time"
)

func TestCostBudget_ZeroValues(t *testing.T) {
	b := CostBudget{}
	if b.ID != "" {
		t.Error("expected zero ID")
	}
	if b.Currency != "" {
		t.Error("expected empty Currency")
	}
	if b.IsActive != false {
		t.Error("expected IsActive to be false")
	}
}

func TestCostBudget_DefaultValues(t *testing.T) {
	agent := "agent-1"
	desc := "Test budget"
	now := time.Now()
	b := CostBudget{
		TenantID:     "tenant-1",
		AgentID:      &agent,
		Description:  &desc,
		BudgetAmount: 100.0,
		Currency:     "USD",
		Period:       "monthly",
		SoftLimitPct: 80,
		HardLimitPct: 95,
		IsActive:     true,
		StartedAt:    now,
	}
	if b.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", b.TenantID)
	}
	if b.BudgetAmount != 100.0 {
		t.Errorf("expected 100.0, got %f", b.BudgetAmount)
	}
	if b.Period != "monthly" {
		t.Errorf("expected monthly, got %s", b.Period)
	}
}

func TestCostBudget_NilAgentID(t *testing.T) {
	b := CostBudget{
		TenantID: "tenant-1",
		AgentID:  nil,
	}
	if b.AgentID != nil {
		t.Error("expected nil AgentID")
	}
}

func TestCostEvent_ZeroValues(t *testing.T) {
	e := CostEvent{}
	if e.ID != "" {
		t.Error("expected zero ID")
	}
	if e.SourceModule != "" {
		t.Error("expected empty SourceModule")
	}
	if e.CostUSD != 0 {
		t.Error("expected zero CostUSD")
	}
}

func TestCostEvent_ValidValues(t *testing.T) {
	agent := "agent-1"
	model := "gpt-4"
	sourceID := "call-123"
	billingTag := "prod"
	ts := time.Now()
	e := CostEvent{
		TenantID:         "tenant-1",
		AgentID:          &agent,
		SourceModule:     "m12",
		SourceID:         &sourceID,
		ModelName:        &model,
		CostUSD:          0.05,
		PromptTokens:     100,
		CompletionTokens: 50,
		EventType:        "model_call",
		BillingTag:       &billingTag,
		EventTimestamp:   ts,
	}
	if e.SourceModule != "m12" {
		t.Errorf("expected m12, got %s", e.SourceModule)
	}
	if e.CostUSD != 0.05 {
		t.Errorf("expected 0.05, got %f", e.CostUSD)
	}
}

func TestCostAlert_ZeroValues(t *testing.T) {
	a := CostAlert{}
	if a.ID != "" {
		t.Error("expected zero ID")
	}
	if a.AlertType != "" {
		t.Error("expected empty AlertType")
	}
	if a.IsResolved != false {
		t.Error("expected IsResolved to be false")
	}
}

func TestCostAlert_ValidValues(t *testing.T) {
	budgetID := "b1"
	a := CostAlert{
		TenantID:       "tenant-1",
		BudgetID:       &budgetID,
		AlertType:      "soft_limit",
		CurrentSpend:   80.0,
		BudgetAmount:   100.0,
		PercentageUsed: 80.0,
		Severity:       "warning",
		IsResolved:     false,
	}
	if a.AlertType != "soft_limit" {
		t.Errorf("expected soft_limit, got %s", a.AlertType)
	}
	if a.Severity != "warning" {
		t.Errorf("expected warning, got %s", a.Severity)
	}
}

func TestThrottleState_Default(t *testing.T) {
	ts := ThrottleState{}
	if ts.State != "" {
		t.Errorf("expected empty state, got %s", ts.State)
	}
}

func TestBudgetStore_NewBudgetStore(t *testing.T) {
	s := NewBudgetStore(nil)
	if s == nil {
		t.Fatal("expected non-nil BudgetStore")
	}
}

func TestCostEventStore_NewCostEventStore(t *testing.T) {
	s := NewCostEventStore(nil)
	if s == nil {
		t.Fatal("expected non-nil CostEventStore")
	}
}

func TestAlertStore_NewAlertStore(t *testing.T) {
	s := NewAlertStore(nil)
	if s == nil {
		t.Fatal("expected non-nil AlertStore")
	}
}

func TestCostBudget_Periods(t *testing.T) {
	validPeriods := []string{"daily", "weekly", "monthly", "quarterly"}
	for _, p := range validPeriods {
		b := CostBudget{Period: p}
		if b.Period != p {
			t.Errorf("expected period %s, got %s", p, b.Period)
		}
	}
}

func TestCostEvent_SourceModules(t *testing.T) {
	validModules := []string{"m12", "m08", "manual"}
	for _, m := range validModules {
		e := CostEvent{SourceModule: m}
		if e.SourceModule != m {
			t.Errorf("expected source_module %s, got %s", m, e.SourceModule)
		}
	}
}

func TestCostAlert_AlertTypes(t *testing.T) {
	validTypes := []string{"soft_limit", "hard_limit", "budget_exceeded", "budget_reset"}
	for _, at := range validTypes {
		a := CostAlert{AlertType: at}
		if a.AlertType != at {
			t.Errorf("expected alert_type %s, got %s", at, a.AlertType)
		}
	}
}

func TestCostAlert_Severities(t *testing.T) {
	validSeverities := []string{"info", "warning", "critical", "fatal"}
	for _, sev := range validSeverities {
		a := CostAlert{Severity: sev}
		if a.Severity != sev {
			t.Errorf("expected severity %s, got %s", sev, a.Severity)
		}
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Fatal("ErrNotFound should not be nil")
	}
	if ErrNotFound.Error() != "record not found" {
		t.Errorf("unexpected error message: %s", ErrNotFound.Error())
	}
}

// ptr is a helper for *string
func ptr(s string) *string {
	return &s
}
