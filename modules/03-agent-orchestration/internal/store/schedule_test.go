package store

import (
	"testing"
)

func TestScheduleStore_Create(t *testing.T) {
	store := NewScheduleStore()

	t.Run("creates schedule with auto-generated ID", func(t *testing.T) {
		schedule := &Schedule{
			TenantID:         "tenant-1",
			Name:             "Daily Cleanup",
			Cron:             "0 0 * * *",
			WorkflowTemplateID: "wf-1",
			Enabled:          true,
		}

		created, err := store.Create(schedule)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if created.ID == "" {
			t.Error("Expected auto-generated ID")
		}
		if created.TenantID != "tenant-1" {
			t.Errorf("Expected tenant-1, got %s", created.TenantID)
		}
		if !created.Enabled {
			t.Error("Expected enabled=true")
		}
	})
}

func TestScheduleStore_List(t *testing.T) {
	store := NewScheduleStore()

	// Create 3 schedules
	for i := 1; i <= 3; i++ {
		schedule := &Schedule{
			TenantID:     "tenant-1",
			Name:         "Schedule " + string(rune('A'+i-1)),
			Cron:         "0 0 * * *",
			WorkflowTemplateID: "wf-1",
		}
		store.Create(schedule)
	}

	t.Run("returns all schedules", func(t *testing.T) {
		schedules, total, hasMore := store.List("tenant-1", 1, 50, nil)
		if total != 3 {
			t.Errorf("Expected 3 schedules, got %d", total)
		}
		if len(schedules) != 3 {
			t.Errorf("Expected 3 in slice, got %d", len(schedules))
		}
		if hasMore {
			t.Error("Expected hasMore=false")
		}
	})

	t.Run("tenant isolation", func(t *testing.T) {
		store := NewScheduleStore()
		s1 := &Schedule{
			TenantID: "tenant-a",
			Name:     "Schedule A",
			Cron:     "0 0 * * *",
		}
		s2 := &Schedule{
			TenantID: "tenant-b",
			Name:     "Schedule B",
			Cron:     "0 0 * * *",
		}
		store.Create(s1)
		store.Create(s2)

		_, total, _ := store.List("tenant-a", 1, 50, nil)
		if total != 1 {
			t.Errorf("Expected 1 schedule for tenant-a, got %d", total)
		}
	})

	t.Run("enabled filter", func(t *testing.T) {
		store := NewScheduleStore()
		enabled := &Schedule{
			TenantID:     "tenant-1",
			Name:         "Enabled Schedule",
			Cron:         "0 0 * * *",
			Enabled:      true,
		}
		disabled := &Schedule{
			TenantID:     "tenant-1",
			Name:         "Disabled Schedule",
			Cron:         "0 0 * * *",
			Enabled:      false,
		}
		store.Create(enabled)
		store.Create(disabled)

		_, total, _ := store.List("tenant-1", 1, 50, boolPtr(true))
		if total != 1 {
			t.Errorf("Expected 1 enabled schedule, got %d", total)
		}

		_, total, _ = store.List("tenant-1", 1, 50, boolPtr(false))
		if total != 1 {
			t.Errorf("Expected 1 disabled schedule, got %d", total)
		}
	})
}

func TestScheduleStore_Patch(t *testing.T) {
	store := NewScheduleStore()

	schedule := &Schedule{
		TenantID:     "tenant-1",
		Name:         "Test Schedule",
		Cron:         "0 0 * * *",
		WorkflowTemplateID: "wf-1",
		Enabled:      true,
	}
	created, _ := store.Create(schedule)

	t.Run("pause schedule (disable)", func(t *testing.T) {
		enabled := boolPtr(false)
		_, err := store.Patch(created.ID, nil, nil, nil, nil, enabled)
		if err != nil {
			t.Fatalf("Patch failed: %v", err)
		}

		got, _ := store.GetByID(created.ID)
		if got.Enabled {
			t.Error("Expected disabled after patch")
		}
	})

	t.Run("resume schedule (enable)", func(t *testing.T) {
		enabled := boolPtr(true)
		_, err := store.Patch(created.ID, nil, nil, nil, nil, enabled)
		if err != nil {
			t.Fatalf("Patch failed: %v", err)
		}

		got, _ := store.GetByID(created.ID)
		if !got.Enabled {
			t.Error("Expected enabled after patch")
		}
	})

	t.Run("update cron", func(t *testing.T) {
		cron := boolPtr(true) // just to get a non-nil cron value
		_ = cron
		cronExpr := "0 12 * * *"
		_, err := store.Patch(created.ID, nil, &cronExpr, nil, nil, nil)
		if err != nil {
			t.Fatalf("Patch failed: %v", err)
		}

		got, _ := store.GetByID(created.ID)
		if got.Cron != "0 12 * * *" {
			t.Errorf("Expected cron '0 12 * * *', got %s", got.Cron)
		}
	})

	t.Run("not found", func(t *testing.T) {
		name := "New Name"
		_, err := store.Patch("non-existent", &name, nil, nil, nil, nil)
		if err == nil {
			t.Error("Expected error for non-existent schedule")
		}
	})
}

func TestScheduleStore_Delete(t *testing.T) {
	store := NewScheduleStore()

	schedule := &Schedule{
		TenantID:     "tenant-1",
		Name:         "Test Schedule",
		Cron:         "0 0 * * *",
		WorkflowTemplateID: "wf-1",
	}
	created, _ := store.Create(schedule)

	t.Run("deletes schedule", func(t *testing.T) {
		err := store.Delete(created.ID)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = store.GetByID(created.ID)
		if err == nil {
			t.Error("Expected error after deletion")
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := store.Delete("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent schedule")
		}
	})
}

func boolPtr(b bool) *bool {
	return &b
}
