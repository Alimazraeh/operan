package deploy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/clients"
	"github.com/operan/modules/05-department-template-engine/internal/events"
	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func tmplFixture() *store.Template {
	return &store.Template{
		ID: "tmpl-1", TenantID: "t1", Name: "IT Department", Category: "it", Version: "1.0.0",
		Agents: []store.AgentDefinition{
			{ID: "mgr", Role: "IT Manager", Name: "IT Manager", AutonomyTier: "coordinate",
				Capabilities: []string{"coordination"}, Services: []string{"svc-desk"}},
			{ID: "sd", Role: "Service Desk", Name: "Service Desk", ReportsTo: "mgr", AutonomyTier: "draft",
				EscalationPath: []string{"mgr", "human"}},
		},
		OrgChart: []store.Position{
			{ID: "pos-mgr", Title: "IT Manager", RoleType: "manager", HolderType: "ai_agent", AgentDefID: "mgr"},
			{ID: "pos-sd", Title: "Service Desk", RoleType: "support", HolderType: "ai_agent", AgentDefID: "sd", ReportsTo: "pos-mgr"},
		},
		Services: []store.ServiceOffering{{ID: "svc-desk", Name: "Service Desk",
			SLA: &store.SLA{ResponseTime: "15m", Coverage: "24x7"}, Consumers: []string{"all-employees"}}},
		KPIS:         []store.KPIDefinition{{ID: "kpi-1", Name: "MTTR", MetricType: "timer"}},
		Integrations: []store.IntegrationDefinition{{ID: "ig-1", Type: "messaging", Name: "Slack", Config: map[string]interface{}{}}},
	}
}

type fixture struct {
	orch        *Orchestrator
	deployments *store.DeploymentStore
	departments *store.DepartmentStore
	registryHit *int64
	memoryHit   *int64
	authSeen    *atomic.Value
}

