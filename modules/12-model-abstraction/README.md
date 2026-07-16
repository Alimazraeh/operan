# Module 12 — Model Abstraction Layer

Operan **Module 12** decouples all AI inference from specific model providers by providing a unified `POST /v1/models/completions` and `POST /v1/models/embeddings` interface.

## Purpose

Agents, orchestration, and workflows **never** call OpenAI, Anthropic, or local models directly. They call M12, which:

1. Resolves the model name → provider from the **Model Registry** database
2. Transforms the request/response to the provider's native format
3. Tracks token usage and calculates cost
4. Publishes Kafka events for observability and billing
5. Handles **automatic failover** to backup providers if the primary fails

## Architecture

```
┌─────────────┐    ┌──────────────┐    ┌──────────────────┐
│  M03 Orch.  │    │  M13 Routing │    │  External Providers│
│             │───▶│              │───▶│  (OpenAI, Anthropic│
│  M04 Agent  │    │              │    │   Ollama, Azure,   │
│  M19 Arabic │    │              │    │   LiteLLM)         │
└─────────────┘    └──────┬───────┘    └──────────────────┘
                          │
                    ┌─────▼─────┐
                    │  Module 12 │
                    │            │
                    │  /v1/models│
                    │  /completions│
                    │  /embeddings│
                    │            │
                    │  Provider   │
                    │  Adapters   │
                    │            │
                    │  Model      │
                    │  Registry   │
                    │            │
                    │  Cost      │
                    │  Tracking  │
                    │            │
                    │  Failover  │
                    │  Engine    │
                    └─────┬─────┘
                          │
                    ┌─────▼─────┐    ┌──────────┐
                    │  Postgres  │    │   Kafka   │
                    │  (3 tables)│    │ (5 events)│
                    └────────────┘    └──────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Kafka (or compatible event broker)

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `IAM_TOKEN_SECRET` | **yes** | HMAC secret for JWT validation (fail-closed if missing) |
| `DB_DSN` | **yes** | PostgreSQL connection string |
| `EVENT_BROKER_URL` | no | Kafka/AMQP broker URL (events logged to stdout if missing) |
| `PROVIDER_API_KEYS` | no | JSON array of provider keys: `[{"name":"openai","key":"sk-..."}]` |
| `HTTP_PORT` | no | Server port (default: 8012) |

### Build

```bash
go build -o model-abstraction .
```

### Run

```bash
export IAM_TOKEN_SECRET="your-jwt-secret"
export DB_DSN="postgresql://user:pass@localhost:5432/operan?sslmode=disable"
export PROVIDER_API_KEYS='[{"name":"openai","key":"sk-xxx"}]'

./model-abstraction
```

### Docker

```bash
docker build -t operan/model-abstraction .
docker run -p 8012:8012 \
  -e IAM_TOKEN_SECRET="your-secret" \
  -e DB_DSN="postgresql://..." \
  operan/model-abstraction
```

### Helm

```bash
helm install model-abstraction ./chart \
  --set env.IAM_TOKEN_SECRET="your-secret" \
  --set env.DB_DSN="postgresql://..."
