package store

import (
	"testing"
	"time"
)

func TestWorkflowStore_Create(t *testing.T) {
	store := NewWorkflowStore()

	t.Run("creates workflow with auto-generated ID", func(t *testing.T) {
		now := time.Now()
		timeNow = func() time.Time { return now }
		defer func() { timeNow = time.Now }()

		wf := &Workflow{
			TenantID:     "tenant-1",
			DepartmentID: "dept-1",
			Name:         "Test Workflow",
			Version:      "1.0.0",
			Status:       WorkflowStatusPending,
			Graph: WorkflowGraph{
				Nodes: []WorkflowNode{
					{ID: "node-1", Type: WorkflowNodeAgent, Action: "process"},
				},
				Edges: []WorkflowEdge{
					{From: "node-1", To: "node-2"},
				},
			},
			Priority: 5,
			CreatedBy: "user-1",
		}

		created, err := store.Create(wf)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if created.ID == "" {
			t.Error("Expected auto-generated ID")
		}
		if created.TenantID != "tenant-1" {
			t.Errorf("Expected tenant-1, got %s", created.TenantID)
		}
		if created.Status != WorkflowStatusPending {
			t.Errorf("Expected Pending status, got %s", created.Status)
		}
		if created.CreatedAt.IsZero() {
			t.Error("CreatedAt should not be zero")
		}
	})

	t.Run("tenant isolation", func(t *testing.T) {
		store := NewWorkflowStore()
		wf1 := &Workflow{
			TenantID: "tenant-a",
			Name:     "Workflow A",
			Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
		}
		wf2 := &Workflow{
			TenantID: "tenant-b",
			Name:     "Workflow B",
			Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n2", Type: WorkflowNodeAgent}}},
		}

		store.Create(wf1)
		store.Create(wf2)

		_, total, _ := store.List("tenant-a", 1, 50, nil)
		if total != 1 {
			t.Errorf("Expected 1 workflow for tenant-a, got %d", total)
		}
	})
}

func TestWorkflowStore_GetByID(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	created, _ := store.Create(wf)

	t.Run("returns workflow by ID", func(t *testing.T) {
		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("Expected ID %s, got %s", created.ID, got.ID)
		}
		if got.Name != "Test Workflow" {
			t.Errorf("Expected 'Test Workflow', got %s", got.Name)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent ID")
		}
	})
}

func TestWorkflowStore_List(t *testing.T) {
	store := NewWorkflowStore()

	// Create 5 workflows
	for i := 1; i <= 5; i++ {
		wf := &Workflow{
			TenantID: "tenant-1",
			Name:     "Workflow " + string(rune('A'+i-1)),
			Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
		}
		store.Create(wf)
	}

	t.Run("returns all workflows without filter", func(t *testing.T) {
		workflows, total, hasMore := store.List("tenant-1", 1, 50, nil)
		if total != 5 {
			t.Errorf("Expected 5 workflows, got %d", total)
		}
		if len(workflows) != 5 {
			t.Errorf("Expected 5 in slice, got %d", len(workflows))
		}
		if hasMore {
			t.Error("Expected hasMore=false")
		}
	})

	t.Run("pagination", func(t *testing.T) {
		workflows, total, hasMore := store.List("tenant-1", 1, 2, nil)
		if total != 5 {
			t.Errorf("Expected total=5, got %d", total)
		}
		if len(workflows) != 2 {
			t.Errorf("Expected 2 workflows, got %d", len(workflows))
		}
		if !hasMore {
			t.Error("Expected hasMore=true")
		}

		workflows, total, hasMore = store.List("tenant-1", 2, 2, nil)
		if len(workflows) != 2 {
			t.Errorf("Expected 2 workflows on page 2, got %d", len(workflows))
		}
		if !hasMore {
			t.Error("Expected hasMore=true for page 2")
		}

		workflows, total, hasMore = store.List("tenant-1", 3, 2, nil)
		if len(workflows) != 1 {
			t.Errorf("Expected 1 workflow on page 3, got %d", len(workflows))
		}
		if hasMore {
			t.Error("Expected hasMore=false on last page")
		}
	})

	t.Run("status filter", func(t *testing.T) {
		wfRunning := &Workflow{
			TenantID: "tenant-1",
			Name:     "Running Workflow",
			Status:   WorkflowStatusRunning,
			Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
		}
		store.Create(wfRunning)

		workflows, total, _ := store.List("tenant-1", 1, 50, strPtr(string(WorkflowStatusRunning)))
		if total < 1 {
			t.Errorf("Expected at least 1 running workflow, got %d", total)
		}
		for _, wf := range workflows {
			if wf.Status != WorkflowStatusRunning {
				t.Errorf("Expected all running, got %s", wf.Status)
			}
		}
	})
}

