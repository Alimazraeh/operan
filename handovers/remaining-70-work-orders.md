# Operan — Implementation Work Orders (the "remaining 70%")

**Source plan:** revision 2 of the four-stage execution plan (26 workstreams, 12–14 months).
**This document:** the subset of that plan which is *safely delegatable to a coding agent*, decomposed
into work orders with machine-checkable acceptance criteria.
**Baseline:** HEAD `256220a`, 2026-07-27.

---

## Scope boundary — read this first

The full plan is a multi-person, multi-quarter programme. Most of it **must not** be handed to an
agent. This document exists to draw that line explicitly.

### Delegatable to an implementation agent
Bounded, verifiable, no external credentials required, no irreversible side effects.
WO-1 … WO-5 below.

### NOT delegatable — founder or human-in-the-loop
| Plan item | Why it cannot be delegated |
|---|---|
| `S0-B` build-vs-buy ADRs | Architectural decisions binding the company; needs runway context an agent does not have |
| `S0-D` design-partner outreach | Not engineering work at all |
| `S1-A` durable execution | Multi-week architectural rewrite; the Path A/B fork is a business decision keyed to runway |
| `S1-C` MCP real write | Requires a real external system, real credentials, and a real side effect on someone's data |
| `S1-D` credential store | Security-critical cryptography; requires human review before it holds a real key |
| `S1-H` grading | The harness is delegatable; the *rubric scoring* requires a domain professional |
| `S1-I` load measurement | Requires driving the shared LiteLLM gateway in another namespace — human authorisation |
| All of Stage 2 / Stage 3 | Gated on Gate 1 and on customer evidence that does not exist yet |

### Standing rules for any agent working these orders
1. **Branch, never `main`.** One branch per work order: `wo-<n>-<slug>`.
2. **Do not push. Do not open PRs.** Commit locally; the human reviews and pushes.
3. **Do not touch the cluster.** No `kubectl`, no deploys. These orders are source-only.
4. **Do not touch any namespace other than `operan`** if cluster access is ever granted later.
5. **Tests are the deliverable, not the evidence.** A fix without a test that fails on the old code
   is not done.
6. **Report honestly.** If an acceptance criterion cannot be met, say so and stop. Do not adjust the
   criterion. This codebase's defining quality is that it refuses to fake success; preserve that.

---

## WO-1 — Rune-safe truncation, stale claim, toolchain pin  `[S0-C]`

**Why:** `bound()` in the orchestration execution path slices by **bytes**, so any multi-byte text —
Arabic above all — can be cut mid-character, producing invalid UTF-8. It sits on the hot path that
trims agent drafts, request bodies, gate instructions and capability inputs. Three sibling
implementations in this same repo already do it correctly with runes, so this is a missed call site,
not a missing idea. Arabic-first is a positioning claim for this product, which makes this defect
both a correctness bug and a credibility bug.

### 1a — Fix byte-slicing truncation (two sites)
| File | Line | Current | Required |
|---|---|---|---|
| `modules/03-agent-orchestration/internal/execution/node_handler.go` | ~398 | `s[:n]` byte slice | rune-safe |
| `modules/03-agent-orchestration/internal/execution/dag_engine.go` | ~421-425 | inline `text[:8000]` | rune-safe |

Correct reference implementation already in the tree —
`modules/03-agent-orchestration/internal/capability/client.go:109`:
```go
func bound(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
```
`node_handler.go`'s `bound()` has **9 call sites** (lines ~109, 135, 138, 162, 175, 202, 358, 380, 383).
Do not change its signature or semantics beyond rune-safety.

In `dag_engine.go`, prefer extracting the inline truncation to a shared helper rather than duplicating
a fourth copy — but if that creates an import cycle, an inline rune-safe fix is acceptable. State
which you did and why.

### 1b — Delete the stale claim in the portal header comment
`modules/21-experience-portal/main.go:1-7` still states the JWT is *"minted client-side from the
tenant's signing secret."* That has been untrue since real per-user login shipped. Rewrite the package
comment to describe what the portal actually does: serves the embedded SPA and reverse-proxies
`/svc/<name>/` to platform services, same-origin, with the JWT obtained from Module 02's login
endpoint.

### 1c — Pin one Go toolchain
`modules/*/go.mod` currently spans **1.21, 1.23, 1.23.0, 1.25, 1.25.0**. Pin all to the highest
version already in use (`1.25.0`). Check `.github/workflows/docker-publish.yml` and every
`Dockerfile` for a builder image that must match; a mismatch here has already cost this project a CI
failure.