```

## Database Schema

### `model_providers`

Registered LLM backends.

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `tenant_id` | VARCHAR(255) | Tenant scope |
| `name` | VARCHAR(100) | Provider name (e.g., "openai") |
| `type` | VARCHAR(30) | `openai`, `anthropic`, `litellm`, `ollama`, `azure`, `custom` |
| `base_url` | VARCHAR(500) | API endpoint |
| `api_key_secret_name` | VARCHAR(255) | Vault/K8s secret reference |
| `is_active` | BOOLEAN | Soft-delete flag |
| `priority` | INT | Failover priority (higher = preferred) |
| `max_retries` | INT | Retry count |
| `timeout_ms` | INT | Request timeout |
| `metadata` | JSONB | Provider-specific config |

### `model_registry`

Model name → provider mapping.

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `tenant_id` | VARCHAR(255) | Tenant scope |
| `model_name` | VARCHAR(200) | Logical name (e.g., "gpt-4") |
| `provider_id` | UUID | FK → model_providers |
| `provider_model_name` | VARCHAR(200) | Provider's internal model name |
| `supports_chat` | BOOLEAN | Can do completions |
| `supports_embed` | BOOLEAN | Can do embeddings |
| `max_tokens` | INT | Context window limit |
| `cost_per_token` | JSONB | `{"prompt": 0.00001, "completion": 0.00002}` |
| `is_default` | BOOLEAN | Default model for this name |
| `is_active` | BOOLEAN | Soft-delete flag |

### `model_calls`

Inference call audit trail.

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `tenant_id` | VARCHAR(255) | Tenant scope |
| `agent_id` | VARCHAR(255) | Calling agent (nullable) |
| `workflow_id` | VARCHAR(255) | Calling workflow (nullable) |
| `model_name` | VARCHAR(200) | Model used |
| `provider_id` | UUID | FK → model_providers |
| `prompt_tokens` | INT | Input tokens |
| `completion_tokens` | INT | Output tokens |
| `total_tokens` | INT | Total tokens |
| `cost_usd` | FLOAT | Calculated cost |
| `status` | VARCHAR(20) | `success`, `error`, `timeout`, `failover` |
| `error_message` | TEXT | Error details |
| `latency_ms` | INT | Call duration |

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | none | Health check |
| `POST` | `/v1/models/completions` | JWT + X-Tenant-ID | Unified chat completion |
| `POST` | `/v1/models/embeddings` | JWT + X-Tenant-ID | Unified embeddings |
| `GET` | `/v1/model-providers` | JWT + X-Tenant-ID | List providers |
| `POST` | `/v1/model-providers` | JWT + X-Tenant-ID + model_admin | Register provider |
| `PATCH` | `/v1/model-providers/{id}` | JWT + X-Tenant-ID + model_admin | Update provider |
| `DELETE` | `/v1/model-providers/{id}` | JWT + X-Tenant-ID + model_admin | Soft-delete provider |
| `GET` | `/v1/model-registry` | JWT + X-Tenant-ID | List model registry |
| `POST` | `/v1/model-registry` | JWT + X-Tenant-ID + model_admin | Register model |
| `PATCH` | `/v1/model-registry/{id}` | JWT + X-Tenant-ID + model_admin | Update model |

## Provider Adapters

| Adapter | Base URL Format | Notes |
|---------|----------------|-------|
| **OpenAI** | `https://api.openai.com` | Standard OpenAI API v1 |
| **Anthropic** | `https://api.anthropic.com` | Uses `/v1/messages` |
| **Ollama** | `http://localhost:11434` | Local model serving |
| **LiteLLM** | Custom | OpenAI-compatible proxy |
| **Azure OpenAI** | Custom | Uses deployment names, `api-key` auth |

## Kafka Events

| Topic | Payload | Consumer |
|-------|---------|----------|
| `operan.model.model_call_completed` | `{tenant_id, agent_id, model_name, provider, prompt_tokens, completion_tokens, cost_usd, latency_ms, status}` | M11 (Observability), M17 (Cost) |
| `operan.model.model_failover` | `{tenant_id, model_name, from_provider, to_provider, reason}` | M11 (Observability) |
| `operan.model.model_cost_recorded` | `{tenant_id, agent_id, model_name, cost_usd, billing_tag}` | M17 (Cost Governance) |

## Failover Logic

1. Resolve model name → provider from registry
2. Attempt call to primary provider
3. If failed, try next provider with same model by priority
4. Publish `model_failover` event on failover
5. Record call with `status=failover`

## Testing

```bash
go test ./... -count=1 -v
```

64 tests covering:
- Provider adapters (OpenAI, Anthropic, Ollama, LiteLLM)
- Config loading (defaults, fail-closed, custom port, provider keys)
- Event publishing (no-op broker, Kafka publisher)
- Middleware (JWT validation, tenant injection, RBAC)
- Handlers (completions, embeddings, provider CRUD, model CRUD)
- Store models (zero values, error types)