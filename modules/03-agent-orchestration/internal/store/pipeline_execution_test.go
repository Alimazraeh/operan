package store

import (
	"testing"
)

// ─── PipelineStore tests ─────────────────────────────────────────────────────

func TestPipelineStore_Create(t *testing.T) {
	store := NewPipelineStore()

	p := &Pipeline{
		TenantID: "tenant-1",
		Name:     "Test Pipeline",
		Status:   PipelineStatusActive,
		TriggerType: PipelineTriggerManual,
		Steps: []PipelineStep{
			{ID: "step-1", Name: "Step One", Type: PipelineStepAPI},
		},
	}

	t.Run("creates pipeline with auto-generated ID", func(t *testing.T) {
		created, err := store.Create(p)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if created.ID == "" {
			t.Error("Expected auto-generated ID")
		}
	})

	t.Run("default status is active", func(t *testing.T) {
		store := NewPipelineStore()
		p := &Pipeline{
			TenantID: "tenant-1",
			Name:     "No Status Pipeline",
			Steps:    []PipelineStep{},
		}
		created, _ := store.Create(p)
		if created.Status != PipelineStatusActive {
			t.Errorf("Expected status %s, got %s", PipelineStatusActive, created.Status)
		}
	})

	t.Run("duplicate ID returns error", func(t *testing.T) {
		store := NewPipelineStore()
		p1 := &Pipeline{ID: "dup-1", TenantID: "tenant-1", Name: "First", Steps: []PipelineStep{}}
		p2 := &Pipeline{ID: "dup-1", TenantID: "tenant-1", Name: "Second", Steps: []PipelineStep{}}

		_, err := store.Create(p1)
		if err != nil {
			t.Fatalf("First Create failed: %v", err)
		}
		_, err = store.Create(p2)
		if err == nil {
			t.Error("Expected error for duplicate ID")
		}
	})

	t.Run("tracks by tenant", func(t *testing.T) {
		store := NewPipelineStore()
		p1 := &Pipeline{TenantID: "tenant-a", Name: "Pipeline A", Steps: []PipelineStep{}}
		p2 := &Pipeline{TenantID: "tenant-b", Name: "Pipeline B", Steps: []PipelineStep{}}

		store.Create(p1)
		store.Create(p2)

		items, total, _ := store.List("tenant-a", 1, 50, nil)
		if total != 1 {
			t.Errorf("Expected 1 pipeline for tenant-a, got %d", total)
		}
		if len(items) != 1 || items[0].ID != p1.ID {
			t.Error("Expected only pipeline-a for tenant-a")
		}
	})
}

func TestPipelineStore_GetByID(t *testing.T) {
	store := NewPipelineStore()

	p := &Pipeline{
		TenantID: "tenant-1",
		Name:     "Test Pipeline",
		Steps:    []PipelineStep{{ID: "step-1", Name: "Step One", Type: PipelineStepAPI}},
	}
	created, _ := store.Create(p)

	t.Run("returns pipeline by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.Name != "Test Pipeline" {
			t.Errorf("Expected 'Test Pipeline', got %s", got.Name)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent pipeline")
		}
	})
}

func TestPipelineStore_Update(t *testing.T) {
	store := NewPipelineStore()

	p := &Pipeline{
		TenantID: "tenant-1",
		Name:     "Old Name",
		Steps:    []PipelineStep{{ID: "s1", Name: "Old Step", Type: PipelineStepAPI}},
	}
	created, _ := store.Create(p)

	newName := "New Name"
	newStatus := PipelineStatusInactive
	newSteps := []PipelineStep{{ID: "s2", Name: "New Step", Type: PipelineStepAgent}}

	t.Run("updates pipeline fields", func(t *testing.T) {
		updated, err := store.Update(created.ID, &newName, nil, &newSteps, nil, nil, nil, &newStatus, nil, nil)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if updated.Name != "New Name" {
			t.Errorf("Expected 'New Name', got %s", updated.Name)
		}
		if updated.Status != PipelineStatusInactive {
			t.Errorf("Expected status %s, got %s", PipelineStatusInactive, updated.Status)
		}
		if len(updated.Steps) != 1 || updated.Steps[0].Name != "New Step" {
			t.Errorf("Expected new step, got %+v", updated.Steps)
		}
	})

	t.Run("update non-existent pipeline", func(t *testing.T) {
		name := "ghost"
		_, err := store.Update("non-existent", &name, nil, nil, nil, nil, nil, nil, nil, nil)
		if err == nil {
			t.Error("Expected error for non-existent pipeline")
		}
	})
}

