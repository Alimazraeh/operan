# Module 07 — Memory Fabric

Vector memory storage, semantic search, and retention management for Operan
agents. Agents (via Module 03) store and retrieve semantic memories here;
Module 06 feeds it ingested knowledge.

Contracts: [`openapi-07-memory-fabric.yaml`](../../contracts/v1/openapi-07-memory-fabric.yaml) ·
[`asyncapi-07-memory-fabric.yaml`](../../contracts/v1/asyncapi-07-memory-fabric.yaml) ·
[`schema-07-memory-fabric.json`](../../contracts/v1/schema-07-memory-fabric.json)

## Endpoints (10 operations)

| Method | Path | Operation |
|--------|------|-----------|
| POST | `/vectors` | Batch-ingest memory vectors |
| GET | `/vectors` | List vectors (filters: `embedding_type`, `segment_type`, `document_id`) |
| GET | `/vectors/{id}` | Get vector |
| PUT | `/vectors/{id}` | Update vector (content, metadata, segment type, TTL — `null` clears) |
| DELETE | `/vectors/{id}` | Delete vector |
| POST | `/search` | Semantic search within one embedding scope |
| GET | `/agents/{id}` | Aggregate agent memory state |
| POST | `/gc` | Trigger garbage collection (supports `dry_run`) |
| GET | `/retention-policies` | List retention policies |
| POST | `/retention-policies` | Create retention policy |

All endpoints except `/health` require `Authorization: Bearer <JWT>` (HMAC-S256)
and `X-Tenant-ID`. Errors use the contract schema
`{code, message, details, request_id}`.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MODULE07_PORT` | `8007` | HTTP listen port |
| `MODULE07_JWT_SECRET` | _(required)_ | HMAC-S256 secret; startup fails if unset or default |
| `MODULE07_EVENT_BROKER_URL` | *(empty)* | Kafka broker `host:port`; empty = log-only events |
| `MODULE07_MAX_PAGE_SIZE` | `100` | Pagination clamp |
| `MODULE07_GC_BATCH_SIZE` | `1000` | Max vectors removed per GC run |
| `MODULE07_EMBEDDINGS_URL` | *(empty)* | OpenAI-compatible gateway (LiteLLM); empty = token-overlap fallback |
| `MODULE07_EMBEDDINGS_API_KEY` | *(empty)* | Gateway API key |
| `MODULE07_EMBEDDINGS_MODEL` | `qwen3-embedding-4b` | Embeddings model id |
| `MODULE07_DATA_DIR` | *(empty)* | Snapshot dir for restart persistence (k8s: hostPath at `/data`) |
| `MODULE07_DB_URL` / `MODULE07_REDIS_URL` | *(empty)* | Reserved — stores are in-memory (see Known Limitations) |

## Events (Kafka, 5 AsyncAPI channels)

`operan.memory.vector.{ingested,searched,updated,deleted,garbage_collected}` —
keyed by tenant ID, with the platform envelope
(`correlationId`, `tenantId`, `messageId`, `timestamp`).

## Search semantics

- With `MODULE07_EMBEDDINGS_URL` configured (the platform LiteLLM gateway),
  ingest vectorizes content through the embeddings model
  (default `qwen3-embedding-4b`) and search embeds the query — results rank
  by **cosine similarity** over real embeddings.
- Callers may still supply `query_vector` / `embedding_vector` directly.
- Without a gateway (or if a call fails), a deterministic **token-overlap**
  score over `semantic_content` is the fallback — ingest never fails on an
  embeddings outage.
- Scores are clamped to `[0, 1]`; `relevance_threshold` filters,
  `top_n` truncates (default 10).

## Build & run

```bash
go test ./...
MODULE07_JWT_SECRET="a-strong-secret-32-chars-or-more!" go run .
curl localhost:8007/health
```

Docker: multi-stage build, non-root user, port 8007, `/health` healthcheck.
Helm: `chart/` (deployment, service, ingress, HPA, serviceaccount).

## Known Limitations

| # | Limitation | Severity |
|---|-----------|----------|
| 1 | No database backend — stores are in-memory with JSON snapshot persistence (`MODULE07_DATA_DIR`) | Medium |
| 2 | ~~Token-overlap search~~ — RESOLVED: real embeddings via LiteLLM gateway (token overlap remains the no-gateway fallback) | Done |
| 3 | JWT auth uses local secret (MVP) — should delegate to Module 02 IAM | P1 |
| 4 | Retention policies are stored but not auto-enforced; GC is manual (`POST /gc`) | Medium |
| 5 | API-initiated deletes publish reason `document_deleted` (closest enum value) | Low |
| 6 | Agent ephemeral window is a fixed default (8192 tokens / 3600 s) | Low |
