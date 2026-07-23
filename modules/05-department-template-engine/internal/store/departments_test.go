package store

import (
	"testing"
)

func deptFixture(tenant string) *Department {
	return &Department{
		TenantID: tenant,
		Name:     "IT Department",
		Slug:     "it-department",
		Category: "it",
		Mission:  "Keep everyone productive",
		OrgChart: []Position{
			{ID: "pos-mgr", Title: "IT Manager", RoleType: "manager", HolderType: "ai_agent", AgentDefID: "it-mgr-01", AutonomyTier: "coordinate"},
			{ID: "pos-sd", Title: "Service Desk", RoleType: "support", HolderType: "ai_agent", AgentDefID: "sd-01", ReportsTo: "pos-mgr", AutonomyTier: "draft"},
		},
		Services: []ServiceOffering{{ID: "svc-desk", Name: "Service Desk", SLA: &SLA{ResponseTime: "15m"}}},
		Risks:    []RiskItem{{ID: "r1", Name: "Backup failure", Severity: "high", Likelihood: "unlikely"}},
	}
}

func TestDepartmentCRUDAndTenantIsolation(t *testing.T) {
	s := NewDepartmentStore()

	created, err := s.Create(deptFixture("t1"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || created.Status != "provisioning" {
		t.Fatalf("bad defaults: %+v", created)
	}

	// Tenant isolation on read
	if _, err := s.GetByIDAndTenant(created.ID, "t2"); err == nil {
		t.Fatal("cross-tenant read must fail")
	}
	got, err := s.GetByIDAndTenant(created.ID, "t1")
	if err != nil || got.Name != "IT Department" {
		t.Fatalf("get: %v %+v", err, got)
	}

	// List with filters
	s.Create(deptFixture("t1"))
	other := deptFixture("t1")
	other.Category = "finance"
	s.Create(other)
	cat := "it"
	items, total, _ := s.List("t1", 1, 10, &cat, nil)
	if total != 2 || len(items) != 2 {
		t.Fatalf("category filter: total=%d len=%d", total, len(items))
	}
	empty := ""
	all, total, _ := s.List("t1", 1, 10, &empty, &empty)
	if total != 3 || len(all) != 3 {
		t.Fatalf("list all: total=%d", total)
	}

	// Patch: status + services replaced wholesale
	updated, err := s.UpdateByTenant(created.ID, "t1", map[string]interface{}{
		"status":  "operational",
		"mission": "New mission",
		"services": []interface{}{
			map[string]interface{}{"id": "svc-2", "name": "Patching"},
		},
	})
	if err != nil || updated.Status != "operational" || updated.Mission != "New mission" {
		t.Fatalf("update: %v %+v", err, updated)
	}
	if len(updated.Services) != 1 || updated.Services[0].ID != "svc-2" {
		t.Fatalf("services not replaced: %+v", updated.Services)
	}

	// Cross-tenant patch fails
	if _, err := s.UpdateByTenant(created.ID, "t2", map[string]interface{}{"status": "suspended"}); err == nil {
		t.Fatal("cross-tenant update must fail")
	}

	// Archive
	archived, err := s.Archive(created.ID, "t1")
	if err != nil || archived.Status != "archived" {
		t.Fatalf("archive: %v %+v", err, archived)
	}
}

func TestDepartmentReplaceIsDeepCopy(t *testing.T) {
	s := NewDepartmentStore()
	created, _ := s.Create(deptFixture("t1"))

	// Simulate the orchestrator owning its instance
	own := deptFixture("t1")
	own.ID = created.ID
	own.AgentIDs = []string{"a1"}
	if _, err := s.Replace(own); err != nil {
		t.Fatalf("replace: %v", err)
	}
	// Mutate the orchestrator's copy AFTER replace; store must not see it.
	own.AgentIDs = append(own.AgentIDs, "a2")
	own.OrgChart[0].AgentID = "mutated"

	got, _ := s.GetByIDAndTenant(created.ID, "t1")
	if len(got.AgentIDs) != 1 || got.OrgChart[0].AgentID == "mutated" {
		t.Fatalf("replace shared memory with caller: %+v", got)
	}
}

func TestDepartmentExportImportRoundTrip(t *testing.T) {
	s := NewDepartmentStore()
	created, _ := s.Create(deptFixture("t1"))
	s.Create(deptFixture("t2"))

	data, err := s.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	restored := NewDepartmentStore()
	if err := restored.Import(data); err != nil {
		t.Fatalf("import: %v", err)
	}
	got, err := restored.GetByIDAndTenant(created.ID, "t1")
	if err != nil || got.Name != "IT Department" || len(got.OrgChart) != 2 {
		t.Fatalf("round-trip: %v %+v", err, got)
	}
	if _, total, _ := restored.List("t2", 1, 10, nil, nil); total != 1 {
		t.Fatalf("t2 missing after import")
	}
}

func TestAllStoresExportImport(t *testing.T) {
	// TemplateStore
	ts := NewTemplateStore()
	tmpl, _ := ts.Create(&Template{TenantID: "t1", Name: "X", Category: "it", Version: "1.0.0"})
	data, _ := ts.Export()
	ts2 := NewTemplateStore()
	if err := ts2.Import(data); err != nil {
		t.Fatalf("template import: %v", err)
	}
	if _, err := ts2.GetByIDAndTenant(tmpl.ID, "t1"); err != nil {
		t.Fatalf("template round-trip: %v", err)
	}

	// DeploymentStore (with stages + department link)
	ds := NewDeploymentStore()
	dep, _ := ds.Create(&TemplateDeployment{TenantID: "t1", TemplateID: tmpl.ID, Version: "1.0.0", Status: "select"})
	ds.Mutate(dep.ID, func(d *TemplateDeployment) {
		d.DepartmentID = "dept-1"
		d.Stages = append(d.Stages, StageRecord{Stage: "configure", Status: "completed"})
	})
	data, _ = ds.Export()
	ds2 := NewDeploymentStore()
	if err := ds2.Import(data); err != nil {
		t.Fatalf("deployment import: %v", err)
	}
	got, _ := ds2.GetByIDAndTenant(dep.ID, "t1")
	if got.DepartmentID != "dept-1" || len(got.Stages) != 1 {
		t.Fatalf("deployment round-trip: %+v", got)
	}
}
