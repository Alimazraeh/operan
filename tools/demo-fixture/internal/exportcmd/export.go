// Package exportcmd implements `demo-fixture export`: it walks the live API
// surface of a running tenant and assembles a fixture.Fixture, exactly the
// way a human operator would if they were clicking through the portal —
// nothing here reads a database or a module's internal store directly.
package exportcmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/operan/tools/demo-fixture/internal/apiclient"
	"github.com/operan/tools/demo-fixture/internal/fixture"
)

// Config carries everything Run needs: where each module lives, the
// bootstrap credential, and which tenant/department to export. No field
// here is a secret that gets written back into the fixture — AdminPassword
// is used to obtain a token and then discarded.
type Config struct {
	TenantControlPlaneURL string
	IdentityAccessURL     string
	AgentRegistryURL      string
	DepartmentsURL        string
	ToolExecutionURL      string

	AdminPassword string
	TenantName    string

	// TemplateID selects which deployed department to export, in case the
	// tenant has more than one. DepartmentName narrows further when a
	// tenant deployed the same template twice under different names; empty
	// matches a department whose name was never overridden at deploy (see
	// DepartmentsClient.FindDepartment).
	TemplateID     string
	DepartmentName string

	// MaxHistoryItems caps how many past requests (and their invocations)
	// get captured into Fixture.History, keeping the committed file a
	// reasonable size. 0 uses a small sane default.
	MaxHistoryItems int

	FixtureName        string
	FixtureDescription string
	SourceNote         string

	// Out receives human-readable progress and warnings (e.g. "M01 has no
	// tenant record for this name — falling back to flag-supplied
	// metadata"). Tests pass a strings.Builder; the CLI passes os.Stderr.
	Out io.Writer
}

func (c Config) maxHistory() int {
	if c.MaxHistoryItems > 0 {
		return c.MaxHistoryItems
	}
	return 5
}

func (c Config) logf(format string, args ...interface{}) {
	if c.Out == nil {
		return
	}
	fmt.Fprintf(c.Out, format+"\n", args...)
}

// Clients bundles the apiclient wrappers Run needs. Exported as a struct
// (rather than Run taking Config and constructing everything itself) so
// tests can point every client at its own httptest.Server.
type Clients struct {
	Tenant      *apiclient.TenantClient
	IAM         *apiclient.IAMClient
	Registry    *apiclient.RegistryClient
	Departments *apiclient.DepartmentsClient
	Invocations *apiclient.InvocationsClient
}

// NewClients builds a Clients bundle from Config's base URLs, sharing one
// Doer (and therefore one underlying *http.Client) across all of them.
func NewClients(cfg Config) *Clients {
	d := apiclient.NewDoer()
	return &Clients{
		Tenant:      &apiclient.TenantClient{BaseURL: cfg.TenantControlPlaneURL, Doer: d},
		IAM:         &apiclient.IAMClient{BaseURL: cfg.IdentityAccessURL, Doer: d},
		Registry:    &apiclient.RegistryClient{BaseURL: cfg.AgentRegistryURL, Doer: d},
		Departments: &apiclient.DepartmentsClient{BaseURL: cfg.DepartmentsURL, Doer: d},
		Invocations: &apiclient.InvocationsClient{BaseURL: cfg.ToolExecutionURL, Doer: d},
	}
}

