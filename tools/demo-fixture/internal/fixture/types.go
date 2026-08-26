// Package fixture defines the versioned, human-readable seed format for
// restoring a demo tenant — currently smoke-tenant — from source instead of
// from a live database that Stage 1's persistence and execution changes may
// not survive.
//
// A Fixture is built exclusively from what the platform's own HTTP APIs
// return; nothing in this package or its siblings (apiclient, exportcmd,
// restorecmd) reaches into any module's internal Go packages or database.
// That mirrors the precedent set by Module 04's caller-supplied agent id:
// when demo data needs to be reproducible, the fix is an API affordance,
// never a direct write.
package fixture

// SchemaVersion is the current fixture format version. Bump it whenever a
// field is removed or its meaning changes incompatibly; additive fields
// (new optional keys) do not require a bump. Restore refuses to run against
// a SchemaVersion it does not recognize rather than guessing.
const SchemaVersion = 1

// Fixture is the top-level document. It is designed to marshal to both YAML
// and JSON via the same struct tags (yaml tags are lowercase snake_case,
// json tags match) so `export --format json` and the default YAML output
// describe identical data.
type Fixture struct {
	SchemaVersion int        `yaml:"schema_version" json:"schema_version"`
	Metadata      Metadata   `yaml:"metadata" json:"metadata"`
	Tenant        Tenant     `yaml:"tenant" json:"tenant"`
	Users         []User     `yaml:"users" json:"users"`
	Agents        []Agent    `yaml:"agents,omitempty" json:"agents,omitempty"`
	Department    Department `yaml:"department" json:"department"`

	// History is a read-only, best-effort snapshot of past requests captured
	// at export time. It documents that the demo tenant once produced real
	// governed capability execution; restore never replays it verbatim
	// (the originating request/invocation ids, timestamps and SLA clocks
	// belong to the source cluster and cannot be reproduced through the
	// public API — there is no caller-supplied-id affordance for requests,
	// unlike agents). See Replay for the thing restore actually re-runs.
	History []HistoricalRequest `yaml:"history,omitempty" json:"history,omitempty"`

	// Replay names one representative request that `restore --replay`
	// raises for real and drives to completion, to demonstrate that the
	// loop (draft → gate → approve → governed capability invocation →
	// completed) still works end to end after a restore. This is the
	// "replays one request to completion" step the CI job performs.
	Replay *ReplaySpec `yaml:"replay,omitempty" json:"replay,omitempty"`
}

// Metadata carries provenance so nobody mistakes a hand-assembled fixture
// for a verified live export, or vice versa.
type Metadata struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Provenance is one of:
	//   "live-export"     — produced by `demo-fixture export` against a
	//                        real cluster; ExportedAt and SourceCluster are
	//                        populated.
	//   "hand-assembled"   — transcribed from documentation/handoff notes by
	//                        someone who did not have cluster access at the
	//                        time. Must be replaced by a live export at the
	//                        first opportunity; restore --dry-run prints a
	//                        warning when it sees this value.
	Provenance string `yaml:"provenance" json:"provenance"`

	// ExportedAt is RFC 3339. Empty when Provenance is "hand-assembled".
	ExportedAt string `yaml:"exported_at,omitempty" json:"exported_at,omitempty"`

	// SourceNote is a free-text pointer to where the data came from — a
	// cluster description or a handoff document, never a hostname/URL with
	// embedded credentials.
	SourceNote string `yaml:"source_note,omitempty" json:"source_note,omitempty"`
}

const (
	ProvenanceLiveExport    = "live-export"
	ProvenanceHandAssembled = "hand-assembled"
)

// Tenant maps to Module 01's POST /v1/tenants body. Name is the natural key:
// M01 enforces it unique, and every other module's X-Tenant-ID header and
// JWT tenant_id claim use this same string, not the M01-assigned UUID — so
// restore treats Name as the identity and only the tenant control-plane
// record as best-effort (see restorecmd for why M01 registration is
// separable from the rest of the tenant "existing" in practice).
type Tenant struct {
	Name           string `yaml:"name" json:"name"`
	DisplayName    string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Plan           string `yaml:"plan" json:"plan"`                       // saas | enterprise | sovereign
	Region         string `yaml:"region" json:"region"`                   // me-east-1 | eu-west-1 | us-east-1 | ap-south-1 | on-prem
	IsolationLevel string `yaml:"isolation_level" json:"isolation_level"` // namespace | encryption | network_policy
	ContactEmail   string `yaml:"contact_email,omitempty" json:"contact_email,omitempty"`
}

// User maps to Module 02's POST /api/v1/iam/users body plus the fields
// restore needs to reference this user elsewhere in the fixture. There is
// deliberately no password or password-hash field anywhere on this type:
// credentials are supplied at restore time via flag or environment
// variable, never committed. validate.go additionally rejects any fixture
// whose raw YAML/JSON contains a handful of credential-shaped keys, as a
// second line of defense against a future field being added carelessly.
type User struct {
	// Ref is a local, fixture-only identifier (not sent to any API) used by
	// SeatBinding.UserRef and ReplaySpec.ApproverRef to point at this user
	// without knowing Module 02's generated id in advance.
	Ref         string   `yaml:"ref" json:"ref"`
	Email       string   `yaml:"email" json:"email"`
	DisplayName string   `yaml:"display_name" json:"display_name"`
	RoleIDs     []string `yaml:"role_ids" json:"role_ids"`
	MFAEnabled  bool     `yaml:"mfa_enabled,omitempty" json:"mfa_enabled,omitempty"`
}

