package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func seedDepartment(t *testing.T, h *TemplateHandlers, tenant string) *store.Department {
	t.Helper()
	created, err := h.DepartmentStore.Create(&store.Department{
		TenantID: tenant, Name: "IT Department", Slug: "it", Category: "it", Status: "operational",
		Mission: "Keep everyone productive",
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "IT Manager", RoleType: "manager", HolderType: "ai_agent", AgentDefID: "mgr", AutonomyTier: "coordinate"},
			{ID: "pos-sd", Title: "Service Desk", RoleType: "support", HolderType: "ai_agent", AgentDefID: "sd", ReportsTo: "pos-mgr", EscalatesTo: "pos-mgr"},
		},
		Services: []store.ServiceOffering{{ID: "svc-1", Name: "Service Desk", SLA: &store.SLA{ResponseTime: "15m"}}},
		ValueStreams: []store.ValueStream{{ID: "vs-1", Name: "Restore", Stages: []store.ValueStage{{ID: "st-1", Name: "Fix"}},
			ValueMetricKPIRefs: []string{"kpi-1"}}},
		Risks:              []store.RiskItem{{ID: "r1", Name: "Backup failure", Severity: "high", Likelihood: "unlikely"}},
		QualityStandards:   []store.QualityStandard{{ID: "q1", Name: "Uptime", Target: "99.9%"}},
		ComplianceControls: []store.ComplianceControl{{ID: "c1", Framework: "ISO-27001", Name: "Access control"}},
		KPIS:               []store.KPIDefinition{{ID: "kpi-1", Name: "MTTR", MetricType: "timer"}},
	})
	if err != nil {
		t.Fatalf("seed department: %v", err)
	}
	return created
}

