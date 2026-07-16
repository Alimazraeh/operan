# Module 16 — Execution Sandbox

Isolated execution environments for agent tool calls and code execution.

## Architecture

### Isolation Model

M16 provides containerless isolation using Go's syscall credentials, resource limits, and network gating — not Docker. Each sandbox execution:

1. **Process isolation**: Runs as non-root (UID 65534) via `syscall.Credential`
2. **Chroot filesystem**: Minimal chroot with only the tool's required files + sandbox workspace
3. **Resource limits**: `RLIMIT_CPU`, `RLIMIT_AS` restrict CPU time and memory
4. **Network gating**: Outbound network access gated by M10 policy
5. **Timeout enforcement**: Hard kill via `time.AfterFunc` + `SIGKILL`
6. **Output limits**: stdout/stderr buffered to configured max size

### Tool Execution Flow

```
1. Validate tool is in profile's allowed_tools list
2. Create isolated workspace directory (tenant-scoped)
3. Build execution environment (env vars, cwd, credentials)
4. Set resource limits (RLIMIT_CPU, RLIMIT_AS)
5. Start process as non-root
6. Monitor: collect stdout/stderr with size limits
7. On timeout/kill: SIGKILL
8. Capture: exit code, CPU time, memory peak
9. Clean up: remove workspace
```

### Policy Integration

Before executing any tool, M16 calls M10's policy engine (`POST /v1/policies/check`). If M10 denies the execution, the instance is recorded with `status=policy_denied`.

### Kafka Events

| Topic | Trigger |
|-------|---------|
| `operan.sandbox.execution_started` | Execution begins |
| `operan.sandbox.execution_completed` | Execution finished successfully |
| `operan.sandbox.execution_failed` | Execution failed with error |
| `operan.sandbox.execution_timeout` | Execution timed out |
| `operan.sandbox.policy_denied` | M10 policy denied execution |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/sandboxes/execute` | Execute a tool in a sandbox |
| GET | `/v1/sandboxes/instances` | Paginated execution history |
| GET | `/v1/sandboxes/instances/{id}` | Execution result |
| GET | `/v1/sandbox-profiles` | Paginated profiles |
| POST | `/v1/sandbox-profiles` | Create execution profile |
| PATCH | `/v1/sandbox-profiles/{id}` | Update profile |
| DELETE | `/v1/sandbox-profiles/{id}` | Remove profile |
| POST | `/v1/sandboxes/instances/{id}/cancel` | Kill a running instance |
| GET | `/health` | Health check (unauthenticated) |

All routes except `/health` require `Bearer` JWT + `X-Tenant-ID` header.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `JWT_SECRET` | Yes | — | HMAC-S256 secret for JWT validation |
| `JWT_ISSUER` | No | `operan-tenant-control-plane` | Expected JWT issuer |
| `DB_DSN` | Yes | — | PostgreSQL connection string |
| `EVENT_BROKER_URL` | No | — | Kafka broker URL (log-only fallback if empty) |
| `M10_BASE_URL` | No | `http://localhost:8010` | Policy governance engine URL |
| `HTTP_PORT` | No | `8016` | HTTP listen port |

## Startup

```bash
# Build
go build -o execution-sandbox .

# Run
JWT_SECRET=secret DB_DSN="postgres://..." ./execution-sandbox

# Docker
docker build -t execution-sandbox:latest .
docker run -p 8016:8016 \
  -e JWT_SECRET=secret \
  -e DB_DSN="postgres://..." \
  execution-sandbox:latest

# Helm
helm install execution-sandbox ./chart \
  --set image.repository=registry.operan.dev/execution-sandbox \
  --set image.tag=1.0.0 \
  --set env.JWT_SECRET=secret \
  --set env.DB_DSN="postgres://..."
```

## Database Migrations

```bash
# Apply migrations
psql $DB_DSN -f migrations/001_create_schema.sql
```

## Integration

| Module | Integration |
|--------|------------|
| M10 Policy Governance | M16 calls M10 to check tool execution before running |
| M12 Model Abstraction | Local model inference must go through M16 |
| M03 Orchestration | M03 calls M16's execute endpoint for tool-dependent workflow nodes |
| M08 Tool Execution | M08 delegates execution to M16 sandboxes |
| M21 Experience Portal | M21 calls M16's instances endpoint for execution history |

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover
```