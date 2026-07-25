package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func TestKPIMeasurementsFromLedger(t *testing.T) {
	h := &TemplateHandlers{
		DepartmentStore: store.NewDepartmentStore(),
		RequestStore:    store.NewRequestStore(),
		TokenRate:       3.0,
	}
	dept, err := h.DepartmentStore.Create(&store.Department{
		TenantID: "t1", Name: "IT", Status: "operational",
		KPIS: []store.KPIDefinition{
			{ID: "k1", Name: "First Response SLA compliance", MetricType: "gauge"},
			{ID: "k2", Name: "Mean ticket resolution time", MetricType: "timer"},
			{ID: "k3", Name: "Employee satisfaction survey", MetricType: "gauge"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	mk := func(status string, cycle time.Duration, respLate bool, gate time.Duration, tokens int) {
		created := now.Add(-48 * time.Hour)
		resp := created.Add(10 * time.Minute)
		respDue := created.Add(15 * time.Minute)
		if respLate {
			resp = created.Add(20 * time.Minute)
		}
		resDue := created.Add(4 * time.Hour)
		completed := created.Add(cycle)
		r := &store.ServiceRequest{
			TenantID: "t1", DepartmentID: dept.ID, Title: "x", Status: status,
			TokensUsed:     tokens,
			SLAResponseDue: &respDue, SLAResolutionDue: &resDue,
			FirstResponseAt: &resp,
		}
		if status == "completed" {
			r.CompletedAt = &completed
		}
		if gate > 0 {
			r.Timeline = []store.RequestEvent{
				{At: created.Add(30 * time.Minute), Kind: "gate_raised", Node: "g1"},
				{At: created.Add(30*time.Minute + gate), Kind: "gate_responded", Node: "g1"},
			}
		}
		cr, err := h.RequestStore.Create(r)
		if err != nil {
			t.Fatal(err)
		}
		// Create stamps CreatedAt with now — mutate it back for the window math.
		if err := h.RequestStore.Mutate(cr.ID, func(sr *store.ServiceRequest) { sr.CreatedAt = created }); err != nil {
			t.Fatal(err)
		}
	}
	mk("completed", 1*time.Hour, false, 10*time.Minute, 1000) // in SLA both clocks
	mk("completed", 6*time.Hour, true, 20*time.Minute, 2000)  // late both clocks
	mk("failed", 0, false, 0, 500)

	req := httptest.NewRequest("GET", "/departments/"+dept.ID+"/kpi-measurements", nil)
	w := httptest.NewRecorder()
	h.kpiMeasurements(w, req, dept)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Windows map[string]windowMetrics `json:"windows"`
		KPIs    []kpiMeasurement         `json:"kpi_measurements"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	m := out.Windows["7d"]
	if m.Requests != 3 || m.Completed != 2 || m.Failed != 1 {
		t.Fatalf("counts = %+v", m)
	}
	if m.SLAResponseCompliancePct == nil || *m.SLAResponseCompliancePct != 66 {
		t.Errorf("response compliance = %v, want 66", m.SLAResponseCompliancePct)
	}
	if m.SLAResolutionCompliancePct == nil || *m.SLAResolutionCompliancePct != 50 {
		t.Errorf("resolution compliance = %v, want 50", m.SLAResolutionCompliancePct)
	}
	if m.MedianCycleSeconds == nil || *m.MedianCycleSeconds != int((6*time.Hour).Seconds()) {
		t.Errorf("median cycle = %v", m.MedianCycleSeconds)
	}
	if m.MedianGateTurnaroundSeconds == nil || *m.MedianGateTurnaroundSeconds != int((20*time.Minute).Seconds()) {
		t.Errorf("gate turnaround = %v", m.MedianGateTurnaroundSeconds)
	}
	if m.TokensUsed != 3500 || m.EstimatedCostUSD == nil {
		t.Errorf("tokens = %d cost = %v", m.TokensUsed, m.EstimatedCostUSD)
	}

	// KPI mapping: SLA + resolution-time measured; satisfaction honestly not.
	byID := map[string]kpiMeasurement{}
	for _, k := range out.KPIs {
		byID[k.KPIID] = k
	}
	if !byID["k1"].Measured || !byID["k2"].Measured {
		t.Errorf("k1/k2 should be measured: %+v %+v", byID["k1"], byID["k2"])
	}
	if byID["k3"].Measured || byID["k3"].Note != "no data source yet" {
		t.Errorf("k3 must be honestly unmeasured: %+v", byID["k3"])
	}
}
