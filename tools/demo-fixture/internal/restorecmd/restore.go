// Package restorecmd implements `demo-fixture restore`: it replays a
// fixture.Fixture through the same public APIs a human operator would use,
// idempotently (a second run reuses what the first run created instead of
// duplicating it), with a --dry-run mode that plans every call without
// making any of them.
//
// Almost none of the platform's create endpoints are idempotent on their
// own — see the doc comments on apiclient's CreateUser, DeployTemplate and
// FindDepartment for the specific store-level reasons. This package is
// therefore built around "find, then create only if not found" for every
// resource that has no caller-supplied-id affordance, and relies on the
// affordance itself (Module 04's agent id) or on naturally-idempotent verbs
// (PUT for seat bindings) where one exists.
package restorecmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/operan/tools/demo-fixture/internal/apiclient"
	"github.com/operan/tools/demo-fixture/internal/fixture"
)

// Config carries connection info, credentials and behavior flags. Nothing
// here is written back into any fixture file.
type Config struct {
	TenantControlPlaneURL string
	IdentityAccessURL     string
	AgentRegistryURL      string
	DepartmentsURL        string
	ToolExecutionURL      string
	HumanSupervisionURL   string

	// AdminPassword is the bootstrap credential used for every provisioning
	// call (tenant/user/agent creation, department deploy — all of which
	// need an "admin" role). Supplied by flag or environment variable at
	// the call site, never read from the fixture.
	AdminPassword string

	// UserPassword, when non-empty, is set as the initial password for
	// every fixture user on every run (SetPassword is naturally idempotent,
	// so this is safe to repeat). Leave empty to skip password provisioning
	// entirely (e.g. when restoring org-chart state only).
	UserPassword string

	DryRun bool

	// Out receives a human-readable log of what happened (live mode) or
	// what would happen (dry-run mode) — one line per planned/executed API
	// call, in order. This is what makes dry-run reviewable.
	Out io.Writer
}

func (c Config) logf(format string, args ...interface{}) {
	if c.Out == nil {
		return
	}
	fmt.Fprintf(c.Out, format+"\n", args...)
}

// Clients bundles every apiclient wrapper Provision and Replay need.
type Clients struct {
	Tenant      *apiclient.TenantClient
	IAM         *apiclient.IAMClient
	Registry    *apiclient.RegistryClient
	Departments *apiclient.DepartmentsClient
	Supervision *apiclient.SupervisionClient
}

// NewClients builds a Clients bundle from Config's base URLs, sharing one
// Doer (and its *http.Client) across all of them.
func NewClients(cfg Config) *Clients {
	d := apiclient.NewDoer()
	return &Clients{
		Tenant:      &apiclient.TenantClient{BaseURL: cfg.TenantControlPlaneURL, Doer: d},
		IAM:         &apiclient.IAMClient{BaseURL: cfg.IdentityAccessURL, Doer: d},
		Registry:    &apiclient.RegistryClient{BaseURL: cfg.AgentRegistryURL, Doer: d},
		Departments: &apiclient.DepartmentsClient{BaseURL: cfg.DepartmentsURL, Doer: d},
		Supervision: &apiclient.SupervisionClient{BaseURL: cfg.HumanSupervisionURL, Doer: d},
	}
}

// ResourceOutcome records whether Provision found an existing resource or
// created a new one — the concrete evidence that a second run is
// idempotent, not just a claim.
type ResourceOutcome struct {
	Ref     string // fixture Ref, or a fixed label for singletons like "tenant"
	ID      string
	Created bool // false means "found, reused"
}

// Result is everything Provision produced, keyed for both human reporting
// and for Replay to consume without re-deriving ids.
type Result struct {
	DryRun bool

	Tenant ResourceOutcome
	Users  []ResourceOutcome
	Agents []ResourceOutcome

	Department       ResourceOutcome
	SeatBindingsSet  int
	WorkflowsSynced  bool
	WorkflowsChanged int

	// UserIDByRef and AgentIDByRef let Replay (and tests) resolve a
	// fixture Ref to the real platform id Provision created or found.
	UserIDByRef  map[string]string
	AgentIDByRef map[string]string

	// AdminToken is the bootstrap JWT minted during this run. In dry-run
	// mode it is a placeholder — no login call is made, per this tool's
	// hard requirement that --dry-run performs zero network calls.
	AdminToken string
}

const dryRunPlaceholder = "<dry-run-would-be-assigned>"