func TestPipelineStore_UpdateStatus(t *testing.T) {
	store := NewPipelineStore()

	p := &Pipeline{TenantID: "tenant-1", Name: "Test Pipeline", Steps: []PipelineStep{}}
	created, _ := store.Create(p)

	t.Run("update to inactive", func(t *testing.T) {
		err := store.UpdateStatus(created.ID, PipelineStatusInactive)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if got.Status != PipelineStatusInactive {
			t.Errorf("Expected %s, got %s", PipelineStatusInactive, got.Status)
		}
	})

	t.Run("update to archived", func(t *testing.T) {
		err := store.UpdateStatus(created.ID, PipelineStatusArchived)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if got.Status != PipelineStatusArchived {
			t.Errorf("Expected %s, got %s", PipelineStatusArchived, got.Status)
		}
	})

	t.Run("update non-existent pipeline", func(t *testing.T) {
		err := store.UpdateStatus("non-existent", PipelineStatusActive)
		if err == nil {
			t.Error("Expected error for non-existent pipeline")
		}
	})
}

func TestPipelineStore_Delete(t *testing.T) {
	store := NewPipelineStore()

	p := &Pipeline{TenantID: "tenant-1", Name: "Test Pipeline", Steps: []PipelineStep{}}
	created, _ := store.Create(p)

	t.Run("delete pipeline", func(t *testing.T) {
		err := store.Delete(created.ID)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err = store.GetByID(created.ID)
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	t.Run("delete non-existent pipeline", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent pipeline")
		}
	})

	t.Run("tenant list reflects deletion", func(t *testing.T) {
		store := NewPipelineStore()
		p2 := &Pipeline{TenantID: "tenant-1", Name: "Kept", Steps: []PipelineStep{}}
		keep, _ := store.Create(p2)

		store.Delete(created.ID)
		items, total, _ := store.List("tenant-1", 1, 50, nil)
		if total != 1 || items[0].ID != keep.ID {
			t.Errorf("Expected 1 pipeline after delete, got %d", total)
		}
	})
}

func TestPipelineStore_List(t *testing.T) {
	store := NewPipelineStore()

	for i := 1; i <= 5; i++ {
		p := &Pipeline{
			TenantID: "tenant-1",
			Name:     "Pipeline " + string(rune('A'+i-1)),
			Steps:    []PipelineStep{{Type: PipelineStepAPI}},
		}
		store.Create(p)
	}

	t.Run("returns all pipelines", func(t *testing.T) {
		items, total, hasMore := store.List("tenant-1", 1, 50, nil)
		if total != 5 {
			t.Errorf("Expected 5 pipelines, got %d", total)
		}
		if len(items) != 5 {
			t.Errorf("Expected 5 in slice, got %d", len(items))
		}
		if hasMore {
			t.Error("Expected hasMore=false")
		}
	})

	t.Run("pagination", func(t *testing.T) {
		items, total, hasMore := store.List("tenant-1", 1, 2, nil)
		if total != 5 {
			t.Errorf("Expected total=5, got %d", total)
		}
		if len(items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(items))
		}
		if !hasMore {
			t.Error("Expected hasMore=true")
		}

		items, _, hasMore = store.List("tenant-1", 3, 2, nil)
		if len(items) != 1 {
			t.Errorf("Expected 1 item on last page, got %d", len(items))
		}
		if hasMore {
			t.Error("Expected hasMore=false on last page")
		}
	})

	t.Run("status filter", func(t *testing.T) {
		store := NewPipelineStore()
		active := &Pipeline{TenantID: "tenant-1", Name: "Active", Status: PipelineStatusActive, Steps: []PipelineStep{}}
		inactive := &Pipeline{TenantID: "tenant-1", Name: "Inactive", Status: PipelineStatusInactive, Steps: []PipelineStep{}}
		store.Create(active)
		store.Create(inactive)

		items, total, _ := store.List("tenant-1", 1, 50, strPtr(string(PipelineStatusInactive)))
		if total != 1 {
			t.Errorf("Expected 1 inactive pipeline, got %d", total)
		}
		if items[0].Status != PipelineStatusInactive {
			t.Errorf("Expected inactive, got %s", items[0].Status)
		}
	})

	t.Run("empty tenant returns empty", func(t *testing.T) {
		items, total, hasMore := store.List("non-existent-tenant", 1, 50, nil)
		if total != 0 {
			t.Errorf("Expected 0 pipelines, got %d", total)
		}
		if len(items) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(items))
		}
		if hasMore {
			t.Error("Expected hasMore=false for empty result")
		}
	})
}