// Run performs the export. It never writes a file — cmd/demo-fixture wires
// the result to fixture.SaveYAML (or MarshalJSON) after validating it, so a
// caller can also unit test Run's assembly logic against the returned
// struct directly.
func Run(ctx context.Context, cfg Config, c *Clients) (*fixture.Fixture, error) {
	if cfg.TenantName == "" {
		return nil, fmt.Errorf("export: tenant name is required")
	}
	if cfg.TemplateID == "" {
		return nil, fmt.Errorf("export: template id is required (which department to export)")
	}

	login, err := c.IAM.AdminLogin(ctx, cfg.AdminPassword, cfg.TenantName)
	if err != nil {
		return nil, fmt.Errorf("export: admin login: %w", err)
	}
	token := login.Token

	f := &fixture.Fixture{
		SchemaVersion: fixture.SchemaVersion,
		Metadata: fixture.Metadata{
			Name:        firstNonEmpty(cfg.FixtureName, cfg.TenantName+"-demo"),
			Description: cfg.FixtureDescription,
			Provenance:  fixture.ProvenanceLiveExport,
			ExportedAt:  time.Now().UTC().Format(time.RFC3339),
			SourceNote:  cfg.SourceNote,
		},
	}

	if err := exportTenant(ctx, cfg, c, token, f); err != nil {
		return nil, err
	}

	userRefByID, err := exportUsers(ctx, cfg, c, token, f)
	if err != nil {
		return nil, err
	}

	dept, err := c.Departments.FindDepartment(ctx, token, cfg.TenantName, cfg.TemplateID, cfg.DepartmentName)
	if err != nil {
		return nil, fmt.Errorf("export: list departments: %w", err)
	}
	if dept == nil {
		return nil, fmt.Errorf("export: no department found for template %q (name %q) in tenant %q — has it been deployed?",
			cfg.TemplateID, cfg.DepartmentName, cfg.TenantName)
	}

	full, err := c.Departments.GetDepartment(ctx, token, cfg.TenantName, dept.ID)
	if err != nil {
		return nil, fmt.Errorf("export: get department %s: %w", dept.ID, err)
	}

	if err := exportOrgChart(ctx, cfg, c, token, f, full, userRefByID); err != nil {
		return nil, err
	}

	f.Department.TemplateID = full.TemplateID
	f.Department.Name = full.Name
	f.Department.Environment = full.Environment
	f.Department.SyncWorkflows = true

	if err := exportHistory(ctx, cfg, c, token, f, full); err != nil {
		return nil, err
	}

	deriveReplaySpec(f)

	return f, nil
}

func exportTenant(ctx context.Context, cfg Config, c *Clients, token string, f *fixture.Fixture) error {
	t, err := c.Tenant.FindTenantByName(ctx, token, cfg.TenantName)
	if err != nil {
		cfg.logf("WARNING: could not query Module 01 for tenant %q (%v) — writing flag-supplied tenant metadata only", cfg.TenantName, err)
		t = nil
	}
	if t == nil {
		cfg.logf("WARNING: Module 01 has no tenant control-plane record named %q. "+
			"Other modules key tenant data off the X-Tenant-ID/JWT tenant string alone, so the "+
			"tenant can be fully operational without one — but the fixture's plan/region/isolation_level "+
			"below are therefore export tool defaults, not read from a live record. Fill them in by hand "+
			"if that matters, or pass --tenant-plan/--tenant-region/--tenant-isolation flags.", cfg.TenantName)
		f.Tenant = fixture.Tenant{
			Name:           cfg.TenantName,
			Plan:           "saas",
			Region:         "me-east-1",
			IsolationLevel: "namespace",
		}
		return nil
	}
	f.Tenant = fixture.Tenant{
		Name:           t.Name,
		DisplayName:    t.DisplayName,
		Plan:           t.Plan,
		Region:         t.Region,
		IsolationLevel: t.IsolationLevel,
	}
	return nil
}

// exportUsers pulls every user in the tenant and returns a map from Module
// 02 user id to the local Ref assigned in the fixture, so later steps
// (org-chart seat bindings, replay approver) can translate ids back to
// refs.
func exportUsers(ctx context.Context, cfg Config, c *Clients, token string, f *fixture.Fixture) (map[string]string, error) {
	var all []*apiclient.User
	const pageSize = 50
	for page := 1; page <= 200; page++ {
		items, total, err := c.IAM.ListUsers(ctx, token, cfg.TenantName, page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("export: list users: %w", err)
		}
		all = append(all, items...)
		if page*pageSize >= total || len(items) == 0 {
			break
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Email < all[j].Email })

	refByID := map[string]string{}
	used := map[string]bool{}
	for _, u := range all {
		ref := uniqueSlug(localPart(u.Email), used)
		used[ref] = true
		refByID[u.ID] = ref
		f.Users = append(f.Users, fixture.User{
			Ref:         ref,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			RoleIDs:     u.RoleIDs,
		})
	}
	if len(f.Users) == 0 {
		cfg.logf("WARNING: tenant %q has zero Module 02 users — the fixture will have no one who can log in", cfg.TenantName)
	}
	return refByID, nil
}

