package funnel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/operan/modules/08-tool-execution/internal/policyclient"
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

func newFunnel(t *testing.T, policy *policyclient.Client) *Funnel {
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

func executeActor() store.Actor {
	return store.Actor{Type: "agent", ID: "agent-1", PositionID: "pos-1", AutonomyTier: "execute"}
}

// An unbound capability blocks with a recorded reason — never a silent
// pass-through, never an invented provider. This is the funnel's first
// promise and the one that replaces the old echo.
func TestUnboundCapabilityBlocksHonestly(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"))
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

// Input that does not match the capability contract never reaches a provider.
func TestInvalidInputNeverReachesTheProvider(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"))
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
func TestPolicyDenyStopsTheAction(t *testing.T) {
	f := newFunnel(t, policyStub(t, false, "destructive verbs frozen during month-end close"))
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

// The org chart is the tool-authorization boundary: a draft-tier seat cannot
// perform an execute-tier verb, and an actor with no established tier ranks
// below everything.
func TestAuthorityBelowMinimumIsDenied(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"))
	bindSimulated(f, "t1", "", "identity.access.grant")

	for _, tier := range []string{"draft", "recommend", ""} {
		inv, _ := f.Invoke(context.Background(), "Bearer test", "t1", Request{
			CapabilityID: "identity.access.grant",
			Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
			Actor:        store.Actor{Type: "agent", ID: "a1", AutonomyTier: tier},
		})
		if inv.Status != store.InvocationDeniedAuthority {
			t.Fatalf("tier %q: status = %s, want denied_authority", tier, inv.Status)
		}
	}
	// coordinate outranks execute and passes.
	inv, _ := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "identity.access.grant",
		Input:        map[string]interface{}{"request_ref": "req-1", "grant": "reporting access"},
		Actor:        store.Actor{Type: "user", ID: "dana", AutonomyTier: "coordinate"},
	})
	if inv.Status != store.InvocationCompleted {
		t.Fatalf("coordinate tier refused: %s (%s)", inv.Status, inv.Error)
	}
}

// The happy path: completed, flagged simulated, carrying an external ref and
// the policy decision — the full sentence the layer exists to be able to say.
func TestCompletedInvocationCarriesTheWholeRecord(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed by change-window policy"))
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
}

// A department's own binding overrides the tenant default.
func TestDepartmentBindingOverridesTenantDefault(t *testing.T) {
	f := newFunnel(t, policyStub(t, true, "allowed"))
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
	f := newFunnel(t, policyStub(t, true, "allowed"))
	_, err := f.Invoke(context.Background(), "Bearer test", "t1", Request{
		CapabilityID: "itsm.ticket.teleport",
		Input:        map[string]interface{}{},
		Actor:        executeActor(),
	})
	if err == nil {
		t.Fatal("unknown capability was accepted")
	}
}