func TestWorkflowStore_UpdateStatus(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	created, _ := store.Create(wf)

	t.Run("valid status transition pending->running", func(t *testing.T) {
		err := store.UpdateStatus(created.ID, WorkflowStatusRunning)
		if err != nil {
			t.Fatalf("UpdateStatus failed: %v", err)
		}

		got, _ := store.GetByID(created.ID)
		if got.Status != WorkflowStatusRunning {
			t.Errorf("Expected Running, got %s", got.Status)
		}
	})

	t.Run("invalid status transition running->pending", func(t *testing.T) {
		err := store.UpdateStatus(created.ID, WorkflowStatusPending)
		if err == nil {
			t.Error("Expected error for invalid status transition")
		}
	})

	t.Run("non-existent workflow", func(t *testing.T) {
		err := store.UpdateStatus("non-existent", WorkflowStatusRunning)
		if err == nil {
			t.Error("Expected error for non-existent workflow")
		}
	})
}

func TestWorkflowStore_Checkpoints(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	store.Create(wf)

	cp1 := Checkpoint{
		ID:        "cp-1",
		NodeID:    wf.ID,
		Timestamp: time.Now(),
	}
	cp2 := Checkpoint{
		ID:        "cp-2",
		NodeID:    wf.ID,
		Timestamp: time.Now().Add(time.Second),
	}
	store.AddCheckpoint(cp1)
	store.AddCheckpoint(cp2)

	t.Run("get checkpoints for workflow", func(t *testing.T) {
		checkpoints := store.GetCheckpoints(wf.ID)
		if len(checkpoints) != 2 {
			t.Errorf("Expected 2 checkpoints, got %d", len(checkpoints))
		}
	})

	t.Run("get checkpoints for non-existent node", func(t *testing.T) {
		checkpoints := store.GetCheckpoints("non-existent")
		if len(checkpoints) != 0 {
			t.Errorf("Expected 0 checkpoints, got %d", len(checkpoints))
		}
	})
}

func TestWorkflowStore_ExecutionHistory(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	store.Create(wf)

	event := ExecutionEvent{
		EventID:   "evt-1",
		NodeID:    "n1",
		EventType: "started",
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"node_id": "n1"},
	}
	store.AddEvent(wf.ID, event)

	t.Run("get execution events", func(t *testing.T) {
		events := store.GetExecutionHistory(wf.ID)
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		if events[0].EventID != "evt-1" {
			t.Errorf("Expected evt-1, got %s", events[0].EventID)
		}
	})
}

func strPtr(s string) *string {
	return &s
}

// ─── WorkflowStore UpdateCurrentNodes tests ──────────────────────────────────