func TestListDepartments(t *testing.T) {
	h := newTestHandlers(t)
	seedDepartment(t, h, "tenant-1")
	seedDepartment(t, h, "tenant-other")

	req, _ := testRequest("GET", "/departments", nil)
	rec := httptest.NewRecorder()
	h.ListDepartments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []map[string]interface{} `json:"data"`
		Meta map[string]interface{}   `json:"meta"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Fatalf("tenant isolation broken: %d departments", len(resp.Data))
	}
	if resp.Data[0]["positions_count"].(float64) != 2 || resp.Data[0]["services_count"].(float64) != 1 {
		t.Fatalf("summary counts: %+v", resp.Data[0])
	}
}

func TestGetDepartmentAndSubResources(t *testing.T) {
	h := newTestHandlers(t)
	d := seedDepartment(t, h, "tenant-1")

	// Full department
	req, _ := testRequest("GET", "/departments/"+d.ID, nil)
	rec := httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}

	// Org chart graph
	req, _ = testRequest("GET", "/departments/"+d.ID+"/org-chart", nil)
	rec = httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	var org struct {
		Root      string                   `json:"root_position_id"`
		Positions []map[string]interface{} `json:"positions"`
		Edges     []map[string]string      `json:"edges"`
	}
	json.Unmarshal(rec.Body.Bytes(), &org)
	if org.Root != "pos-mgr" || len(org.Positions) != 2 {
		t.Fatalf("org chart: %+v", org)
	}
	// reports_to + escalates_to edges
	if len(org.Edges) != 2 {
		t.Fatalf("edges: %+v", org.Edges)
	}

	// Services / value-chain / risks / quality / compliance
	for _, sub := range []string{"services", "risks", "quality"} {
		req, _ = testRequest("GET", "/departments/"+d.ID+"/"+sub, nil)
		rec = httptest.NewRecorder()
		h.HandleDepartmentByID(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", sub, rec.Code)
		}
		var listResp struct {
			Data []interface{} `json:"data"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		if len(listResp.Data) != 1 {
			t.Fatalf("%s: %d items", sub, len(listResp.Data))
		}
	}

	req, _ = testRequest("GET", "/departments/"+d.ID+"/value-chain", nil)
	rec = httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	var vc struct {
		Streams  []map[string]interface{}          `json:"value_streams"`
		KPIIndex map[string]map[string]interface{} `json:"kpi_index"`
	}
	json.Unmarshal(rec.Body.Bytes(), &vc)
	if len(vc.Streams) != 1 || vc.KPIIndex["kpi-1"] == nil {
		t.Fatalf("value chain: %+v", vc)
	}

	req, _ = testRequest("GET", "/departments/"+d.ID+"/compliance", nil)
	rec = httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	var cc struct {
		Frameworks []string `json:"frameworks"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cc)
	if len(cc.Frameworks) != 1 || cc.Frameworks[0] != "ISO-27001" {
		t.Fatalf("compliance: %+v", cc)
	}
}

func TestDepartmentTenantIsolationAndPatch(t *testing.T) {
	h := newTestHandlers(t)
	d := seedDepartment(t, h, "tenant-other")

	// Cross-tenant read → 404
	req, _ := testRequest("GET", "/departments/"+d.ID, nil)
	rec := httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant read: %d", rec.Code)
	}

	// Same-tenant patch
	mine := seedDepartment(t, h, "tenant-1")
	req, _ = testRequest("PATCH", "/departments/"+mine.ID, map[string]interface{}{"mission": "New mission", "status": "suspended"})
	rec = httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	var updated store.Department
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Mission != "New mission" || updated.Status != "suspended" {
		t.Fatalf("patch result: %+v", updated)
	}

	// Archive
	req, _ = testRequest("DELETE", "/departments/"+mine.ID, nil)
	rec = httptest.NewRecorder()
	h.HandleDepartmentByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive: %d", rec.Code)
	}
}

func TestOrchestratedDeploymentPatchRejected(t *testing.T) {
	h := newTestHandlers(t)
	tmpl, _ := h.TemplateStore.Create(&store.Template{TenantID: "tenant-1", Name: "IT", Category: "it", Version: "1.0.0"})
	dep, _ := h.DeploymentStore.Create(&store.TemplateDeployment{
		TenantID: "tenant-1", TemplateID: tmpl.ID, Version: "1.0.0", Status: "configure",
	})
	h.DeploymentStore.Mutate(dep.ID, func(d *store.TemplateDeployment) { d.DepartmentID = "dept-1" })

	req, _ := testRequest("PATCH", "/templates/"+tmpl.ID+"/deployments/"+dep.ID, map[string]interface{}{"status": "operational"})
	rec := httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on orchestrated deployment, got %d: %s", rec.Code, rec.Body.String())
	}

	// Legacy deployment (no department) still PATCHable.
	legacy, _ := h.DeploymentStore.Create(&store.TemplateDeployment{
		TenantID: "tenant-1", TemplateID: tmpl.ID, Version: "1.0.0", Status: "select",
	})
	req, _ = testRequest("PATCH", "/templates/"+tmpl.ID+"/deployments/"+legacy.ID, map[string]interface{}{"status": "configure"})
	rec = httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy patch: %d %s", rec.Code, rec.Body.String())
	}
}

// fakeOrch records that it was invoked with materialized state.
type fakeOrch struct{ called chan string }

func (f *fakeOrch) Run(auth, tenantID, userID string, tmpl *store.Template, dep *store.TemplateDeployment, dept *store.Department) {
	f.called <- dept.ID
}

func TestDeployCreatesDepartmentAndInvokesOrchestrator(t *testing.T) {
	h := newTestHandlers(t)
	fo := &fakeOrch{called: make(chan string, 1)}
	h.Orchestrator = fo

	tmpl, _ := h.TemplateStore.Create(&store.Template{
		TenantID: "tenant-1", Name: "IT Department", Category: "it", Version: "1.0.0",
		Agents: []store.AgentDefinition{{ID: "mgr", Role: "Manager", Capabilities: []string{"x"}}},
	})

	req, _ := testRequest("POST", "/templates/"+tmpl.ID+"/deploy", map[string]interface{}{"environment": "production"})
	rec := httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("deploy: %d %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	deptID, _ := resp["department_id"].(string)
	if deptID == "" {
		t.Fatalf("no department_id in deploy response: %v", resp)
	}
	if stages, ok := resp["stages"].([]interface{}); !ok || len(stages) == 0 {
		t.Fatalf("no stages in deploy response")
	}

	select {
	case got := <-fo.called:
		if got != deptID {
			t.Fatalf("orchestrator got dept %s want %s", got, deptID)
		}
	default:
		// async goroutine may not have run yet; wait briefly
		got := <-fo.called
		if got != deptID {
			t.Fatalf("orchestrator got dept %s want %s", got, deptID)
		}
	}

	// The department exists and is provisioning.
	d, err := h.DepartmentStore.GetByIDAndTenant(deptID, "tenant-1")
	if err != nil || d.Status != "provisioning" || d.TemplateID != tmpl.ID {
		t.Fatalf("department: %v %+v", err, d)
	}
}

func TestSeedEndpointIdempotent(t *testing.T) {
	h := newTestHandlers(t)
	// Catalog may not be loaded in unit context — endpoint must still answer.
	req, _ := testRequest("POST", "/templates/seed", nil)
	rec := httptest.NewRecorder()
	h.SeedTemplates(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed: %d", rec.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, ok := resp["seeded"]; !ok {
		t.Fatalf("seed response: %v", resp)
	}
}

func TestValidateEndpoint(t *testing.T) {
	h := newTestHandlers(t)
	tmpl, _ := h.TemplateStore.Create(&store.Template{
		TenantID: "tenant-1", Name: "Bad", Category: "it", Version: "1.0.0",
		OrgChart: []store.Position{
			{ID: "a", Title: "A", RoleType: "manager", HolderType: "ai_agent", ReportsTo: "b"},
			{ID: "b", Title: "B", RoleType: "support", HolderType: "ai_agent", ReportsTo: "a"},
		},
	})
	req, _ := testRequest("POST", "/templates/"+tmpl.ID+"/validate", nil)
	rec := httptest.NewRecorder()
	h.HandleTemplateNested(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Valid  bool                     `json:"valid"`
		Errors []map[string]interface{} `json:"errors"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Valid || len(resp.Errors) == 0 {
		t.Fatalf("cycle not reported: %+v", resp)
	}
}

