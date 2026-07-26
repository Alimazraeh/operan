package fixture

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// secretKeyMarkers are lower-cased substrings that must never appear as a
// map key anywhere in a fixture document. This is deliberately broad — a
// false positive costs someone a rename; a false negative costs a
// committed credential.
var secretKeyMarkers = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"api_key",
	"apikey",
	"credential",
	"private_key",
	"privatekey",
	"access_key",
	"accesskey",
	"client_secret",
	"auth_header",
	"authorization",
	"bearer",
	"ssh_key",
	"pgp_key",
}

// secretValueMarkers are substrings/prefixes that, when found as (or inside)
// a scalar string value, strongly suggest a live credential leaked into the
// fixture even under an innocuously-named key.
var secretValueMarkers = []string{
	"bearer ",
	"-----BEGIN ",
	"AKIA", // AWS access key id prefix
}

// ValidationError collects every problem found, rather than stopping at the
// first — a fixture is hand-edited YAML and a human fixing it wants the
// whole list in one pass.
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("fixture validation failed (%d issue(s)):\n  - %s",
		len(e.Issues), strings.Join(e.Issues, "\n  - "))
}

// Validate runs every check in this file and returns a single error
// aggregating all findings, or nil if the fixture is clean. raw is the
// original serialized document (YAML or JSON, both parse as YAML — JSON is
// a subset) and is used for the secret scan, which deliberately does not
// trust the typed struct: a field neither Fixture nor its nested types
// declare would otherwise be silently dropped by unmarshal and never
// checked.
func Validate(f *Fixture, raw []byte) error {
	var issues []string
	issues = append(issues, validateSchema(f)...)
	issues = append(issues, validateReferences(f)...)
	issues = append(issues, ScanForSecrets(raw)...)

	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}

func validateSchema(f *Fixture) []string {
	var issues []string
	req := func(cond bool, msg string) {
		if !cond {
			issues = append(issues, msg)
		}
	}

	req(f.SchemaVersion == SchemaVersion,
		fmt.Sprintf("schema_version: got %d, this tool understands %d", f.SchemaVersion, SchemaVersion))

	req(f.Metadata.Name != "", "metadata.name is required")
	req(f.Metadata.Provenance == ProvenanceLiveExport || f.Metadata.Provenance == ProvenanceHandAssembled,
		fmt.Sprintf("metadata.provenance must be %q or %q, got %q",
			ProvenanceLiveExport, ProvenanceHandAssembled, f.Metadata.Provenance))
	if f.Metadata.Provenance == ProvenanceLiveExport {
		req(f.Metadata.ExportedAt != "", "metadata.exported_at is required when provenance is live-export")
	}

	req(f.Tenant.Name != "", "tenant.name is required")
	switch f.Tenant.Plan {
	case "saas", "enterprise", "sovereign":
	default:
		issues = append(issues, fmt.Sprintf("tenant.plan must be one of saas|enterprise|sovereign, got %q", f.Tenant.Plan))
	}
	switch f.Tenant.Region {
	case "me-east-1", "eu-west-1", "us-east-1", "ap-south-1", "on-prem":
	default:
		issues = append(issues, fmt.Sprintf("tenant.region must be one of me-east-1|eu-west-1|us-east-1|ap-south-1|on-prem, got %q", f.Tenant.Region))
	}
	switch f.Tenant.IsolationLevel {
	case "namespace", "encryption", "network_policy":
	default:
		issues = append(issues, fmt.Sprintf("tenant.isolation_level must be one of namespace|encryption|network_policy, got %q", f.Tenant.IsolationLevel))
	}

	req(len(f.Users) > 0, "users: at least one user is required (the demo needs someone to log in as)")
	seenUserRef := map[string]bool{}
	seenEmail := map[string]bool{}
	for i, u := range f.Users {
		p := fmt.Sprintf("users[%d]", i)
		req(u.Ref != "", p+".ref is required")
		req(u.Email != "", p+".email is required")
		req(u.DisplayName != "", p+".display_name is required")
		req(len(u.RoleIDs) > 0, p+".role_ids: at least one role is required")
		if u.Ref != "" {
			if seenUserRef[u.Ref] {
				issues = append(issues, p+".ref: duplicate ref "+u.Ref)
			}
			seenUserRef[u.Ref] = true
		}
		if u.Email != "" {
			if seenEmail[u.Email] {
				issues = append(issues, p+".email: duplicate email "+u.Email)
			}
			seenEmail[u.Email] = true
		}
	}

	seenAgentRef := map[string]bool{}
	for i, a := range f.Agents {
		p := fmt.Sprintf("agents[%d]", i)
		req(a.Ref != "", p+".ref is required")
		req(a.ID != "", p+".id is required (caller-supplied UUID — see M04 CreateAgentRequest.ID)")
		req(isUUID(a.ID), p+".id must be a UUID, got "+a.ID)
		req(a.Name != "", p+".name is required")
		req(a.Role != "", p+".role is required")
		if a.Ref != "" {
			if seenAgentRef[a.Ref] {
				issues = append(issues, p+".ref: duplicate ref "+a.Ref)
			}
			seenAgentRef[a.Ref] = true
		}
	}

	req(f.Department.TemplateID != "", "department.template_id is required")
	req(f.Department.Environment != "", "department.environment is required (M05 DeployRequest rejects an empty environment)")

	for i, sb := range f.Department.SeatBindings {
		p := fmt.Sprintf("department.seat_bindings[%d]", i)
		req(sb.PositionID != "", p+".position_id is required")
		switch sb.HolderType {
		case "human":
			req(sb.UserRef != "", p+".user_ref is required when holder_type is human")
			req(sb.AgentRef == "", p+".agent_ref must be empty when holder_type is human")
		case "ai_agent":
			req(sb.AgentRef != "", p+".agent_ref is required when holder_type is ai_agent")
			req(sb.UserRef == "", p+".user_ref must be empty when holder_type is ai_agent")
		case "vacant":
			req(sb.UserRef == "" && sb.AgentRef == "", p+": vacant seats must not set user_ref or agent_ref")
		default:
			issues = append(issues, fmt.Sprintf("%s.holder_type must be human|ai_agent|vacant, got %q", p, sb.HolderType))
		}
	}

	if f.Replay != nil {
		req(f.Replay.ServiceID != "", "replay.service_id is required")
		req(f.Replay.Title != "", "replay.title is required")
	}

	return issues
}

