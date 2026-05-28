package store

import (
	"testing"
	"time"
)

func TestAgentStore_SetAndGetAvailability(t *testing.T) {
	store := NewAgentStore()

	avail := &AgentAvailability{
		AgentID:          "agent-1",
		Status:           AgentStatusAvailable,
		CurrentWorkflows: 3,
		MaxConcurrency:   10,
		LastSeenAt:       ptrTime(time.Now()),
	}

	t.Run("sets availability", func(t *testing.T) {
		store.SetAgentAvailability(avail)

		got, err := store.GetAgentAvailability("agent-1")
		if err != nil {
			t.Fatalf("GetAgentAvailability failed: %v", err)
		}
		if got.AgentID != "agent-1" {
			t.Errorf("Expected agent-1, got %s", got.AgentID)
		}
		if got.Status != AgentStatusAvailable {
			t.Errorf("Expected available status, got %s", got.Status)
		}
		if got.MaxConcurrency != 10 {
			t.Errorf("Expected max concurrency 10, got %d", got.MaxConcurrency)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetAgentAvailability("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent agent")
		}
	})

	t.Run("updates availability", func(t *testing.T) {
		updated := &AgentAvailability{
			AgentID:          "agent-1",
			Status:           AgentStatusBusy,
			CurrentWorkflows: 8,
			MaxConcurrency:   10,
		}
		store.SetAgentAvailability(updated)

		got, _ := store.GetAgentAvailability("agent-1")
		if got.Status != AgentStatusBusy {
			t.Errorf("Expected busy status, got %s", got.Status)
		}
		if got.CurrentWorkflows != 8 {
			t.Errorf("Expected 8 workflows, got %d", got.CurrentWorkflows)
		}
	})
}

func TestAgentStore_ListAgentAvailability(t *testing.T) {
	store := NewAgentStore()

	store.SetAgentAvailability(&AgentAvailability{
		AgentID:   "agent-1",
		Status:    AgentStatusAvailable,
		MaxConcurrency: 10,
	})
	store.SetAgentAvailability(&AgentAvailability{
		AgentID:   "agent-2",
		Status:    AgentStatusBusy,
		MaxConcurrency: 5,
	})
	store.SetAgentAvailability(&AgentAvailability{
		AgentID:   "agent-3",
		Status:    AgentStatusOffline,
		MaxConcurrency: 20,
	})

	t.Run("lists all availability", func(t *testing.T) {
		availabilities := store.ListAgentAvailability()
		if len(availabilities) != 3 {
			t.Errorf("Expected 3 availability entries, got %d", len(availabilities))
		}
	})

	t.Run("empty list", func(t *testing.T) {
		emptyStore := NewAgentStore()
		availabilities := emptyStore.ListAgentAvailability()
		if len(availabilities) != 0 {
			t.Errorf("Expected 0 availability entries, got %d", len(availabilities))
		}
	})
}

func TestAgentStore_Assignments(t *testing.T) {
	store := NewAgentStore()

	t.Run("create assignment", func(t *testing.T) {
		assignment := &AgentAssignment{
			ID:         "assign-1",
			TenantID:   "tenant-1",
			WorkflowID: "wf-1",
			NodeID:     "node-1",
			AgentID:    "agent-1",
		}

		created, err := store.CreateAssignment(assignment)
		if err != nil {
			t.Fatalf("CreateAssignment failed: %v", err)
		}
		if created.ID != "assign-1" {
			t.Errorf("Expected ID 'assign-1', got %s", created.ID)
		}
		if created.WorkflowID != "wf-1" {
			t.Errorf("Expected wf-1, got %s", created.WorkflowID)
		}
	})

	t.Run("auto-generate ID", func(t *testing.T) {
		store := NewAgentStore()
		assignment := &AgentAssignment{
			TenantID:   "tenant-1",
			WorkflowID: "wf-2",
			NodeID:     "node-2",
			AgentID:    "agent-2",
		}

		created, err := store.CreateAssignment(assignment)
		if err != nil {
			t.Fatalf("CreateAssignment failed: %v", err)
		}
		if created.ID == "" {
			t.Error("Expected auto-generated ID")
		}
	})

	t.Run("get assignment by ID", func(t *testing.T) {
		store := NewAgentStore()
		assignment := &AgentAssignment{
			ID:         "assign-2",
			TenantID:   "tenant-1",
			WorkflowID: "wf-3",
			NodeID:     "node-3",
			AgentID:    "agent-3",
		}
		store.CreateAssignment(assignment)

		got, err := store.GetByID("assign-2")
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.WorkflowID != "wf-3" {
			t.Errorf("Expected wf-3, got %s", got.WorkflowID)
		}
		if got.NodeID != "node-3" {
			t.Errorf("Expected node-3, got %s", got.NodeID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := NewAgentStore()
		_, err := store.GetByID("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent assignment")
		}
	})

	t.Run("list by workflow", func(t *testing.T) {
		store := NewAgentStore()

		assignment1 := &AgentAssignment{
			ID:         "assign-a",
			TenantID:   "tenant-1",
			WorkflowID: "wf-4",
			NodeID:     "node-a",
			AgentID:    "agent-a",
		}
		assignment2 := &AgentAssignment{
			ID:         "assign-b",
			TenantID:   "tenant-1",
			WorkflowID: "wf-4",
			NodeID:     "node-b",
			AgentID:    "agent-b",
		}
		store.CreateAssignment(assignment1)
		store.CreateAssignment(assignment2)

		assignments, err := store.ListByWorkflow("wf-4")
		if err != nil {
			t.Fatalf("ListByWorkflow failed: %v", err)
		}
		if len(assignments) != 2 {
			t.Errorf("Expected 2 assignments, got %d", len(assignments))
		}
	})

	t.Run("list by workflow with no assignments", func(t *testing.T) {
		store := NewAgentStore()
		assignments, err := store.ListByWorkflow("non-existent")
		if err != nil {
			t.Fatalf("ListByWorkflow failed: %v", err)
		}
		if len(assignments) != 0 {
			t.Errorf("Expected 0 assignments, got %d", len(assignments))
		}
	})

	t.Run("assignment with parameters", func(t *testing.T) {
		store := NewAgentStore()
		assignment := &AgentAssignment{
			ID:         "assign-params",
			TenantID:   "tenant-1",
			WorkflowID: "wf-5",
			NodeID:     "node-5",
			AgentID:    "agent-5",
			Parameters: map[string]interface{}{
				"batch_size": 100,
				"timeout":    30,
			},
		}

		created, err := store.CreateAssignment(assignment)
		if err != nil {
			t.Fatalf("CreateAssignment failed: %v", err)
		}

		got, err := store.GetByID(created.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.Parameters["batch_size"] != 100 {
			t.Errorf("Expected batch_size 100, got %v", got.Parameters["batch_size"])
		}
	})
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
