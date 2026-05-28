package store

import (
	"testing"
	"time"
)

func TestEscalationStore_Create(t *testing.T) {
	store := NewEscalationStore()

	esc := &Escalation{
		WorkflowID: "wf-1",
		NodeID:     "node-1",
		TenantID:   "tenant-1",
		Reason:     "Node failure",
		Status:     EscalationPending,
		Severity:   EscalationHigh,
	}

	t.Run("creates escalation record", func(t *testing.T) {
		store.Create(esc)
		if esc.ID == "" {
			t.Error("Expected auto-generated ID")
		}
		if esc.WorkflowID != "wf-1" {
			t.Errorf("Expected workflow-id wf-1, got %s", esc.WorkflowID)
		}
	})
}

func TestEscalationStore_ListByWorkflow(t *testing.T) {
	store := NewEscalationStore()

	store.Create(&Escalation{WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", Severity: EscalationHigh, Status: EscalationPending})
	store.Create(&Escalation{WorkflowID: "wf-1", NodeID: "n2", TenantID: "tenant-1", Severity: EscalationMedium, Status: EscalationAcknowledged})
	store.Create(&Escalation{WorkflowID: "wf-2", NodeID: "n3", TenantID: "tenant-1", Severity: EscalationLow, Status: EscalationResolved})

	t.Run("list escalations by workflow", func(t *testing.T) {
		records := store.ListByWorkflow("wf-1")
		if len(records) != 2 {
			t.Errorf("Expected 2 records for wf-1, got %d", len(records))
		}
	})

	t.Run("list escalations for non-existent workflow", func(t *testing.T) {
		records := store.ListByWorkflow("wf-999")
		if len(records) != 0 {
			t.Errorf("Expected 0 records, got %d", len(records))
		}
	})
}

func TestEscalationStore_Acknowledge(t *testing.T) {
	store := NewEscalationStore()

	store.Create(&Escalation{ID: "esc-1", WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", Severity: EscalationHigh, Status: EscalationPending})

	t.Run("acknowledge escalation", func(t *testing.T) {
		ok := store.Acknowledge("esc-1")
		if !ok {
			t.Error("Expected Acknowledge to succeed")
		}

		records := store.ListByWorkflow("wf-1")
		if len(records) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(records))
		}
		if records[0].Status != EscalationAcknowledged {
			t.Errorf("Expected Acknowledged status, got %s", records[0].Status)
		}
		if records[0].AcknowledgedAt == nil {
			t.Error("Expected AcknowledgedAt to be set")
		}
	})

	t.Run("acknowledge non-existent", func(t *testing.T) {
		ok := store.Acknowledge("non-existent")
		if ok {
			t.Error("Expected Acknowledge to fail for non-existent")
		}
	})

	t.Run("acknowledge already acknowledged", func(t *testing.T) {
		store.Create(&Escalation{ID: "esc-2", WorkflowID: "wf-2", NodeID: "n1", TenantID: "tenant-1", Status: EscalationAcknowledged})
		ok := store.Acknowledge("esc-2")
		if ok {
			t.Error("Expected Acknowledge to fail for already acknowledged")
		}
	})
}

func TestEscalationStore_Resolve(t *testing.T) {
	store := NewEscalationStore()

	store.Create(&Escalation{ID: "esc-3", WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", Status: EscalationPending})

	t.Run("resolve escalation", func(t *testing.T) {
		ok := store.Resolve("esc-3")
		if !ok {
			t.Error("Expected Resolve to succeed")
		}

		records := store.ListByWorkflow("wf-1")
		if len(records) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(records))
		}
		if records[0].Status != EscalationResolved {
			t.Errorf("Expected Resolved status, got %s", records[0].Status)
		}
		if records[0].ResolvedAt == nil {
			t.Error("Expected ResolvedAt to be set")
		}
	})
}

func TestRetryRecordStore_Create(t *testing.T) {
	store := NewRetryRecordStore()

	record := &RetryRecord{
		WorkflowID:    "wf-1",
		NodeID:        "node-1",
		TenantID:      "tenant-1",
		AttemptNumber: 1,
		Status:        RetryPending,
		ErrorCode:     "node_timeout",
	}

	t.Run("creates retry record", func(t *testing.T) {
		store.Create(record)
		if record.ID == "" {
			t.Error("Expected auto-generated ID")
		}
	})
}

func TestRetryRecordStore_ListByWorkflow(t *testing.T) {
	store := NewRetryRecordStore()

	store.Create(&RetryRecord{WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", AttemptNumber: 1, Status: RetryPending})
	store.Create(&RetryRecord{WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", AttemptNumber: 2, Status: RetrySuccess})
	store.Create(&RetryRecord{WorkflowID: "wf-2", NodeID: "n2", TenantID: "tenant-1", AttemptNumber: 1, Status: RetryPending})

	t.Run("list retry records by workflow", func(t *testing.T) {
		records := store.ListByWorkflow("wf-1")
		if len(records) != 2 {
			t.Errorf("Expected 2 records for wf-1, got %d", len(records))
		}
	})

	t.Run("list retry records for non-existent workflow", func(t *testing.T) {
		records := store.ListByWorkflow("wf-999")
		if len(records) != 0 {
			t.Errorf("Expected 0 records, got %d", len(records))
		}
	})
}

func TestRetryRecordStore_UpdateStatus(t *testing.T) {
	store := NewRetryRecordStore()

	store.Create(&RetryRecord{ID: "retry-1", WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", AttemptNumber: 1, Status: RetryPending})

	t.Run("update retry status", func(t *testing.T) {
		ok := store.UpdateStatus("retry-1", RetrySuccess, "", "success")
		if !ok {
			t.Error("Expected UpdateStatus to succeed")
		}

		records := store.ListByWorkflow("wf-1")
		if len(records) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(records))
		}
		if records[0].Status != RetrySuccess {
			t.Errorf("Expected Success status, got %s", records[0].Status)
		}
		if records[0].CompletedAt == nil {
			t.Error("Expected CompletedAt to be set")
		}
	})

	t.Run("update non-existent", func(t *testing.T) {
		ok := store.UpdateStatus("non-existent", RetrySuccess, "", "success")
		if ok {
			t.Error("Expected UpdateStatus to fail for non-existent")
		}
	})
}

func TestStackHealthStore_Create(t *testing.T) {
	store := NewStackHealthStore()

	health := &StackHealthEntry{
		StackType: "langgraph",
		StackName: "default",
		TenantID:  "tenant-1",
		Status:    StackHealthy,
		Config:    map[string]interface{}{"max_nodes": 1000},
	}

	t.Run("creates health entry", func(t *testing.T) {
		store.Create(health)
		if health.ID == "" {
			t.Error("Expected auto-generated ID")
		}
	})
}

func TestStackHealthStore_GetLatest(t *testing.T) {
	store := NewStackHealthStore()

	now1 := time.Now().UTC()
	now2 := now1.Add(time.Second)

	e1 := &StackHealthEntry{StackType: "langgraph", StackName: "default", TenantID: "tenant-1", Status: StackHealthy, At: now1}
	e2 := &StackHealthEntry{StackType: "langgraph", StackName: "default", TenantID: "tenant-1", Status: StackDegraded, At: now2}

	store.Create(e1)
	store.Create(e2)

	t.Run("get latest health entry", func(t *testing.T) {
		latest := store.GetLatest()
		if latest == nil {
			t.Fatal("Expected latest entry, got nil")
		}
		if latest.Status != StackDegraded {
			t.Errorf("Expected latest status '%s', got %s", StackDegraded, latest.Status)
		}
	})

	t.Run("get latest for empty store", func(t *testing.T) {
		emptyStore := NewStackHealthStore()
		latest := emptyStore.GetLatest()
		if latest != nil {
			t.Errorf("Expected nil for empty store, got %+v", latest)
		}
	})
}

func TestStackHealthStore_Update(t *testing.T) {
	store := NewStackHealthStore()

	entry := &StackHealthEntry{StackType: "langgraph", StackName: "default", TenantID: "tenant-1", Status: StackHealthy}
	store.Create(entry)

	t.Run("update health entry", func(t *testing.T) {
		entry.Status = StackDegraded
		store.Update(entry)

		latest := store.GetLatest()
		if latest.Status != StackDegraded {
			t.Errorf("Expected 'degraded', got %s", latest.Status)
		}
	})

	t.Run("update non-existent entry", func(t *testing.T) {
		entry := &StackHealthEntry{ID: "non-existent", Status: StackHealthy}
		store.Update(entry) // Should silently ignore
	})
}

func TestStackHealthStore_Delete(t *testing.T) {
	store := NewStackHealthStore()

	entry := &StackHealthEntry{StackType: "langgraph", StackName: "default", TenantID: "tenant-1", Status: StackHealthy}
	store.Create(entry)

	t.Run("delete health entry", func(t *testing.T) {
		ok := store.Delete(entry.ID)
		if !ok {
			t.Error("Expected Delete to succeed")
		}

		_, ok = store.GetByID(entry.ID)
		if ok {
			t.Error("Expected GetByID to fail after deletion")
		}
	})

	t.Run("delete non-existent entry", func(t *testing.T) {
		ok := store.Delete("non-existent")
		if ok {
			t.Error("Expected Delete to fail for non-existent")
		}
	})
}

func TestStackHealthStore_ListByStack(t *testing.T) {
	store := NewStackHealthStore()

	store.Create(&StackHealthEntry{
		TenantID: "tenant-1",
		StackType: "langgraph",
		Status: StackHealthy,
		Stacks: map[string]*StackHealthModule{
			"langgraph": {Status: StackHealthy},
		},
	})
	store.Create(&StackHealthEntry{
		TenantID: "tenant-1",
		StackType: "temporal",
		Status: StackHealthy,
		Stacks: map[string]*StackHealthModule{
			"temporal": {Status: StackHealthy},
		},
	})
	store.Create(&StackHealthEntry{
		TenantID: "tenant-2",
		StackType: "langgraph",
		Status: StackDegraded,
		Stacks: map[string]*StackHealthModule{
			"langgraph": {Status: StackDegraded},
		},
	})

	t.Run("list health entries by stack and tenant", func(t *testing.T) {
		// Note: ListByStack returns all entries for the tenant, 
		// but filters the Stacks map to only contain the requested stack type
		entries := store.ListByStack("tenant-1", "langgraph")
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries for tenant-1 (returns all, filters map), got %d", len(entries))
		}
		// The first entry (langgraph) should have langgraph in Stacks
		// The second entry (temporal) should have empty Stacks (since it doesn't have langgraph)
		hasLanggraphMap := false
		hasEmptyMap := false
		for _, e := range entries {
			if _, ok := e.Stacks["langgraph"]; ok {
				hasLanggraphMap = true
			}
			if len(e.Stacks) == 0 {
				hasEmptyMap = true
			}
		}
		if !hasLanggraphMap {
			t.Error("Expected at least one entry with langgraph in Stacks map")
		}
		if !hasEmptyMap {
			t.Error("Expected at least one entry with empty Stacks map")
		}
	})

	t.Run("list health entries across tenants", func(t *testing.T) {
		// When tenantID is empty string, it won't match any entries (all have non-empty TenantID)
		entries := store.ListByStack("", "langgraph")
		if len(entries) != 0 {
			t.Errorf("Expected 0 entries for empty tenantID, got %d", len(entries))
		}
	})

	t.Run("list non-existent stack", func(t *testing.T) {
		// Returns all tenant-1 entries but with empty Stacks map
		entries := store.ListByStack("tenant-1", "nonexistent")
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries for tenant-1 (returns all, filters map), got %d", len(entries))
		}
		// Verify Stacks maps are empty
		for _, e := range entries {
			if len(e.Stacks) != 0 {
				t.Errorf("Expected empty Stacks map for non-matching stackType, got %v", e.Stacks)
			}
		}
	})
}

func TestEscalationStore_GetByID(t *testing.T) {
	store := NewEscalationStore()

	store.Create(&Escalation{ID: "esc-1", WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", Status: EscalationPending, Severity: EscalationHigh})

	t.Run("get escalation by ID", func(t *testing.T) {
		esc, ok := store.GetByID("esc-1")
		if !ok {
			t.Fatal("Expected to find escalation")
		}
		if esc.WorkflowID != "wf-1" {
			t.Errorf("Expected wf-1, got %s", esc.WorkflowID)
		}
	})

	t.Run("get non-existent escalation", func(t *testing.T) {
		_, ok := store.GetByID("non-existent")
		if ok {
			t.Error("Expected false for non-existent escalation")
		}
	})
}

func TestRetryRecordStore_GetByID(t *testing.T) {
	store := NewRetryRecordStore()

	store.Create(&RetryRecord{ID: "retry-1", WorkflowID: "wf-1", NodeID: "n1", TenantID: "tenant-1", AttemptNumber: 1, Status: RetryPending})

	t.Run("get retry record by ID", func(t *testing.T) {
		record, ok := store.GetByID("retry-1")
		if !ok {
			t.Fatal("Expected to find retry record")
		}
		if record.WorkflowID != "wf-1" {
			t.Errorf("Expected wf-1, got %s", record.WorkflowID)
		}
	})

	t.Run("get non-existent retry record", func(t *testing.T) {
		_, ok := store.GetByID("non-existent")
		if ok {
			t.Error("Expected false for non-existent retry record")
		}
	})
}

func TestStackHealthStore_GetByID(t *testing.T) {
	store := NewStackHealthStore()

	store.Create(&StackHealthEntry{ID: "health-1", StackType: "langgraph", TenantID: "tenant-1", Status: StackHealthy})

	t.Run("get health entry by ID", func(t *testing.T) {
		entry, ok := store.GetByID("health-1")
		if !ok {
			t.Fatal("Expected to find health entry")
		}
		if entry.StackType != "langgraph" {
			t.Errorf("Expected langgraph, got %s", entry.StackType)
		}
	})

	t.Run("get non-existent health entry", func(t *testing.T) {
		_, ok := store.GetByID("non-existent")
		if ok {
			t.Error("Expected false for non-existent health entry")
		}
	})
}