### Acceptance criteria — all must hold
- [ ] A new test in the `execution` package truncates an **Arabic** string at a boundary that splits a
      multi-byte rune, and asserts the result is valid UTF-8 (`utf8.ValidString`) and has exactly the
      expected rune count. **Verify it fails against the byte-sliced version** and record that in the
      report.
- [ ] Equivalent coverage for the `dag_engine.go` site.
- [ ] `go test ./... -race -count=1` passes in `modules/03-agent-orchestration`.
- [ ] `go build ./...` passes in every module whose `go.mod` was touched.
- [ ] `grep -h '^go ' modules/*/go.mod | sort -u` returns exactly one line.
- [ ] No Dockerfile or CI builder image references a Go version older than the pin.
- [ ] `git diff --stat` touches only the files named in this order.

---

## WO-2 — Stop the catalog offering unauthored departments  `[S0-E]`

**Why:** `modules/21-experience-portal/web/app/js/views/departments.js:73` renders every seeded
template as a deployable department. Only `it-medium-001` has authored SOPs with capability bindings
(11 of 639 catalogue steps carry a capability, all in that one template). The other 30 would deploy a
department whose steps are LLM prose and recorded pass-throughs. This is the same facade pattern as
the disconnected modules, reproduced in the surface a prospect meets first.

**Mechanical rule for "operational":** a template is operational if **at least one step in at least
one of its workflows carries `config.capability`**. Everything else is an outline.

**Required behaviour:** outline templates remain *visible* (they show intended breadth) but are
**not deployable** — the deploy affordance is replaced by an explicit state that says why, in the
product's own voice. Follow the existing honesty phrasing conventions in this codebase; do not invent
a new tone.

Decide and justify where the flag is computed: portal-side from the template payload, or M05-side as
a field on the template list response. Prefer M05-side if the payload already carries workflows;
prefer portal-side if it does not, and say which you found.

### Acceptance criteria
- [ ] The IT template remains deployable; the other 30 are visibly marked and cannot be deployed.
- [ ] Clicking an outline template explains why, naming the missing thing (authored SOPs).
- [ ] If the rule is computed server-side, a Go test covers it including the boundary case of a
      template with workflows but zero capability-bearing steps.
- [ ] No console errors on the departments view.
- [ ] The check is derived from template content, never a hardcoded template-ID allowlist.

---

## WO-3 — Demo tenant as a restorable fixture  `[S0-F]`

**Why:** `smoke-tenant` — Dana's seat binding, the IT department, the request history with its
capability invocations — is currently the entire sales asset, and it is the data most likely to be
destroyed by the Stage 1 persistence and execution changes. It must become reproducible from source
before those land.

**Deliverable:** a versioned seed fixture plus a one-command restore that rebuilds the demo tenant on
an empty cluster, and a CI job that provisions from the fixture and replays one full request to
completion.

**Constraint:** the fixture must be built from **API calls**, not direct database writes. This project
has already established that pattern — M04's caller-supplied `id` API exists precisely because direct
`psql` repair was refused.

### Acceptance criteria
- [ ] One command rebuilds the demo tenant from fixture on an empty cluster.
- [ ] The fixture is committed source, human-readable, and contains no secrets.
- [ ] CI runs the provision-and-replay on every push.
- [ ] Re-running the restore is idempotent.

---

## WO-4 — Server-side authority + actor-keyed policy cache  `[S1-E]`

**Why:** the platform's headline claim — "the org chart is the authorization graph" — is true for
routing and **false for enforcement**. Two defects:

1. `modules/08-tool-execution/internal/funnel/funnel.go:122` compares `req.Actor.AutonomyTier`, a
   field supplied by the **caller**. Nothing resolves the seat's real tier server-side. Any
   authenticated caller can claim the highest tier.
2. `modules/10-policy-governance/internal/engine/engine.go:273-275` — `buildCacheKey` is
   `tenant:resource:action_type:data_class` with **no actor**. One agent's allow answers for every
   agent in the tenant for the cache TTL.

**Required:**
- M08 gains a Module 05 client resolving `actor_position_id → seat → autonomy_tier` at invoke time.
  M05 already exposes the org chart and `/me/assignments`.
- An unresolvable seat is **no authority** — not a default, not the caller's claim.
- The invocation record carries **both** the claimed and the resolved tier, so a mismatch is visible
  in the audit trail rather than silently corrected.