func TestPipelineStore_IncrementExecutionCount(t *testing.T) {
	store := NewPipelineStore()

	p := &Pipeline{
		TenantID:       "tenant-1",
		Name:           "Test Pipeline",
		Steps:          []PipelineStep{},
		ExecutionCount: 0,
		SuccessRate:    0,
	}
	created, _ := store.Create(p)

	t.Run("first success increments count and sets rate", func(t *testing.T) {
		store.IncrementExecutionCount(created.ID, true)
		got, _ := store.GetByID(created.ID)
		if got.ExecutionCount != 1 {
			t.Errorf("Expected count 1, got %d", got.ExecutionCount)
		}
		if got.SuccessRate != 100.0 {
			t.Errorf("Expected 100%% success rate, got %f", got.SuccessRate)
		}
	})

	t.Run("failure decreases success rate", func(t *testing.T) {
		store.IncrementExecutionCount(created.ID, false)
		got, _ := store.GetByID(created.ID)
		if got.ExecutionCount != 2 {
			t.Errorf("Expected count 2, got %d", got.ExecutionCount)
		}
		if got.SuccessRate != 50.0 {
			t.Errorf("Expected 50%% success rate, got %f", got.SuccessRate)
		}
	})

	t.Run("multiple executions accumulate", func(t *testing.T) {
		store := NewPipelineStore()
		p := &Pipeline{TenantID: "tenant-1", Name: "Accum Pipeline", Steps: []PipelineStep{}}
		created, _ := store.Create(p)

		store.IncrementExecutionCount(created.ID, true)  // 1/1 = 100%
		store.IncrementExecutionCount(created.ID, true)  // 2/2 = 100%
		store.IncrementExecutionCount(created.ID, false) // 2/3 = 66.67%
		store.IncrementExecutionCount(created.ID, true)  // 3/4 = 75%

		got, _ := store.GetByID(created.ID)
		if got.ExecutionCount != 4 {
			t.Errorf("Expected count 4, got %d", got.ExecutionCount)
		}
		// Use tolerance for float comparison due to floating-point precision
		if got.SuccessRate < 74.99 || got.SuccessRate > 75.01 {
			t.Errorf("Expected ~75%% success rate, got %f", got.SuccessRate)
		}
	})

	t.Run("non-existent pipeline is no-op", func(t *testing.T) {
		store.IncrementExecutionCount("non-existent", true)
		// Should not panic or error
	})
}

// ─── ExecutionStore tests ────────────────────────────────────────────────────

func TestExecutionStore_Create(t *testing.T) {
	store := NewExecutionStore()

	e := &PipelineExecution{
		PipelineID: "pipeline-1",
		TenantID:   "tenant-1",
		Status:     PipelineExecutionPending,
	}

	t.Run("creates execution with auto-generated ID", func(t *testing.T) {
		created, err := store.Create(e)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if created.ID == "" {
			t.Error("Expected auto-generated ID")
		}
	})

	t.Run("default status is pending", func(t *testing.T) {
		store := NewExecutionStore()
		e := &PipelineExecution{
			PipelineID: "pipeline-1",
			TenantID:   "tenant-1",
		}
		created, _ := store.Create(e)
		if created.Status != PipelineExecutionPending {
			t.Errorf("Expected status %s, got %s", PipelineExecutionPending, created.Status)
		}
	})

	t.Run("duplicate ID returns error", func(t *testing.T) {
		store := NewExecutionStore()
		e1 := &PipelineExecution{ID: "exec-dup", PipelineID: "pipeline-1", TenantID: "tenant-1"}
		e2 := &PipelineExecution{ID: "exec-dup", PipelineID: "pipeline-1", TenantID: "tenant-1"}

		_, err := store.Create(e1)
		if err != nil {
			t.Fatalf("First Create failed: %v", err)
		}
		_, err = store.Create(e2)
		if err == nil {
			t.Error("Expected error for duplicate ID")
		}
	})

	t.Run("tracked by pipeline and tenant", func(t *testing.T) {
		store := NewExecutionStore()
		e1 := &PipelineExecution{PipelineID: "pipe-1", TenantID: "tenant-1"}
		e2 := &PipelineExecution{PipelineID: "pipe-2", TenantID: "tenant-1"}

		store.Create(e1)
		store.Create(e2)

		_, total, _ := store.ListByPipeline("pipe-1", 1, 50, nil, 0)
		if total != 1 {
			t.Errorf("Expected 1 execution for pipe-1, got %d", total)
		}
	})
}

