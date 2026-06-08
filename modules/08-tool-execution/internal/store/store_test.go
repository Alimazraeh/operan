package store

import "testing"

func TestToolStore_CRUDAndList(t *testing.T) {
	s := NewToolStore()

	if _, err := s.Create(&Tool{Name: "x"}); err != ErrTenantMismatch {
		t.Errorf("Create without tenant should fail with ErrTenantMismatch, got %v", err)
	}
	if _, err := s.Create(&Tool{TenantID: "t1"}); err != ErrValidation {
		t.Errorf("Create without name should fail with ErrValidation, got %v", err)
	}

	tool, err := s.Create(&Tool{TenantID: "t1", Name: "search", Category: "knowledge"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tool.ID == "" || tool.Version != "1.0.0" || tool.Status != "active" {
		t.Errorf("defaults not applied: %+v", tool)
	}

	got, err := s.GetByIDAndTenant(tool.ID, "t1")
	if err != nil || got.Name != "search" {
		t.Fatalf("GetByIDAndTenant: %+v, %v", got, err)
	}
	if _, err := s.GetByIDAndTenant(tool.ID, "other"); err != ErrNotFound {
		t.Errorf("cross-tenant get should be ErrNotFound, got %v", err)
	}

	// second tool, different category
	_, _ = s.Create(&Tool{TenantID: "t1", Name: "email", Category: "communication"})
	cat := "knowledge"
	items, total, hasMore := s.List("t1", 1, 10, &cat, nil)
	if total != 1 || len(items) != 1 || hasMore {
		t.Errorf("category filter: total=%d items=%d hasMore=%v", total, len(items), hasMore)
	}
	all, total, _ := s.List("t1", 1, 1, nil, nil)
	if total != 2 || len(all) != 1 {
		t.Errorf("pagination: total=%d page1=%d", total, len(all))
	}

	upd, err := s.Update(tool.ID, "t1", func(x *Tool) { x.Status = "deprecated" })
	if err != nil || upd.Status != "deprecated" {
		t.Errorf("Update: %+v, %v", upd, err)
	}
	if _, err := s.Update(tool.ID, "other", func(*Tool) {}); err != ErrNotFound {
		t.Errorf("cross-tenant update should be ErrNotFound, got %v", err)
	}
}

func TestVersionStore(t *testing.T) {
	s := NewVersionStore()
	if _, err := s.Create(&ToolVersion{}); err != ErrValidation {
		t.Errorf("Create without tool_id should fail, got %v", err)
	}
	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		if _, err := s.Create(&ToolVersion{ToolID: "tool1", Version: v}); err != nil {
			t.Fatalf("Create %s: %v", v, err)
		}
	}
	items, total, hasMore := s.ListByTool("tool1", 1, 2)
	if total != 3 || len(items) != 2 || !hasMore {
		t.Errorf("ListByTool: total=%d items=%d hasMore=%v", total, len(items), hasMore)
	}
	// newest first
	if items[0].Version != "2.0.0" {
		t.Errorf("expected newest first, got %s", items[0].Version)
	}
	if _, total, _ := s.ListByTool("missing", 1, 10); total != 0 {
		t.Errorf("missing tool should have 0 versions, got %d", total)
	}
}

func TestExecutionStore_AndCost(t *testing.T) {
	s := NewExecutionStore()

	if _, err := s.Create(&ToolExecution{AgentID: "a", Tool: "t"}); err != ErrTenantMismatch {
		t.Errorf("Create without tenant should fail, got %v", err)
	}
	if _, err := s.Create(&ToolExecution{TenantID: "t1", Tool: "t"}); err != ErrValidation {
		t.Errorf("Create without agent should fail, got %v", err)
	}

	e, err := s.Create(&ToolExecution{TenantID: "t1", AgentID: "a1", Tool: "search"})
	if err != nil || e.Status != ExecQueued {
		t.Fatalf("Create: %+v, %v", e, err)
	}

	got, err := s.GetByIDAndTenant(e.ID, "t1")
	if err != nil || got.AgentID != "a1" {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if _, err := s.GetByIDAndTenant(e.ID, "other"); err != ErrNotFound {
		t.Errorf("cross-tenant should be ErrNotFound, got %v", err)
	}

	upd, _ := s.Update(e.ID, "t1", func(x *ToolExecution) {
		x.Status = ExecCompleted
		x.Cost = &CostPerCall{Amount: 0.25, Currency: "USD"}
	})
	if upd.Status != ExecCompleted {
		t.Errorf("Update status: %s", upd.Status)
	}

	// second execution with cost for aggregation
	e2, _ := s.Create(&ToolExecution{TenantID: "t1", AgentID: "a1", Tool: "search"})
	_, _ = s.Update(e2.ID, "t1", func(x *ToolExecution) { x.Cost = &CostPerCall{Amount: 0.75, Currency: "USD"} })

	status := string(ExecCompleted)
	items, total, _ := s.List("t1", 1, 10, nil, &status)
	if total != 1 || len(items) != 1 {
		t.Errorf("status filter: total=%d", total)
	}

	tool := "search"
	cost := s.AggregateCost("t1", &tool)
	if cost.TotalCalls != 2 || cost.TotalCost != 1.0 {
		t.Errorf("AggregateCost: calls=%d cost=%v", cost.TotalCalls, cost.TotalCost)
	}
}
