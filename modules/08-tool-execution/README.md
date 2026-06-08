# Module 08 — Tool Execution

Secure execution layer for Operan. Registers tools, versions their schemas,
executes them on behalf of agents, and tracks execution records and cost. The
agent orchestrator (Module 03) calls this service so agents can take actions.

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
