package cadence

import (
	"context"
	"testing"
	"time"

	"github.com/operan/modules/05-department-template-engine/internal/store"
)

func dept(id string, cadences ...store.CadenceEntry) *store.Department {
	return &store.Department{
		ID: id, TenantID: "t1", Name: "IT", Status: "operational",
		BusinessLogic: &store.BusinessLogic{OperatingCadence: cadences},
	}
}

func TestDueDailyFiresOncePerSlot(t *testing.T) {
	s := &Scheduler{Briefings: NewBriefingStore(), BriefHour: 7}
	d := dept("d1")
	entry := store.CadenceEntry{Name: "Daily digest", Frequency: "daily"}

	// 06:59 — today's slot hasn't arrived; yesterday's slot is the anchor,
	// and with no briefing at all it is due (catch-up).
	s.Now = func() time.Time { return time.Date(2026, 7, 25, 6, 59, 0, 0, time.UTC) }
	if !s.due(d, entry) {
		t.Fatal("no briefing ever filed — should be due")
	}
	s.Briefings.Add(Briefing{DepartmentID: "d1", CadenceName: "Daily digest",
		CreatedAt: time.Date(2026, 7, 24, 7, 1, 0, 0, time.UTC)})
	if s.due(d, entry) {
		t.Fatal("yesterday's slot already served — 06:59 must not refire")
	}
	// 07:01 — new slot, due again.
	s.Now = func() time.Time { return time.Date(2026, 7, 25, 7, 1, 0, 0, time.UTC) }
	if !s.due(d, entry) {
		t.Fatal("07:01 with yesterday's briefing — today's slot is due")
	}
	s.Briefings.Add(Briefing{DepartmentID: "d1", CadenceName: "Daily digest",
		CreatedAt: time.Date(2026, 7, 25, 7, 2, 0, 0, time.UTC)})
	if s.due(d, entry) {
		t.Fatal("already served today's slot — must not refire")
	}
}

func TestDueWeeklyAnchorsOnMonday(t *testing.T) {
	s := &Scheduler{Briefings: NewBriefingStore(), BriefHour: 7}
	d := dept("d1")
	entry := store.CadenceEntry{Name: "Weekly review", Frequency: "weekly"}

	// Saturday 2026-07-25; last Monday slot = 2026-07-20 07:00.
	s.Now = func() time.Time { return time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC) }
	s.Briefings.Add(Briefing{DepartmentID: "d1", CadenceName: "Weekly review",
		CreatedAt: time.Date(2026, 7, 20, 7, 5, 0, 0, time.UTC)})
	if s.due(d, entry) {
		t.Fatal("this week's Monday already served")
	}
	s.Now = func() time.Time { return time.Date(2026, 7, 27, 7, 30, 0, 0, time.UTC) } // next Monday
	if !s.due(d, entry) {
		t.Fatal("new Monday slot — due")
	}
}

func TestFireDueFilesStatsOnlyBriefingWithoutOrch(t *testing.T) {
	depts := store.NewDepartmentStore()
	reqs := store.NewRequestStore()
	d := dept("ignored", store.CadenceEntry{Name: "Daily digest", Frequency: "daily"})
	created, err := depts.Create(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.Create(&store.ServiceRequest{
		TenantID: "t1", DepartmentID: created.ID, Title: "open one",
		Status: "in_progress", TokensUsed: 500,
	}); err != nil {
		t.Fatal(err)
	}

	s := &Scheduler{Departments: depts, Requests: reqs, Briefings: NewBriefingStore(),
		BriefHour: 7, TestEvery: time.Second}
	s.Now = func() time.Time { return time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC) }
	s.FireDue(context.Background())

	got := s.Briefings.List("t1", created.ID, 10)
	if len(got) != 1 {
		t.Fatalf("briefings = %d, want 1", len(got))
	}
	if got[0].Stats.OpenRequests != 1 || got[0].Stats.TokensLast24h != 500 {
		t.Errorf("stats = %+v", got[0].Stats)
	}
	if got[0].Content == "" {
		t.Error("stats-only briefing must still carry content")
	}
	// Second pass within TestEvery: not due again.
	s.FireDue(context.Background())
	if n := len(s.Briefings.List("t1", created.ID, 10)); n != 1 {
		t.Errorf("refired within interval: %d briefings", n)
	}
}

func TestBriefingStoreRoundtripAndBound(t *testing.T) {
	s := NewBriefingStore()
	for i := 0; i < keepPerDepartment+5; i++ {
		s.Add(Briefing{TenantID: "t1", DepartmentID: "d1", CadenceName: "Daily digest",
			Content: "x", CreatedAt: time.Now().Add(time.Duration(i) * time.Minute)})
	}
	if n := len(s.List("t1", "d1", 0)); n != keepPerDepartment {
		t.Fatalf("bound = %d, want %d", n, keepPerDepartment)
	}
	data, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	s2 := NewBriefingStore()
	if err := s2.Import(data); err != nil {
		t.Fatal(err)
	}
	if n := len(s2.List("t1", "d1", 0)); n != keepPerDepartment {
		t.Fatalf("roundtrip = %d, want %d", n, keepPerDepartment)
	}
	if s2.Latest("d1", "Daily digest") == nil {
		t.Fatal("Latest lost after import")
	}
}
