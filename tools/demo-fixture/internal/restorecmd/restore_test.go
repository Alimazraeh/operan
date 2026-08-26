package restorecmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/tools/demo-fixture/internal/fixture"
)

func sampleFixture() *fixture.Fixture {
	return &fixture.Fixture{
		SchemaVersion: fixture.SchemaVersion,
		Metadata:      fixture.Metadata{Name: "smoke-tenant-demo", Provenance: fixture.ProvenanceLiveExport, ExportedAt: "2026-07-27T00:00:00Z"},
		Tenant: fixture.Tenant{
			Name: "smoke-tenant", Plan: "saas", Region: "me-east-1", IsolationLevel: "namespace",
		},
		Users: []fixture.User{
			{Ref: "dana", Email: "dana@adri.nz", DisplayName: "Dana Q", RoleIDs: []string{"department_head"}},
		},
		Agents: []fixture.Agent{
			{Ref: "triage-agent", ID: "3a0c0c3c-c849-4b74-883a-9ccf85b14b5c", Name: "Triage Agent", Role: "specialist"},
		},
		Department: fixture.Department{
			TemplateID: "it-medium-001", Name: "IT Department", Environment: "production",
			SyncWorkflows: true,
			SeatBindings: []fixture.SeatBinding{
				{PositionID: "pos-it-manager-01", HolderType: "human", UserRef: "dana"},
				{PositionID: "pos-triage-01", HolderType: "ai_agent", AgentRef: "triage-agent"},
			},
		},
		Replay: &fixture.ReplaySpec{
			ServiceID: "svc-1", Title: "Grant replay-test read access", Priority: "normal", ApproverRef: "dana",
		},
	}
}

func testConfig(cfg *mockPlatform, dryRun bool, out *strings.Builder) Config {
	return Config{
		TenantControlPlaneURL: cfg.URL(),
		IdentityAccessURL:     cfg.URL(),
		AgentRegistryURL:      cfg.URL(),
		DepartmentsURL:        cfg.URL(),
		HumanSupervisionURL:   cfg.URL(),
		AdminPassword:         "operan-admin-2026",
		UserPassword:          "dana-operan-2026!",
		DryRun:                dryRun,
		Out:                   out,
	}
}

func TestProvisionCreatesEverythingOnFirstRun(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	var out strings.Builder
	cfg := testConfig(mp, false, &out)
	f := sampleFixture()

	res, err := Provision(context.Background(), cfg, f, NewClients(cfg))
	if err != nil {
		t.Fatalf("Provision: %v\nlog:\n%s", err, out.String())
	}

	if !res.Tenant.Created {
		t.Error("expected Tenant.Created = true on first run")
	}
	if len(res.Users) != 1 || !res.Users[0].Created {
		t.Errorf("expected the one user to be created, got %+v", res.Users)
	}
	if len(res.Agents) != 1 || !res.Agents[0].Created {
		t.Errorf("expected the one agent to be created, got %+v", res.Agents)
	}
	if !res.Department.Created {
		t.Error("expected Department.Created = true on first run")
	}
	if res.SeatBindingsSet != 2 {
		t.Errorf("SeatBindingsSet = %d, want 2", res.SeatBindingsSet)
	}
	if !res.WorkflowsSynced {
		t.Error("expected WorkflowsSynced = true")
	}

	if mp.CreateTenantCalls != 1 || mp.CreateUserCalls != 1 || mp.CreateAgentCalls != 1 || mp.DeployCalls != 1 {
		t.Errorf("server-side create call counts = tenant:%d user:%d agent:%d deploy:%d, want all 1",
			mp.CreateTenantCalls, mp.CreateUserCalls, mp.CreateAgentCalls, mp.DeployCalls)
	}
	if mp.SetHolderCalls != 2 {
		t.Errorf("SetHolderCalls = %d, want 2", mp.SetHolderCalls)
	}
	if mp.SyncWorkflowCalls != 1 {
		t.Errorf("SyncWorkflowCalls = %d, want 1", mp.SyncWorkflowCalls)
	}
}

