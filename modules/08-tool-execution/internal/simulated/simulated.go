// Package simulated is the first capability provider: it answers every verb
// realistically without touching any external system.
//
// It exists so the governed execution path — binding, validation, policy,
// authority, audit — can be built and proven before any live integration, and
// so a customer can watch their SOPs run end to end before pointing them at
// production systems. The one iron rule is that it never passes for real:
// every record it produces is flagged simulated, the flag survives into the
// audit trail and the request timeline, and swapping it for a live provider
// is a binding change, not a code change.
package simulated

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/operan/modules/08-tool-execution/internal/store"
)

// Kind is the provider kind this package implements.
const Kind = "simulated"

// Execute performs one capability against nothing at all, returning output
// shaped like the capability's contract plus an ExternalRef into a simulated
// system, so downstream consumers exercise exactly the shapes a live
// provider will produce.
func Execute(capabilityID string, input map[string]interface{}) (map[string]interface{}, *store.ExternalRef, error) {
	id := "SIM-" + strings.ToUpper(uuid.NewString()[:8])
	str := func(k string) string {
		if v, ok := input[k].(string); ok {
			return v
		}
		return ""
	}
	ref := func(system, kind string) *store.ExternalRef {
		return &store.ExternalRef{
			System: system, Kind: kind, ID: id,
			URL: fmt.Sprintf("https://%s.simulated.operan.local/%s/%s", system, kind, id),
		}
	}

	switch capabilityID {
	case "itsm.ticket.assign":
		return map[string]interface{}{
			"assigned_team": str("team"),
			"ticket":        id,
			"note":          "assigned in the simulated ITSM — no live system was touched",
		}, ref("simulated-itsm", "ticket"), nil

	case "itsm.ticket.close":
		return map[string]interface{}{
			"closed":     true,
			"resolution": bound(str("resolution"), 500),
		}, ref("simulated-itsm", "ticket"), nil

	case "identity.access.grant":
		return map[string]interface{}{
			"granted": true,
			"grant":   bound(str("grant"), 300),
			"note":    "grant recorded in the simulated directory — no live system was touched",
		}, ref("simulated-idp", "grant"), nil

	case "identity.access.revoke":
		return map[string]interface{}{
			"revoked": true,
			"revoke":  bound(str("revoke"), 300),
		}, ref("simulated-idp", "revocation"), nil

	case "itops.monitor.check":
		return map[string]interface{}{
			"target": bound(str("target"), 200),
			"status": "healthy",
			"checks": []string{"reachability", "service-status", "recent-errors"},
			"note":   "checks answered by the simulated monitoring stack",
		}, ref("simulated-monitoring", "check"), nil

	case "itops.backup.restore":
		return map[string]interface{}{
			"restored": true,
			"restore":  bound(str("restore"), 300),
			"note":     "restore recorded by the simulated backup system — no data moved",
		}, ref("simulated-backup", "restore-job"), nil

	case "itsm.asset.update":
		return map[string]interface{}{
			"updated": true,
			"action":  str("action"),
			"details": bound(str("details"), 300),
		}, ref("simulated-cmdb", "asset"), nil

	case "comms.message.send":
		return map[string]interface{}{
			"delivered": true,
			"channel":   str("channel"),
			"note":      "delivered to the simulated channel — nobody real was messaged",
		}, ref("simulated-comms", "message"), nil
	}

	// A capability the simulated provider does not know is an honest failure,
	// not an improvised success.
	return nil, nil, fmt.Errorf("the simulated provider does not implement %s", capabilityID)
}

func bound(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