// Provision rebuilds the tenant, users, agents, department and seat
// bindings described by f. It is safe to call twice: every step either
// finds-and-reuses an existing resource or uses a naturally idempotent
// verb. It does not raise or replay any request — see Replay for that.
func Provision(ctx context.Context, cfg Config, f *fixture.Fixture, c *Clients) (*Result, error) {
	if f.Metadata.Provenance == fixture.ProvenanceHandAssembled {
		cfg.logf("NOTE: this fixture's provenance is %q, not %q — it was transcribed from documentation, not produced by `demo-fixture export` against a live cluster. Treat its exact field values as best-effort until a live export replaces it.",
			fixture.ProvenanceHandAssembled, fixture.ProvenanceLiveExport)
	}

	res := &Result{
		DryRun:       cfg.DryRun,
		UserIDByRef:  map[string]string{},
		AgentIDByRef: map[string]string{},
	}

	// --- Step 0: bootstrap credential ---------------------------------
	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: POST %s/api/v1/iam/admin/login  body={\"tenant\":%q}  (admin password withheld from output)",
			cfg.IdentityAccessURL, f.Tenant.Name)
		res.AdminToken = dryRunPlaceholder
	} else {
		if cfg.AdminPassword == "" {
			return nil, fmt.Errorf("restore: --admin-password (or the configured env var) is required and was empty")
		}
		login, err := c.IAM.AdminLogin(ctx, cfg.AdminPassword, f.Tenant.Name)
		if err != nil {
			return nil, fmt.Errorf("restore: admin login: %w", err)
		}
		res.AdminToken = login.Token
		cfg.logf("admin login ok (tenant=%s)", f.Tenant.Name)
	}
	token := res.AdminToken

	// --- Step 1: tenant -------------------------------------------------
	tenantOutcome, err := provisionTenant(ctx, cfg, c, token, f.Tenant)
	if err != nil {
		return nil, fmt.Errorf("restore: tenant: %w", err)
	}
	res.Tenant = tenantOutcome

	// --- Step 2: users ----------------------------------------------------
	for _, u := range f.Users {
		outcome, err := provisionUser(ctx, cfg, c, token, f.Tenant.Name, u)
		if err != nil {
			return nil, fmt.Errorf("restore: user %s: %w", u.Ref, err)
		}
		res.Users = append(res.Users, outcome)
		res.UserIDByRef[u.Ref] = outcome.ID
	}

	// --- Step 3: agents -----------------------------------------------
	for _, a := range f.Agents {
		outcome, err := provisionAgent(ctx, cfg, c, token, f.Tenant.Name, a)
		if err != nil {
			return nil, fmt.Errorf("restore: agent %s: %w", a.Ref, err)
		}
		res.Agents = append(res.Agents, outcome)
		res.AgentIDByRef[a.Ref] = outcome.ID
	}

	// --- Step 4: department ---------------------------------------------
	deptOutcome, err := provisionDepartment(ctx, cfg, c, token, f.Tenant.Name, f.Department)
	if err != nil {
		return nil, fmt.Errorf("restore: department: %w", err)
	}
	res.Department = deptOutcome

	// --- Step 5: seat bindings -------------------------------------------
	for _, sb := range f.Department.SeatBindings {
		if err := provisionSeatBinding(ctx, cfg, c, token, f.Tenant.Name, deptOutcome.ID, sb, res.UserIDByRef, res.AgentIDByRef); err != nil {
			return nil, fmt.Errorf("restore: seat binding %s: %w", sb.PositionID, err)
		}
		res.SeatBindingsSet++
	}

	// --- Step 6: sync workflows -------------------------------------------
	if f.Department.SyncWorkflows {
		changed, err := provisionSyncWorkflows(ctx, cfg, c, token, f.Tenant.Name, deptOutcome.ID)
		if err != nil {
			return nil, fmt.Errorf("restore: sync workflows: %w", err)
		}
		res.WorkflowsSynced = true
		res.WorkflowsChanged = changed
	}

	return res, nil
}

func provisionTenant(ctx context.Context, cfg Config, c *Clients, token string, spec fixture.Tenant) (ResourceOutcome, error) {
	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: GET  %s/v1/tenants  (page through, looking for name=%q)", cfg.TenantControlPlaneURL, spec.Name)
		cfg.logf("[dry-run] PLAN: IF NOT FOUND: POST %s/v1/tenants  body={name:%q, plan:%q, region:%q, isolation_level:%q}",
			cfg.TenantControlPlaneURL, spec.Name, spec.Plan, spec.Region, spec.IsolationLevel)
		return ResourceOutcome{Ref: "tenant", ID: dryRunPlaceholder, Created: true}, nil
	}

	existing, err := c.Tenant.FindTenantByName(ctx, token, spec.Name)
	if err != nil {
		return ResourceOutcome{}, fmt.Errorf("find tenant: %w", err)
	}
	if existing != nil {
		cfg.logf("tenant %q already exists (id=%s) — reused", spec.Name, existing.ID)
		return ResourceOutcome{Ref: "tenant", ID: existing.ID, Created: false}, nil
	}

	created, err := c.Tenant.CreateTenant(ctx, token, apiclient.CreateTenantRequest{
		Name: spec.Name, DisplayName: spec.DisplayName, Plan: spec.Plan,
		Region: spec.Region, IsolationLevel: spec.IsolationLevel, ContactEmail: spec.ContactEmail,
	})
	if err != nil {
		if apiclient.IsConflict(err) {
			// Lost a race with something else that just created it —
			// look it up rather than fail outright.
			again, findErr := c.Tenant.FindTenantByName(ctx, token, spec.Name)
			if findErr == nil && again != nil {
				cfg.logf("tenant %q was created concurrently (id=%s) — reused", spec.Name, again.ID)
				return ResourceOutcome{Ref: "tenant", ID: again.ID, Created: false}, nil
			}
		}
		return ResourceOutcome{}, fmt.Errorf("create tenant: %w", err)
	}
	cfg.logf("tenant %q created (id=%s)", spec.Name, created.ID)
	return ResourceOutcome{Ref: "tenant", ID: created.ID, Created: true}, nil
}