func TestExecutionStore_GetByID(t *testing.T) {
	store := NewExecutionStore()

	e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
	created, _ := store.Create(e)

	t.Run("returns execution by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.PipelineID != "pipeline-1" {
			t.Errorf("Expected pipeline-1, got %s", got.PipelineID)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent execution")
		}
	})
}

func TestExecutionStore_UpdateStatus(t *testing.T) {
	store := NewExecutionStore()

	e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
	created, _ := store.Create(e)

	t.Run("update to running sets started_at", func(t *testing.T) {
		err := store.UpdateStatus(created.ID, PipelineExecutionRunning)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if got.Status != PipelineExecutionRunning {
			t.Errorf("Expected Running, got %s", got.Status)
		}
		if got.StartedAt == nil {
			t.Error("Expected StartedAt to be set")
		}
	})

	t.Run("update to completed sets completed_at", func(t *testing.T) {
		err := store.UpdateStatus(created.ID, PipelineExecutionCompleted)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if got.Status != PipelineExecutionCompleted {
			t.Errorf("Expected Completed, got %s", got.Status)
		}
		if got.CompletedAt == nil {
			t.Error("Expected CompletedAt to be set")
		}
	})

	t.Run("update to failed sets completed_at", func(t *testing.T) {
		store := NewExecutionStore()
		e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
		created, _ := store.Create(e)

		err := store.UpdateStatus(created.ID, PipelineExecutionFailed)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if got.Status != PipelineExecutionFailed {
			t.Errorf("Expected Failed, got %s", got.Status)
		}
		if got.CompletedAt == nil {
			t.Error("Expected CompletedAt to be set on failure")
		}
	})

	t.Run("update to cancelled sets completed_at", func(t *testing.T) {
		store := NewExecutionStore()
		e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
		created, _ := store.Create(e)

		err := store.UpdateStatus(created.ID, PipelineExecutionCancelled)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if got.Status != PipelineExecutionCancelled {
			t.Errorf("Expected Cancelled, got %s", got.Status)
		}
	})

	t.Run("update non-existent execution", func(t *testing.T) {
		err := store.UpdateStatus("non-existent", PipelineExecutionRunning)
		if err == nil {
			t.Error("Expected error for non-existent execution")
		}
	})
}

func TestExecutionStore_Delete(t *testing.T) {
	store := NewExecutionStore()

	e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
	created, _ := store.Create(e)

	t.Run("delete execution", func(t *testing.T) {
		err := store.Delete(created.ID)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err = store.GetByID(created.ID)
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	t.Run("delete non-existent execution", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent execution")
		}
	})

	t.Run("steps are also deleted", func(t *testing.T) {
		store := NewExecutionStore()
		e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
		created, _ := store.Create(e)

		step := &PipelineExecutionStep{ExecutionID: created.ID, StepID: "s1", StepName: "Test Step"}
		store.AddStep(step)

		steps := store.GetSteps(created.ID)
		if len(steps) != 1 {
			t.Fatalf("Expected 1 step before delete, got %d", len(steps))
		}

		store.Delete(created.ID)
		steps = store.GetSteps(created.ID)
		if len(steps) != 0 {
			t.Errorf("Expected 0 steps after delete, got %d", len(steps))
		}
	})
}

func TestExecutionStore_ListByPipeline(t *testing.T) {
	store := NewExecutionStore()

	for i := 1; i <= 5; i++ {
		status := PipelineExecutionCompleted
		if i <= 2 {
			status = PipelineExecutionRunning
		}
		e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1", Status: status}
		store.Create(e)
	}

	t.Run("returns all executions for pipeline", func(t *testing.T) {
		items, total, hasMore := store.ListByPipeline("pipeline-1", 1, 50, nil, 0)
		if total != 5 {
			t.Errorf("Expected 5 executions, got %d", total)
		}
		if len(items) != 5 {
			t.Errorf("Expected 5 in slice, got %d", len(items))
		}
		if hasMore {
			t.Error("Expected hasMore=false")
		}
	})

	t.Run("status filter", func(t *testing.T) {
		items, total, _ := store.ListByPipeline("pipeline-1", 1, 50, strPtr(string(PipelineExecutionCompleted)), 0)
		if total != 3 {
			t.Errorf("Expected 3 completed executions, got %d", total)
		}
		for _, e := range items {
			if e.Status != PipelineExecutionCompleted {
				t.Errorf("Expected all completed, got %s", e.Status)
			}
		}
	})

	t.Run("limit parameter", func(t *testing.T) {
		items, _, _ := store.ListByPipeline("pipeline-1", 1, 50, nil, 2)
		if len(items) > 2 {
			t.Errorf("Expected at most 2 items with limit=2, got %d", len(items))
		}
	})

	t.Run("non-existent pipeline", func(t *testing.T) {
		items, total, hasMore := store.ListByPipeline("non-existent", 1, 50, nil, 0)
		if total != 0 {
			t.Errorf("Expected 0 executions, got %d", total)
		}
		if len(items) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(items))
		}
		if hasMore {
			t.Error("Expected hasMore=false for empty result")
		}
	})
}