// TestProvisionIsIdempotent is the test this work order is really about:
// running restore twice must leave the same state, not duplicate it. It
// asserts this the only convincing way — by counting how many times the
// mock server's create endpoints actually fired across BOTH runs, not just
// by checking the second Run() didn't return an error.
func TestProvisionIsIdempotent(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()

	var out1 strings.Builder
	cfg1 := testConfig(mp, false, &out1)
	res1, err := Provision(context.Background(), cfg1, f, NewClients(cfg1))
	if err != nil {
		t.Fatalf("first Provision: %v\nlog:\n%s", err, out1.String())
	}

	var out2 strings.Builder
	cfg2 := testConfig(mp, false, &out2)
	res2, err := Provision(context.Background(), cfg2, f, NewClients(cfg2))
	if err != nil {
		t.Fatalf("second Provision: %v\nlog:\n%s", err, out2.String())
	}

	// The defining assertion is END STATE, not call count: two Provision
	// runs must leave exactly one of each resource, never two. Call counts
	// alone would mislead here — Module 04's agent creation is idempotent
	// via its OWN 409-on-conflict response (this tool calls CreateAgent
	// unconditionally both runs and relies on that), whereas tenant/user/
	// department creation is idempotent because THIS tool finds-before-
	// creating (those APIs offer no server-side conflict signal to lean
	// on — see apiclient's doc comments). Both are legitimate idempotency
	// mechanisms; what matters is neither leaves a duplicate behind.
	mp.mu.Lock()
	tenantCount := len(mp.tenants)
	userCount := 0
	for _, u := range mp.users {
		if u.Email == "dana@adri.nz" {
			userCount++
		}
	}
	agentCount := len(mp.agents)
	deptCount := 0
	for _, d := range mp.departments {
		if d.TemplateID == "it-medium-001" {
			deptCount++
		}
	}
	mp.mu.Unlock()

	if tenantCount != 1 {
		t.Errorf("tenants stored = %d after 2 runs, want 1 (tenant must not be duplicated)", tenantCount)
	}
	if userCount != 1 {
		t.Errorf("users with dana's email stored = %d after 2 runs, want 1 (user must not be duplicated)", userCount)
	}
	if agentCount != 1 {
		t.Errorf("agents stored = %d after 2 runs, want 1 (agent must not be duplicated)", agentCount)
	}
	if deptCount != 1 {
		t.Errorf("departments for the template stored = %d after 2 runs, want 1 (department must not be re-deployed)", deptCount)
	}
	// CreateAgentCalls IS expected to be 2 (one per run) — see comment
	// above; that is Module 04's conflict response doing its job, not a
	// duplication bug. Confirm the second call really did conflict rather
	// than silently double-creating for some other reason.
	if mp.CreateAgentCalls != 2 {
		t.Errorf("CreateAgentCalls = %d, want 2 (this tool calls create every run and lets M04's 409 make it idempotent)", mp.CreateAgentCalls)
	}
	// Seat binding and sync-workflows use naturally idempotent verbs and are
	// EXPECTED to fire every run — that is not a duplication bug.
	if mp.SetHolderCalls != 4 {
		t.Errorf("SetHolderCalls = %d, want 4 (2 seats x 2 runs, each a safe PUT)", mp.SetHolderCalls)
	}

	// The second run must report everything as reused, not created.
	if res2.Tenant.Created {
		t.Error("second run: Tenant.Created = true, want false (reused)")
	}
	if res2.Users[0].Created {
		t.Error("second run: Users[0].Created = true, want false (reused)")
	}
	if res2.Agents[0].Created {
		t.Error("second run: Agents[0].Created = true, want false (reused)")
	}
	if res2.Department.Created {
		t.Error("second run: Department.Created = true, want false (reused)")
	}

	// And it must be the SAME resource, not a different one with the same
	// name — ids must match across runs.
	if res1.Tenant.ID != res2.Tenant.ID {
		t.Errorf("tenant id changed across runs: %s vs %s", res1.Tenant.ID, res2.Tenant.ID)
	}
	if res1.Department.ID != res2.Department.ID {
		t.Errorf("department id changed across runs: %s vs %s", res1.Department.ID, res2.Department.ID)
	}
	if res1.Users[0].ID != res2.Users[0].ID {
		t.Errorf("user id changed across runs: %s vs %s", res1.Users[0].ID, res2.Users[0].ID)
	}
}

func TestProvisionDryRunMakesZeroNetworkCalls(t *testing.T) {
	var requestsSeen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsSeen++
		t.Errorf("dry-run made a real HTTP call: %s %s — dry-run must make ZERO network calls", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	var out strings.Builder
	cfg := Config{
		TenantControlPlaneURL: srv.URL, IdentityAccessURL: srv.URL, AgentRegistryURL: srv.URL,
		DepartmentsURL: srv.URL, HumanSupervisionURL: srv.URL,
		AdminPassword: "operan-admin-2026", UserPassword: "x", DryRun: true, Out: &out,
	}
	f := sampleFixture()

	res, err := Provision(context.Background(), cfg, f, NewClients(cfg))
	if err != nil {
		t.Fatalf("Provision (dry-run): unexpected error: %v", err)
	}
	if requestsSeen != 0 {
		t.Fatalf("dry-run triggered %d real HTTP request(s)", requestsSeen)
	}
	if !res.DryRun {
		t.Error("Result.DryRun = false, want true")
	}

	// The plan must still be informative — spot check a few expected lines.
	plan := out.String()
	for _, want := range []string{
		"POST " + srv.URL + "/api/v1/iam/admin/login",
		"POST " + srv.URL + "/v1/tenants",
		"POST " + srv.URL + "/api/v1/iam/users",
		"POST " + srv.URL + "/registry/agents",
		"POST " + srv.URL + "/templates/it-medium-001/deploy",
		"holder",
		"sync-workflows",
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("dry-run plan output missing expected mention of %q; full output:\n%s", want, plan)
		}
	}
}

func TestProvisionFailsClearlyWhenAdminPasswordMissing(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	var out strings.Builder
	cfg := testConfig(mp, false, &out)
	cfg.AdminPassword = ""
	f := sampleFixture()

	_, err := Provision(context.Background(), cfg, f, NewClients(cfg))
	if err == nil {
		t.Fatal("Provision: expected an error when AdminPassword is empty, got nil")
	}
	if !strings.Contains(err.Error(), "admin-password") {
		t.Errorf("error should mention the missing admin password, got: %v", err)
	}
}

func TestProvisionSurfacesHandAssembledProvenanceWarning(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	var out strings.Builder
	cfg := testConfig(mp, false, &out)
	f := sampleFixture()
	f.Metadata.Provenance = fixture.ProvenanceHandAssembled

	_, err := Provision(context.Background(), cfg, f, NewClients(cfg))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if !strings.Contains(out.String(), "hand-assembled") {
		t.Errorf("expected a provenance warning in the log, got:\n%s", out.String())
	}
}
