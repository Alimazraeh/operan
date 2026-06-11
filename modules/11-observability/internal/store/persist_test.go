package store

import "testing"

func TestObservabilityPersistRoundTrip(t *testing.T) {
	m := NewMetricStore()
	m.Record(&Metric{TenantID: "t1", MetricName: "tokens", MetricValue: 5, MetricType: MetricCounter})
	data, _ := m.Export()
	m2 := NewMetricStore()
	if err := m2.Import(data); err != nil {
		t.Fatalf("metric import: %v", err)
	}
	if _, total, _ := m2.List("t1", 1, 10, MetricFilter{}); total != 1 {
		t.Errorf("metrics = %d", total)
	}

	sp := NewSpanStore()
	a := span("t1", "tr-1", "step-a", SpanMemory)
	a.DurationMs = 4
	sp.Add(a)
	data, _ = sp.Export()
	sp2 := NewSpanStore()
	if err := sp2.Import(data); err != nil {
		t.Fatalf("span import: %v", err)
	}
	spans, total, err := sp2.Trace("tr-1", "t1")
	if err != nil || len(spans) != 1 || total != 4 {
		t.Errorf("trace index after restore: %d spans %d ms err=%v", len(spans), total, err)
	}

	al := NewAlertStore()
	fired, _ := al.Fire(&Alert{TenantID: "t1", AlertName: "x", Severity: SeverityWarning})
	data, _ = al.Export()
	al2 := NewAlertStore()
	al2.Import(data)
	if _, err := al2.Resolve(fired.ID, "t1", "u"); err != nil {
		t.Errorf("resolve after restore: %v", err)
	}

	h := NewHealthStore()
	h.Upsert("t1", "memory", "memory", Degraded, "errs")
	data, _ = h.Export()
	h2 := NewHealthStore()
	h2.Import(data)
	if hs, err := h2.Get("t1", "memory"); err != nil || hs.NewStatus != Degraded {
		t.Errorf("health after restore: %+v err=%v", hs, err)
	}
}