func newFixture(t *testing.T, registryFail bool) (*fixture, func()) {
	var regHits, memHits int64
	var authSeen atomic.Value

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSeen.Store(r.Header.Get("Authorization") + "|" + r.Header.Get("X-Tenant-ID"))
		if r.Method == http.MethodGet { // pre-flight ping
			w.Write([]byte(`{"items":[]}`))
			return
		}
		atomic.AddInt64(&regHits, 1)
		if registryFail {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
			return
		}
		var req clients.CreateAgentRequest
		json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"id": fmt.Sprintf("m04-%d", atomic.LoadInt64(&regHits)), "name": req.Name, "role": req.Role,
		})
	}))
	memory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&memHits, 1)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"stored":true}`))
	}))

	f := &fixture{
		deployments: store.NewDeploymentStore(),
		departments: store.NewDepartmentStore(),
		registryHit: &regHits,
		memoryHit:   &memHits,
		authSeen:    &authSeen,
	}
	f.orch = &Orchestrator{
		Deployments: f.deployments,
		Departments: f.departments,
		Publisher:   events.NewPublisher(),
		Registry:    &clients.RegistryClient{BaseURL: registry.URL},
		Memory:      &clients.MemoryClient{BaseURL: memory.URL},
		Timeout:     10 * time.Second,
	}
	return f, func() { registry.Close(); memory.Close() }
}

func runDeploy(t *testing.T, f *fixture, tmpl *store.Template) (*store.TemplateDeployment, *store.Department) {
	now := time.Now()
	dep, err := f.deployments.Create(&store.TemplateDeployment{
		TenantID: "t1", TemplateID: tmpl.ID, Version: "1.0.0", Status: "select",
		Environment: "production", StartedAt: &now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	dept := MaterializeDepartment(tmpl, dep, "", "user-1")
	created, err := f.departments.Create(dept)
	if err != nil {
		t.Fatalf("create department: %v", err)
	}
	dept.ID = created.ID
	f.deployments.Mutate(dep.ID, func(d *store.TemplateDeployment) { d.DepartmentID = dept.ID })

	f.orch.Run("Bearer tok", "t1", "user-1", tmpl, dep, dept) // synchronous in tests
	final, _ := f.deployments.GetByIDAndTenant(dep.ID, "t1")
	finalDept, _ := f.departments.GetByIDAndTenant(dept.ID, "t1")
	return final, finalDept
}

func TestDeployHappyPath(t *testing.T) {
	f, cleanup := newFixture(t, false)
	defer cleanup()

	dep, dept := runDeploy(t, f, tmplFixture())

	if dep.Status != "operational" {
		t.Fatalf("deployment status = %s (%s)", dep.Status, dep.ErrorMessage)
	}
	if dept.Status != "operational" {
		t.Fatalf("department status = %s", dept.Status)
	}
	if len(dept.AgentIDs) != 2 {
		t.Fatalf("agent ids = %v", dept.AgentIDs)
	}
	// Org chart positions backfilled with real M04 ids.
	for _, p := range dept.OrgChart {
		if p.AgentID == "" {
			t.Errorf("position %s has no provisioned agent id", p.ID)
		}
	}
	// Memory provisioned: charter + 1 service doc.
	if len(dept.MemoryRefs) != 2 {
		t.Fatalf("memory refs = %v", dept.MemoryRefs)
	}
	// Provisioned entities recorded.
	if dep.ProvisionedEntities["department_id"] != dept.ID {
		t.Fatalf("provisioned_entities: %+v", dep.ProvisionedEntities)
	}
	agents, _ := dep.ProvisionedEntities["agents"].([]map[string]interface{})
	if len(agents) != 2 {
		// After store round-trips this may be []interface{}; accept either.
		if arr, ok := dep.ProvisionedEntities["agents"].([]interface{}); !ok || len(arr) != 2 {
			t.Fatalf("provisioned agents: %#v", dep.ProvisionedEntities["agents"])
		}
	}
	// All six stages recorded, all completed.
	if len(dep.Stages) < 5 {
		t.Fatalf("stages: %+v", dep.Stages)
	}
	for _, s := range dep.Stages {
		if s.Status != "completed" {
			t.Errorf("stage %s = %s", s.Stage, s.Status)
		}
	}
	// Caller credentials forwarded.
	if got := f.authSeen.Load().(string); got != "Bearer tok|t1" {
		t.Fatalf("forwarded auth = %q", got)
	}
}

func TestDeployRegistryFailureDegrades(t *testing.T) {
	f, cleanup := newFixture(t, true)
	defer cleanup()

	dep, dept := runDeploy(t, f, tmplFixture())

	if dep.Status != "failed" || dep.ErrorMessage == "" {
		t.Fatalf("deployment: %s / %q", dep.Status, dep.ErrorMessage)
	}
	if dept.Status != "degraded" {
		t.Fatalf("department status = %s", dept.Status)
	}
}

func TestDeployInvalidTemplateFailsAtConfigure(t *testing.T) {
	f, cleanup := newFixture(t, false)
	defer cleanup()

	tmpl := tmplFixture()
	tmpl.OrgChart[0].ReportsTo = "pos-sd" // cycle → validation error

	dep, dept := runDeploy(t, f, tmpl)
	if dep.Status != "failed" {
		t.Fatalf("deployment status = %s", dep.Status)
	}
	if dept.Status != "degraded" {
		t.Fatalf("department status = %s", dept.Status)
	}
	if atomic.LoadInt64(f.registryHit) != 0 {
		t.Fatal("registry must not be called when validation fails")
	}
}

func TestMaterializeDepartment(t *testing.T) {
	tmpl := tmplFixture()
	tmpl.BusinessLogic = &store.BusinessLogic{Purpose: "Keep the lights on"}
	dep := &store.TemplateDeployment{ID: "dep-1", TenantID: "t1", Environment: "production"}

	dept := MaterializeDepartment(tmpl, dep, "", "u1")
	if dept.Name != "IT Department" || dept.Slug != "it-department" {
		t.Fatalf("name/slug: %s/%s", dept.Name, dept.Slug)
	}
	if dept.Mission != "Keep the lights on" {
		t.Fatalf("mission: %s", dept.Mission)
	}
	if len(dept.OrgChart) != 2 || len(dept.Services) != 1 || len(dept.KPIS) != 1 {
		t.Fatalf("materialized sections incomplete")
	}
	// Custom name override
	named := MaterializeDepartment(tmpl, dep, "Riyadh IT", "u1")
	if named.Name != "Riyadh IT" || named.Slug != "riyadh-it" {
		t.Fatalf("override: %s/%s", named.Name, named.Slug)
	}
}

// Step-type mapping is what decides whether a catalogue step does real work
// or silently passes through, so it is asserted directly.
func TestNodeTypeForMapping(t *testing.T) {
	for step, want := range map[string]string{
		"agent_call": "agent",
		// transformation carries {agent, task} like agent_call — it is agent
		// work, not a pass-through.
		"transformation": "agent",
		"approval":       "human_gate",
		"human_gate":     "human_gate",
		"conditional":    "condition",
		// Still honest pass-throughs until the capability layer binds them.
		"tool_call":    "action",
		"notification": "action",
		"unknown_type": "action",
	} {
		if got := nodeTypeFor(step); got != want {
			t.Errorf("nodeTypeFor(%q) = %q, want %q", step, got, want)
		}
	}
}
