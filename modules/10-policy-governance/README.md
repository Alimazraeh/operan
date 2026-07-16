# Policy Governance Engine

The **Policy Governance Engine** (Module 10) is the centralized rules engine for the Operan platform. It evaluates policies before any agent action, enforcing access control, data usage restrictions, rate limits, audit requirements, and compliance rules.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│   M08       │────▶│              │────▶│    Kafka     │
│ Tool Exec   │     │  Policy      │     │  Events      │
│   M12       │────▶│  Engine      │────▶│  (evaluation │
│   M17       │     │              │     │   + violation)│
│   M04       │◀────│              │     └──────────────┘
└─────────────┘     └──────────────┘              │
       │                   │                       │
       │              ┌──────────┐                 │
       │              │  Policy  │◀────────────────┘
       │              │  Store   │
       │              │(Postgres)│
       │              └──────────┘
       │
┌──────▼──────────┐
│  Experience     │
│  Portal (M21)   │
└─────────────────┘
```

## Policy Evaluation Algorithm

1. **Load** active policies for the tenant, ordered by priority (highest first)
2. **Short-circuit**: If any `deny` policy matches → deny immediately
3. **Scope matching**: Only evaluate policies whose scope matches (agent < department < tenant < global)
4. **Condition evaluation**: For each policy, check if `condition_expression` matches the request
5. **Apply effects**:
   - `enforce + deny` → deny, return immediately
   - `enforce + allow` → allow, return immediately
   - `enforce + proxy` → return "proxied" (needs human approval via M09)
   - `warn` → collect warnings, continue evaluating
   - `log` → just audit, continue
6. **Default**: If no policy matches, default to deny

## Condition Expression Format

Conditions are JSON expressions evaluated by the engine:

```json
{
  "op": "and",
  "conditions": [
    {"field": "action", "op": "in", "value": ["send_email", "send_invoice"]},
    {"field": "cost", "op": "lt", "value": 1000},
    {"field": "data_class", "op": "not_in", "value": ["restricted"]}
  ]
}
```

### Supported Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `eq` | Equal | `{"field": "action", "op": "eq", "value": "send_email"}` |
| `neq` | Not equal | `{"field": "scope", "op": "neq", "value": "global"}` |
| `in` | Value in array | `{"field": "action", "op": "in", "value": ["send_email", "send_invoice"]}` |
| `not_in` | Value not in array | `{"field": "data_class", "op": "not_in", "value": ["restricted"]}` |
| `gt` | Greater than | `{"field": "cost", "op": "gt", "value": 1000}` |
| `lt` | Less than | `{"field": "cost", "op": "lt", "value": 1000}` |
| `gte` | Greater than or equal | `{"field": "cost", "op": "gte", "value": 500}` |
| `lte` | Less than or equal | `{"field": "cost", "op": "lte", "value": 500}` |
| `exists` | Field exists and non-empty | `{"field": "metadata.sensitive", "op": "exists"}` |
| `and` | All conditions must be true | `{"op": "and", "conditions": [...]}` |
| `or` | Any condition must be true | `{"op": "or", "conditions": [...]}` |

## Policy Scopes

| Scope | Description |
|-------|-------------|
| `global` | Applies to all tenants, agents, and departments |
| `tenant` | Applies to all agents within a specific tenant |
| `department` | Applies to all agents within a specific department |
| `agent` | Applies to a specific agent |

## API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/v1/policies` | Create a policy rule | JWT + Tenant |
| GET | `/v1/policies` | List policies (paginated) | JWT + Tenant |
| GET | `/v1/policies/{id}` | Get policy detail | JWT + Tenant |
| PATCH | `/v1/policies/{id}` | Update policy | JWT + Tenant |
| DELETE | `/v1/policies/{id}` | Delete policy (soft) | JWT + Tenant |
| POST | `/v1/policies/evaluate` | Evaluate policies for a request | JWT + Tenant |
| GET | `/v1/policy-groups` | List policy groups (paginated) | JWT + Tenant |
| POST | `/v1/policy-groups` | Create policy group | JWT + Tenant |
| GET | `/v1/policy-groups/{id}` | Get policy group detail | JWT + Tenant |
| PATCH | `/v1/policy-groups/{id}` | Update policy group | JWT + Tenant |
| DELETE | `/v1/policy-groups/{id}` | Delete policy group (soft) | JWT + Tenant |
| GET | `/v1/audit` | List policy evaluation audit log | JWT + Tenant |
| GET | `/health` | Health check | None |

## Kafka Events

| Topic | Description |
|-------|-------------|
| `operan.policy.evaluation` | Published on every policy evaluation |
| `operan.policy.violation` | Published when a policy is violated |
| `operan.policy.policy_updated` | Published when a policy is updated |
| `operan.policy.group_updated` | Published when a policy group is updated |

## Integration Points

| Module | How |
|--------|-----|
| M02 IAM | M10 validates agent identity via M02 before evaluating policies |
| M04 Agent Registry | M04 calls M10's `/v1/policies/evaluate` before executing agent tools |
| M08 Tool Execution | M08 calls M10's `/v1/policies/evaluate` before executing any tool |
| M09 Human Supervision | M10 `proxy` effect triggers M09 to create an approval gate |
| M12 Model Abstraction | M12 calls M10's `/v1/policies/evaluate` before making model calls |
| M17 Cost Governance | M17 calls M10's `/v1/policies/evaluate` before spending beyond budget |
| M21 Experience Portal | M21 shows policy dashboard, audit log, and policy management UI |

## Setup

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | Port to listen on | `8010` |
| `JWT_SECRET` | Secret for JWT validation | *(required in production)* |
| `ISSUER` | Expected JWT issuer | `operan-tenant-control-plane` |
| `DB_DSN` | PostgreSQL connection string | `postgres://...` |
| `EVENT_BROKER_URL` | Kafka broker URL | `localhost:9092` |
| `M04_BASE_URL` | M04 Agent Registry URL | `http://localhost:8004` |

### Database Migrations

```bash
psql $DB_DSN -f migrations/001_create_schema.sql
```

### Running

```bash
go build -o policy-governance .
./policy-governance
```

### Docker

```bash
docker build -t policy-governance .
docker run -p 8010:8010 \
  -e JWT_SECRET=your-secret \
  -e DB_DSN=your-db-conn \
  policy-governance
```

### Helm

```bash
helm install policy-governance ./chart \
  --set config.dbDsn="your-db-conn" \
  --set secrets.jwtSecret="your-secret"
```

## Testing

```bash
go test ./... -v -cover
```

### Test Requirements (40+ tests)

- **Engine**: evaluate with no matching policies (default deny), single allow policy, single deny policy, deny overrides allow, proxy triggers warning, multiple policies priority ordering, condition expression evaluation (eq, in, lt, and, or, not_in)
- **Policy CRUD**: create all action types, list with filters, get by ID, update fields, soft delete
- **Group CRUD**: create, list, get, update, delete (with policy references)
- **Audit**: list with filters (agent, result, date range), pagination
- **Condition evaluation**: simple eq, complex and/or, nested conditions, unknown field, missing field
- **Cache**: policy cache hit, cache miss, invalidation on policy update
- **Middleware**: JWT validation, tenant isolation
- **M04 client**: agent validation, M04 unavailable
- **Kafka events**: publish evaluation, violation, policy_updated