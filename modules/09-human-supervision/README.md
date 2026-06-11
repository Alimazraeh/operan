# Module 09 — Human Supervision

The human-in-the-loop layer for Operan. Agent workflows (Module 03) raise
**approval gates** here and pause; humans approve, reject, or delegate.
Incidents become **escalations**; supervisors issue **interventions**
(pause/stop/restrict an agent); everything pending lands in one **review
queue**, summarized by a **risk dashboard**.

Contracts: [`openapi-09-human-supervision.yaml`](../../contracts/v1/openapi-09-human-supervision.yaml) ·
[`asyncapi-09-human-supervision.yaml`](../../contracts/v1/asyncapi-09-human-supervision.yaml) ·
[`schema-09-human-supervision.json`](../../contracts/v1/schema-09-human-supervision.json)

## Endpoints (20 operations)

| Area | Operations |
|------|-----------|
| Approvals | `POST /approvals`, `GET/PATCH/DELETE /approvals/{id}`, `POST /approvals/{id}/{approve,reject,delegate}` |
| Escalations | `POST /escalations`, `GET/PATCH/DELETE /escalations/{id}`, `POST /escalations/{id}/resolve` |
| Interventions | `POST /interventions`, `GET/PATCH/DELETE /interventions/{id}`, `POST /interventions/{id}/revoke` |
| Queue | `GET /queue` (merged approvals + escalations + interventions; `type`/`user_id` filters) |
| Risk | `GET /risk-dashboard` (counts, severity-weighted risk score 0–100, breakdowns) |
| HITL | `POST /hitl/{request_id}/answer` (one answer per request; approve/reject answers also decide the originating gate) |

All endpoints except `/health` require `Authorization: Bearer <JWT>` (HMAC-S256)
and `X-Tenant-ID`. Errors use the module 09 contract schema:
`{ "error": { "code": "NOT_FOUND", "message", "details", "request_id" } }`.

## Decision semantics

- **Default**: first approval approves; first rejection rejects.
- **Threshold type**: `threshold_config.min_approvals` approvals required;
  rejections beyond `max_rejections` auto-reject.
- **Expiry**: `expires_at` is enforced lazily — a read or action on a
  past-deadline approval transitions it to `expired` and publishes
  `gate.timeout`. Actions on terminal approvals return **409**.
- **Delegate** reassigns the gate (status `delegated`, still decidable)
  and publishes `gate.escalated`.

## Events (Kafka, 5 AsyncAPI channels)

`operan.supervision.gate.{raised,responded,escalated,timeout}` +
`operan.supervision.policy.violation_detected` — tenant-keyed, envelope
(`correlationId`, `tenantId`, `messageId`, `timestamp`) in message headers.

| Trigger | Event |
|---------|-------|
| Approval created | `gate.raised` |
| Approve / reject (incl. via HITL answer) | `gate.responded` |
| Delegate | `gate.escalated` |
| Expiry detected | `gate.timeout` |
| Security/compliance escalation with a source agent | `policy.violation_detected` |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MODULE09_PORT` | `8009` | HTTP listen port |
| `MODULE09_JWT_SECRET` | _(required)_ | HMAC-S256 secret; startup fails if unset or default |
| `MODULE09_EVENT_BROKER_URL` | *(empty)* | Kafka `host:port`; empty = log-only events |
| `MODULE09_MAX_PAGE_SIZE` | `100` | Pagination clamp |

## Build & run

```bash
go test ./...
MODULE09_JWT_SECRET="a-strong-secret-32-chars-or-more!" go run .
curl localhost:8009/health
```

Docker: multi-stage build, non-root user, port 8009. Helm: `chart/`.

## Known Limitations

| # | Limitation | Severity |
|---|-----------|----------|
| 1 | No database backend — all stores are in-memory | P1 |
| 2 | JWT auth uses local secret (MVP) — should delegate to Module 02 IAM | P1 |
| 3 | Expiry is lazy (on read/action); no background timer publishes `gate.timeout` for untouched approvals | Medium |
| 4 | Interventions are recorded but not enforced — Module 03 must consume them to actually pause agents | Medium |
| 5 | `conditional` approval type accepts config but evaluates like `parallel` (no expression engine) | Medium |
| 6 | Approval list endpoint not in contract; queue is the discovery surface | Info |
