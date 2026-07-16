# Module 17 — Cost Governance Engine

> Prevent runaway LLM costs with real-time cost tracking, configurable budgets, automatic throttling, and alerting.

---

## Overview

The Cost Governance Engine (M17) is the financial guardrail for the Operan platform. It subscribes to cost events from M12 (model calls) and M08 (tool execution), enforces per-tenant/per-agent budgets, and can automatically throttle or hard-block spending when limits are reached.

### Key Concepts

- **Budgets**: Per-tenant or per-agent spending limits with configurable time periods (daily/weekly/monthly/quarterly). Each budget has soft and hard throttle thresholds.
- **Cost Events**: Ingested from M12 (model calls), M08 (tool execution), or manually via the API.
- **Throttling**: Two levels — **soft throttle** (warn + rate-limit, events still accepted) and **hard throttle** (reject future events).
- **Real-time tracking**: Every cost event is checked against active budgets before being accepted.
- **Alerting**: Threshold-based alerts at 80%, 90%, and 100% budget usage.

---

## Architecture

```
┌──────────┐   ┌──────────────────────┐   ┌──────────────┐
│  M12     │──►│                      │   │              │
│ (Model   │   │  Cost Governance     │──►│ Kafka        │
│  Abstr.) │   │  Engine (M17)        │   │ (events out) │
└──────────┘   │                      │   └──────────────┘
┌──────────┐   │  ┌────────────────┐  │
│  M08     │──►│  │ Cost Event     │  │
│ (Tool    │   │  │ Consumer       │  │
│  Exec.)  │   │  └────────────────┘  │
└──────────┘   │         │            │
               │         ▼            │
               │  ┌────────────────┐  │
               │  │ Budget Check   │  │
               │  │ Engine         │◄─┘
               │  └────────────────┘  │
               │         │            │
               │         ▼            │
               │  ┌────────────────┐  │
               │  │ PostgreSQL     │  │
               │  │ • cost_budgets │  │
               │  │ • cost_events  │  │
               │  │ • cost_alerts  │  │
               │  └────────────────┘  │
               │                      │
               │  ┌────────────────┐  │
               │  │ HTTP API       │  │
               │  │ (REST)         │  │
               │  └────────────────┘  │
               └──────────────────────┘
                        │
                        ▼
               ┌────────────────┐
               │  M21           │
               │  (Portal)      │
               │  Dashboard     │
               └────────────────┘
```

---

## Cost Event Flow

```
1. M12/M08 publishes model_cost_recorded event to Kafka
   └─► M17 CostEventConsumer receives it

2. M17 stores the event in cost_events table
   └─► Returns event ID

3. M17 BudgetEngine evaluates all active budgets
   ├─► percentage_used < soft_limit  → accepted, no action
   ├─► soft_limit ≤ percentage < hard_limit → soft throttle (warn)
   └─► percentage ≥ hard_limit → hard throttle (reject)

4. M17 creates alerts on threshold crossings
   └─► Publishes throttle_triggered to Kafka

5. M21 (Experience Portal) polls GET /v1/summary for dashboard
```

---

## Setup

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Kafka (for event consumer — optional for API-only mode)

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `IAM_TOKEN_SECRET` | Yes | JWT signing secret |
| `DB_DSN` | Yes | PostgreSQL connection string |
| `M12_BASE_URL` | No | M12 service URL (for integration) |
| `EVENT_BROKER_URL` | No | Kafka connection URL |
| `HTTP_PORT` | No | HTTP listen port (default: 8017) |

### Database Migration

```bash
psql $DB_DSN -f migrations/001_create_schema.sql
```

### Build & Run

```bash
go build -o cost-governance .
./cost-governance
```

### Docker

```bash
docker build -t operan/cost-governance:latest .
docker run -p 8017:8017 \
  -e IAM_TOKEN_SECRET=your-secret \
  -e DB_DSN="postgres://user:pass@host:5432/costgov" \
  operan/cost-governance:latest
```

### Kubernetes (Helm)

```bash
helm install cost-governance ./chart \
  --set env.IAM_TOKEN_SECRET=your-secret \
  --set env.DB_DSN="postgres://user:pass@host:5432/costgov"
```

---

## API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/health` | Health check | None |
| POST | `/v1/budgets` | Create budget | JWT + Tenant |
| GET | `/v1/budgets` | List budgets | JWT + Tenant |
| GET | `/v1/budgets/{id}` | Get budget | JWT + Tenant |
| PATCH | `/v1/budgets/{id}` | Update budget | JWT + Tenant |
| DELETE | `/v1/budgets/{id}` | Delete budget | JWT + Tenant |
| POST | `/v1/cost-events` | Ingest cost event | JWT + Tenant |
| GET | `/v1/cost-events` | List cost events | JWT + Tenant |
| GET | `/v1/summary` | Cost summary | JWT + Tenant |
| GET | `/v1/alerts` | List alerts | JWT + Tenant |
| GET | `/v1/throttle` | Get throttle state | JWT + Tenant |
| PATCH | `/v1/throttle/{status}` | Override throttle | JWT + Tenant |

See `contracts/openapi-17-cost-governance.yaml` for full API specification.

---

## Integration Points

| Module | How |
|--------|-----|
| **M12 Model Abstraction** | M12 publishes `model_cost_recorded` events → M17 consumes asynchronously. M17 maintains its own cost ledger. |
| **M08 Tool Execution** | M08 publishes cost events → M17 consumes via the same consumer pattern. |
| **M21 Experience Portal** | M21 calls `GET /v1/summary` for dashboard spending display. |
| **M03 Orchestration** | Optional: M03 can check M17 throttle before starting expensive workflows. |

---

## Throttle States

| State | Behavior |
|-------|----------|
| `none` | Default. All cost events accepted. |
| `soft` | Warnings emitted. Events still accepted but rate-limited. |
| `hard` | All cost events rejected. Manual override required to release. |

Throttle can be triggered automatically when budgets are exceeded, or manually via the API.

---

## Kafka Events

| Topic | Direction | Description |
|-------|-----------|-------------|
| `operan.model.model_cost_recorded` | In | From M12 |
| `operan.tool-execution.cost_recorded` | In | From M08 |
| `operan.cost.budget_exceeded` | Out | Budget exceeded alert |
| `operan.cost.throttle_triggered` | Out | Throttle state change |
| `operan.cost.throttle_released` | Out | Throttle cleared |
| `operan.cost.alert_created` | Out | New alert created |
| `operan.cost.cost_recorded` | Out | Cost event recorded confirmation |

---

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests (requires database)
go test -tags=integration ./...
```

---

*Next: Module 18 — Enterprise Connector Fabric*