func provisionUser(ctx context.Context, cfg Config, c *Clients, token, tenantName string, spec fixture.User) (ResourceOutcome, error) {
	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: GET  %s/api/v1/iam/users  (page through, looking for email=%q)", cfg.IdentityAccessURL, spec.Email)
		cfg.logf("[dry-run] PLAN: IF NOT FOUND: POST %s/api/v1/iam/users  body={email:%q, display_name:%q, role_ids:%v}",
			cfg.IdentityAccessURL, spec.Email, spec.DisplayName, spec.RoleIDs)
		if cfg.UserPassword != "" {
			cfg.logf("[dry-run] PLAN: POST %s/api/v1/iam/users/{id}/password  (password withheld from output)", cfg.IdentityAccessURL)
		}
		return ResourceOutcome{Ref: spec.Ref, ID: dryRunPlaceholder, Created: true}, nil
	}

	outcome := ResourceOutcome{Ref: spec.Ref}
	existing, err := c.IAM.FindUserByEmail(ctx, token, tenantName, spec.Email)
	if err != nil {
		return ResourceOutcome{}, fmt.Errorf("find user: %w", err)
	}
	if existing != nil {
		cfg.logf("user %q already exists (id=%s) — reused", spec.Email, existing.ID)
		outcome.ID, outcome.Created = existing.ID, false
	} else {
		mfa := spec.MFAEnabled
		created, err := c.IAM.CreateUser(ctx, token, tenantName, apiclient.CreateUserRequest{
			Email: spec.Email, DisplayName: spec.DisplayName, RoleIDs: spec.RoleIDs, MFAEnabled: &mfa,
		})
		if err != nil {
			return ResourceOutcome{}, fmt.Errorf("create user: %w", err)
		}
		cfg.logf("user %q created (id=%s)", spec.Email, created.ID)
		outcome.ID, outcome.Created = created.ID, true
	}

	if cfg.UserPassword != "" {
		if err := c.IAM.SetPassword(ctx, token, tenantName, outcome.ID, cfg.UserPassword); err != nil {
			return ResourceOutcome{}, fmt.Errorf("set password for %s: %w", spec.Email, err)
		}
		cfg.logf("user %q password set", spec.Email)
	}
	return outcome, nil
}

func provisionAgent(ctx context.Context, cfg Config, c *Clients, token, tenantName string, spec fixture.Agent) (ResourceOutcome, error) {
	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: POST %s/registry/agents  body={id:%q, name:%q, role:%q, capabilities:%v}  (409 if it already exists → reuse)",
			cfg.AgentRegistryURL, spec.ID, spec.Name, spec.Role, spec.Capabilities)
		return ResourceOutcome{Ref: spec.Ref, ID: spec.ID, Created: true}, nil
	}

	created, err := c.Registry.CreateAgent(ctx, token, tenantName, apiclient.CreateAgentRequest{
		ID: spec.ID, Name: spec.Name, Role: spec.Role, Description: spec.Description,
		Capabilities: spec.Capabilities, Tools: spec.Tools,
	})
	if err == nil {
		cfg.logf("agent %q created (id=%s)", spec.Name, created.ID)
		return ResourceOutcome{Ref: spec.Ref, ID: created.ID, Created: true}, nil
	}
	if !apiclient.IsConflict(err) {
		return ResourceOutcome{}, fmt.Errorf("create agent: %w", err)
	}
	// Already registered under this id — confirm it is actually reachable
	// rather than assuming the conflict means what we think it means.
	existing, getErr := c.Registry.GetAgent(ctx, token, tenantName, spec.ID)
	if getErr != nil {
		return ResourceOutcome{}, fmt.Errorf("agent %s conflicted on create and could not be fetched: %w", spec.ID, getErr)
	}
	cfg.logf("agent %q already exists (id=%s) — reused", spec.Name, existing.ID)
	return ResourceOutcome{Ref: spec.Ref, ID: existing.ID, Created: false}, nil
}

