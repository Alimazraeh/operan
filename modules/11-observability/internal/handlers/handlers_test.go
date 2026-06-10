package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/11-observability/internal/ctxkeys"
	"github.com/operan/modules/11-observability/internal/events"
	"github.com/operan/modules/11-observability/internal/store"
)

const testTenant = "11111111-1111-1111-1111-111111111111"

func testHandlers() (*ObservabilityHandlers, *http.ServeMux) {
	h := NewObservabilityHandlers(store.NewMetricStore(), store.NewSpanStore(), store.NewAlertStore(), store.NewHealthStore(), events.NewPublisher(), 100)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return h, mux
}

func do(mux *http.ServeMux, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	ctx := ctxkeys.WithTenantID(context.Background(), testTenant)
	ctx = ctxkeys.WithUserID(ctx, "user-1")
	ctx = ctxkeys.WithRequestID(ctx, "req-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestRecordAndListMetrics(t *testing.T) {
	_, mux := testHandlers()

	w := do(mux, "POST", "/metrics", map[string]interface{}{
		"metric_name":  "tokens_used",
		"metric_value": 1234.0,
		"metric_type":  "counter",
		"labels":       map[string]string{"model": "claude"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("record: status %d: %s", w.Code, w.Body.String())
	}
	var m store.Metric
	json.Unmarshal(w.Body.Bytes(), &m)
	if m.ID == "" || m.TenantID != testTenant || m.MetricValue != 1234 {
		t.Errorf("metric = %+v", m)
	}

	lw := do(mux, "GET", "/metrics?metric_type=counter", nil)
	var list struct {
		Items []store.Metric `json:"items"`
		Total int            `json:"total"`
	}
	json.Unmarshal(lw.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("list total = %d", list.Total)
	}
}

func TestMetricValidationErrors(t *testing.T) {
	_, mux := testHandlers()
	cases := []struct {
		body map[string]interface{}
		want int
	}{
		{map[string]interface{}{"metric_value": 1.0, "metric_type": "counter"}, http.StatusBadRequest},                            // no name
		{map[string]interface{}{"metric_name": "x", "metric_type": "bogus"}, http.StatusBadRequest},                               // bad type
		{map[string]interface{}{"metric_name": "x", "metric_type": "gauge", "tenant_id": "wrong"}, http.StatusForbidden},          // tenant mismatch
	}
	for i, c := range cases {
		if w := do(mux, "POST", "/metrics", c.body); w.Code != c.want {
			t.Errorf("case %d: status %d, want %d", i, w.Code, c.want)
		}
	}
	if w := do(mux, "GET", "/metrics?metric_type=bogus", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad type filter: status %d", w.Code)
	}
	if w := do(mux, "GET", "/metrics?start=not-a-date", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad start: status %d", w.Code)
	}
	if w := do(mux, "GET", "/metrics?tenant_id=other", nil); w.Code != http.StatusForbidden {
		t.Errorf("tenant mismatch query: status %d", w.Code)
	}
}

func TestSpansAndTrace(t *testing.T) {
	h, mux := testHandlers()
	h.Spans.Add(&store.TraceSpan{TraceID: "tr-1", SpanID: "sp-1", TenantID: testTenant, SpanName: "a", SpanType: store.SpanMemory, DurationMs: 7, Status: store.SpanOK})
	h.Spans.Add(&store.TraceSpan{TraceID: "tr-1", SpanID: "sp-2", TenantID: testTenant, SpanName: "b", SpanType: store.SpanTool, DurationMs: 3, Status: store.SpanError})

	w := do(mux, "GET", "/spans?span_type=memory", nil)
	var list struct {
		Items []store.TraceSpan `json:"items"`
		Total int               `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Items[0].SpanID != "sp-1" {
		t.Errorf("span filter: %+v", list)
	}

	tw := do(mux, "GET", "/traces/tr-1", nil)
	if tw.Code != http.StatusOK {
		t.Fatalf("trace: status %d", tw.Code)
	}
	var trace struct {
		Spans           []store.TraceSpan `json:"spans"`
		TotalDurationMs int               `json:"total_duration_ms"`
	}
	json.Unmarshal(tw.Body.Bytes(), &trace)
	if len(trace.Spans) != 2 || trace.TotalDurationMs != 10 {
		t.Errorf("trace = %d spans %d ms", len(trace.Spans), trace.TotalDurationMs)
	}

	if w := do(mux, "GET", "/traces/unknown", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown trace: status %d", w.Code)
	}
	if w := do(mux, "GET", "/spans?span_type=bogus", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad span_type: status %d", w.Code)
	}
}

func TestAlertsLifecycle(t *testing.T) {
	h, mux := testHandlers()
	fired, _ := h.Alerts.Fire(&store.Alert{TenantID: testTenant, AlertName: "cpu", Severity: store.SeverityCritical})

	w := do(mux, "GET", "/alerts?resolved=false", nil)
	var list struct {
		Items []store.Alert `json:"items"`
		Total int           `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Fatalf("unresolved alerts = %d", list.Total)
	}

	rw := do(mux, "POST", "/alerts/"+fired.ID+"/resolve", nil)
	if rw.Code != http.StatusOK {
		t.Fatalf("resolve: status %d", rw.Code)
	}
	var resolved store.Alert
	json.Unmarshal(rw.Body.Bytes(), &resolved)
	if resolved.ResolvedAt == nil || resolved.ResolvedBy != "user-1" {
		t.Errorf("resolved = %+v", resolved)
	}

	if w := do(mux, "POST", "/alerts/nonexistent/resolve", nil); w.Code != http.StatusNotFound {
		t.Errorf("resolve missing: status %d", w.Code)
	}
	if w := do(mux, "GET", "/alerts?resolved=banana", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad resolved: status %d", w.Code)
	}
	if w := do(mux, "GET", "/alerts?severity=bogus", nil); w.Code != http.StatusBadRequest {
		t.Errorf("bad severity: status %d", w.Code)
	}
}

func TestHealthEndpoints(t *testing.T) {
	h, mux := testHandlers()
	h.Health.Upsert(testTenant, "memory", "memory", store.Healthy, "ok")
	h.Health.Upsert(testTenant, "tools", "tool", store.Degraded, "errors")

	w := do(mux, "GET", "/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("health: status %d", w.Code)
	}
	var overview struct {
		Components    []store.HealthStatus `json:"components"`
		OverallStatus string               `json:"overall_status"`
	}
	json.Unmarshal(w.Body.Bytes(), &overview)
	if len(overview.Components) != 2 || overview.OverallStatus != "degraded" {
		t.Errorf("overview = %+v", overview)
	}

	cw := do(mux, "GET", "/health/memory", nil)
	if cw.Code != http.StatusOK {
		t.Fatalf("component: status %d", cw.Code)
	}
	var hs store.HealthStatus
	json.Unmarshal(cw.Body.Bytes(), &hs)
	if hs.ComponentID != "memory" || hs.NewStatus != store.Healthy {
		t.Errorf("component = %+v", hs)
	}

	if w := do(mux, "GET", "/health/unknown", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown component: status %d", w.Code)
	}
}

func TestErrorSchemaShape(t *testing.T) {
	_, mux := testHandlers()
	w := do(mux, "GET", "/traces/unknown", nil)
	var e map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &e)
	for _, f := range []string{"code", "message", "request_id"} {
		if _, ok := e[f]; !ok {
			t.Errorf("error missing contract field %q: %s", f, w.Body.String())
		}
	}
}
