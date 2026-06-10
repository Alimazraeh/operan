package store

import (
	"testing"
	"time"
)

// ─── Metrics ─────────────────────────────────────────────────────────────────

func TestMetricRecordAndList(t *testing.T) {
	s := NewMetricStore()
	m, err := s.Record(&Metric{TenantID: "t1", MetricName: "tokens_used", MetricValue: 42, MetricType: MetricCounter})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if m.ID == "" || m.RecordedAt.IsZero() {
		t.Error("Record should assign ID and timestamp")
	}

	items, total, hasMore := s.List("t1", 1, 10, MetricFilter{})
	if total != 1 || len(items) != 1 || hasMore {
		t.Errorf("List = %d/%d/%v", total, len(items), hasMore)
	}
}

func TestMetricValidationAndIsolation(t *testing.T) {
	s := NewMetricStore()
	if _, err := s.Record(&Metric{MetricName: "x", MetricType: MetricGauge}); err != ErrTenantMismatch {
		t.Errorf("missing tenant: %v", err)
	}
	if _, err := s.Record(&Metric{TenantID: "t1", MetricType: MetricGauge}); err != ErrValidation {
		t.Errorf("missing name: %v", err)
	}
	if _, err := s.Record(&Metric{TenantID: "t1", MetricName: "x", MetricType: "bogus"}); err != ErrValidation {
		t.Errorf("bad type: %v", err)
	}

	s.Record(&Metric{TenantID: "t1", MetricName: "x", MetricValue: 1, MetricType: MetricCounter})
	if _, total, _ := s.List("t2", 1, 10, MetricFilter{}); total != 0 {
		t.Errorf("cross-tenant List total = %d", total)
	}
}

func TestMetricFilters(t *testing.T) {
	s := NewMetricStore()
	s.Record(&Metric{TenantID: "t1", MetricName: "a", MetricValue: 1, MetricType: MetricCounter, SourceID: "s1"})
	s.Record(&Metric{TenantID: "t1", MetricName: "b", MetricValue: 2, MetricType: MetricGauge, SourceID: "s2"})

	mt := "counter"
	if _, total, _ := s.List("t1", 1, 10, MetricFilter{MetricType: &mt}); total != 1 {
		t.Errorf("type filter total = %d", total)
	}
	name := "b"
	if _, total, _ := s.List("t1", 1, 10, MetricFilter{MetricName: &name}); total != 1 {
		t.Errorf("name filter total = %d", total)
	}
	src := "s2"
	if _, total, _ := s.List("t1", 1, 10, MetricFilter{SourceID: &src}); total != 1 {
		t.Errorf("source filter total = %d", total)
	}
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	if _, total, _ := s.List("t1", 1, 10, MetricFilter{Start: &past, End: &future}); total != 2 {
		t.Errorf("range filter total = %d", total)
	}
	if _, total, _ := s.List("t1", 1, 10, MetricFilter{Start: &future}); total != 0 {
		t.Errorf("future start filter total = %d", total)
	}
}

// ─── Spans ───────────────────────────────────────────────────────────────────

func span(tenant, trace, name string, st SpanType) *TraceSpan {
	return &TraceSpan{
		TraceID:  trace,
		SpanID:   name + "-span",
		TenantID: tenant,
		SpanName: name,
		SpanType: st,
		Status:   SpanOK,
	}
}

func TestSpanAddListAndTrace(t *testing.T) {
	s := NewSpanStore()
	a := span("t1", "trace-1", "step-a", SpanOrchestration)
	a.DurationMs = 10
	s.Add(a)
	b := span("t1", "trace-1", "step-b", SpanMemory)
	b.DurationMs = 5
	s.Add(b)
	s.Add(span("t1", "trace-2", "other", SpanTool))

	tid := "trace-1"
	if _, total, _ := s.List("t1", 1, 10, SpanFilter{TraceID: &tid}); total != 2 {
		t.Errorf("trace filter total = %d", total)
	}
	st := "memory"
	if _, total, _ := s.List("t1", 1, 10, SpanFilter{SpanType: &st}); total != 1 {
		t.Errorf("span_type filter total = %d", total)
	}

	spans, totalMs, err := s.Trace("trace-1", "t1")
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if len(spans) != 2 || totalMs != 15 {
		t.Errorf("Trace = %d spans, %d ms", len(spans), totalMs)
	}

	if _, _, err := s.Trace("trace-1", "t2"); err != ErrNotFound {
		t.Errorf("cross-tenant Trace: %v", err)
	}
	if _, _, err := s.Trace("nope", "t1"); err != ErrNotFound {
		t.Errorf("unknown trace: %v", err)
	}
}