// validateReferences checks that every *Ref field points at something that
// actually exists in the fixture — a dangling ref would only surface at
// restore time otherwise, mid-run, against a real (or in CI, mocked)
// cluster.
func validateReferences(f *Fixture) []string {
	var issues []string

	userRefs := map[string]bool{}
	for _, u := range f.Users {
		if u.Ref != "" {
			userRefs[u.Ref] = true
		}
	}
	agentRefs := map[string]bool{}
	for _, a := range f.Agents {
		if a.Ref != "" {
			agentRefs[a.Ref] = true
		}
	}

	for i, sb := range f.Department.SeatBindings {
		p := fmt.Sprintf("department.seat_bindings[%d]", i)
		if sb.UserRef != "" && !userRefs[sb.UserRef] {
			issues = append(issues, fmt.Sprintf("%s.user_ref %q does not match any users[].ref", p, sb.UserRef))
		}
		if sb.AgentRef != "" && !agentRefs[sb.AgentRef] {
			issues = append(issues, fmt.Sprintf("%s.agent_ref %q does not match any agents[].ref", p, sb.AgentRef))
		}
	}

	if f.Replay != nil && f.Replay.ApproverRef != "" && !userRefs[f.Replay.ApproverRef] {
		issues = append(issues, fmt.Sprintf("replay.approver_ref %q does not match any users[].ref", f.Replay.ApproverRef))
	}

	return issues
}

// ScanForSecrets walks the raw document as a generic tree (independent of
// the typed Fixture struct, so an unrecognized field cannot hide from it)
// looking for key names or scalar values that suggest a committed
// credential.
func ScanForSecrets(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var doc interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		// Not this function's job to report parse errors — Load already did.
		return nil
	}
	var findings []string
	scanNode("$", doc, &findings)
	sort.Strings(findings)
	return findings
}

func scanNode(path string, node interface{}, findings *[]string) {
	switch v := node.(type) {
	case map[string]interface{}:
		for k, val := range v {
			lk := strings.ToLower(k)
			for _, marker := range secretKeyMarkers {
				if strings.Contains(lk, marker) {
					*findings = append(*findings, fmt.Sprintf("%s.%s: key name looks credential-shaped (matches %q) — fixtures must contain no secrets", path, k, marker))
					break
				}
			}
			scanNode(path+"."+k, val, findings)
		}
	case []interface{}:
		for i, val := range v {
			scanNode(fmt.Sprintf("%s[%d]", path, i), val, findings)
		}
	case string:
		lower := strings.ToLower(v)
		for _, marker := range secretValueMarkers {
			if strings.Contains(lower, strings.ToLower(marker)) {
				*findings = append(*findings, fmt.Sprintf("%s: value looks credential-shaped (contains %q) — fixtures must contain no secrets", path, marker))
				break
			}
		}
	}
}

func isUUID(s string) bool {
	// RFC 4122 textual form: 8-4-4-4-12 hex digits. Deliberately not using
	// google/uuid here to keep this package dependency-free besides yaml.
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHex(byte(c)) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