- The policy cache key includes the actor.
- Fail-closed behaviour is preserved: M05 unreachable must deny, never allow.

⚠️ **This work order changes a security boundary. It requires human review before merge, and the
agent must not push.**

### Acceptance criteria
- [ ] A request body claiming `coordinate` for a seat holding `draft` is refused with
      `denied_authority`, and the invocation record shows both values.
- [ ] Two actors in one tenant with different policies receive different decisions **inside** the
      cache TTL.
- [ ] M05 unreachable → refusal, with the reason recorded.
- [ ] Existing capability funnel tests still pass; new tests cover all three cases above.

---

## WO-5 — Event-driven request ledger  `[S1-B]`

**Blocked on `S1-A` (durable execution).** Specified here for completeness; **do not start** until the
execution-path decision is made and implemented, or the work will be written twice.

**Why:** `modules/05-department-template-engine/internal/workloop/workloop.go:181-326` polls M03 over
HTTP every 15s and decides "rejected vs failed" by testing whether a node ID starts with `"gate"` or
an error string contains `"rejected"` (`workloop.go:362-378`). A customer naming a step "gateway
review" breaks it.

Half the fix already exists and is dead code: `modules/03-agent-orchestration/internal/events/events.go:616`
`PublishNodeCompleted` is **never called**, and M05 has **no Kafka consumer at all** — only publishers.

---

## WO-6 — Remaining byte-slicing truncation sites  `[follow-on from WO-1]`

Found during WO-1, verified by the supervisor, deliberately left unfixed because WO-1's scope named
only the two module-03 sites. Same defect class: byte-oriented slicing that splits multi-byte UTF-8.

| File | Line | Code | Notes |
|---|---|---|---|
| `modules/05-department-template-engine/internal/clients/clients.go` | 86 | `string(b[:n]) + "…"` | `truncate()`, called from `doJSON()` (lines 70, 74) to bound upstream M04/M07 error bodies to 200 bytes before embedding them in an error message |
| `modules/16-execution-sandbox/internal/sandbox/executor.go` | 164, 177 | `string(output[:maxOut])` | Raw sandboxed stdout/stderr capped at `MaxOutputSizeKB*1024`; no ellipsis |

**Sequencing note:** module 16 is slated for archival in `S0-A`. Fix M05 unconditionally; fix M16 only
if `S0-A` has been reconsidered. Do not fix M16 and then archive it.

Rune-safe reference implementations already in-tree: `03/internal/capability/client.go:109`,
`08/internal/simulated/simulated.go:106`, `05/internal/workloop/workloop.go:389`.

---

## Status log

### WO-1 — ✅ COMPLETE, independently verified · 2026-07-27
Branch `wo-1-rune-safe-truncation`, commit `8591dbf`, branched from `256220a`. **Not pushed.**
26 files: 3 source + 1 new test + 16 `go.mod` + 6 `Dockerfile`.

Supervisor verification performed (not accepted on report):
- Reverted both source files to `main` with the new tests in place → **all three tests fail**, output
  showing the dangling `\xd9` lead byte. Restored → pass. The tests genuinely catch the bug.
- `go test ./... -race -count=1` in module 03: 9 packages `ok`.
- `grep -h '^go ' modules/*/go.mod | sort -u` → exactly `go 1.25.0`.
- No Dockerfile builder older than the pin.
- No scope leakage: every touched file falls within the order's named scope.
- Checked the semantic change (`n` becomes a rune cap, not a byte cap, at 9 call sites): no downstream
  byte-size limit exists that this could overflow, and JSON Schema `maxLength` counts characters, so
  rune-capping is the more correct reading.

Outstanding nits, not blocking:
- `truncation_test.go:32-34` carries a comment pointing at "the WO-1 report" — a dead reference for
  any future reader. Trim to the part that explains what the test catches.
- The order had the agent pin toolchains in modules 12–16 and 19, which `S0-A` will archive. That is a
  defect in the work order's sequencing, not in the implementation: **`S0-A` should run before
  `S0-C`.** Corrected for future orders.

### WO-2 — ✅ ACCEPTED, independently verified · 2026-07-27
Branch `wo-2-outline-templates`, commit `6ad0e40`. **Not pushed.** 5 files, +222/-3.

M05 computes `operational` in `toTemplateListResponse`; the portal gates the deploy button on it.
Verified: rule is content-derived (`s.Config["capability"]` presence, reusing the same check
`deploy/orchestrator.go:609` already applies) with no template-ID allowlist; M05 suite green under
`-race`; classification is 1 operational (`it-medium-001`) / 30 outline, matching the catalogue census.

