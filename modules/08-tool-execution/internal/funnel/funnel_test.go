package funnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/08-tool-execution/internal/policyclient"
	"github.com/operan/modules/08-tool-execution/internal/positionclient"
	"github.com/operan/modules/08-tool-execution/internal/schema"
	"github.com/operan/modules/08-tool-execution/internal/store"
	"github.com/operan/modules/08-tool-execution/internal/vocab"
)

// allowAll and denyAll stand in for Module 10.
func policyStub(t *testing.T, allow bool, reason string) *policyclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/policies/evaluate" {
			t.Errorf("policy path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if allow {
			w.Write([]byte(`{"allowed":true,"reason":"` + reason + `"}`))
		} else {
			w.Write([]byte(`{"allowed":false,"reason":"` + reason + `"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return policyclient.New(srv.URL)
}

// positionStub stands in for Module 05's GET /departments/{id}/org-chart.
// tiers maps position ID to the seat's real, server-resolved autonomy tier —
// the fact the funnel must check, never the actor's own claim. Department
// scoping is M05's own concern, not this stub's: it answers the same
// position list for any department path, which is enough to exercise M08's
// client (URL construction, headers, response parsing) faithfully without
// re-testing M05's per-department isolation.
func positionStub(t *testing.T, tiers map[string]string) *positionclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/departments/") || !strings.HasSuffix(r.URL.Path, "/org-chart") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Tenant-ID") == "" {
			t.Errorf("position lookup missing X-Tenant-ID")
		}
		type pos struct {
			ID           string `json:"id"`
			AutonomyTier string `json:"autonomy_tier"`
		}
		positions := make([]pos, 0, len(tiers))
		for id, tier := range tiers {
			positions = append(positions, pos{ID: id, AutonomyTier: tier})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"root_position_id": "",
			"positions":        positions,
			"edges":            []interface{}{},
		})
	}))
	t.Cleanup(srv.Close)
	return positionclient.New(srv.URL)
}

// positionStubUnreachable returns a client pointed at a server that is
// already closed, so every request fails at the transport level — Module 05
// down, not just answering unfavourably.
func positionStubUnreachable(t *testing.T) *positionclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	return positionclient.New(url)
}

func newFunnel(t *testing.T, policy *policyclient.Client, positions *positionclient.Client) *Funnel {
	t.Helper()
	caps := store.NewCapabilityStore()
	vocab.SeedCapabilities(caps)
	f := &Funnel{
		Capabilities: caps,
		Providers:    store.NewProviderStore(),
		Bindings:     store.NewBindingStore(),
		Invocations:  store.NewInvocationStore(),
		Validator:    schema.NewValidator(),
		Policy:       policy,
		Positions:    positions,
	}
	return f
}

func bindSimulated(f *Funnel, tenant, dept, capability string) {
	pid := "prov-" + tenant
	if _, ok := f.Providers.Get(tenant, pid); !ok {
		f.Providers.Put(&store.Provider{ID: pid, TenantID: tenant, Kind: "simulated", Name: "Sim", Status: "active"})
	}
	f.Bindings.Put(&store.CapabilityBinding{
		ID: "bind-" + tenant + "-" + dept + "-" + capability, TenantID: tenant, DepartmentID: dept,
		CapabilityID: capability, ProviderID: pid, ProviderTool: capability,
		Enabled: true, Simulated: true,
	})
}

// executeActor claims the "execute" tier for position "pos-1". Tests that
// use it must pair it with a position stub that actually resolves "pos-1" to
// a real tier — the claim alone proves nothing since WO-4.
func executeActor() store.Actor {
	return store.Actor{Type: "agent", ID: "agent-1", PositionID: "pos-1", AutonomyTier: "execute"}
}

// An unbound capability blocks with a recorded reason — never a silent
// pass-through, never an invented provider. This is the funnel's first
// promise and the one that replaces the old echo. The funnel stops before
// the authority stage, so no position resolution is needed.
func TestUnboundCapabilityBlocksHonestly(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"), positionStub(t, nil))
	inv, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "itsm.ticket.assign",
		Input:        map[string]interface{}{"team": "helpdesk"},
		Actor:        executeActor(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != store.InvocationBlockedNoBind {
		t.Fatalf("status = %s, want blocked_no_binding", inv.Status)
	}
	if !strings.Contains(inv.Error, "bind it to a provider") {
		t.Fatalf("the refusal must tell the operator what to do: %q", inv.Error)
	}
	// Refusals are audit records too.
	if got := f.Invocations.List("t1", "", "", "", 10); len(got) != 1 {
		t.Fatalf("refusal was not recorded: %d rows", len(got))
	}
}

// Input that does not match the capability contract never reaches a
// provider. The funnel stops before the authority stage.
func TestInvalidInputNeverReachesTheProvider(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"), positionStub(t, nil))
	bindSimulated(f, "t1", "", "itsm.ticket.assign")
	inv, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "itsm.ticket.assign",
		Input:        map[string]interface{}{"reason": "no team named"}, // "team" is required
		Actor:        executeActor(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != store.InvocationInvalidInput {
		t.Fatalf("status = %s, want invalid_input", inv.Status)
	}
	if inv.Output != nil || inv.ExternalRef != nil {
		t.Fatal("invalid input produced provider output")
	}
}

// A policy deny stops the action and the decision text lands on the record.
// The funnel stops before the authority stage.
func TestPolicyDenyStopsTheAction(t *testing.T) {
	f := newFunnel(t, policyStub(t, false, "destructive verbs frozen during month-end close"), positionStub(t, nil))
	bindSimulated(f, "t1", "", "itops.backup.restore")
	inv, _ := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "itops.backup.restore",
		Input:        map[string]interface{}{"request_ref": "req-1", "restore": "/finance"},
		Actor:        executeActor(),
	})
	if inv.Status != store.InvocationDeniedPolicy {
		t.Fatalf("status = %s, want denied_policy", inv.Status)
	}
	if !strings.Contains(inv.PolicyDecision, "month-end") {
		t.Fatalf("policy reason not recorded: %q", inv.PolicyDecision)
	}
}

// The org chart is the tool-authorization boundary: a seat the funnel
// resolves below the capability's minimum is refused, regardless of what the
// caller claims for itself.
func TestAuthorityBelowMinimumIsDenied(t *testing.T) {
	positions := positionStub(t, map[string]string{
		"pos-draft":     "draft",
		"pos-recommend": "recommend",
	})
	f := newFunnel(t, policyStub(t, true, "allowed"), positions)
	bindSimulated(f, "t1", "", "identity.access.grant")

	for _, tc := range []struct{ name, positionID string }{
		{"resolved draft", "pos-draft"},
		{"resolved recommend", "pos-recommend"},
		{"no position named on the request", ""},
	} {
		inv, _ := f.Invoke(context.Background(), "Bearer test", "t1", Request{
			CapabilityID: "identity.access.grant",
			Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
			// AutonomyTier claims "execute" in every case — irrelevant, since
			// the decision runs on the resolved tier, not the claim.
			Actor:       store.Actor{Type: "agent", ID: "a1", PositionID: tc.positionID, AutonomyTier: "execute"},
			Correlation: store.Correlation{DepartmentID: "dept-1"},
		})
		if inv.Status != store.InvocationDeniedAuthority {
			t.Fatalf("%s: status = %s, want denied_authority", tc.name, inv.Status)
		}
	}
}

// A seat the funnel resolves at or above the minimum passes — decided by
// what Module 05's org chart says, not by what the caller wrote in the
// request. The claim here deliberately understates the seat's real tier, to
// prove the claim is not the thing being checked.
func TestAuthorityAtOrAboveMinimumPasses(t *testing.T) {
	positions := positionStub(t, map[string]string{"pos-coordinate": "coordinate"})
	f := newFunnel(t, policyStub(t, true, "allowed"), positions)
	bindSimulated(f, "t1", "", "identity.access.grant")

	inv, _ := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "identity.access.grant",
		Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
		Actor:        store.Actor{Type: "user", ID: "dana", PositionID: "pos-coordinate", AutonomyTier: "recommend"},
		Correlation:  store.Correlation{DepartmentID: "dept-1"},
	})
	if inv.Status != store.InvocationCompleted {
		t.Fatalf("coordinate-resolved seat refused: %s (%s)", inv.Status, inv.Error)
	}
	if inv.ResolvedAutonomyTier != "coordinate" {
		t.Fatalf("resolved tier not recorded: %q", inv.ResolvedAutonomyTier)
	}
}

// This is the defect WO-4 fixes: before this client existed, the funnel
// compared the caller's own claim against the capability's minimum, so any
// authenticated caller could assert "coordinate" and clear any authority
// check. A body claiming coordinate for a seat that actually holds draft
// must be refused, and both the claim and the resolution must land on the
// invocation record — a caller misrepresenting its own authority is a
// security-relevant fact, not something to silently correct.
func TestClaimedTierNeverOverridesResolvedTier(t *testing.T) {
	positions := positionStub(t, map[string]string{"pos-42": "draft"})
	f := newFunnel(t, policyStub(t, true, "allowed"), positions)
	bindSimulated(f, "t1", "", "identity.access.grant")

	inv, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "identity.access.grant",
		Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
		Actor:        store.Actor{Type: "agent", ID: "agent-x", PositionID: "pos-42", AutonomyTier: "coordinate"},
		Correlation:  store.Correlation{DepartmentID: "dept-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != store.InvocationDeniedAuthority {
		t.Fatalf("status = %s, want denied_authority (claimed coordinate, seat actually holds draft)", inv.Status)
	}
	if inv.Actor.AutonomyTier != "coordinate" {
		t.Fatalf("claimed tier not preserved on the record: %q", inv.Actor.AutonomyTier)
	}
	if inv.ResolvedAutonomyTier != "draft" {
		t.Fatalf("resolved tier not recorded: %q", inv.ResolvedAutonomyTier)
	}
}

// A position absent from the org chart resolves to no authority — not a
// default tier, not the caller's claim.
func TestUnresolvablePositionIsNoAuthority(t *testing.T) {
	positions := positionStub(t, map[string]string{"pos-real": "coordinate"}) // "pos-missing" is not in this org chart
	f := newFunnel(t, policyStub(t, true, "allowed"), positions)
	bindSimulated(f, "t1", "", "identity.access.grant")

	inv, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "identity.access.grant",
		Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
		Actor:        store.Actor{Type: "agent", ID: "agent-x", PositionID: "pos-missing", AutonomyTier: "coordinate"},
		Correlation:  store.Correlation{DepartmentID: "dept-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != store.InvocationDeniedAuthority {
		t.Fatalf("status = %s, want denied_authority for a position absent from the org chart", inv.Status)
	}
	if inv.ResolvedAutonomyTier != "" {
		t.Fatalf("an unresolvable position must not resolve to a tier: %q", inv.ResolvedAutonomyTier)
	}
}

// Module 05 unreachable must deny, never fall back to the caller's claim or
// to any default tier — the same fail-closed trade policyclient makes when
// Module 10 cannot be reached.
func TestPositionResolutionUnreachableDeniesClosed(t *testing.T) {
	positions := positionStubUnreachable(t)
	f := newFunnel(t, policyStub(t, true, "allowed"), positions)
	bindSimulated(f, "t1", "", "identity.access.grant")

	inv, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "identity.access.grant",
		Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
		Actor:        store.Actor{Type: "agent", ID: "agent-x", PositionID: "pos-42", AutonomyTier: "coordinate"},
		Correlation:  store.Correlation{DepartmentID: "dept-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != store.InvocationDeniedAuthority {
		t.Fatalf("status = %s, want denied_authority when Module 05 is unreachable", inv.Status)
	}
	if !strings.Contains(inv.Error, "department engine unreachable") {
		t.Fatalf("the refusal reason must say why: %q", inv.Error)
	}
	if inv.ResolvedAutonomyTier != "" {
		t.Fatalf("an unreachable department engine must not resolve a tier: %q", inv.ResolvedAutonomyTier)
	}
}

// The happy path: completed, flagged simulated, carrying an external ref,
// the policy decision, and the resolved tier — the full sentence the layer
// exists to be able to say.
func TestCompletedInvocationCarriesTheWholeRecord(t *testing.T) {
	positions := positionStub(t, map[string]string{"pos-1": "execute"})
	f := newFunnel(t, policyStub(t, true, "allowed by change-window policy"), positions)
	bindSimulated(f, "t1", "", "itsm.ticket.assign")
	inv, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "itsm.ticket.assign",
		Input:        map[string]interface{}{"team": "helpdesk", "ticket_ref": "req-9"},
		Actor:        executeActor(),
		Correlation:  store.Correlation{RequestID: "req-9", NodeID: "ticket-02", DepartmentID: "dept-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != store.InvocationCompleted {
		t.Fatalf("status = %s (%s)", inv.Status, inv.Error)
	}
	if !inv.Simulated {
		t.Fatal("a simulated execution not flagged simulated is the credibility trap the plan warned about")
	}
	if inv.ExternalRef == nil || inv.ExternalRef.System != "simulated-itsm" {
		t.Fatalf("external ref missing or wrong: %+v", inv.ExternalRef)
	}
	if inv.PolicyDecision != "allowed by change-window policy" {
		t.Fatalf("policy decision not recorded: %q", inv.PolicyDecision)
	}
	if inv.Actor.ID != "agent-1" || inv.Correlation.RequestID != "req-9" {
		t.Fatal("actor or correlation lost")
	}
	if inv.ResolvedAutonomyTier != "execute" {
		t.Fatalf("resolved tier not recorded: %q", inv.ResolvedAutonomyTier)
	}
}

// A department's own binding overrides the tenant default.
func TestDepartmentBindingOverridesTenantDefault(t *testing.T) {
	positions := positionStub(t, map[string]string{"pos-1": "execute"})
	f := newFunnel(t, policyStub(t, true, "allowed"), positions)
	bindSimulated(f, "t1", "", "comms.message.send")       // tenant default
	bindSimulated(f, "t1", "dept-9", "comms.message.send") // department override
	inv, _ := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "comms.message.send",
		Input:        map[string]interface{}{"channel": "ops", "message": "hello"},
		Actor:        executeActor(),
		Correlation:  store.Correlation{DepartmentID: "dept-9"},
	})
	if inv.Status != store.InvocationCompleted {
		t.Fatalf("status = %s (%s)", inv.Status, inv.Error)
	}
	// The resolved binding is the department one.
	b := f.Bindings.Resolve("t1", "dept-9", "comms.message.send")
	if b == nil || b.DepartmentID != "dept-9" {
		t.Fatalf("department binding did not win: %+v", b)
	}
}

// An unknown verb is refused before anything records: the vocabulary is
// deliberate.
func TestUnknownCapabilityIsRefusedOutright(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"), positionStub(t, nil))
	_, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "itsm.ticket.teleport",
		Input:        map[string]interface{}{},
		Actor:        executeActor(),
	})
	if err == nil {
		t.Fatal("unknown capability was accepted")
	}
}