func TestSpanValidation(t *testing.T) {
	s := NewSpanStore()
	if _, err := s.Add(&TraceSpan{TraceID: "tr", SpanID: "sp", SpanName: "x", SpanType: SpanTool}); err != ErrTenantMismatch {
		t.Errorf("missing tenant: %v", err)
	}
	if _, err := s.Add(&TraceSpan{TenantID: "t1", SpanID: "sp", SpanName: "x", SpanType: SpanTool}); err != ErrValidation {
		t.Errorf("missing trace id: %v", err)
	}
	if _, err := s.Add(&TraceSpan{TenantID: "t1", TraceID: "tr", SpanID: "sp", SpanName: "x", SpanType: "bogus"}); err != ErrValidation {
		t.Errorf("bad span type: %v", err)
	}
}

// ─── Alerts ──────────────────────────────────────────────────────────────────

func TestAlertFireResolveList(t *testing.T) {
	s := NewAlertStore()
	a, err := s.Fire(&Alert{TenantID: "t1", AlertName: "cpu_high", Severity: SeverityWarning})
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if a.ID == "" || a.ResolvedAt != nil {
		t.Error("Fire should assign ID and leave unresolved")
	}

	unresolved := false
	resolvedOnly := true
	if _, total, _ := s.List("t1", 1, 10, nil, &resolvedOnly); total != 0 {
		t.Errorf("resolved filter total = %d", total)
	}
	_ = unresolved

	r, err := s.Resolve(a.ID, "t1", "user-9")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.ResolvedAt == nil || r.ResolvedBy != "user-9" {
		t.Errorf("Resolve = %+v", r)
	}

	if _, err := s.Resolve(a.ID, "t2", "x"); err != ErrNotFound {
		t.Errorf("cross-tenant Resolve: %v", err)
	}

	sev := "warning"
	if _, total, _ := s.List("t1", 1, 10, &sev, nil); total != 1 {
		t.Errorf("severity filter total = %d", total)
	}
}

func TestAlertUnresolvedCounts(t *testing.T) {
	s := NewAlertStore()
	s.Fire(&Alert{TenantID: "t1", AlertName: "a", Severity: SeverityCritical})
	s.Fire(&Alert{TenantID: "t1", AlertName: "b", Severity: SeverityWarning})
	fired, _ := s.Fire(&Alert{TenantID: "t1", AlertName: "c", Severity: SeverityWarning})
	s.Resolve(fired.ID, "t1", "u")

	counts := s.UnresolvedBySeverity("t1")
	if counts[SeverityCritical] != 1 || counts[SeverityWarning] != 1 {
		t.Errorf("counts = %v", counts)
	}
}

// ─── Health ──────────────────────────────────────────────────────────────────

func TestHealthUpsertAndOverview(t *testing.T) {
	s := NewHealthStore()

	hs, changed := s.Upsert("t1", "memory", "memory", Healthy, "first event")
	if !changed || hs.PreviousStatus != "" {
		t.Errorf("first upsert: changed=%v prev=%q", changed, hs.PreviousStatus)
	}

	// Same state again — no change.
	if _, changed := s.Upsert("t1", "memory", "memory", Healthy, "again"); changed {
		t.Error("same-state upsert should not report change")
	}

	// Degraded transition.
	hs, changed = s.Upsert("t1", "memory", "memory", Degraded, "errors observed")
	if !changed || hs.PreviousStatus != Healthy {
		t.Errorf("transition: changed=%v prev=%q", changed, hs.PreviousStatus)
	}

	s.Upsert("t1", "tools", "tool", Healthy, "ok")
	components, overall := s.Overview("t1")
	if len(components) != 2 || overall != Degraded {
		t.Errorf("overview = %d components, overall %s", len(components), overall)
	}

	s.Upsert("t1", "memory", "memory", Unhealthy, "down")
	if _, overall := s.Overview("t1"); overall != Unhealthy {
		t.Errorf("overall should be unhealthy, got %s", overall)
	}

	if _, err := s.Get("t1", "memory"); err != nil {
		t.Errorf("Get: %v", err)
	}
	if _, err := s.Get("t2", "memory"); err != ErrNotFound {
		t.Errorf("cross-tenant Get: %v", err)
	}
	if _, overall := s.Overview("t-empty"); overall != Healthy {
		t.Errorf("empty tenant overall = %s, want healthy", overall)
	}
}