func TestWorkflowStore_UpdateCurrentNodes(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	created, _ := store.Create(wf)

	t.Run("updates current nodes", func(t *testing.T) {
		err := store.UpdateCurrentNodes(created.ID, []string{"n1", "n2"})
		if err != nil {
			t.Fatalf("UpdateCurrentNodes failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if len(got.CurrentNodes) != 2 {
			t.Errorf("Expected 2 current nodes, got %d", len(got.CurrentNodes))
		}
		if got.CurrentNodes[0] != "n1" || got.CurrentNodes[1] != "n2" {
			t.Errorf("Expected [n1, n2], got %v", got.CurrentNodes)
		}
	})

	t.Run("clears current nodes", func(t *testing.T) {
		err := store.UpdateCurrentNodes(created.ID, []string{})
		if err != nil {
			t.Fatalf("UpdateCurrentNodes failed: %v", err)
		}
		got, _ := store.GetByID(created.ID)
		if len(got.CurrentNodes) != 0 {
			t.Errorf("Expected 0 current nodes, got %d", len(got.CurrentNodes))
		}
	})

	t.Run("non-existent workflow", func(t *testing.T) {
		err := store.UpdateCurrentNodes("non-existent", []string{"n1"})
		if err == nil {
			t.Error("Expected error for non-existent workflow")
		}
	})
}

// ─── WorkflowStore Variable tests ─────────────────────────────────────────────

func TestWorkflowStore_AddVariable(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	created, _ := store.Create(wf)

	t.Run("add variable to new workflow", func(t *testing.T) {
		err := store.AddVariable(created.ID, "tenant-1", "key1", "value1")
		if err != nil {
			t.Fatalf("AddVariable failed: %v", err)
		}
		vars, err := store.GetVariables(created.ID)
		if err != nil {
			t.Fatalf("GetVariables failed: %v", err)
		}
		if vars.Variables["key1"] != "value1" {
			t.Errorf("Expected key1=value1, got %v", vars.Variables["key1"])
		}
		if vars.Version != 1 {
			t.Errorf("Expected version 1, got %d", vars.Version)
		}
	})

	t.Run("add multiple variables increments version", func(t *testing.T) {
		store := NewWorkflowStore()
		wf := &Workflow{TenantID: "tenant-1", Name: "Test Workflow", Graph: WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}}}
		created, _ := store.Create(wf)

		store.AddVariable(created.ID, "tenant-1", "a", 1)
		store.AddVariable(created.ID, "tenant-1", "b", 2)
		store.AddVariable(created.ID, "tenant-1", "a", 3) // update

		vars, _ := store.GetVariables(created.ID)
		if vars.Version != 3 {
			t.Errorf("Expected version 3, got %d", vars.Version)
		}
		if vars.Variables["a"] != 3 {
			t.Errorf("Expected a=3, got %v", vars.Variables["a"])
		}
		if vars.Variables["b"] != 2 {
			t.Errorf("Expected b=2, got %v", vars.Variables["b"])
		}
	})

	t.Run("get variables for non-existent workflow", func(t *testing.T) {
		_, err := store.GetVariables("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent workflow")
		}
	})
}

func TestWorkflowStore_SetVariables(t *testing.T) {
	store := NewWorkflowStore()

	wf := &Workflow{
		TenantID: "tenant-1",
		Name:     "Test Workflow",
		Graph:    WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}},
	}
	created, _ := store.Create(wf)

	t.Run("set variables on new workflow", func(t *testing.T) {
		vars := map[string]interface{}{
			"x": 10,
			"y": "hello",
		}
		err := store.SetVariables(created.ID, "tenant-1", vars)
		if err != nil {
			t.Fatalf("SetVariables failed: %v", err)
		}
		got, err := store.GetVariables(created.ID)
		if err != nil {
			t.Fatalf("GetVariables failed: %v", err)
		}
		if got.Variables["x"] != 10 {
			t.Errorf("Expected x=10, got %v", got.Variables["x"])
		}
		if got.Variables["y"] != "hello" {
			t.Errorf("Expected y=hello, got %v", got.Variables["y"])
		}
		if got.Version != 1 {
			t.Errorf("Expected version 1, got %d", got.Version)
		}
	})

	t.Run("set variables replaces existing", func(t *testing.T) {
		store := NewWorkflowStore()
		wf := &Workflow{TenantID: "tenant-1", Name: "Test Workflow", Graph: WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}}}
		created, _ := store.Create(wf)

		store.AddVariable(created.ID, "tenant-1", "old", "value")
		store.SetVariables(created.ID, "tenant-1", map[string]interface{}{"new": "data"})

		got, _ := store.GetVariables(created.ID)
		if _, exists := got.Variables["old"]; exists {
			t.Error("Expected 'old' key to be removed after SetVariables")
		}
		if got.Variables["new"] != "data" {
			t.Errorf("Expected new=data, got %v", got.Variables["new"])
		}
		// Version should be 1 because SetVariables creates a new entry when one exists but treats it as a full replace
	})

	t.Run("set variables on existing increments version", func(t *testing.T) {
		store := NewWorkflowStore()
		wf := &Workflow{TenantID: "tenant-1", Name: "Test Workflow", Graph: WorkflowGraph{Nodes: []WorkflowNode{{ID: "n1", Type: WorkflowNodeAgent}}}}
		created, _ := store.Create(wf)

		store.SetVariables(created.ID, "tenant-1", map[string]interface{}{"v": 1})
		store.SetVariables(created.ID, "tenant-1", map[string]interface{}{"v": 2})

		got, _ := store.GetVariables(created.ID)
		if got.Version != 2 {
			t.Errorf("Expected version 2, got %d", got.Version)
		}
	})
}
