# demo-fixture

Exports and restores the Operan demo tenant (`smoke-tenant`: Dana's seat binding,
the IT department, the request history with its capability invocations) as a
versioned, human-readable fixture — so that data survives the Stage 1
persistence and execution changes even if the cluster it currently lives on
does not.

Implements Work Order 3 (`handovers/remaining-70-work-orders.md`, `[S0-F]`).

## Why this exists, and why it works the way it does

`smoke-tenant` is currently the entire sales asset for this product, and it
only exists as live state in one cluster. This tool makes it reproducible
from source: a fixture file checked into git, an `export` command that
produces one from a live tenant, and a `restore` command that rebuilds a
tenant from one — all through the platform's own public HTTP APIs, never
through a database.

That "APIs, never a database" constraint is not incidental. It is the same
principle that gave Module 04's agent registry a caller-supplied `id` field
on `POST /registry/agents` — see that endpoint's doc comment: the department
deployer needed to re-register an agent under its original identity, and
direct `psql` repair was refused by policy in favor of an API affordance.
This tool is that same idea applied to the whole tenant.

## Layout and why it lives here

```
tools/demo-fixture/
  cmd/demo-fixture/       CLI entrypoint (export, restore subcommands)
  internal/fixture/       the fixture file format: types, YAML/JSON I/O, validation
  internal/apiclient/     typed HTTP clients for M01/M02/M04/M05/M08/M09
  internal/exportcmd/     export: walks the API, assembles a fixture
  internal/restorecmd/    restore: replays a fixture through the API, idempotently
  fixtures/smoke-tenant.yaml   the committed fixture (see its own header — READ IT)
```

**Placement: top-level `tools/`, not `modules/<n>/cmd/`.** This tool is a
client of five different modules' public APIs and owns no server-side
code — it has no Go-level dependency on any module's internal packages (by
design: see the constraint above). Nesting it under, say,
`modules/05-department-template-engine/cmd/` would misrepresent it as
belonging to that module, and would risk being swept into
`.github/workflows/docker-publish.yml`'s per-module Docker build matrix
(which builds every `modules/<name>/Dockerfile` — this tool has none and
should never get one; it is an operator CLI, not a deployed service). A new
top-level `tools/` directory did not exist before this work order; it is the
natural place for cross-cutting operator tooling that talks to the platform
from outside it, the same way `tools/` directories work in most Go
monorepos.

Go module, `go 1.25.0` (matching the repo-wide pin from WO-1), one external
dependency (`gopkg.in/yaml.v3`, pinned to the same version already used
elsewhere in this repo).

## The fixture format

See `internal/fixture/types.go` for the authoritative schema (every field is
doc-commented there). Summary:

- `schema_version` — currently `1`. Restore refuses a version it doesn't recognize.
- `metadata.provenance` — `live-export` (produced by `demo-fixture export`) or
  `hand-assembled` (transcribed by a human/agent from documentation, not a
  live run). Restore prints a warning for the latter. **The committed
  `fixtures/smoke-tenant.yaml` is currently hand-assembled — see below.**
- `tenant`, `users`, `agents`, `department` (with `seat_bindings` and
  `sync_workflows`) — what gets provisioned.
- `history` — a **read-only** snapshot of past requests and their Module 08
  capability invocations, kept for documentation value. Restore never
  replays these: Module 05 has no caller-supplied-id for requests, so a
  historical request's exact id/timestamps cannot be reproduced through the
  public API, only its shape can be recorded.
- `replay` — the one request `restore --replay` actually raises for real and
  drives to completion, proving the loop (draft → gate → approve → governed
  capability invocation → completed) still works after a restore.

No field anywhere in this schema is a credential. `internal/fixture/validate.go`
additionally scans the **raw** document (not just the typed struct, so a
carelessly hand-added field cannot hide from it) for credential-shaped key
names and values, and `Validate`/`Load` refuse the fixture if it finds any.

## Usage

```
demo-fixture export  [flags]   write a fixture from a live tenant
demo-fixture restore [flags]   provision a tenant from a fixture
```

