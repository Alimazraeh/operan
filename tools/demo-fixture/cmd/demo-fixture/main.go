// Command demo-fixture exports and restores the demo tenant (smoke-tenant)
// as a versioned, human-readable fixture — see the tool's README for the
// full design rationale and, critically, what has and has not been
// verified against a real cluster.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/operan/tools/demo-fixture/internal/exportcmd"
	"github.com/operan/tools/demo-fixture/internal/fixture"
	"github.com/operan/tools/demo-fixture/internal/restorecmd"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("a subcommand is required")
	}
	switch args[0] {
	case "export":
		return runExport(args[1:])
	case "restore":
		return runRestore(args[1:])
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `demo-fixture — export/restore the Operan demo tenant as a versioned fixture.

Usage:
  demo-fixture export  [flags]   write a fixture from a live tenant
  demo-fixture restore [flags]   provision a tenant from a fixture

Run "demo-fixture export -h" or "demo-fixture restore -h" for flag details.

Every module base URL defaults to its in-cluster DNS name (see deploy/k8s/modules.yaml).
Point them at localhost when working through a port-forward instead.
Credentials (admin password, per-user password) are never read from the fixture file —
always supplied by flag or environment variable at the call site.
`)
}

// moduleURLFlags registers the five/six base-URL flags shared by both
// subcommands, each falling back to an environment variable and then to
// the module's in-cluster DNS name (deploy/k8s/modules.yaml's Service
// name + containerPort — verified against that file, not guessed).
type moduleURLFlags struct {
	m01, m02, m04, m05, m08, m09 *string
}

