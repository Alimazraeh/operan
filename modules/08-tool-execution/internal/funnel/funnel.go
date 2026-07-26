// Package funnel is the one path every capability execution takes.
//
// Anyone can call a vendor's API. What this layer sells is the sentence "an
// AI performed this action, within its authority, under policy, and here is
// the record" — and that sentence is only true if there is exactly one door
// and everything goes through it:
//
//	resolve binding → validate input → policy check → authority check
//	→ dispatch → record.
//
// Refusals are first-class outcomes. An unbound capability blocks with
// blocked_no_binding rather than pretending; invalid input never reaches a
// provider; a policy deny and an authority shortfall each stop the action and
// say so. Every attempt — refused or completed — lands in the immutable
// invocation record, because a denied action is as much a fact as a done one.
package funnel

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/operan/modules/08-tool-execution/internal/policyclient"
	"github.com/operan/modules/08-tool-execution/internal/positionclient"
	"github.com/operan/modules/08-tool-execution/internal/schema"
	"github.com/operan/modules/08-tool-execution/internal/simulated"
	"github.com/operan/modules/08-tool-execution/internal/store"
)

// Request is one attempt to perform a capability.
type Request struct {
	CapabilityID string                 `json:"capability_id"`
	Input        map[string]interface{} `json:"input"`
	Actor        store.Actor            `json:"actor"`
	Correlation  store.Correlation      `json:"correlation"`
}

// Funnel wires the stages together.
type Funnel struct {
	Capabilities *store.CapabilityStore
	Providers    *store.ProviderStore
	Bindings     *store.BindingStore
	Invocations  *store.InvocationStore
	Validator    *schema.Validator
	Policy       *policyclient.Client
	// Positions resolves the acting seat's real autonomy tier from Module 05
	// at invoke time. The authority stage never trusts req.Actor.AutonomyTier
	// for the decision — that field is the caller's claim, echoed onto the
	// record for audit, not evidence.
	Positions *positionclient.Client
}

// Invoke runs the funnel. The returned invocation is already recorded; err is
// non-nil only for malformed requests that never got far enough to record.
// authorization is the caller's bearer credential, forwarded to the policy
// engine so its audit sees who was really asking.
func (f *Funnel) Invoke(ctx context.Context, authorization, tenantID string, req Request) (*store.Invocation, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant is required")
	}
	if req.CapabilityID == "" {
		return nil, fmt.Errorf("capability_id is required")
	}
	cap, ok := f.Capabilities.Get(req.CapabilityID)
	if !ok {
		return nil, fmt.Errorf("unknown capability %q — the vocabulary is deliberate; verbs are added on evidence", req.CapabilityID)
	}

	started := time.Now()
	inv := &store.Invocation{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		CapabilityID: cap.ID,
		SideEffect:   cap.SideEffect,
		Actor:        req.Actor,
		Correlation:  req.Correlation,
		Input:        req.Input,
	}
	finish := func(status, errMsg string) *store.Invocation {
		inv.Status = status
		inv.Error = errMsg
		inv.DurationMS = time.Since(started).Milliseconds()
		f.Invocations.Append(inv)
		log.Printf("[CAPABILITY] %s %s actor=%s/%s dept=%s → %s %s",
			cap.ID, cap.SideEffect, inv.Actor.Type, inv.Actor.ID,
			inv.Correlation.DepartmentID, status, errMsg)
		return inv
	}

	// 1 — Resolve the binding: department override, then tenant default.
	// Unbound is an explicit, recorded stop.
	binding := f.Bindings.Resolve(tenantID, req.Correlation.DepartmentID, cap.ID)
	if binding == nil {
		return finish(store.InvocationBlockedNoBind,
			fmt.Sprintf("no enabled binding for %s — bind it to a provider before SOPs can perform it", cap.ID)), nil
	}
	provider, ok := f.Providers.Get(tenantID, binding.ProviderID)
	if !ok || provider.Status != "active" {
		return finish(store.InvocationBlockedNoBind,
			fmt.Sprintf("binding %s names provider %s, which is missing or disabled", binding.ID, binding.ProviderID)), nil
	}
	inv.ProviderID = provider.ID
	inv.ProviderKind = provider.Kind
	inv.ProviderTool = binding.ProviderTool
	inv.Simulated = binding.Simulated || provider.Kind == simulated.Kind

	// 2 — Validate the input against the capability contract. Writes must be
	// typed; nothing malformed reaches a provider.
	if err := f.Validator.Validate(cap.ID, cap.InputSchema, req.Input); err != nil {
		return finish(store.InvocationInvalidInput, "input does not match the capability contract: "+err.Error()), nil
	}

	// 3 — Policy. Deny closed: every answer other than an explicit allow
	// stops the action.
	decision := f.Policy.Check(ctx, authorization, tenantID, req.Actor.ID, cap.ID, cap.SideEffect)
	inv.PolicyDecision = decision.Reason
	if !decision.Allowed {
		return finish(store.InvocationDeniedPolicy, "policy: "+decision.Reason), nil
	}

	// 4 — Authority. The org chart is the tool-authorization boundary: the
	// seat's autonomy tier must clear the capability's minimum. That tier is
	// never taken from the request — req.Actor.AutonomyTier is the caller's
	// claim, and any authenticated caller can claim anything. The funnel
	// resolves the seat's real tier from Module 05, live, every time. An
	// actor whose tier cannot be established ranks below every tier and is
	// refused write verbs — unknown authority is not authority, and neither
	// is an unverifiable claim.
	resolution := f.Positions.Resolve(ctx, authorization, tenantID, req.Correlation.DepartmentID, req.Actor.PositionID)
	inv.ResolvedAutonomyTier = resolution.Tier
	if store.AutonomyRank(resolution.Tier) < store.AutonomyRank(cap.MinAutonomy) {
		return finish(store.InvocationDeniedAuthority,
			fmt.Sprintf("%s requires autonomy %q; the acting seat resolves to %q (caller claimed %q) — %s",
				cap.ID, cap.MinAutonomy, orUnknown(resolution.Tier), orUnknown(req.Actor.AutonomyTier), resolution.Reason)), nil
	}

	// 5 — Dispatch. Only the simulated provider executes today; registering
	// any other kind is declaring intent, not capability, and it refuses
	// honestly rather than pretending.
	switch provider.Kind {
	case simulated.Kind:
		output, ref, err := simulated.Execute(cap.ID, req.Input)
		if err != nil {
			return finish(store.InvocationFailed, err.Error()), nil
		}
		inv.Output = output
		inv.ExternalRef = ref
		return finish(store.InvocationCompleted, ""), nil
	default:
		return finish(store.InvocationFailed,
			fmt.Sprintf("provider kind %q is registered but has no executor yet — only %q executes today", provider.Kind, simulated.Kind)), nil
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "no established tier"
	}
	return s
}