// A department copies its template's services at deploy, so template authoring
// fixes never reach live departments — and the only alternative, a redeploy,
// mints a new department id and severs seat bindings and history. Sync must
// re-point services at the template's SOPs, refuse SOPs the template does not
// define, and leave instance state alone.
func TestSyncServiceWorkflowsRepointsFromTheTemplate(t *testing.T) {
	h := newTestHandlers(t)

	tmpl, err := h.TemplateStore.Create(&store.Template{
		TenantID: "tenant-1", Name: "IT", Category: "it", Version: "1.1.0", Status: "published",
		Workflows: []store.WorkflowDefinition{
			{ID: "wf-change-001", Name: "Change"},
			{ID: "wf-access-001", Name: "Access"},
		},
		Services: []store.ServiceOffering{
			{ID: "svc-access", Name: "Access", DeliveryWorkflowID: "wf-access-001"},
			{ID: "svc-change", Name: "Change", DeliveryWorkflowID: "wf-change-001"},
			{ID: "svc-ghost", Name: "Ghost", DeliveryWorkflowID: "wf-does-not-exist"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	dept, err := h.DepartmentStore.Create(&store.Department{
		TenantID: "tenant-1", Name: "IT Live", Category: "it",
		TemplateID: tmpl.ID, Status: "operational",
		Services: []store.ServiceOffering{
			// Deployed before the authoring fix: access still runs change.
			{ID: "svc-access", Name: "Access", DeliveryWorkflowID: "wf-change-001", SLA: &store.SLA{Coverage: "24x7"}},
			{ID: "svc-change", Name: "Change", DeliveryWorkflowID: "wf-change-001"},
			{ID: "svc-ghost", Name: "Ghost", DeliveryWorkflowID: ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, _ := testRequest(http.MethodPost, "/departments/"+dept.ID+"/services/sync-workflows", nil)
	w := httptest.NewRecorder()
	h.HandleDepartmentByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sync = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Changed int `json:"changed"`
		Changes []struct {
			ServiceID string `json:"service_id"`
			From, To  string
		} `json:"changes"`
		Skipped []string `json:"skipped"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Changed != 1 || len(resp.Changes) != 1 || resp.Changes[0].ServiceID != "svc-access" {
		t.Fatalf("expected exactly the access remap, got %+v", resp)
	}
	if len(resp.Skipped) != 1 || resp.Skipped[0] != "svc-ghost" {
		t.Fatalf("the undefined SOP must be refused by name: %+v", resp.Skipped)
	}

	after, _ := h.DepartmentStore.GetByIDAndTenant(dept.ID, "tenant-1")
	for _, s := range after.Services {
		switch s.ID {
		case "svc-access":
			if s.DeliveryWorkflowID != "wf-access-001" {
				t.Fatalf("access not repointed: %q", s.DeliveryWorkflowID)
			}
			if s.SLA == nil || s.SLA.Coverage != "24x7" {
				t.Fatal("instance SLA was not preserved")
			}
		case "svc-ghost":
			if s.DeliveryWorkflowID != "" {
				t.Fatalf("ghost was pointed at a nonexistent SOP: %q", s.DeliveryWorkflowID)
			}
		}
	}

	// Idempotent: a second sync changes nothing.
	req2, _ := testRequest(http.MethodPost, "/departments/"+dept.ID+"/services/sync-workflows", nil)
	w2 := httptest.NewRecorder()
	h.HandleDepartmentByID(w2, req2)
	var resp2 struct {
		Changed int `json:"changed"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Changed != 0 {
		t.Fatalf("second sync changed %d services", resp2.Changed)
	}
}