func provisionDepartment(ctx context.Context, cfg Config, c *Clients, token, tenantName string, spec fixture.Department) (ResourceOutcome, error) {
	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: GET  %s/departments  (page through, looking for template_id=%q, name=%q)",
			cfg.DepartmentsURL, spec.TemplateID, spec.Name)
		cfg.logf("[dry-run] PLAN: IF NOT FOUND: POST %s/templates/%s/deploy  body={environment:%q, department_name:%q}",
			cfg.DepartmentsURL, spec.TemplateID, spec.Environment, spec.Name)
		return ResourceOutcome{Ref: "department", ID: dryRunPlaceholder, Created: true}, nil
	}

	existing, err := c.Departments.FindDepartment(ctx, token, tenantName, spec.TemplateID, spec.Name)
	if err != nil {
		return ResourceOutcome{}, fmt.Errorf("find department: %w", err)
	}
	if existing != nil {
		cfg.logf("department for template %q already deployed (id=%s) — reused, NOT re-deployed", spec.TemplateID, existing.ID)
		return ResourceOutcome{Ref: "department", ID: existing.ID, Created: false}, nil
	}

	deployed, err := c.Departments.DeployTemplate(ctx, token, tenantName, spec.TemplateID, apiclient.DeployRequest{
		Environment: spec.Environment, DepartmentName: spec.Name,
	})
	if err != nil {
		return ResourceOutcome{}, fmt.Errorf("deploy template %s: %w", spec.TemplateID, err)
	}
	if deployed.DepartmentID == "" {
		return ResourceOutcome{}, fmt.Errorf("deploy template %s: response carried no department_id (deployment id=%s, status=%s) — Module 05's synchronous MaterializeDepartment step may have changed",
			spec.TemplateID, deployed.ID, deployed.Status)
	}
	cfg.logf("department deployed from template %q (department id=%s, deployment id=%s, status=%s)",
		spec.TemplateID, deployed.DepartmentID, deployed.ID, deployed.Status)
	return ResourceOutcome{Ref: "department", ID: deployed.DepartmentID, Created: true}, nil
}

func provisionSeatBinding(ctx context.Context, cfg Config, c *Clients, token, tenantName, deptID string, sb fixture.SeatBinding, userIDByRef, agentIDByRef map[string]string) error {
	req := apiclient.SetHolderRequest{HolderType: sb.HolderType}
	switch sb.HolderType {
	case "human":
		id, ok := userIDByRef[sb.UserRef]
		if !ok {
			return fmt.Errorf("user_ref %q has no resolved id (was it provisioned?)", sb.UserRef)
		}
		req.HumanRef = id
	case "ai_agent":
		id, ok := agentIDByRef[sb.AgentRef]
		if !ok {
			return fmt.Errorf("agent_ref %q has no resolved id (was it provisioned?)", sb.AgentRef)
		}
		req.AgentID = id
	case "vacant":
		// no id to resolve
	default:
		return fmt.Errorf("unknown holder_type %q", sb.HolderType)
	}

	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: PUT  %s/departments/{department-id}/org-chart/%s/holder  body={holder_type:%q, human_ref:%q, agent_id:%q}",
			cfg.DepartmentsURL, sb.PositionID, req.HolderType, req.HumanRef, req.AgentID)
		return nil
	}

	if _, err := c.Departments.SetPositionHolder(ctx, token, tenantName, deptID, sb.PositionID, req); err != nil {
		return err
	}
	cfg.logf("seat %s bound (%s)", sb.PositionID, sb.HolderType)
	return nil
}

func provisionSyncWorkflows(ctx context.Context, cfg Config, c *Clients, token, tenantName, deptID string) (int, error) {
	if cfg.DryRun {
		cfg.logf("[dry-run] PLAN: POST %s/departments/{department-id}/services/sync-workflows  (no body)", cfg.DepartmentsURL)
		return 0, nil
	}
	out, err := c.Departments.SyncWorkflows(ctx, token, tenantName, deptID)
	if err != nil {
		return 0, err
	}
	cfg.logf("workflows synced (%d service(s) changed, template_version=%s)", out.Changed, out.TemplateVersion)
	return out.Changed, nil
}

// pollUntil calls check every interval until it returns (true, nil), an
// error, or the deadline (interval*maxAttempts) passes. sleep is injectable
// so tests can run this at full speed without waiting on a real clock,
// while still exercising the "poll N times" control flow for real.
func pollUntil(ctx context.Context, interval time.Duration, maxAttempts int, sleep func(time.Duration), check func() (bool, error)) error {
	if sleep == nil {
		sleep = time.Sleep
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		sleep(interval)
	}
	return fmt.Errorf("timed out after %d attempts (interval %s)", maxAttempts, interval)
}