func TestExecutionStore_ListByTenant(t *testing.T) {
	store := NewExecutionStore()

	store.Create(&PipelineExecution{PipelineID: "pipe-1", TenantID: "tenant-1"})
	store.Create(&PipelineExecution{PipelineID: "pipe-2", TenantID: "tenant-1"})
	store.Create(&PipelineExecution{PipelineID: "pipe-3", TenantID: "tenant-2"})

	t.Run("returns executions for tenant", func(t *testing.T) {
		_, total, _ := store.ListByTenant("tenant-1", 1, 50)
		if total != 2 {
			t.Errorf("Expected 2 executions for tenant-1, got %d", total)
		}
	})

	t.Run("tenant isolation", func(t *testing.T) {
		_, total, _ := store.ListByTenant("tenant-2", 1, 50)
		if total != 1 {
			t.Errorf("Expected 1 execution for tenant-2, got %d", total)
		}
	})

	t.Run("empty tenant", func(t *testing.T) {
		_, total, hasMore := store.ListByTenant("non-existent", 1, 50)
		if total != 0 {
			t.Errorf("Expected 0 executions, got %d", total)
		}
		if hasMore {
			t.Error("Expected hasMore=false for empty result")
		}
	})

	t.Run("pagination", func(t *testing.T) {
		items, total, hasMore := store.ListByTenant("tenant-1", 1, 1)
		if total != 2 {
			t.Errorf("Expected total=2, got %d", total)
		}
		if len(items) != 1 {
			t.Errorf("Expected 1 item, got %d", len(items))
		}
		if !hasMore {
			t.Error("Expected hasMore=true")
		}
	})
}

func TestExecutionStore_Steps(t *testing.T) {
	store := NewExecutionStore()

	e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
	created, _ := store.Create(e)

	t.Run("add and get steps", func(t *testing.T) {
		store.AddStep(&PipelineExecutionStep{ExecutionID: created.ID, StepID: "s1", StepName: "Step One"})
		store.AddStep(&PipelineExecutionStep{ExecutionID: created.ID, StepID: "s2", StepName: "Step Two"})

		steps := store.GetSteps(created.ID)
		if len(steps) != 2 {
			t.Errorf("Expected 2 steps, got %d", len(steps))
		}
	})

	t.Run("returns copies, not references", func(t *testing.T) {
		store := NewExecutionStore()
		e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
		created, _ := store.Create(e)

		store.AddStep(&PipelineExecutionStep{
			ExecutionID: created.ID,
			StepID:      "s1",
			StepName:    "Test",
			Inputs:      map[string]interface{}{"key": "value"},
			Outputs:     map[string]interface{}{"out": "data"},
		})

		steps := store.GetSteps(created.ID)
		if len(steps) != 1 {
			t.Fatalf("Expected 1 step, got %d", len(steps))
		}
		// Mutate the copy
		steps[0].StepName = "mutated"
		steps[0].Inputs["key"] = "mutated"

		// Get again and verify original is unchanged
		steps2 := store.GetSteps(created.ID)
		if steps2[0].StepName != "Test" {
			t.Error("Mutation leaked into subsequent GetSteps")
		}
	})

	t.Run("get steps for non-existent execution", func(t *testing.T) {
		steps := store.GetSteps("non-existent")
		if len(steps) != 0 {
			t.Errorf("Expected 0 steps, got %d", len(steps))
		}
	})
}

func TestExecutionStore_IncrementRetryCount(t *testing.T) {
	store := NewExecutionStore()

	e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
	created, _ := store.Create(e)

	t.Run("increments retry count", func(t *testing.T) {
		count := store.IncrementRetryCount(created.ID)
		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}

		count = store.IncrementRetryCount(created.ID)
		if count != 2 {
			t.Errorf("Expected count 2, got %d", count)
		}
	})

	t.Run("non-existent execution returns 0", func(t *testing.T) {
		count := store.IncrementRetryCount("non-existent")
		if count != 0 {
			t.Errorf("Expected 0 for non-existent, got %d", count)
		}
	})

	t.Run("count stored in execution", func(t *testing.T) {
		store := NewExecutionStore()
		e := &PipelineExecution{PipelineID: "pipeline-1", TenantID: "tenant-1"}
		created, _ := store.Create(e)

		store.IncrementRetryCount(created.ID)
		store.IncrementRetryCount(created.ID)

		got, _ := store.GetByID(created.ID)
		if got.RetryCount != 2 {
			t.Errorf("Expected RetryCount 2, got %d", got.RetryCount)
		}
	})
}