Accepted deviation: the order said "prefer M05-side if the list already carries workflows." The list
response does **not** carry them — which read literally points portal-side. The agent chose M05-side
anyway, because genuine portal-side computation would require either shipping all 639 steps on every
catalog load or an N+1 fetch per card. That reasoning is correct and the order's rule was too crude.

⚠️ **Deploy-ordering constraint:** the portal tests `t.operational === true`, so a portal deployed
ahead of this M05 build reads **every** template as an outline, including IT. Fail-safe, but it would
silently disable the demo. **Deploy M05 before or with the portal.**

### WO-3 — ⚠️ ACCEPTED AS SCAFFOLDING — NOT DONE · 2026-07-27
Branch `wo-3-demo-fixture`, commit `bd357c5`. **Not pushed.** New `tools/demo-fixture` module,
2,768 production Go + 2,363 test Go, 32 files. Zero existing files touched.

Verified: 4 packages green under `-race`; `--dry-run` provably makes no network calls (test server
fails on any request); fixture format validates and secret-scans.

**This work order is NOT complete and must not be reported as complete.** Its acceptance criteria
require a working round-trip, and no command in this tool has ever made a real HTTP call. The
committed `fixtures/smoke-tenant.yaml` is marked `provenance: hand-assembled` — transcribed from
handoff notes, not exported from the cluster. The runbook for the first live round-trip is in the
tool's README; **until step 6 of that runbook passes, treat this as untested tooling.**

Accepted deviation, and the agent's call was right: it refused to wire the provision-and-replay CI job
to fire on every push, because with no reachable service URLs from a GitHub runner it would fail on
every commit — presenting untested automation as live. Gated to `workflow_dispatch` instead.
**The handover's original criterion ("CI runs the provision-and-replay on every push") is hereby
amended**: on-push is correct only after the first successful live run.

⚠️ **Fragility to record:** `--replay`'s `findApprovalForRequest` correlates an M09 queue item to a
request by **title prefix matching**, because M09 exposes no endpoint returning an approval's
originating M05 request id. It refuses on ambiguity rather than guessing, which is the honest failure
mode — but this is the same correlation-by-string-matching class that `WO-5` exists to remove from
`workloop.go`. The real fix is an M09 affordance returning the correlation id, exactly as M04 gained a
caller-supplied `id` rather than tolerating direct DB repair.

### WO-4 — ✅ MERGED + DEPLOYED · HUMAN REVIEW CONFIRMED BY USER 2026-08-27
Branch `wo-4-server-side-authority`, commit `ab4388b`. **Merged to local `main` (fast-forward).**
**Deployed:** 2026-08-28. M08+M10 at `sha-aaa2d15`. The vulnerability is closed in production.

Human review performed 2026-08-26, all four priority items checked:
1. **Test rewrite** — coverage preserved and expanded. The old `// coordinate outranks execute and
   passes` assertion is gone. New tests: `TestClaimedTierNeverOverridesResolvedTier` (the exact
   exploit), `TestAuthorityBelowMinimumIsDenied` (3 sub-cases), `TestAuthorityAtOrAboveMinimumPasses`
   (understated claim), `TestUnresolvablePositionIsNoAuthority`,
   `TestPositionResolutionUnreachableDeniesClosed`, `TestCache_DifferentActorsGetDifferentDecisions`.
   All pass under `-race`.
2. **Double-outbound latency** — `positionclient.New()` has a 5-second timeout. Reasonable. No
   caching by design is documented in the package comment.
3. **Bearer token forwarding** — `positionclient.Resolve` forwards the caller's `Authorization`
   header to M05. Same trust assumption as `policyclient` for M10. If `JWT_SECRET` diverges between
   M08 and M05, everything fails closed. Runbook note needed before the hackathon.
4. **Stub limitation** — `positionStub` returns the same positions regardless of department ID.
   Known gap, not a blocker.

M05's `Position` struct confirms `AutonomyTier` with `json:"autonomy_tier,omitempty"` — matches what
`positionclient.orgChartResponse` parses. Empty tier → `""` → ranks below all tiers → denied.
Fail-closed confirmed.

Build + vet + tests green under `-race` in both M08 and M10.

**Remaining action before deploy:** verify M08 and M05 use the same `JWT_SECRET` in the cluster
environment. If they diverge, the capability layer fails closed and appears dead.

---

