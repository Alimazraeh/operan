package store

import (
	"testing"
	"time"
)

func TestDelegationStore_Create(t *testing.T) {
	store := NewDelegationStore()

	d := &Delegation{
		WorkflowID:       "wf-1",
		NodeID:           "node-1",
		OriginalAgentID:  "agent-1",
		DelegatedAgentID: "agent-2",
		TenantID:         "tenant-1",
		Status:           DelegationPending,
		Reason:           "Agent unavailable",
	}

	t.Run("creates delegation with auto-generated ID", func(t *testing.T) {
		store.Create(d)
		if d.ID == "" {
			t.Error("Expected auto-generated ID")
		}
		if d.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
		if d.Status != DelegationPending {
			t.Errorf("Expected status %s, got %s", DelegationPending, d.Status)
		}
	})
}

func TestDelegationStore_GetByID(t *testing.T) {
	store := NewDelegationStore()

	store.Create(&Delegation{
		ID:              "del-1",
		WorkflowID:      "wf-1",
		NodeID:          "node-1",
		OriginalAgentID: "agent-1",
		DelegatedAgentID: "agent-2",
		TenantID:        "tenant-1",
		Status:          DelegationPending,
	})

	t.Run("get delegation by ID", func(t *testing.T) {
		d, ok := store.GetByID("del-1")
		if !ok {
			t.Fatal("Expected to find delegation")
		}
		if d.WorkflowID != "wf-1" {
			t.Errorf("Expected workflow-id wf-1, got %s", d.WorkflowID)
		}
		if d.OriginalAgentID != "agent-1" {
			t.Errorf("Expected original-agent agent-1, got %s", d.OriginalAgentID)
		}
	})

	t.Run("get non-existent delegation", func(t *testing.T) {
		_, ok := store.GetByID("non-existent")
		if ok {
			t.Error("Expected false for non-existent delegation")
		}
	})

	t.Run("returns copy, not reference", func(t *testing.T) {
		d, ok := store.GetByID("del-1")
		if !ok {
			t.Fatal("Expected to find delegation")
		}
		// Mutate the returned copy
		d.WorkflowID = "mutated"
		// Get again and verify original is unchanged
		d2, ok := store.GetByID("del-1")
		if !ok {
			t.Fatal("Expected to find delegation again")
		}
		if d2.WorkflowID != "wf-1" {
			t.Errorf("Expected original workflow-id wf-1, got %s (mutation leaked)", d2.WorkflowID)
		}
	})
}

func TestDelegationStore_ListByWorkflow(t *testing.T) {
	store := NewDelegationStore()

	store.Create(&Delegation{ID: "del-1", WorkflowID: "wf-1", NodeID: "n1", OriginalAgentID: "a1", DelegatedAgentID: "a2", TenantID: "tenant-1", Status: DelegationPending})
	store.Create(&Delegation{ID: "del-2", WorkflowID: "wf-1", NodeID: "n2", OriginalAgentID: "a1", DelegatedAgentID: "a3", TenantID: "tenant-1", Status: DelegationAccepted})
	store.Create(&Delegation{ID: "del-3", WorkflowID: "wf-2", NodeID: "n3", OriginalAgentID: "a2", DelegatedAgentID: "a4", TenantID: "tenant-1", Status: DelegationRejected})

	t.Run("list delegations by workflow", func(t *testing.T) {
		dels := store.ListByWorkflow("wf-1")
		if len(dels) != 2 {
			t.Errorf("Expected 2 delegations for wf-1, got %d", len(dels))
		}
	})

	t.Run("list delegations for non-existent workflow", func(t *testing.T) {
		dels := store.ListByWorkflow("wf-999")
		if len(dels) != 0 {
			t.Errorf("Expected 0 delegations, got %d", len(dels))
		}
	})

	t.Run("returns copies", func(t *testing.T) {
		dels := store.ListByWorkflow("wf-1")
		if len(dels) == 0 {
			t.Fatal("Expected at least 1 delegation")
		}
		dels[0].WorkflowID = "mutated"
		dels2 := store.ListByWorkflow("wf-1")
		if dels2[0].WorkflowID == "mutated" {
			t.Error("Mutation leaked into subsequent list")
		}
	})
}

func TestDelegationStore_UpdateStatus(t *testing.T) {
	store := NewDelegationStore()

	store.Create(&Delegation{ID: "del-1", WorkflowID: "wf-1", NodeID: "n1", OriginalAgentID: "a1", DelegatedAgentID: "a2", TenantID: "tenant-1", Status: DelegationPending})

	t.Run("update to accepted", func(t *testing.T) {
		ok := store.UpdateStatus("del-1", DelegationAccepted)
		if !ok {
			t.Error("Expected UpdateStatus to succeed")
		}
		d, ok := store.GetByID("del-1")
		if !ok {
			t.Fatal("Expected to find delegation")
		}
		if d.Status != DelegationAccepted {
			t.Errorf("Expected status %s, got %s", DelegationAccepted, d.Status)
		}
	})

	t.Run("update to rejected", func(t *testing.T) {
		ok := store.UpdateStatus("del-1", DelegationRejected)
		if !ok {
			t.Error("Expected UpdateStatus to succeed")
		}
		d, _ := store.GetByID("del-1")
		if d.Status != DelegationRejected {
			t.Errorf("Expected status %s, got %s", DelegationRejected, d.Status)
		}
	})

	t.Run("update to completed", func(t *testing.T) {
		ok := store.UpdateStatus("del-1", DelegationCompleted)
		if !ok {
			t.Error("Expected UpdateStatus to succeed")
		}
		d, _ := store.GetByID("del-1")
		if d.Status != DelegationCompleted {
			t.Errorf("Expected status %s, got %s", DelegationCompleted, d.Status)
		}
	})

	t.Run("update non-existent delegation", func(t *testing.T) {
		ok := store.UpdateStatus("non-existent", DelegationPending)
		if ok {
			t.Error("Expected UpdateStatus to fail for non-existent")
		}
	})

	t.Run("update from pending to rejected", func(t *testing.T) {
		store.Create(&Delegation{ID: "del-2", WorkflowID: "wf-2", NodeID: "n1", OriginalAgentID: "a1", DelegatedAgentID: "a2", TenantID: "tenant-1", Status: DelegationPending})
		ok := store.UpdateStatus("del-2", DelegationRejected)
		if !ok {
			t.Error("Expected UpdateStatus to succeed")
		}
		d, _ := store.GetByID("del-2")
		if d.Status != DelegationRejected {
			t.Errorf("Expected status %s, got %s", DelegationRejected, d.Status)
		}
	})
}

func TestDelegationStore_CreatedAtTime(t *testing.T) {
	store := NewDelegationStore()
	before := time.Now().UTC()

	store.Create(&Delegation{
		WorkflowID:       "wf-1",
		NodeID:           "n1",
		OriginalAgentID:  "a1",
		DelegatedAgentID: "a2",
		TenantID:         "tenant-1",
		Status:           DelegationPending,
	})
	after := time.Now().UTC()

	d, ok := store.GetByID(store.ListByWorkflow("wf-1")[0].ID)
	if !ok {
		t.Fatal("Expected to find delegation")
	}
	if d.CreatedAt.Before(before) || d.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v not within [%v, %v]", d.CreatedAt, before, after)
	}
}
