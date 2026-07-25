// Package vocab seeds the capability vocabulary.
//
// Every entry here is a permanent contract, so the list is grown from what the
// SOP catalogue actually names, never in anticipation. This first set covers
// the IT slice: ticketing, access, operations checks, restores, asset records
// and notifications — the verbs the it-medium template's tool_call and
// notification steps carry today.
package vocab

import "github.com/operan/modules/08-tool-execution/internal/store"

func obj(required []string, props map[string]interface{}) map[string]interface{} {
	s := map[string]interface{}{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "minLength": 1, "description": desc}
}

func optStr(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

// Capabilities returns the seeded vocabulary.
func Capabilities() []*store.Capability {
	return []*store.Capability{
		{
			ID: "itsm.ticket.assign", Domain: "itsm", Name: "Assign ticket",
			Description: "Route a ticket to the team that will work it.",
			SideEffect:  "write", MinAutonomy: "draft",
			InputSchema: obj([]string{"team"}, map[string]interface{}{
				"team":       str("team the ticket is assigned to"),
				"ticket_ref": optStr("the platform request this ticket tracks"),
				"reason":     optStr("why it goes to this team"),
			}),
		},
		{
			ID: "itsm.ticket.close", Domain: "itsm", Name: "Close ticket",
			Description: "Close a ticket with its resolution on the record.",
			SideEffect:  "write", MinAutonomy: "draft",
			InputSchema: obj([]string{"resolution"}, map[string]interface{}{
				"ticket_ref": optStr("the platform request this ticket tracks"),
				"resolution": str("what was done"),
			}),
		},
		{
			ID: "identity.access.grant", Domain: "identity", Name: "Grant access",
			Description: "Provision an approved access grant.",
			SideEffect:  "write", MinAutonomy: "execute",
			InputSchema: obj([]string{"request_ref", "grant"}, map[string]interface{}{
				"request_ref":   str("the approved request this grant implements"),
				"grant":         str("what is being granted, as approved"),
				"justification": optStr("why, for the audit record"),
			}),
		},
		{
			ID: "identity.access.revoke", Domain: "identity", Name: "Revoke access",
			Description: "Remove an existing access grant.",
			SideEffect:  "destructive", MinAutonomy: "execute",
			InputSchema: obj([]string{"request_ref", "revoke"}, map[string]interface{}{
				"request_ref":   str("the request this revocation implements"),
				"revoke":        str("what is being revoked"),
				"justification": optStr("why, for the audit record"),
			}),
		},
		{
			ID: "itops.monitor.check", Domain: "itops", Name: "Run monitoring check",
			Description: "Run a health or monitoring check against a target. Read-only.",
			SideEffect:  "read", MinAutonomy: "recommend",
			InputSchema: obj([]string{"target"}, map[string]interface{}{
				"target": str("what to check"),
				"scope":  optStr("why the check is being run"),
			}),
		},
		{
			ID: "itops.backup.restore", Domain: "itops", Name: "Restore from backup",
			Description: "Restore data from a snapshot. Overwrites the target.",
			SideEffect:  "destructive", MinAutonomy: "execute",
			InputSchema: obj([]string{"request_ref", "restore"}, map[string]interface{}{
				"request_ref": str("the approved request this restore implements"),
				"restore":     str("what to restore, as approved"),
			}),
		},
		{
			ID: "itsm.asset.update", Domain: "itsm", Name: "Update asset record",
			Description: "Create or update an asset or vendor record.",
			SideEffect:  "write", MinAutonomy: "draft",
			InputSchema: obj([]string{"action", "details"}, map[string]interface{}{
				"request_ref": optStr("the request driving the change"),
				"action":      str("create | update | retire"),
				"details":     str("the record change"),
			}),
		},
		{
			ID: "comms.message.send", Domain: "comms", Name: "Send message",
			Description: "Send a notification to a channel or address.",
			SideEffect:  "write", MinAutonomy: "recommend",
			InputSchema: obj([]string{"channel", "message"}, map[string]interface{}{
				"channel": str("email | ops | chat channel"),
				"to":      optStr("recipient, when the channel needs one"),
				"subject": optStr("subject line, when the channel has one"),
				"message": str("the message body"),
			}),
		},
	}
}
