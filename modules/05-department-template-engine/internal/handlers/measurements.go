package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/middleware"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

// The measurement layer: every number here is DERIVED from the request
// ledger — nothing is estimated except the USD figure, which is labeled with
// the rate that produced it. KPI definitions ship on templates; only the
// ones a ledger metric genuinely backs are reported as measured.

type windowMetrics struct {
	Requests                    int      `json:"requests"`
	Completed                   int      `json:"completed"`
	Failed                      int      `json:"failed"`
	CompletionRatePct           *int     `json:"completion_rate_pct,omitempty"`
	SLAResponseCompliancePct    *int     `json:"sla_response_compliance_pct,omitempty"`
	SLAResolutionCompliancePct  *int     `json:"sla_resolution_compliance_pct,omitempty"`
	MedianCycleSeconds          *int     `json:"median_cycle_seconds,omitempty"`
	MedianGateTurnaroundSeconds *int     `json:"median_gate_turnaround_seconds,omitempty"`
	TokensUsed                  int      `json:"tokens_used"`
	EstimatedCostUSD            *float64 `json:"estimated_cost_usd,omitempty"`
}

type kpiMeasurement struct {
	KPIID    string      `json:"kpi_id"`
	Name     string      `json:"name"`
	Unit     string      `json:"unit,omitempty"`
	Measured bool        `json:"measured"`
	Value    interface{} `json:"value,omitempty"`
	Source   string      `json:"source,omitempty"`
	Note     string      `json:"note,omitempty"`
}

// kpiMeasurements handles GET /departments/{id}/kpi-measurements.
func (h *TemplateHandlers) kpiMeasurements(w http.ResponseWriter, r *http.Request, dept *store.Department) {
	_ = middleware.RequestIDFromContext(r.Context())
	now := time.Now().UTC()
	reqs, _, _ := h.RequestStore.ListByDepartment(dept.TenantID, dept.ID, nil, 1, 1000)

	windows := map[string]windowMetrics{
		"7d":  computeWindow(reqs, now.Add(-7*24*time.Hour), now, h.TokenRate),
		"30d": computeWindow(reqs, now.Add(-30*24*time.Hour), now, h.TokenRate),
	}
	m30 := windows["30d"]

	// Conservative mapping: a KPI is "measured" only when a ledger metric
	// genuinely backs it. Everything else says so, honestly.
	kpis := make([]kpiMeasurement, 0, len(dept.KPIS))
	for _, k := range dept.KPIS {
		km := kpiMeasurement{KPIID: k.ID, Name: k.Name, Unit: k.Unit}
		name := strings.ToLower(k.Name + " " + k.Description)
		switch {
		case strings.Contains(name, "sla") && (strings.Contains(name, "resolution") || strings.Contains(name, "resolve")):
			if m30.SLAResolutionCompliancePct != nil {
				km.Measured, km.Value, km.Unit, km.Source = true, *m30.SLAResolutionCompliancePct, "%", "request_ledger:sla_resolution_compliance_30d"
			}
		case strings.Contains(name, "sla") || strings.Contains(name, "response time") || strings.Contains(name, "first response"):
			if m30.SLAResponseCompliancePct != nil {
				km.Measured, km.Value, km.Unit, km.Source = true, *m30.SLAResponseCompliancePct, "%", "request_ledger:sla_response_compliance_30d"
			}
		case strings.Contains(name, "cycle") || strings.Contains(name, "resolution time") || strings.Contains(name, "mttr"):
			if m30.MedianCycleSeconds != nil {
				km.Measured, km.Value, km.Unit, km.Source = true, *m30.MedianCycleSeconds, "seconds (median)", "request_ledger:median_cycle_30d"
			}
		case strings.Contains(name, "ticket") || strings.Contains(name, "request") || strings.Contains(name, "volume") || strings.Contains(name, "throughput"):
			km.Measured, km.Value, km.Unit, km.Source = true, m30.Completed, "completed/30d", "request_ledger:completed_30d"
		case strings.Contains(name, "cost") || strings.Contains(name, "token") || strings.Contains(name, "spend") || strings.Contains(name, "budget"):
			km.Measured, km.Value, km.Unit, km.Source = true, m30.TokensUsed, "tokens/30d", "request_ledger:tokens_30d"
		case strings.Contains(name, "approval") || strings.Contains(name, "gate") || strings.Contains(name, "sign-off"):
			if m30.MedianGateTurnaroundSeconds != nil {
				km.Measured, km.Value, km.Unit, km.Source = true, *m30.MedianGateTurnaroundSeconds, "seconds (median)", "request_ledger:gate_turnaround_30d"
			}
		}
		if !km.Measured {
			km.Note = "no data source yet"
		}
		writeBack := km
		kpis = append(kpis, writeBack)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"department_id":              dept.ID,
		"generated_at":               now,
		"token_rate_usd_per_million": h.TokenRate,
		"windows":                    windows,
		"kpi_measurements":           kpis,
	})
}

func computeWindow(reqs []store.ServiceRequest, from, now time.Time, tokenRate float64) windowMetrics {
	m := windowMetrics{}
	var cycles, gates []int
	respMet, respMeasured, resMet, resMeasured := 0, 0, 0, 0
	for i := range reqs {
		r := &reqs[i]
		if r.CreatedAt.Before(from) {
			continue
		}
		m.Requests++
		m.TokensUsed += r.TokensUsed
		switch r.Status {
		case "completed":
			m.Completed++
			if r.CompletedAt != nil {
				cycles = append(cycles, int(r.CompletedAt.Sub(r.CreatedAt).Seconds()))
				if r.SLAResolutionDue != nil {
					resMeasured++
					if !r.CompletedAt.After(*r.SLAResolutionDue) {
						resMet++
					}
				}
			}
		case "failed", "cancelled", "rejected":
			m.Failed++
		}
		if r.FirstResponseAt != nil && r.SLAResponseDue != nil {
			respMeasured++
			if !r.FirstResponseAt.After(*r.SLAResponseDue) {
				respMet++
			}
		}
		// Gate turnaround: raised → responded, paired per node.
		raised := map[string]time.Time{}
		for _, ev := range r.Timeline {
			switch ev.Kind {
			case "gate_raised":
				raised[ev.Node] = ev.At
			case "gate_responded":
				if t0, ok := raised[ev.Node]; ok {
					gates = append(gates, int(ev.At.Sub(t0).Seconds()))
					delete(raised, ev.Node)
				}
			}
		}
	}
	if m.Requests > 0 {
		v := m.Completed * 100 / m.Requests
		m.CompletionRatePct = &v
	}
	if respMeasured > 0 {
		v := respMet * 100 / respMeasured
		m.SLAResponseCompliancePct = &v
	}
	if resMeasured > 0 {
		v := resMet * 100 / resMeasured
		m.SLAResolutionCompliancePct = &v
	}
	if v, ok := median(cycles); ok {
		m.MedianCycleSeconds = &v
	}
	if v, ok := median(gates); ok {
		m.MedianGateTurnaroundSeconds = &v
	}
	if tokenRate > 0 && m.TokensUsed > 0 {
		c := float64(m.TokensUsed) / 1e6 * tokenRate
		m.EstimatedCostUSD = &c
	}
	return m
}

func median(xs []int) (int, bool) {
	if len(xs) == 0 {
		return 0, false
	}
	sort.Ints(xs)
	return xs[len(xs)/2], true
}