### WO-1 — ✅ MERGED + DEPLOYED · 2026-08-26 → 2026-08-28
Branch `wo-1-rune-safe-truncation`, commit `8591dbf`. **Merged to local `main`. Deployed 2026-08-28
(M03 at `sha-aaa2d15`).**

### WO-2 — ✅ MERGED + DEPLOYED · 2026-08-26 → 2026-08-28
Branch `wo-2-outline-templates`, commit `6ad0e40`. **Merged to local `main`. Deployed 2026-08-28
(M05 at `sha-aaa2d15`).**
Deploy-ordering constraint: M05 must be deployed before or with the portal. ✅ (M05 first, M21 last.)

### WO-3 — ✅ MERGED + PUSHED — LIVE ROUND-TRIP PENDING CLUSTER ACCESS · 2026-08-26
Branch `wo-3-demo-fixture`, commit `579ce93` (rewritten from `bd357c5` during the 2026-08-28
history scrub of a test fixture that tripped GitHub push protection). **Merged to local `main`,
pushed to origin.**
Build + vet + all 4 packages green under `-race`. Dry-run against committed fixture is clean:
fixture validates, plan is complete and sensible (tenant → user → department → seat binding →
workflow sync).

**Blocked on:** cluster access + M02 admin bootstrap password. The runbook's 7 steps are in
`tools/demo-fixture/README.md`. Step 3 (live export) overwrites the hand-assembled fixture with
`provenance: live-export` data. Step 6 (replay) is the gate — `findApprovalForRequest` has never
run against a real M09 queue.

Supervisor verification performed:
- `GET /departments/{id}/org-chart` **exists** (`departments.go:137`) and its envelope is
  `{root_position_id, positions[], edges[]}` with `Position.autonomy_tier` — matches what
  `positionclient` parses. This was the highest-consequence unverified assumption: a shape mismatch
  would have failed closed on every invocation and silently disabled the whole capability layer.
- `omitempty` on an absent tier yields `""` → ranks below every tier → denied. Correct fail-closed.
- M10 **does** populate `AgentID` (`handler/evaluate.go:69`), so the cache-key fix is real and not a
  no-op. (Worth checking explicitly: the sibling field `ResourceType` is famously never populated.)
- Exactly one production `Funnel` construction site (`main.go:100`), always sets `Positions`;
  `DepartmentURL` has a config default, so the dependency cannot be nil in any production path.
- Both suites green under `-race`; all five pre-existing refusal outcomes still pass unchanged.

🔴 **Finding that outranks the fix — the vulnerability was encoded as a passing test.**
Pre-fix `funnel_test.go` on `main` contained, with an approving comment
*"// coordinate outranks execute and passes"*, an assertion that an actor supplying
`AutonomyTier: "coordinate"` **in the request body** must reach `InvocationCompleted`. The exploit was
not merely unnoticed; it was the documented expected behaviour, and any refactor that accidentally
closed it would have been reverted as a test failure. Rewriting that test was necessary, not
cosmetic — and it means **the test suite cannot be treated as a security oracle** for the rest of
this codebase. Worth a dedicated review pass over other authority-adjacent tests.

For the human reviewer, in priority order:
1. The test rewrite above — confirm coverage was preserved, not just made to pass.
2. Every write-capability invocation now makes **two** live outbound calls (M10 + M05), uncached by
   design (caching authority would reintroduce the staleness class this order closes). Latency and a
   second availability dependency, both deliberate.
3. `positionclient` forwards the caller's bearer token to M05, same trust assumption `policyclient`
   already makes of M10. If signing secrets diverge in a real deployment, everything fails closed —
   safe, but the capability layer appears dead. **Verify against the cluster before merge.**
4. Test stub answers the same positions regardless of department id, so a wrong-department bug in
   M08 would not be caught here.

---

### WO-7 — PostgreSQL persistence for M01 + K8s infrastructure hardening · 2026-08-27 · MERGED + DEPLOYED

**Process note:** This work was started directly on local `main` without a work order, violating
Rule 1 ("Branch, never main"). The code was committed as `18993e1` before being regularized here.
The architect reviewed the engineering and found it sound; the process violation is recorded.
No further work will be started on main without a work order and a branch.

**Deployed:** 2026-08-28. Images built for `linux/amd64`, pushed to Docker Hub as `sha-aaa2d15`,
deployed to the cluster. All 7 affected pods (M01, M03, M04, M05, M08, M10, M21) running with
zero restarts. M03 confirmed connected to PostgreSQL (`orchestration` database).