// Agent maps to Module 04's POST /registry/agents body, using the
// caller-supplied id field that module gained specifically so provisioned
// agents can be restored under their original identity (see M04's
// CreateAgentRequest.ID doc comment — this fixture format leans on exactly
// that affordance).
type Agent struct {
	Ref          string   `yaml:"ref" json:"ref"`
	ID           string   `yaml:"id" json:"id"` // must be a UUID; caller-supplied
	Name         string   `yaml:"name" json:"name"`
	Role         string   `yaml:"role" json:"role"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Tools        []string `yaml:"tools,omitempty" json:"tools,omitempty"`
}

// Department describes one Module 05 department deployed from a template,
// plus the org-chart seat bindings that turn it into Dana's IT department.
// There is no caller-supplied id for departments (M05's DeployRequest has
// no id field) — restore locates an existing deployment by (TemplateID,
// Name) and only deploys when none is found; the department's own id is
// therefore never stable across an empty-cluster restore and is not part
// of this struct.
type Department struct {
	TemplateID string `yaml:"template_id" json:"template_id"`
	// Name overrides the template's own name at deploy time
	// (DeployRequest.department_name). Empty means "use the template's
	// name" and is also the value restore matches on when checking whether
	// the department already exists.
	Name          string        `yaml:"name,omitempty" json:"name,omitempty"`
	Environment   string        `yaml:"environment" json:"environment"` // required by M05's DeployRequest
	SeatBindings  []SeatBinding `yaml:"seat_bindings,omitempty" json:"seat_bindings,omitempty"`
	SyncWorkflows bool          `yaml:"sync_workflows,omitempty" json:"sync_workflows,omitempty"`
}

// SeatBinding maps to Module 05's PUT
// /departments/{id}/org-chart/{positionId}/holder body. Exactly one of
// UserRef/AgentRef is meaningful, selected by HolderType.
type SeatBinding struct {
	PositionID string `yaml:"position_id" json:"position_id"`
	HolderType string `yaml:"holder_type" json:"holder_type"` // human | ai_agent | vacant
	UserRef    string `yaml:"user_ref,omitempty" json:"user_ref,omitempty"`
	AgentRef   string `yaml:"agent_ref,omitempty" json:"agent_ref,omitempty"`
}

// HistoricalRequest is a read-only export of one past Module 05
// ServiceRequest plus its Module 08 capability invocations, kept for
// documentation value — it is what makes this fixture a sales asset and
// not just infrastructure. Restore never re-creates these.
type HistoricalRequest struct {
	ServiceID   string                 `yaml:"service_id" json:"service_id"`
	Title       string                 `yaml:"title" json:"title"`
	Priority    string                 `yaml:"priority,omitempty" json:"priority,omitempty"`
	Status      string                 `yaml:"status,omitempty" json:"status,omitempty"`
	Timeline    []HistoricalEvent      `yaml:"timeline,omitempty" json:"timeline,omitempty"`
	Invocations []HistoricalInvocation `yaml:"invocations,omitempty" json:"invocations,omitempty"`
}

// HistoricalEvent mirrors Module 05's RequestEvent (timeline entry).
type HistoricalEvent struct {
	At     string `yaml:"at,omitempty" json:"at,omitempty"` // RFC3339
	Kind   string `yaml:"kind" json:"kind"`
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"`
	Node   string `yaml:"node,omitempty" json:"node,omitempty"`
}

// HistoricalInvocation is a trimmed read of Module 08's store.Invocation —
// enough to show a real governed capability execution occurred, without
// carrying the full input/output payloads (which may contain data that,
// while not "secrets" in the credential sense, has no business being
// re-committed verbatim into source control).
type HistoricalInvocation struct {
	CapabilityID   string `yaml:"capability_id" json:"capability_id"`
	ProviderKind   string `yaml:"provider_kind,omitempty" json:"provider_kind,omitempty"`
	Status         string `yaml:"status" json:"status"`
	Simulated      bool   `yaml:"simulated" json:"simulated"`
	PolicyDecision string `yaml:"policy_decision,omitempty" json:"policy_decision,omitempty"`
}

// ReplaySpec is the one request restore --replay actually raises through
// Module 05's POST /departments/{id}/requests, then drives to completion by
// polling and, when a gate appears, approving as ApproverRef through
// Module 09.
type ReplaySpec struct {
	ServiceID string `yaml:"service_id" json:"service_id"`
	Title     string `yaml:"title" json:"title"`
	Body      string `yaml:"body,omitempty" json:"body,omitempty"`
	Priority  string `yaml:"priority,omitempty" json:"priority,omitempty"`

	// ApproverRef is a Users[].Ref: the fixture user restore logs in as to
	// approve the gate this request is expected to raise, if any. Empty
	// means the replayed request is expected to complete without a gate.
	ApproverRef string `yaml:"approver_ref,omitempty" json:"approver_ref,omitempty"`
}