// ─── HumanTaskStore tests ────────────────────────────────────────────────────

func TestHumanTaskStore_Create(t *testing.T) {
	store := NewHumanTaskStore()

	t.Run("creates task with auto-generated ID", func(t *testing.T) {
		task := &HumanTask{
			TenantID:            "tenant-1",
			PipelineExecutionID: "exec-1",
			AssigneeType:        HumanTaskAssigneeUser,
			AssigneeID:          "user-1",
			TaskType:            HumanTaskApproval,
			Instructions:        "Approve this task",
		}
		created, err := store.Create(task)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if created.ID == "" {
			t.Error("Expected auto-generated ID")
		}
	})

	t.Run("default status is pending", func(t *testing.T) {
		store := NewHumanTaskStore()
		task := &HumanTask{
			TenantID:            "tenant-1",
			PipelineExecutionID: "exec-1",
			AssigneeType:        HumanTaskAssigneeUser,
			AssigneeID:          "user-1",
			TaskType:            HumanTaskApproval,
			Instructions:        "Test",
		}
		created, _ := store.Create(task)
		if created.Status != HumanTaskStatusPending {
			t.Errorf("Expected status %s, got %s", HumanTaskStatusPending, created.Status)
		}
	})

	t.Run("duplicate ID returns error", func(t *testing.T) {
		store := NewHumanTaskStore()
		t1 := &HumanTask{ID: "task-dup", TenantID: "tenant-1", PipelineExecutionID: "exec-1",
			AssigneeType: HumanTaskAssigneeUser, AssigneeID: "user-1", TaskType: HumanTaskApproval, Instructions: "First"}
		t2 := &HumanTask{ID: "task-dup", TenantID: "tenant-1", PipelineExecutionID: "exec-1",
			AssigneeType: HumanTaskAssigneeUser, AssigneeID: "user-2", TaskType: HumanTaskApproval, Instructions: "Second"}

		_, err := store.Create(t1)
		if err != nil {
			t.Fatalf("First Create failed: %v", err)
		}
		_, err = store.Create(t2)
		if err == nil {
			t.Error("Expected error for duplicate ID")
		}
	})
}

func TestHumanTaskStore_GetByID(t *testing.T) {
	store := NewHumanTaskStore()

	task := &HumanTask{
		TenantID:            "tenant-1",
		PipelineExecutionID: "exec-1",
		AssigneeType:        HumanTaskAssigneeUser,
		AssigneeID:          "user-1",
		TaskType:            HumanTaskApproval,
		Instructions:        "Approve",
	}
	created, _ := store.Create(task)

	t.Run("returns task by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.Instructions != "Approve" {
			t.Errorf("Expected 'Approve', got %s", got.Instructions)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent task")
		}
	})
}

