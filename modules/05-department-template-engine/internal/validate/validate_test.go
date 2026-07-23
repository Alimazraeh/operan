package validate

import (
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func base() *store.Template {
	return &store.Template{
		Name: "IT", Category: "it",
		Agents: []store.AgentDefinition{
			{ID: "mgr", Role: "Manager", AutonomyTier: "coordinate"},
			{ID: "sd", Role: "Support", ReportsTo: "mgr", AutonomyTier: "draft"},
		},
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "Manager", RoleType: "manager", HolderType: "ai_agent", AgentDefID: "mgr"},
			{ID: "pos-sd", Title: "Support", RoleType: "support", HolderType: "ai_agent", AgentDefID: "sd", ReportsTo: "pos-mgr"},
		},
		Workflows: []store.WorkflowDefinition{{ID: "wf-1", Name: "Tickets", Steps: []store.WorkflowStep{{ID: "s1", Type: "agent_call"}}}},
		KPIS:      []store.KPIDefinition{{ID: "kpi-1", Name: "MTTR", MetricType: "timer"}},
	}
}

func errCount(t *store.Template) int { return len(Errors(Template(t))) }

func TestValidTemplatePasses(t *testing.T) {
	if n := errCount(base()); n != 0 {
		t.Fatalf("expected clean, got %d errors: %+v", n, Errors(Template(base())))
	}
}

func TestReportingCycleDetected(t *testing.T) {
	tmpl := base()
	tmpl.Agents[0].ReportsTo = "sd" // mgr → sd → mgr
	if n := errCount(tmpl); n == 0 {
		t.Fatal("agent reporting cycle not detected")
	}

	tmpl2 := base()
	tmpl2.OrgChart[0].ReportsTo = "pos-sd" // position cycle + no root
	if n := errCount(tmpl2); n == 0 {
		t.Fatal("position cycle not detected")
	}
}

func TestDanglingRefsDetected(t *testing.T) {
	tmpl := base()
	tmpl.Services = []store.ServiceOffering{{
		ID: "svc-1", Name: "Desk",
		OwnerPositionID: "pos-nope", DeliveryWorkflowID: "wf-nope", KPIRefs: []string{"kpi-nope"},
	}}
	errs := Errors(Template(tmpl))
	if len(errs) != 3 {
		t.Fatalf("expected 3 dangling-ref errors, got %d: %+v", len(errs), errs)
	}

	tmpl2 := base()
	tmpl2.ValueStreams = []store.ValueStream{{
		ID: "vs-1", Name: "Restore",
		Stages:             []store.ValueStage{{ID: "st-1", Name: "Fix", WorkflowRef: "wf-nope"}},
		ValueMetricKPIRefs: []string{"kpi-nope"},
	}}
	if n := errCount(tmpl2); n != 2 {
		t.Fatalf("value stream dangling refs: got %d errors", n)
	}
}

func TestInvalidEnumsDetected(t *testing.T) {
	tmpl := base()
	tmpl.Risks = []store.RiskItem{{ID: "r1", Name: "X", Severity: "catastrophic", Likelihood: "sometimes"}}
	if n := errCount(tmpl); n != 2 {
		t.Fatalf("risk enums: got %d errors", n)
	}

	tmpl2 := base()
	tmpl2.Agents[0].AutonomyTier = "yolo"
	if n := errCount(tmpl2); n == 0 {
		t.Fatal("invalid autonomy tier not detected")
	}
}

func TestOrgChartMissingRoot(t *testing.T) {
	tmpl := base()
	tmpl.OrgChart[0].ReportsTo = "pos-sd"
	found := false
	for _, i := range Errors(Template(tmpl)) {
		if i.Path == "org_chart" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected org_chart root/cycle error")
	}
}