func registerModuleURLFlags(fs *flag.FlagSet, includeM08, includeM09 bool) moduleURLFlags {
	f := moduleURLFlags{}
	f.m01 = fs.String("tenant-control-plane-url", envOr("DEMO_FIXTURE_M01_URL", "http://tenant-control-plane.operan.svc.cluster.local:8080"),
		"Module 01 (tenant-control-plane) base URL [env DEMO_FIXTURE_M01_URL]")
	f.m02 = fs.String("identity-access-url", envOr("DEMO_FIXTURE_M02_URL", "http://identity-access.operan.svc.cluster.local:8002"),
		"Module 02 (identity-access) base URL [env DEMO_FIXTURE_M02_URL]")
	f.m04 = fs.String("agent-registry-url", envOr("DEMO_FIXTURE_M04_URL", "http://agent-registry.operan.svc.cluster.local:8083"),
		"Module 04 (agent-registry) base URL [env DEMO_FIXTURE_M04_URL]")
	f.m05 = fs.String("departments-url", envOr("DEMO_FIXTURE_M05_URL", "http://department-templates.operan.svc.cluster.local:8005"),
		"Module 05 (department-template-engine) base URL [env DEMO_FIXTURE_M05_URL]")
	if includeM08 {
		f.m08 = fs.String("tool-execution-url", envOr("DEMO_FIXTURE_M08_URL", "http://tool-execution.operan.svc.cluster.local:8008"),
			"Module 08 (tool-execution) base URL, read-only — used by export to capture historical invocations [env DEMO_FIXTURE_M08_URL]")
	}
	if includeM09 {
		f.m09 = fs.String("human-supervision-url", envOr("DEMO_FIXTURE_M09_URL", "http://human-supervision.operan.svc.cluster.local:8009"),
			"Module 09 (human-supervision) base URL — used by restore --replay to approve the demonstration request's gate [env DEMO_FIXTURE_M09_URL]")
	}
	return f
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---- export ----------------------------------------------------------------

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	urls := registerModuleURLFlags(fs, true, false)

	adminPassword := fs.String("admin-password", os.Getenv("DEMO_FIXTURE_ADMIN_PASSWORD"), "M02 admin bootstrap password [env DEMO_FIXTURE_ADMIN_PASSWORD] (required)")
	tenant := fs.String("tenant", envOr("DEMO_FIXTURE_TENANT", ""), "tenant name to export, e.g. smoke-tenant [env DEMO_FIXTURE_TENANT] (required)")
	templateID := fs.String("template-id", envOr("DEMO_FIXTURE_TEMPLATE_ID", "it-medium-001"), "department template id to export [env DEMO_FIXTURE_TEMPLATE_ID]")
	departmentName := fs.String("department-name", "", "department instance name to match, if the template was deployed more than once (empty matches the template's default name)")
	maxHistory := fs.Int("max-history", 5, "maximum number of historical requests (with their capability invocations) to capture")
	out := fs.String("out", "", "output file path (required)")
	format := fs.String("format", "yaml", "output format: yaml or json")
	name := fs.String("name", "", "fixture metadata.name (default: <tenant>-demo)")
	description := fs.String("description", "", "fixture metadata.description")
	sourceNote := fs.String("source-note", "", "fixture metadata.source_note — free text describing where this was exported from (no hostnames/credentials)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *tenant == "" {
		return fmt.Errorf("--tenant (or $DEMO_FIXTURE_TENANT) is required")
	}
	if *out == "" {
		return fmt.Errorf("--out is required")
	}
	if *adminPassword == "" {
		return fmt.Errorf("--admin-password (or $DEMO_FIXTURE_ADMIN_PASSWORD) is required")
	}
	if *format != "yaml" && *format != "json" {
		return fmt.Errorf("--format must be yaml or json, got %q", *format)
	}

	cfg := exportcmd.Config{
		TenantControlPlaneURL: *urls.m01,
		IdentityAccessURL:     *urls.m02,
		AgentRegistryURL:      *urls.m04,
		DepartmentsURL:        *urls.m05,
		ToolExecutionURL:      *urls.m08,
		AdminPassword:         *adminPassword,
		TenantName:            *tenant,
		TemplateID:            *templateID,
		DepartmentName:        *departmentName,
		MaxHistoryItems:       *maxHistory,
		FixtureName:           *name,
		FixtureDescription:    *description,
		SourceNote:            *sourceNote,
		Out:                   os.Stderr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	f, err := exportcmd.Run(ctx, cfg, exportcmd.NewClients(cfg))
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	var raw []byte
	if *format == "json" {
		raw, err = fixture.MarshalJSON(f)
	} else {
		raw, err = fixture.MarshalYAML(f)
	}
	if err != nil {
		return fmt.Errorf("export: marshal fixture: %w", err)
	}
	if err := fixture.Validate(f, raw); err != nil {
		return fmt.Errorf("export: the fixture this tool just built failed its own validation — refusing to write it: %w", err)
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		return fmt.Errorf("export: write %s: %w", *out, err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%d user(s), %d agent(s), %d history entr(y/ies))\n", *out, len(f.Users), len(f.Agents), len(f.History))
	return nil
}

// ---- restore ----------------------------------------------------------------

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	urls := registerModuleURLFlags(fs, false, true)

	fixturePath := fs.String("fixture", "", "path to the fixture file (required)")
	adminPassword := fs.String("admin-password", os.Getenv("DEMO_FIXTURE_ADMIN_PASSWORD"), "M02 admin bootstrap password [env DEMO_FIXTURE_ADMIN_PASSWORD] (required unless --dry-run)")
	userPassword := fs.String("user-password", os.Getenv("DEMO_FIXTURE_USER_PASSWORD"), "initial password set for every fixture user [env DEMO_FIXTURE_USER_PASSWORD] (optional; required for --replay to log in as a named approver)")
	dryRun := fs.Bool("dry-run", false, "print every API call this run would make, without making any of them")
	replay := fs.Bool("replay", false, "after provisioning, raise the fixture's replay request and drive it to completion (see fixture.replay)")
	pollInterval := fs.Duration("replay-poll-interval", 3*time.Second, "how often --replay polls the request's status")
	maxAttempts := fs.Int("replay-max-attempts", 60, "how many times --replay polls before giving up")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *fixturePath == "" {
		return fmt.Errorf("--fixture is required")
	}

	f, _, err := fixture.Load(*fixturePath)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	cfg := restorecmd.Config{
		TenantControlPlaneURL: *urls.m01,
		IdentityAccessURL:     *urls.m02,
		AgentRegistryURL:      *urls.m04,
		DepartmentsURL:        *urls.m05,
		HumanSupervisionURL:   *urls.m09,
		AdminPassword:         *adminPassword,
		UserPassword:          *userPassword,
		DryRun:                *dryRun,
		Out:                   os.Stderr,
	}

	if !*dryRun && cfg.AdminPassword == "" {
		return fmt.Errorf("--admin-password (or $DEMO_FIXTURE_ADMIN_PASSWORD) is required unless --dry-run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	clients := restorecmd.NewClients(cfg)
	result, err := restorecmd.Provision(ctx, cfg, f, clients)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	printProvisionSummary(result)

	if *replay {
		if f.Replay == nil {
			return fmt.Errorf("restore: --replay was requested but the fixture has no replay section")
		}
		replayResult, err := restorecmd.Replay(ctx, cfg, f, clients, result, restorecmd.ReplayOptions{
			PollInterval: *pollInterval, MaxAttempts: *maxAttempts,
		})
		if err != nil {
			return fmt.Errorf("restore --replay: %w", err)
		}
		fmt.Fprintf(os.Stderr, "replay: request %s reached status %q after %d poll(s), approved=%v\n",
			replayResult.RequestID, replayResult.FinalStatus, replayResult.Attempts, replayResult.Approved)
	}

	return nil
}

func printProvisionSummary(r *restorecmd.Result) {
	verb := func(created bool) string {
		if created {
			return "created"
		}
		return "reused"
	}
	prefix := ""
	if r.DryRun {
		prefix = "[dry-run] "
	}
	fmt.Fprintf(os.Stderr, "%stenant %s (%s)\n", prefix, r.Tenant.ID, verb(r.Tenant.Created))
	for _, u := range r.Users {
		fmt.Fprintf(os.Stderr, "%suser %s: %s (%s)\n", prefix, u.Ref, u.ID, verb(u.Created))
	}
	for _, a := range r.Agents {
		fmt.Fprintf(os.Stderr, "%sagent %s: %s (%s)\n", prefix, a.Ref, a.ID, verb(a.Created))
	}
	fmt.Fprintf(os.Stderr, "%sdepartment %s (%s), %d seat binding(s) set, workflows synced=%v\n",
		prefix, r.Department.ID, verb(r.Department.Created), r.SeatBindingsSet, r.WorkflowsSynced)
}