func TestHumanTaskStore_Respond(t *testing.T) {
	store := NewHumanTaskStore()

	task := &HumanTask{
		ID:                  "task-1",
		TenantID:            "tenant-1",
		PipelineExecutionID: "exec-1",
		AssigneeType:        HumanTaskAssigneeUser,
		AssigneeID:          "user-1",
		TaskType:            HumanTaskApproval,
		Instructions:        "Approve or reject",
		Status:              HumanTaskStatusPending,
	}
	store.Create(task)

	t.Run("respond with approve", func(t *testing.T) {
		responded, err := store.Respond("task-1", "approve", map[string]interface{}{"comment": "looks good"}, "user-1", "approved")
		if err != nil {
			t.Fatalf("Respond failed: %v", err)
		}
		if responded.Status != HumanTaskStatusApproved {
			t.Errorf("Expected Approved, got %s", responded.Status)
		}
		if responded.RespondedBy != "user-1" {
			t.Errorf("Expected respondedBy user-1, got %s", responded.RespondedBy)
		}
		if responded.RespondedAt == nil {
			t.Error("Expected RespondedAt to be set")
		}
		if responded.Response == nil || responded.Response["comment"] != "looks good" {
			t.Errorf("Expected response data, got %v", responded.Response)
		}
	})

	t.Run("respond with reject", func(t *testing.T) {
		store := NewHumanTaskStore()
		task := &HumanTask{
			ID:                  "task-2",
			TenantID:            "tenant-1",
			PipelineExecutionID: "exec-1",
			AssigneeType:        HumanTaskAssigneeUser,
			AssigneeID:          "user-1",
			TaskType:            HumanTaskReview,
			Instructions:        "Review",
			Status:              HumanTaskStatusPending,
		}
		store.Create(task)

		responded, err := store.Respond("task-2", "reject", nil, "user-2", "needs changes")
		if err != nil {
			t.Fatalf("Respond failed: %v", err)
		}
		if responded.Status != HumanTaskStatusRejected {
			t.Errorf("Expected Rejected, got %s", responded.Status)
		}
	})

	t.Run("respond with request_info (rejected)", func(t *testing.T) {
		store := NewHumanTaskStore()
		task := &HumanTask{
			ID:                  "task-3",
			TenantID:            "tenant-1",
			PipelineExecutionID: "exec-1",
			AssigneeType:        HumanTaskAssigneeUser,
			AssigneeID:          "user-1",
			TaskType:            HumanTaskInput,
			Instructions:        "Provide info",
			Status:              HumanTaskStatusPending,
		}
		store.Create(task)

		responded, _ := store.Respond("task-3", "request_info", nil, "user-3", "need more data")
		if responded.Status != HumanTaskStatusRejected {
			t.Errorf("Expected Rejected for request_info, got %s", responded.Status)
		}
	})

	t.Run("cannot respond to non-pending task", func(t *testing.T) {
		store := NewHumanTaskStore()
		task := &HumanTask{
			ID:                  "task-4",
			TenantID:            "tenant-1",
			PipelineExecutionID: "exec-1",
			AssigneeType:        HumanTaskAssigneeUser,
			AssigneeID:          "user-1",
			TaskType:            HumanTaskApproval,
			Instructions:        "Test",
			Status:              HumanTaskStatusApproved,
		}
		store.Create(task)

		_, err := store.Respond("task-4", "approve", nil, "user-1", "")
		if err == nil {
			t.Error("Expected error when responding to non-pending task")
		}
	})

	t.Run("respond to non-existent task", func(t *testing.T) {
		_, err := store.Respond("non-existent", "approve", nil, "user-1", "")
		if err == nil {
			t.Error("Expected error for non-existent task")
		}
	})
}

func TestHumanTaskStore_List(t *testing.T) {
	store := NewHumanTaskStore()

	store.Create(&HumanTask{TenantID: "tenant-1", PipelineExecutionID: "exec-1", AssigneeType: HumanTaskAssigneeUser, AssigneeID: "u1", TaskType: HumanTaskApproval, Instructions: "T1", Status: HumanTaskStatusPending})
	store.Create(&HumanTask{TenantID: "tenant-1", PipelineExecutionID: "exec-2", AssigneeType: HumanTaskAssigneeRole, AssigneeID: "role-1", TaskType: HumanTaskReview, Instructions: "T2", Status: HumanTaskStatusPending})
	store.Create(&HumanTask{TenantID: "tenant-1", PipelineExecutionID: "exec-3", AssigneeType: HumanTaskAssigneeUser, AssigneeID: "u2", TaskType: HumanTaskConfirm, Instructions: "T3", Status: HumanTaskStatusApproved})
	store.Create(&HumanTask{TenantID: "tenant-2", PipelineExecutionID: "exec-4", AssigneeType: HumanTaskAssigneeUser, AssigneeID: "u3", TaskType: HumanTaskApproval, Instructions: "T4", Status: HumanTaskStatusPending})

	t.Run("returns all tasks for tenant", func(t *testing.T) {
		_, total := store.List("tenant-1", nil)
		if total != 3 {
			t.Errorf("Expected 3 tasks for tenant-1, got %d", total)
		}
	})

	t.Run("tenant isolation", func(t *testing.T) {
		_, total := store.List("tenant-2", nil)
		if total != 1 {
			t.Errorf("Expected 1 task for tenant-2, got %d", total)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		tasks, total := store.List("tenant-1", strPtr(string(HumanTaskStatusApproved)))
		if total != 1 {
			t.Errorf("Expected 1 approved task, got %d", total)
		}
		for _, tk := range tasks {
			if tk.Status != HumanTaskStatusApproved {
				t.Errorf("Expected approved, got %s", tk.Status)
			}
		}
	})

	t.Run("empty tenant returns empty", func(t *testing.T) {
		tasks, total := store.List("non-existent", nil)
		if total != 0 {
			t.Errorf("Expected 0 tasks, got %d", total)
		}
		if len(tasks) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(tasks))
		}
	})
}

// ─── PipelineStep type constant tests ────────────────────────────────────────

