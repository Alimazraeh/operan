package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func tenantCtx(tenant string) context.Context {
	return ctxWithTenant(context.Background(), tenant)
}

// Pagination sliced a map iteration, so page 1 and page 2 could return the same
// agent twice and omit another entirely. Every agent must appear exactly once
// across the pages, and the order must not change between calls.
func TestListPaginationIsStableAndComplete(t *testing.T) {
	s := NewAgentStore()
	ctx := tenantCtx("smoke-tenant")
	base := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	const total = 25
	for i := 0; i < total; i++ {
		id := string(rune('a'+i%26)) + "-agent-" + time.Duration(i).String()
		// Distinct created_at for most, two identical to exercise the id tiebreak.
		at := base.Add(time.Duration(i/2) * time.Minute)
		a := &Agent{ID: id, TenantID: "smoke-tenant", Name: id, Role: "specialist", CreatedAt: at}
		if err := s.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
		// Create stamps CreatedAt itself; put the intended value back so the
		// ordering under test is the one being asserted.
		_ = s.Patch(ctx, id, func(x *Agent) { x.CreatedAt = at })
	}

	seen := map[string]int{}
	var firstPass []string
	for page := 1; page <= 3; page++ {
		got, gotTotal, err := s.List(ctx, "", "", "", page, 10)
		if err != nil {
			t.Fatal(err)
		}
		if gotTotal != total {
			t.Fatalf("page %d reported total %d, want %d", page, gotTotal, total)
		}
		for _, a := range got {
			seen[a.ID]++
			firstPass = append(firstPass, a.ID)
		}
	}
	if len(seen) != total {
		t.Fatalf("saw %d distinct agents across all pages, want %d", len(seen), total)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("agent %s appeared %d times across pages", id, n)
		}
	}

	// A second identical sweep must produce the identical sequence.
	var secondPass []string
	for page := 1; page <= 3; page++ {
		got, _, _ := s.List(ctx, "", "", "", page, 10)
		for _, a := range got {
			secondPass = append(secondPass, a.ID)
		}
	}
	for i := range firstPass {
		if firstPass[i] != secondPass[i] {
			t.Fatalf("page order changed between calls at %d: %s then %s", i, firstPass[i], secondPass[i])
		}
	}
}

// The nested value objects travel to the database as one JSON document. If that
// document cannot carry them faithfully, a restart returns agents stripped of
// their objectives, budgets and constraints — which looks like data corruption
// rather than the storage bug it is.
func TestAgentDetailSurvivesTheRoundTrip(t *testing.T) {
	dept := "dept-1"
	orig := &Agent{
		ID: "agent-1", TenantID: "smoke-tenant", Name: "Triage", Role: "specialist",
		Description: "First line", DepartmentID: &dept, Status: AgentStatusActive,
		Objectives:         []Objective{{Description: "Resolve fast", Metric: "mttr", Weight: 0.7, Tier: "draft"}},
		Capabilities:       []string{"triage", "categorise"},
		Tools:              []string{"itsm.ticket.assign"},
		MemoryAccess:       &MemoryAccess{Scope: "department", AllowedTypes: []string{"episodic"}, IsolationLevel: "strict"},
		EscalationRules:    []string{"sev1 → human"},
		GovernancePolicies: []string{"pol-1"},
		SupportedLanguages: []string{"en", "ar"},
		RuntimeConstraints: &RuntimeConstraints{MaxConcurrent: 4, MaxDurationSeconds: 300, RateLimitPerMinute: 60,
			ResourceQuota: &ResourceQuota{CPUMillicores: 500, MemoryMB: 512}},
		CostProfile:     &CostProfile{CostPerExecution: 0.02, CostPerToken: 0.000001, BudgetLimit: 100, BillingTag: "it"},
		ExecutionBudget: &ExecutionBudget{DailyTokenLimit: 100000, MaxRunSeconds: 600, MonthlyBudgetUSD: 250},
		AccessControl:   &AccessControl{Scope: "department", AllowedRoles: []string{"department_head"}},
	}

	blob, err := json.Marshal(agentDetail{
		Objectives: orig.Objectives, MemoryAccess: orig.MemoryAccess,
		EscalationRules: orig.EscalationRules, GovernancePolicies: orig.GovernancePolicies,
		SupportedLanguages: orig.SupportedLanguages, Capabilities: orig.Capabilities,
		Tools: orig.Tools, RuntimeConstraints: orig.RuntimeConstraints,
		CostProfile: orig.CostProfile, ExecutionBudget: orig.ExecutionBudget,
		AccessControl: orig.AccessControl,
	})
	if err != nil {
		t.Fatal(err)
	}
	var back agentDetail
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatal(err)
	}

	if len(back.Objectives) != 1 || back.Objectives[0].Metric != "mttr" || back.Objectives[0].Weight != 0.7 {
		t.Fatalf("objectives lost: %+v", back.Objectives)
	}
	if back.RuntimeConstraints == nil || back.RuntimeConstraints.ResourceQuota == nil ||
		back.RuntimeConstraints.ResourceQuota.MemoryMB != 512 {
		t.Fatalf("nested resource quota lost: %+v", back.RuntimeConstraints)
	}
	if back.ExecutionBudget == nil || back.ExecutionBudget.MonthlyBudgetUSD != 250 {
		t.Fatalf("execution budget lost: %+v", back.ExecutionBudget)
	}
	if back.CostProfile == nil || back.CostProfile.BillingTag != "it" {
		t.Fatalf("cost profile lost: %+v", back.CostProfile)
	}
	if back.AccessControl == nil || len(back.AccessControl.AllowedRoles) != 1 {
		t.Fatalf("access control lost: %+v", back.AccessControl)
	}
	if len(back.Capabilities) != 2 || len(back.Tools) != 1 {
		t.Fatalf("capabilities/tools lost: %v %v", back.Capabilities, back.Tools)
	}
	if len(back.SupportedLanguages) != 2 {
		t.Fatalf("languages lost: %v", back.SupportedLanguages)
	}
}

// With no database attached the stores must behave exactly as they always did.
// A nil sink that panicked would take out every deployment running in memory.
func TestStoresWorkWithNoDatabaseAttached(t *testing.T) {
	ctx := tenantCtx("smoke-tenant")
	agents := NewAgentStore()
	if err := agents.Create(ctx, &Agent{ID: "a1", TenantID: "smoke-tenant", Name: "A", Role: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := agents.Patch(ctx, "a1", func(a *Agent) { a.Name = "B" }); err != nil {
		t.Fatal(err)
	}
	if err := agents.Delete(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	versions := NewVersionStore()
	if err := versions.Create(ctx, &AgentVersion{ID: "v1", AgentID: "a1", TenantID: "smoke-tenant", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
}

// Patch must move updated_at, or the durable row keeps the original timestamp
// and there is no way to tell a stale write from a current one.
func TestPatchStampsUpdatedAt(t *testing.T) {
	ctx := tenantCtx("smoke-tenant")
	s := NewAgentStore()
	if err := s.Create(ctx, &Agent{ID: "a1", TenantID: "smoke-tenant", Name: "A", Role: "r"}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetByID(ctx, "a1")
	was := before.UpdatedAt
	time.Sleep(2 * time.Millisecond)
	if err := s.Patch(ctx, "a1", func(a *Agent) { a.Name = "B" }); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetByID(ctx, "a1")
	if !after.UpdatedAt.After(was) {
		t.Fatalf("updated_at did not move: %v then %v", was, after.UpdatedAt)
	}
}
