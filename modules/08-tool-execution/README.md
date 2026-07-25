# Module 08 — Tool Execution

Secure execution layer for Operan. Registers tools, versions their schemas,
executes them on behalf of agents, and tracks execution records and cost. The
agent orchestrator (Module 03) calls this service so agents can take actions.

## The capability layer

SOPs name **business verbs** (capabilities), never vendors. Which system
performs a verb for a tenant is a **binding**; every execution passes one
governed funnel:

    resolve binding → validate input (JSON Schema) → policy check (M10, deny
    closed, caller's auth forwarded) → authority check (the acting seat's
    autonomy tier ≥ the verb's minimum) → dispatch → immutable invocation

Refusals — `blocked_no_binding`, `invalid_input`, `denied_policy`,
`denied_authority` — are recorded outcomes, not transport errors. Only the
**simulated provider** executes today; every record it produces carries
`simulated: true`, and swapping it for a live system is a binding change.

Surface: `GET /capabilities`, `GET|POST /providers`, `GET|POST /bindings`,
`POST /invoke`, `GET /invocations`. The legacy echo executor answers 410.

### Configuration

| Variable | Meaning |
|---|---|
| `MODULE08_DB_URL` | PostgreSQL DSN. **Set it** — bindings are customer config and invocations are the audit trail; configured-but-unreachable refuses to start. |
| `MODULE08_POLICY_URL` | Module 10 base URL. The funnel denies closed when it cannot answer. |
| `MODULE08_SEED_SIMULATED_TENANTS` | Comma-separated tenants that get the simulated provider bound to every verb at boot, idempotently. Demo bootstrap only. |

### Governance is tenant configuration

Module 10 is **default-deny**, so no capability executes until an allow policy
exists. Policies live in a policy group (scope `tenant`, `resource_type:
"all"`, `resource_target: "capability:<id>"`, effect `enforce`, action
`allow`). The demo tenant deliberately has no allow policy for
`identity.access.revoke`, so the default-deny path stays demonstrable.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/tools/register` | Register a new tool |
| GET | `/tools` | List tools (filter: `category`, `status`; paginated) |
| GET | `/tools/{id}` | Get tool details |
| PATCH | `/tools/{id}` | Update tool metadata |
| GET | `/tools/{id}/versions` | List a tool's versions |
| POST | `/execute` | Execute a tool for an agent |
| GET | `/executions` | List execution records (filter: `tool`, `status`) |
| GET | `/executions/{id}` | Get an execution record |
| POST | `/executions/{id}/retry` | Retry a failed execution |
| GET | `/cost` | Cost summary (optional `tool` scope) |
| GET | `/health` | Liveness probe (no auth) |

All API routes require `Authorization: Bearer <JWT>` and `X-Tenant-ID`. Tenant
isolation is enforced in every store.

## Execution model

`/execute` records the invocation, runs it through the in-process executor
(which currently echoes input as output and applies the tool's configured
`cost_per_call`), and emits lifecycle events
(`requested → started → completed`). A production deployment swaps the in-process
executor for a dispatch to the Module 16 sandbox without changing the API.

## Configuration (env)

| Var | Default | Notes |
|-----|---------|-------|
| `MODULE08_PORT` | `8008` | HTTP port |
| `MODULE08_JWT_SECRET` | — | **must** be set in production |
| `MODULE08_MAX_PAGE_SIZE` | `100` | Pagination clamp |
| `MODULE08_DEFAULT_TIMEOUT_MS` | `30000` | Default execution timeout |
| `MODULE08_EVENT_BROKER_URL` | — | If set, real broker (else log-only) |

## Contracts

- `contracts/v1/openapi-08-tool-execution.yaml` — OpenAPI specification
- `contracts/v1/asyncapi-08-tool-execution.yaml` — event channels
- `contracts/v1/schema-08-tool-execution.json` — JSON Schema definitions

## Development

```bash
go test ./... -cover   # all packages
go build ./...
```
