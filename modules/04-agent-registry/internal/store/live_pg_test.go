package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/operan/modules/04-agent-registry/internal/database"
)

// Exercises the real SQL against a real PostgreSQL. Skipped unless
// M04_TEST_DSN is set, so it never blocks CI.
func TestLiveRoundTripAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("M04_TEST_DSN")
	if dsn == "" {
		t.Skip("M04_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, dsn, 5)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Migrate must be idempotent — it runs at every boot.
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate (idempotency): %v", err)
	}
	db := database.NewAgentStore(pool)

	tctx := ctxWithTenant(ctx, "live-test-tenant")
	dept := "dept-live"
	agents := NewAgentStore()
	agents.Persist(db)
	a := &Agent{
		ID: "live-agent-1", TenantID: "live-test-tenant", Name: "Live Triage",
		Role: "specialist", Description: "d", DepartmentID: &dept,
		Objectives:         []Objective{{Description: "fast", Metric: "mttr", Weight: 0.5}},
		Capabilities:       []string{"triage"},
		Tools:              []string{"itsm.ticket.assign"},
		RuntimeConstraints: &RuntimeConstraints{MaxConcurrent: 3, ResourceQuota: &ResourceQuota{MemoryMB: 256}},
		ExecutionBudget:    &ExecutionBudget{MonthlyBudgetUSD: 99},
	}
	if err := agents.Create(tctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := agents.Patch(tctx, a.ID, func(x *Agent) { x.Name = "Live Triage v2" }); err != nil {
		t.Fatalf("patch: %v", err)
	}

	versions := NewVersionStore()
	versions.Persist(db)
	v := &AgentVersion{
		ID: "live-version-1", AgentID: a.ID, TenantID: "live-test-tenant",
		Version: "1.0.0", Status: VersionStatusActive, CreatedBy: "test",
		ModelConfig: map[string]any{"model": "qwen3.6-35b", "temperature": 0.2},
		PromotedTo:  map[string]string{"production": "live-version-1"},
		CreatedAt:   time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := versions.Create(tctx, v); err != nil {
		t.Fatalf("create version: %v", err)
	}

	// A fresh process: new empty stores, hydrated from the database only.
	fresh := NewAgentStore()
	nA, err := fresh.HydrateAgents(ctx, db)
	if err != nil {
		t.Fatalf("hydrate agents: %v", err)
	}
	got, err := fresh.GetByID(tctx, a.ID)
	if err != nil {
		t.Fatalf("agent did not survive: %v (loaded %d)", err, nA)
	}
	if got.Name != "Live Triage v2" {
		t.Fatalf("patch not persisted: name = %q", got.Name)
	}
	if got.DepartmentID == nil || *got.DepartmentID != dept {
		t.Fatalf("department lost: %v", got.DepartmentID)
	}
	if got.RuntimeConstraints == nil || got.RuntimeConstraints.ResourceQuota == nil ||
		got.RuntimeConstraints.ResourceQuota.MemoryMB != 256 {
		t.Fatalf("nested detail lost: %+v", got.RuntimeConstraints)
	}
	if got.ExecutionBudget == nil || got.ExecutionBudget.MonthlyBudgetUSD != 99 {
		t.Fatalf("budget lost: %+v", got.ExecutionBudget)
	}
	if len(got.Objectives) != 1 || got.Objectives[0].Metric != "mttr" {
		t.Fatalf("objectives lost: %+v", got.Objectives)
	}

	freshV := NewVersionStore()
	if _, err := freshV.HydrateVersions(ctx, db); err != nil {
		t.Fatalf("hydrate versions: %v", err)
	}
	gotV, err := freshV.GetByID(tctx, v.ID)
	if err != nil {
		t.Fatalf("version did not survive: %v", err)
	}
	if gotV.ModelConfig["model"] != "qwen3.6-35b" {
		t.Fatalf("model_config lost: %+v", gotV.ModelConfig)
	}
	if gotV.PromotedTo["production"] != "live-version-1" {
		t.Fatalf("promoted_to lost: %+v", gotV.PromotedTo)
	}

	// Delete must remove the row and its versions, or a restart resurrects it.
	if err := agents.Delete(tctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	after := NewAgentStore()
	if _, err := after.HydrateAgents(ctx, db); err != nil {
		t.Fatalf("hydrate after delete: %v", err)
	}
	if _, err := after.GetByID(tctx, a.ID); err == nil {
		t.Fatal("deleted agent came back after rehydration")
	}
	afterV := NewVersionStore()
	if _, err := afterV.HydrateVersions(ctx, db); err != nil {
		t.Fatalf("hydrate versions after delete: %v", err)
	}
	if _, err := afterV.GetByID(tctx, v.ID); err == nil {
		t.Fatal("versions of a deleted agent survived")
	}
}