// exportOrgChart translates the department's live org chart into
// fixture.SeatBinding entries, and pulls the Module 04 record for every
// ai_agent holder found so the fixture is self-contained (restore can
// recreate that agent under its original id before binding it).
func exportOrgChart(ctx context.Context, cfg Config, c *Clients, token string, f *fixture.Fixture, dept *apiclient.Department, userRefByID map[string]string) error {
	agentRefByID := map[string]string{}
	used := map[string]bool{}

	for _, pos := range dept.OrgChart {
		switch pos.HolderType {
		case "human":
			ref, ok := userRefByID[pos.HumanRef]
			if !ok {
				cfg.logf("WARNING: position %s (%s) is held by user id %s, which was not found in the tenant's user list — leaving this seat's binding out of the fixture", pos.ID, pos.Title, pos.HumanRef)
				continue
			}
			f.Department.SeatBindings = append(f.Department.SeatBindings, fixture.SeatBinding{
				PositionID: pos.ID, HolderType: "human", UserRef: ref,
			})
		case "ai_agent":
			ref, ok := agentRefByID[pos.AgentID]
			if !ok {
				agent, err := c.Registry.GetAgent(ctx, token, cfg.TenantName, pos.AgentID)
				if err != nil {
					cfg.logf("WARNING: position %s (%s) is held by agent id %s, which Module 04 could not return (%v) — leaving this seat's binding out of the fixture", pos.ID, pos.Title, pos.AgentID, err)
					continue
				}
				ref = uniqueSlug(slugify(agent.Name), used)
				used[ref] = true
				agentRefByID[pos.AgentID] = ref
				f.Agents = append(f.Agents, fixture.Agent{
					Ref: ref, ID: agent.ID, Name: agent.Name, Role: agent.Role,
					Description: agent.Description, Capabilities: agent.Capabilities, Tools: agent.Tools,
				})
			}
			f.Department.SeatBindings = append(f.Department.SeatBindings, fixture.SeatBinding{
				PositionID: pos.ID, HolderType: "ai_agent", AgentRef: ref,
			})
		case "vacant":
			// Nothing to record — restore leaves unlisted positions at
			// their template default, which is vacant.
		}
	}
	return nil
}

// exportHistory captures up to Config.maxHistory() past requests for the
// department, each with its Module 08 capability invocations, as read-only
// documentation (see fixture.HistoricalRequest's doc comment for why this
// is never replayed verbatim).
func exportHistory(ctx context.Context, cfg Config, c *Clients, token string, f *fixture.Fixture, dept *apiclient.Department) error {
	items, _, err := c.Departments.ListRequests(ctx, token, cfg.TenantName, dept.ID, 1, cfg.maxHistory())
	if err != nil {
		return fmt.Errorf("export: list requests for department %s: %w", dept.ID, err)
	}

	for _, r := range items {
		hr := fixture.HistoricalRequest{
			ServiceID: r.ServiceID,
			Title:     r.Title,
			Priority:  r.Priority,
			Status:    r.Status,
		}
		for _, ev := range r.Timeline {
			hr.Timeline = append(hr.Timeline, fixture.HistoricalEvent{
				At: ev.At, Kind: ev.Kind, Detail: ev.Detail, Node: ev.Node,
			})
		}
		invocations, err := c.Invocations.ListInvocationsForRequest(ctx, token, cfg.TenantName, r.ID, 50)
		if err != nil {
			cfg.logf("WARNING: could not list Module 08 invocations for request %s (%v) — history entry will have no invocations recorded", r.ID, err)
		}
		for _, inv := range invocations {
			hr.Invocations = append(hr.Invocations, fixture.HistoricalInvocation{
				CapabilityID: inv.CapabilityID, ProviderKind: inv.ProviderKind,
				Status: inv.Status, Simulated: inv.Simulated, PolicyDecision: inv.PolicyDecision,
			})
		}
		f.History = append(f.History, hr)
	}
	if len(f.History) == 0 {
		cfg.logf("WARNING: department %s has no request history yet — the fixture's history section will be empty and no replay spec can be derived from it", dept.ID)
	}
	return nil
}

// deriveReplaySpec picks the most recent history entry as the template for
// `restore --replay`, when the caller did not already set one. This is a
// convenience default, not a requirement — cmd/demo-fixture lets a flag
// override every field.
func deriveReplaySpec(f *fixture.Fixture) {
	if f.Replay != nil || len(f.History) == 0 {
		return
	}
	src := f.History[len(f.History)-1]
	title := src.Title
	if !strings.Contains(title, "replay") {
		title = title + " (replay)"
	}
	f.Replay = &fixture.ReplaySpec{
		ServiceID: src.ServiceID,
		Title:     title,
		Priority:  src.Priority,
	}
	if len(f.Users) > 0 {
		f.Replay.ApproverRef = f.Users[0].Ref
	}
}

func localPart(email string) string {
	if i := strings.IndexByte(email, '@'); i >= 0 {
		return email[:i]
	}
	return email
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "ref"
	}
	return out
}

func uniqueSlug(base string, used map[string]bool) string {
	base = slugify(base)
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
