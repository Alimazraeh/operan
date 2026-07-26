package restorecmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/operan/tools/demo-fixture/internal/fixture"
)

// noSleep drives pollUntil at full speed in tests — the loop's decision
// logic still runs once per configured attempt, it just doesn't wait on a
// real clock between them.
func noSleep(time.Duration) {}

// provisionForReplayTest runs a real Provision against mp so replay tests
// exercise Replay against an actually-resolved user id, not a fabricated
// one — the same way cmd/demo-fixture chains the two calls.
func provisionForReplayTest(t *testing.T, mp *mockPlatform, f *fixture.Fixture) (*Result, Config) {
	t.Helper()
	var out strings.Builder
	cfg := testConfig(mp, false, &out)
	res, err := Provision(context.Background(), cfg, f, NewClients(cfg))
	if err != nil {
		t.Fatalf("Provision (setup for replay test): %v\nlog:\n%s", err, out.String())
	}
	return res, cfg
}

func TestReplayHappyPathWithApproval(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	pr, cfg := provisionForReplayTest(t, mp, f)

	danaID := pr.UserIDByRef["dana"]
	if danaID == "" {
		t.Fatal("setup: dana's user id was not resolved by Provision")
	}
	mp.seedApprovalForUser(danaID, "appr-1", "Grant replay-test read access")
	mp.requestStatusScript = []string{"open", "awaiting_approval", "completed"}

	var out strings.Builder
	cfg.Out = &out
	result, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 10})
	if err != nil {
		t.Fatalf("Replay: %v\nlog:\n%s", err, out.String())
	}

	if result.FinalStatus != "completed" {
		t.Errorf("FinalStatus = %q, want completed", result.FinalStatus)
	}
	if !result.Approved {
		t.Error("Approved = false, want true")
	}
	if result.ApprovalItemID != "appr-1" {
		t.Errorf("ApprovalItemID = %q, want appr-1", result.ApprovalItemID)
	}
	if result.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3 (open, awaiting_approval, completed)", result.Attempts)
	}

	mp.mu.Lock()
	approvalStatus := mp.approvals["appr-1"].Status
	mp.mu.Unlock()
	if approvalStatus != "approved" {
		t.Errorf("mock approval status = %q, want approved (Approve must have actually been called)", approvalStatus)
	}
}

func TestReplayCompletesWithoutAnyGate(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	pr, cfg := provisionForReplayTest(t, mp, f)

	mp.requestStatusScript = []string{"open", "completed"} // never gates
	var out strings.Builder
	cfg.Out = &out

	result, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 10})
	if err != nil {
		t.Fatalf("Replay: %v\nlog:\n%s", err, out.String())
	}
	if result.FinalStatus != "completed" {
		t.Errorf("FinalStatus = %q, want completed", result.FinalStatus)
	}
	if result.Approved {
		t.Error("Approved = true, want false — no gate ever appeared")
	}
}

func TestReplayDryRunMakesZeroNetworkCalls(t *testing.T) {
	var requestsSeen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsSeen++
		t.Errorf("dry-run replay made a real HTTP call: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	var out strings.Builder
	cfg := Config{
		TenantControlPlaneURL: srv.URL, IdentityAccessURL: srv.URL, AgentRegistryURL: srv.URL,
		DepartmentsURL: srv.URL, HumanSupervisionURL: srv.URL,
		AdminPassword: "x", UserPassword: "y", DryRun: true, Out: &out,
	}
	f := sampleFixture()

	result, err := Replay(context.Background(), cfg, f, NewClients(cfg), nil, ReplayOptions{})
	if err != nil {
		t.Fatalf("Replay (dry-run): unexpected error: %v", err)
	}
	if requestsSeen != 0 {
		t.Fatalf("dry-run replay triggered %d real HTTP request(s)", requestsSeen)
	}
	if result.RequestID != dryRunPlaceholder {
		t.Errorf("RequestID = %q, want the dry-run placeholder", result.RequestID)
	}
	if !strings.Contains(out.String(), "/departments/{department-id}/requests") {
		t.Errorf("dry-run plan should describe the create-request call, got:\n%s", out.String())
	}
}

func TestReplayErrorsOnAmbiguousApprovalMatch(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	pr, cfg := provisionForReplayTest(t, mp, f)

	danaID := pr.UserIDByRef["dana"]
	mp.seedApprovalForUser(danaID, "appr-1", "Grant replay-test read access")
	mp.seedApprovalForUser(danaID, "appr-2", "Grant replay-test read access")
	mp.requestStatusScript = []string{"awaiting_approval"}

	var out strings.Builder
	cfg.Out = &out
	_, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 5})
	if err == nil {
		t.Fatal("Replay: expected an error for an ambiguous approval match, got nil")
	}
	if !strings.Contains(err.Error(), "cannot disambiguate") {
		t.Errorf("error should explain the ambiguity, got: %v", err)
	}
}

func TestReplayErrorsWhenNoApprovalMatches(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	pr, cfg := provisionForReplayTest(t, mp, f)
	// No approval seeded at all.
	mp.requestStatusScript = []string{"awaiting_approval"}

	var out strings.Builder
	cfg.Out = &out
	_, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 5})
	if err == nil {
		t.Fatal("Replay: expected an error when no approval matches, got nil")
	}
	if !strings.Contains(err.Error(), "no pending approval") {
		t.Errorf("error should explain no match was found, got: %v", err)
	}
}

func TestReplayErrorsOnTimeout(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	pr, cfg := provisionForReplayTest(t, mp, f)
	mp.requestStatusScript = []string{"open"} // never advances (idx clamps to last entry)

	var out strings.Builder
	cfg.Out = &out
	result, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 4})
	if err == nil {
		t.Fatal("Replay: expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timing out, got: %v", err)
	}
	if result.Attempts != 4 {
		t.Errorf("Attempts = %d, want 4 (MaxAttempts reached)", result.Attempts)
	}
}

func TestReplayErrorsWhenRequestEndsRejected(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	pr, cfg := provisionForReplayTest(t, mp, f)
	mp.requestStatusScript = []string{"open", "rejected"}

	var out strings.Builder
	cfg.Out = &out
	result, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 10})
	if err == nil {
		t.Fatal("Replay: expected an error when the request ends rejected, got nil")
	}
	if result.FinalStatus != "rejected" {
		t.Errorf("FinalStatus = %q, want rejected", result.FinalStatus)
	}
	if !strings.Contains(err.Error(), "not completed") {
		t.Errorf("error should say it did not complete, got: %v", err)
	}
}

func TestReplayErrorsWhenGatedWithNoApproverRef(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	f.Replay.ApproverRef = "" // raised as admin; nobody to approve as
	pr, cfg := provisionForReplayTest(t, mp, f)
	mp.requestStatusScript = []string{"awaiting_approval"}

	var out strings.Builder
	cfg.Out = &out
	_, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{Sleep: noSleep, MaxAttempts: 5})
	if err == nil {
		t.Fatal("Replay: expected an error when a gate appears but no approver_ref was set, got nil")
	}
	if !strings.Contains(err.Error(), "no named approver") {
		t.Errorf("error should explain there was no named approver, got: %v", err)
	}
}

func TestReplayErrorsWhenNoReplaySpec(t *testing.T) {
	mp := newMockPlatform()
	defer mp.Close()
	f := sampleFixture()
	f.Replay = nil
	pr, cfg := provisionForReplayTest(t, mp, f)

	_, err := Replay(context.Background(), cfg, f, NewClients(cfg), pr, ReplayOptions{})
	if err == nil {
		t.Fatal("Replay: expected an error when the fixture has no replay section, got nil")
	}
}