func TestPipelineStepTypes(t *testing.T) {
	// Verify all expected step types are defined
	types := []PipelineStepType{
		PipelineStepAPI, PipelineStepAgent, PipelineStepData,
		PipelineStepCondition, PipelineStepDelay, PipelineStepHuman,
		PipelineStepParallel, PipelineStepForeach, PipelineStepWebhook,
		PipelineStepCode, PipelineStepNotify,
	}
	for _, tp := range types {
		if string(tp) == "" {
			t.Errorf("PipelineStepType %s is empty", tp)
		}
	}
	_ = types // used to verify compile-time availability
}

func TestPipelineOnErrorActions(t *testing.T) {
	actions := []PipelineOnErrorAction{
		PipelineOnErrorFail, PipelineOnErrorRetry,
		PipelineOnErrorSkip, PipelineOnErrorBranch,
	}
	for _, a := range actions {
		if string(a) == "" {
			t.Errorf("PipelineOnErrorAction %s is empty", a)
		}
	}
}

func TestPipelineErrorStrategies(t *testing.T) {
	strategies := []PipelineErrorStrategyValue{
		PipelineErrorStrategyValueFail, PipelineErrorStrategyValueRetry,
		PipelineErrorStrategyValueSkip, PipelineErrorStrategyValueAbort,
	}
	for _, s := range strategies {
		if string(s) == "" {
			t.Errorf("PipelineErrorStrategyValue %s is empty", s)
		}
	}
}

func TestPipelineBackoffValues(t *testing.T) {
	backoffs := []PipelineBackoffValue{
		PipelineBackoffFixed, PipelineBackoffExponential, PipelineBackoffLinear,
	}
	for _, b := range backoffs {
		if string(b) == "" {
			t.Errorf("PipelineBackoffValue %s is empty", b)
		}
	}
}

func TestPipelineTriggerTypes(t *testing.T) {
	triggers := []PipelineTriggerType{
		PipelineTriggerManual, PipelineTriggerEvent,
		PipelineTriggerSchedule, PipelineTriggerWebhook, PipelineTriggerAPI,
	}
	for _, tr := range triggers {
		if string(tr) == "" {
			t.Errorf("PipelineTriggerType %s is empty", tr)
		}
	}
}

func TestPipelineExecutionStatuses(t *testing.T) {
	statuses := []PipelineExecutionStatus{
		PipelineExecutionPending, PipelineExecutionRunning,
		PipelineExecutionCompleted, PipelineExecutionFailed,
		PipelineExecutionCancelled, PipelineExecutionRetrying,
	}
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("PipelineExecutionStatus %s is empty", s)
		}
	}
}

func TestPipelineExecutionStepStatuses(t *testing.T) {
	statuses := []PipelineExecutionStepStatus{
		PipelineStepPending, PipelineStepRunning,
		PipelineStepCompleted, PipelineStepFailed,
		PipelineStepSkipped, PipelineStepCancelled,
	}
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("PipelineExecutionStepStatus %s is empty", s)
		}
	}
}

func TestHumanTaskTypes(t *testing.T) {
	types := []HumanTaskType{
		HumanTaskApproval, HumanTaskReject,
		HumanTaskInput, HumanTaskReview, HumanTaskConfirm,
	}
	for _, tp := range types {
		if string(tp) == "" {
			t.Errorf("HumanTaskType %s is empty", tp)
		}
	}
}

func TestHumanTaskPriorityLevels(t *testing.T) {
	priorities := []HumanTaskPriority{
		HumanTaskPriorityLow, HumanTaskPriorityNormal,
		HumanTaskPriorityHigh, HumanTaskPriorityUrgent,
	}
	for _, p := range priorities {
		if string(p) == "" {
			t.Errorf("HumanTaskPriority %s is empty", p)
		}
	}
}

func TestHumanTaskStatuses(t *testing.T) {
	statuses := []HumanTaskStatus{
		HumanTaskStatusPending, HumanTaskStatusApproved,
		HumanTaskStatusRejected, HumanTaskStatusTimeout,
		HumanTaskStatusCancelled,
	}
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("HumanTaskStatus %s is empty", s)
		}
	}
}

func TestHumanTaskAssigneeTypes(t *testing.T) {
	types := []HumanTaskAssigneeType{
		HumanTaskAssigneeUser, HumanTaskAssigneeRole,
		HumanTaskAssigneeGroup, HumanTaskAssigneeAgent,
	}
	for _, tp := range types {
		if string(tp) == "" {
			t.Errorf("HumanTaskAssigneeType %s is empty", tp)
		}
	}
}
