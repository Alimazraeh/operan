# Module 13 — Multi-Model Routing Engine

Sits between callers and M12's abstraction layer. Examines each request's task type, context length, and constraints, then selects the optimal model based on weighted scoring, performance history, and cost targets.

## Purpose

Agents don't hardcode model names. They call M13 with a task type (`summarize`, `classify`, `generate`, `extract`, `embed`, `chat`), and M13 resolves which model to use. This enables:

- **Automatic cost optimization** — use cheaper models for simple tasks
- **Reliability** — auto-failover to backup models
- **Learned routing** — scoring adapts from past performance (latency, error rates, quality)

## Routing Algorithm

When a request arrives at `POST /v1/resolve`, the engine:

1. **Fetches active routing rules** matching the request's `task_type` for the tenant
2. **For each rule, fetches associated models** from `routing_rule_models`
3. **Computes a composite score** for each model:

```
score = capability_score * 0.40
      + (100 - cost_weight) * 0.20
      + (100 - latency_weight) * 0.15
      + reliability_weight * 0.25
      + quality_score_from_perf * 0.15
```

4. **Filters** out models exceeding `max_latency_ms` or `max_tokens` constraints
5. **Returns** the highest-scoring model + fallback (2nd highest)
6. **If no rule matches**, returns a default model for the task type

### Default Model Map

| Task Type | Default Model |
|-----------|--------------|
| summarize | qwen-turbo |
| classify | qwen-turbo |
| generate | qwen-max |
| extract | qwen-plus |
| chat | qwen-plus |
| embed | text-embedding-ada-002 |
| general | qwen-plus |

## Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/health` | Health check | None |
| POST | `/v1/resolve` | Main routing endpoint | JWT + Tenant |
| GET | `/v1/rules` | List routing rules | JWT + Tenant |
| POST | `/v1/rules` | Create rule | JWT + Tenant |
| PATCH | `/v1/rules/{id}` | Update rule | JWT + Tenant |
| DELETE | `/v1/rules/{id}` | Delete rule | JWT + Tenant |
| GET | `/v1/performance` | Performance stats | JWT + Tenant |
| GET | `/v1/models` | List available models | JWT + Tenant |

## Database Schema

Three tables:

- **`routing_rules`** — Tenant-scoped routing rules with constraints
- **`routing_rule_models`** — Model assignments per rule with weights
- **`routing_performance`** — Per-model, per-task performance metrics (updated via Kafka events or API)

## Kafka Events

| Topic | When |
|-------|------|
| `operan.model.route.resolved` | A routing decision is made |
| `operan.model.route.fallback_triggered` | Primary model failed, fallback used |
| `operan.model.route.performance_recorded` | Performance metric recorded |

## Integration Points

| Module | How |
|--------|-----|
| M12 Model Abstraction | M13 calls M12's `/v1/models/completions` with the resolved model |
| M03 Orchestration | M03 calls M13's `/v1/resolve` before executing LLM-dependent workflow nodes |
| M04 Agent Registry | M04 stores each agent's preferred routing rule in agent config |
| M21 Experience Portal | M21 calls M13's `/v1/performance` for model selection dashboard |

## Setup

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No (default: 8013) | HTTP listen port |
| `JWT_SECRET` | Yes | HMAC secret for JWT validation |
| `DB_DSN` | Yes | PostgreSQL connection string |
| `M12_BASE_URL` | Yes | URL of the Model Abstraction Layer |
| `EVENT_BROKER_URL` | No | Kafka broker URL (comma-separated) |

### Build

```bash
go build ./...
go test ./...
```

### Docker

```bash
docker build -t model-routing .
docker run -p 8013:8013 --env-file .env model-routing
```

### Helm

```bash
helm install model-routing ./chart/ \
  --set image.tag=latest \
  --set m12BaseUrl=http://model-abstraction:8012
```

## Project Structure

```
modules/13-model-routing/
├── main.go                     # Listen port 8013
├── Dockerfile
├── chart/                      # Helm chart
├── contracts/
│   └── openapi-13-model-routing.yaml
├── migrations/
│   └── 001_create_schema.sql
├── internal/
│   ├── config/                 # JWT_SECRET, M12_BASE_URL, DB_DSN
│   ├── ctxkeys/                # Context key types
│   ├── middleware/             # JWT + tenant isolation
│   ├── handler/                # HTTP handlers
│   ├── store/                  # PostgreSQL persistence
│   ├── engine/                 # Routing + scoring algorithm
│   ├── events/                 # Kafka event publishing
│   └── clients/                # HTTP client for M12
```