**Additional fixes during deploy (not in the original commit):**
- M03: `pgx/stdlib` blank import added (commit `42fec95`) — the repository layer used
  `database/sql` but never registered the pgx driver. Crashed with "unknown driver pgx".
- M03: `orchestration` database created in the cluster (did not exist).
- M04/M08: live cluster env vars patched from missing `secret/operan-postgres` to
  `configMapKeyRef/operan-postgresql/DSN`.
- `operan-postgresql` ConfigMap: `DSN` key added to the live cluster.

**Scope (bundled in commit `18993e1`, 32 files, +2384/-74):**

1. **M01 (Tenant Control Plane) — PostgreSQL persistence from scratch.**
   - New `internal/database/` package: pgxpool + 11-table schema (tenants, subscriptions,
     secrets, deployments, environments, namespaces, resources, agents, invoices,
     payment_methods, policies). Complex nested fields stored as JSONB.
   - `internal/store/persist.go`: write-through sink. Every Create/Patch/Update/Delete calls
     `save()` to PostgreSQL. Failed writes log loudly but never fail the request.
   - `main.go`: fail-closed if `DATABASE_URL` set but unreachable. `Hydrate*()` reloads all 11
     stores at boot with a per-entity load summary.
   - `config.go`: `DatabaseURL` from `DATABASE_URL` env var. Empty → in-memory mode (unchanged).
   - K8s: `DATABASE_URL` wired to `operan-postgresql` ConfigMap `DSN` key.

2. **M03 (Agent Orchestration) — PostgreSQL enabled in K8s.**
   - `DB_MODE=postgres` + connection env vars added. The existing 18-table PostgreSQL
     repository layer is now actually used (was in-memory only).

3. **M04/M08 — pre-existing crash bug fixed.**
   - Both referenced a `secret` named `operan-postgres` that does not exist in any manifest.
     Would have crashed with `CreateContainerConfigError` on next deploy.
   - Fixed: added `DSN` key to `operan-postgresql` ConfigMap. Both modules switched to
     `configMapKeyRef`.

4. **K8s — hostPath → PVC.**
   - 4 module data volumes (M05, M07, M09, M11) converted from node-local `hostPath` to
     `ReadWriteOnce` PVCs (10Gi each). New `deploy/k8s/persistent-volumes.yaml`.
   - `fix-data-perms` init containers removed. Zero `hostPath` references remain in module
     deployments.

5. **M05 image sha-pinned** in `modules.yaml` to `sha-bcce0d9` (current cluster state).
   `:latest` would un-pin the cluster to whatever was last pushed to the registry.

6. **ops-all-001 dropped from flagship array** in portal `departments.js`. The template has
   0 capability steps; rendering it as flagship with an outline-gated deploy button was
   identified as a semantic collision.

**Acceptance criteria (verified 2026-08-27):**
- [x] M01: `go build ./...` — clean
- [x] M01: `go vet ./...` — clean
- [x] M01: `go test ./... -race -count=1` — all existing tests pass (config, handler,
      middleware, store packages). When `DATABASE_URL` is empty, stores behave exactly as
      before (`sink == nil` short-circuits every `save`).
- [x] K8s: `python3 -c "import yaml; yaml.safe_load_all(...)"` — modules.yaml (18 docs),
      persistent-volumes.yaml (4 docs), postgresql.yaml (3 docs) all parse clean.
- [x] K8s: Zero `hostPath` references in module deployments.
- [x] K8s: M04/M08 no longer reference the missing `operan-postgres` secret.
- [x] JWT_SECRET parity: M05 and M08 both source from `secret/operan-jwt/secret`
      (verified against the live cluster 2026-08-27).

**Remaining actions before deploy:**
- Build new images for M01, M03, M04, M08, M05 (WO-2), M10 (WO-4), M21 (flagship fix).
- Deploy with constraints: M05 before/with portal, M08+M10 together.
- Update M05 sha pin in `modules.yaml` after the new M05 image is built and pushed.

---

## Supervision protocol

The supervising agent verifies **independently**; it does not accept the implementing agent's report
as evidence.

For every work order:
1. Read the actual diff, not the summary.
2. Re-run the acceptance criteria personally.
3. For any "fixed a bug" claim, confirm the new test **fails** against the pre-fix code.
4. Check for scope leakage — files touched that the order did not name.
5. Check the fix generalises: the same defect class elsewhere in the tree is either fixed or recorded.

Trust is extended one work order at a time. WO-1 is the calibration task.