Every module base URL flag defaults to its in-cluster DNS name (verified
against `deploy/k8s/modules.yaml`'s Service definitions — see
`cmd/demo-fixture/main.go`'s `registerModuleURLFlags`). Point them at
`localhost:<port>` when working through the port-forward rig described in
the project's handoff notes. Credentials are **never** read from the
fixture — always `--admin-password`/`$DEMO_FIXTURE_ADMIN_PASSWORD` and
`--user-password`/`$DEMO_FIXTURE_USER_PASSWORD`.

```sh
# Export the live tenant to a fixture:
demo-fixture export \
  --tenant smoke-tenant --admin-password "$ADMIN_PW" \
  --out fixtures/smoke-tenant.yaml

# See exactly what a restore would do, without doing any of it:
demo-fixture restore --fixture fixtures/smoke-tenant.yaml --dry-run

# Actually provision it:
demo-fixture restore --fixture fixtures/smoke-tenant.yaml \
  --admin-password "$ADMIN_PW" --user-password "$USER_PW"

# Provision AND raise+approve+complete one demonstration request:
demo-fixture restore --fixture fixtures/smoke-tenant.yaml \
  --admin-password "$ADMIN_PW" --user-password "$USER_PW" --replay
```

### Idempotency, and why it is not one mechanism

Running `restore` twice must not duplicate anything. Almost none of the
platform's create endpoints are idempotent by themselves, and the two that
are idempotent get there in *different* ways — both are load-bearing, and
conflating them would be wrong:

- **Module 04 (agents)**: idempotent via the **API's own** conflict
  response. This tool calls `POST /registry/agents` unconditionally, every
  run; the second call 409s because the id already exists, and this tool
  treats that 409 as "already exists, fetch and reuse" rather than an error.
  This is exactly the mechanism M04's caller-supplied `id` was built for.
- **Module 01 (tenant), Module 02 (users), Module 05 (department)**:
  idempotent via **this tool finding before creating**. None of these three
  create endpoints reject a duplicate — `POST /v1/tenants` 409s only on a
  repeat *name*, `POST /api/v1/iam/users` never checks email uniqueness at
  all (a second call creates a second user, confirmed by reading
  `internal/store/user.go`), and `POST /templates/{id}/deploy` mints a new
  department **every single call** (confirmed by reading
  `internal/deploy/orchestrator.go` — `Department.ID` is never set from the
  request). This tool lists first (`GET /v1/tenants`, `GET
  /api/v1/iam/users`, `GET /departments`, all paginated) and only calls
  create when nothing matches.
- **Seat bindings and workflow sync**: idempotent because the verbs
  themselves are — `PUT .../holder` sets a seat rather than appending to
  one, and `POST .../sync-workflows` is documented (and, per the prior
  session's handoff notes, verified live) to no-op safely on a repeat call.
  This tool calls both unconditionally every run.

`internal/restorecmd/restore_test.go`'s `TestProvisionIsIdempotent` asserts
this at the level that actually matters: it runs `Provision` twice against a
stateful mock and checks the **stored resource counts**, not just call
counts (a wrong version of that test — asserting "the create endpoint was
called exactly once" — would have been RIGHT for tenant/user/department and
WRONG for agents, since M04's idempotency legitimately calls create every
time; getting this test right required understanding the distinction above,
not just writing an assertion that happened to pass).

### `--dry-run`

Prints every API call a real run would make — method, URL, and body — without
making any of them. This is deliberately not a simulation layered on top of
the real logic; the same functions that build a request also decide, before
ever touching the network, whether to print it or send it. `restore_test.go`
and `replay_test.go` each have a test that points `--dry-run` at an
`httptest.Server` which **fails the test if it receives any request at
all**, and confirm zero requests arrive. Manually confirmed too: running the
compiled binary with `--dry-run` against the committed fixture and the
(unreachable, from this machine) in-cluster DNS names it defaults to
completes in ~0.05s wall-clock — the time it takes to print text, not the
time it would take to fail a real DNS lookup.

### `--replay`

Raises the fixture's `replay` request, polls it, and — if it reaches
`awaiting_approval` — finds and approves the matching Module 09 approval as
the named `approver_ref` user, then polls to completion. **This is the part
of the tool with the least direct evidence behind it** — see "What remains
unverified" below. The approval-correlation logic in particular
(`internal/restorecmd/replay.go`'s `findApprovalForRequest`) was derived by
reading three modules' source end to end (Module 05's `workloop.go` seeds
`request_title` into the workflow variables it hands to Module 03; Module
03's `node_handler.go` truncates that same string to 100 runes as the
Module 09 approval's `Title`; Module 09 exposes no endpoint that returns an
approval's originating Module 05 request id, only `GET /queue`, which
carries `Title` but not `request_id`), not by watching it happen.

## What was tested, and how

Everything in this list is a real, passing, `-race`-clean test — not a
description of intent:

- **Fixture format**: YAML round trip, JSON round trip, save-then-load,
  schema-version rejection, missing-file handling
  (`internal/fixture/roundtrip_test.go`).
- **Schema/reference validation**: every required-field check, every enum,
  duplicate refs, dangling refs, non-UUID agent ids — each with its own test
  (`internal/fixture/validate_test.go`).
- **Secret scanning**: table-driven tests that plant a `password`, `api_key`,
  nested `client_secret`, a bearer-token-shaped value, a PEM block, and an
  AWS key id, and assert each is caught; a companion test asserts the clean
  fixture produces **zero** findings; an end-to-end test parses hand-written
  YAML with a planted `password:` field neither the `User` struct nor any
  other typed field declares, and confirms `Validate` still rejects it
  (`internal/fixture/validate_test.go`) — this is the test that actually
  proves the raw-document scan is doing something the typed struct alone
  could not.
- **Every apiclient method** (M01 tenant, M02 iam, M04 registry, M05
  departments, M08 invocations, M09 supervision): exact HTTP method, path,
  headers (`Authorization: Bearer`, `X-Tenant-ID`, or their deliberate
  absence on the two unauthenticated login routes), and request/response
  body shape, each verified against an `httptest.Server`, plus pagination
  correctness for every `Find*ByX` helper across multiple pages
  (`internal/apiclient/*_test.go`).
- **Export**: a full run of `exportcmd.Run` against a single mock server
  standing in for all five modules, asserting the assembled fixture's
  tenant/users/agents/seat-bindings/history/derived-replay-spec are all
  correct, including that a vacant org-chart seat produces no binding; a
  separate test for the case where Module 01 has no tenant record at all
  (confirms the fallback-with-warning path, not just the happy path)
  (`internal/exportcmd/export_test.go`).
- **Restore idempotency**: `TestProvisionIsIdempotent` — see above.
- **Restore dry-run**: zero-network-call proof — see above.
- **Replay's state machine**: happy path with an approval in the middle
  (asserts the exact number of polls, that the right queue item was
  approved, and that the mock's approval record actually flipped to
  `approved`); happy path with no gate at all; ambiguous-match error;
  no-match error; timeout error; terminal-but-rejected error; gated-with-
  no-approver error; missing-replay-spec error
  (`internal/restorecmd/replay_test.go`).
- **The compiled CLI binary**, run directly (not through `go test`):
  `-h` / subcommand `-h` output; `restore --dry-run` and `restore --dry-run
  --replay` against the real committed `fixtures/smoke-tenant.yaml`
  (confirms the fixture actually loads and validates, and that the plan
  it prints is complete and sensible); `export`/`restore` against an
  unreachable endpoint (`127.0.0.1:1`) to confirm fast, clean failure with
  no partial file written; missing fixture file; malformed YAML; missing
  required flags. All exit codes and messages are as intended.

Run it yourself:

```sh
cd tools/demo-fixture
go build ./...
go vet ./...
go test ./... -race -count=1
```

## What remains unverified because there was no cluster access

This is the important section. Everything below is either untested or
tested only indirectly (against a mock that encodes this tool's
*understanding* of the real API, not the real API itself). None of it
should be read as working until a human runs it against a real cluster.

1. **The entire live round trip has never executed.** No command in this
   tool has ever made a real HTTP call to a real Operan deployment. Every
   test uses `httptest.Server` or the compiled binary against an
   unreachable address. This is true of `export` and both `restore` modes.
2. **`fixtures/smoke-tenant.yaml` is hand-assembled, not exported.** Its own
   header explains this in detail. Every fact in it is either transcribed
   from a prior session's "VERIFIED LIVE" notes or a clearly-commented
   placeholder default (tenant `plan`/`region`/`isolation_level`
   specifically — Module 01 may not even hold a record for this tenant at
   all, per those same notes describing the portal accepting "any tenant
   slug"). It intentionally omits the four AI-agent-held org-chart seats
   the `it-medium-001` template defines, because no verified agent id/name
   exists in the source notes to put there, and a plausible-looking
   fabricated UUID would be worse than an honest gap.
3. **The approval-correlation heuristic in `--replay` is unverified.**
   `findApprovalForRequest` matches a Module 09 queue item to a raised
   request by title, on the strength of a three-module source trace (see
   "`--replay`" above), never by observing it run. Two ways this could be
   wrong in ways the type system and mocks cannot catch: Module 03's
   `bound()` truncation semantics could differ from what was read (WO-1
   fixed that function to be rune-safe; this tool's `boundTitle` mirrors the
   fixed version), or Module 09's `GET /queue` could paginate/filter
   differently under real load than the single-page mock used in tests.
4. **Whether provisioning is actually synchronous where the code says it
   is.** `provisionDepartment`'s comment (and the decision not to poll
   before seat-binding) rests on reading `nested.go`'s `handleDeploy`
   closely enough to see `DepartmentStore.Create` happen before the handler
   responds, with only the *stage pipeline* continuing in the background —
   never confirmed by timing a real deploy call.
5. **The CI `provision-and-replay` job
   (`.github/workflows/demo-fixture.yml`) has never run.** It is gated to
   manual `workflow_dispatch` with an explicit confirmation input
   specifically so it cannot fire accidentally. Its own header comment
   lists what is missing before it even *could* run for the first time
   (reachable module URLs from the runner, two repository secrets). The
   `test` job in the same file (build/vet/`go test -race`/dry-run-the-fixture)
   has not run in GitHub Actions either, but every command it runs was run
   locally during this work with the output shown above — that job would
   pass today.
6. **Whatever the real M02 admin bootstrap password currently is**, and
   whether the roles this tool requests (`department_head` for Dana) still
   match what the live `smoke-tenant` actually has her holding. Both are
   flag/env inputs this tool takes on faith from the fixture and the
   caller, not something it can check without a cluster.

## First real round-trip: a runbook

1. Get cluster access and the current M02 admin bootstrap password (the
   handoff notes' documented default is `operan-admin-2026`, but treat that
   as unverified-current per point 6 above — confirm it, or get the real
   one, before relying on it).
2. Port-forward (or otherwise expose) the six services this tool talks to:
   `tenant-control-plane` (8080), `identity-access` (8002), `agent-registry`
   (8083), `department-templates` (8005), `tool-execution` (8008, export
   only), `human-supervision` (8009, `--replay` only). The project's
   existing "Local UI verification rig" port-forward setup already covers
   most of these — reuse it.
3. Run `demo-fixture export` against the **real** `smoke-tenant`, pointing
   the `--*-url` flags at your port-forwards, with `--out
   fixtures/smoke-tenant.yaml`. This overwrites the hand-assembled file with
   real, `provenance: live-export` data. Diff the two — every difference is
   either something this tool's schema assumptions got wrong (fix the
   schema/export code) or something the hand-assembled version guessed
   wrong (expected, not a bug).
4. On a throwaway/empty tenant name (do **not** reuse `smoke-tenant` for
   this first test — pick something like `smoke-tenant-restore-test` and
   edit the fixture's `tenant.name` accordingly, or add a `--tenant-name`
   override if one gets added), run `demo-fixture restore --dry-run` first
   and read the plan. Then run it for real with `--admin-password` and
   `--user-password` set.
5. Run it a **second** time against the same throwaway tenant and confirm
   by hand (list users/agents/departments through the portal or the APIs
   directly) that nothing duplicated — this is the live confirmation the
   `TestProvisionIsIdempotent` mock test stands in for today.
6. Run `demo-fixture restore --replay` (or add `--replay` to step 4) and
   watch it raise a request, find the gate, log in as Dana, approve it, and
   report `completed`. If `findApprovalForRequest` cannot find (or finds too
   many) matching queue items, that is exactly the failure this report
   flagged as unverified in point 3 above — the fix is almost certainly in
   that function, not in the platform.
7. Only once 3–6 have passed against a real cluster: remove this section's
   "never been run" framing from `fixtures/smoke-tenant.yaml`'s header and
   from `.github/workflows/demo-fixture.yml`, flip that workflow's
   `provision-and-replay` job to run on a real trigger (or leave it manual
   if that is preferred operationally), and add the two required repository
   secrets.
