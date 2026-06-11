# Module 11 — Observability

The platform's eyes. A Kafka consumer ingests events from every Operan module
into trace spans, metrics, alerts, and component health; the REST API serves
them back as queryable telemetry. This is the module a demo dashboard reads.

Contracts: [`openapi-11-observability.yaml`](../../contracts/v1/openapi-11-observability.yaml) ·
[`asyncapi-11-observability.yaml`](../../contracts/v1/asyncapi-11-observability.yaml) ·
[`schema-11-observability.json`](../../contracts/v1/schema-11-observability.json)

## Endpoints (8 operations)

| Method | Path | Operation |
|--------|------|-----------|
| POST | `/metrics` | Record a metric (counter/gauge/histogram/timer) |
| GET | `/metrics` | Query metrics (filters: type, name, source, time range) |
| GET | `/spans` | Query trace spans (filters: trace, type, workflow, agent, status) |
| GET | `/traces/{id}` | Full trace with all spans + total duration |
| GET | `/alerts` | List alerts (filters: severity, resolved) |
| POST | `/alerts/{id}/resolve` | Resolve an alert |
| GET | `/health` | **Tenant system health** (contract endpoint, auth-required) |
| GET | `/health/{componentId}` | One component's health |

> Service liveness lives at **`/healthz`** (no auth) — the contract's
> `GET /health` is tenant-scoped system health and sits behind JWT + tenant
> middleware. Probes must use `/healthz`.

## Event ingestion (the demo pipeline)

When `MODULE11_EVENT_BROKER_URL` is set, one Kafka reader per topic joins
consumer group `module11-observability` and ingests every platform event:

- **Span** — `trace_id` = the event's `correlationId` (so multi-module flows
  group into one trace), `span_type` derived from the topic
  (memory/tool/policy/orchestration)
- **Metric** — `operan.events.consumed` counter, labeled by topic
- **Health** — the emitting module's component is upserted (healthy, or
  degraded on `.failed` events); transitions publish `health.status_change`
- **Alert** — `.failed` / `.deployment_failed` events fire a warning alert

Default subscription covers every topic modules 01–08 publish
(`internal/config.DefaultConsumeTopics`); override with a comma-separated
`MODULE11_CONSUME_TOPICS`.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MODULE11_PORT` | `8011` | HTTP listen port |
| `MODULE11_JWT_SECRET` | _(required)_ | HMAC-S256 secret; startup fails if unset or default |
| `MODULE11_EVENT_BROKER_URL` | *(empty)* | Kafka `host:port`; empty = log-only events, **consumer disabled** |
| `MODULE11_CONSUMER_GROUP` | `module11-observability` | Kafka consumer group |
| `MODULE11_CONSUME_TOPICS` | *(all platform topics)* | Comma-separated override |
| `MODULE11_MAX_PAGE_SIZE` | `100` | Pagination clamp |
| `MODULE11_DATA_DIR` | *(empty)* | Snapshot dir for restart persistence (k8s: hostPath at `/data`) |

## Events published (Kafka, 5 AsyncAPI channels)

`operan.observability.{metric.recorded, trace.span, trace.flush, alert.fired,
health.status_change}` — keyed by tenant. Per the AsyncAPI contract the
envelope (`correlationId`, `tenantId`, `messageId`, `timestamp`) is carried
in **message headers**, with the payload matching the message schema exactly.

## Build & run

```bash
go test ./...
MODULE11_JWT_SECRET="a-strong-secret-32-chars-or-more!" \
MODULE11_EVENT_BROKER_URL="localhost:9092" go run .
curl localhost:8011/healthz
```

## Known Limitations

| # | Limitation | Severity |
|---|-----------|----------|
| 1 | No database backend — stores are in-memory with JSON snapshot persistence (`MODULE11_DATA_DIR`) | Medium |
| 2 | JWT auth uses local secret (MVP) — should delegate to Module 02 IAM | P1 |
| 3 | Consumed-event spans have `duration_ms: 0` (events are points in time, not intervals) | Medium |
| 4 | `trace.flush` event defined but not wired (no batch persistence yet) | Low |
| 5 | No alert rules engine — alerts fire only on `.failed` event ingestion | Medium |
| 6 | Health derives only from event flow; silence does not mark components unhealthy | Medium |
