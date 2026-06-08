# Module 17 — Cost Governance Engine

> Prevent runaway operational costs through token budgets, execution quotas, agent throttling, usage forecasting, and tenant billing.

---

## Description

The Cost Governance Engine ensures predictable operational spending across the Operan platform. It provides comprehensive cost control mechanisms including token usage budgets, execution quotas, agent-level throttling, usage forecasting with recommendations, and tenant billing management. This module is critical for multi-tenant deployments, cost allocation, and preventing unexpected operational expenditure.

---

## Key Features

- **Token Budgets**: Set and monitor token usage budgets per tenant, department, team, agent, user, or model with configurable alert thresholds.
- **Execution Quotas**: Control the number of executions per target within defined time periods, with override capabilities for privileged roles.
- **Agent Throttling**: Rate-limit agents and models with configurable queuing, delaying, rejection, and exponential backoff strategies.
- **Usage Forecasting**: Predict future token usage, execution counts, costs, and resource utilization with confidence intervals and recommendations.
- **Tenant Billing**: Track and manage billing records with charge itemization, invoice generation, and cost tracking.
- **Cost Dashboard**: Real-time visibility into cost metrics, budget utilization, and forecasting summaries.

---

## API Documentation

| Resource | Method | Endpoint | Description |
|----------|--------|----------|-------------|
| **Token Budgets** | `POST` | `/api/v1/cost-governance/token-budgets` | Create token budget |
| | `GET` | `/api/v1/cost-governance/token-budgets` | List token budgets |
| | `GET` | `/api/v1/cost-governance/token-budgets/{id}` | Get token budget details |
| | `PATCH` | `/api/v1/cost-governance/token-budgets/{id}` | Update token budget |
| | `DELETE` | `/api/v1/cost-governance/token-budgets/{id}` | Delete token budget |
| | `GET` | `/api/v1/cost-governance/token-budgets/{id}/usage` | Get token usage for budget |
| | `POST` | `/api/v1/cost-governance/token-budgets/{id}/alert` | Trigger token budget alert |
| **Execution Quotas** | `POST` | `/api/v1/cost-governance/execution-quotas` | Create execution quota |
| | `GET` | `/api/v1/cost-governance/execution-quotas` | List execution quotas |
| | `GET` | `/api/v1/cost-governance/execution-quotas/{id}` | Get execution quota details |
| | `PATCH` | `/api/v1/cost-governance/execution-quotas/{id}` | Update execution quota |
| | `DELETE` | `/api/v1/cost-governance/execution-quotas/{id}` | Delete execution quota |
| | `POST` | `/api/v1/cost-governance/execution-quotas/{id}/check` | Check execution quota |
| **Agent Throttling** | `POST` | `/api/v1/cost-governance/agent-throttling` | Create agent throttling rule |
| | `GET` | `/api/v1/cost-governance/agent-throttling` | List agent throttling rules |
| | `GET` | `/api/v1/cost-governance/agent-throttling/{id}` | Get agent throttling rule |
| | `PATCH` | `/api/v1/cost-governance/agent-throttling/{id}` | Update agent throttling rule |
| | `DELETE` | `/api/v1/cost-governance/agent-throttling/{id}` | Delete agent throttling rule |
| | `GET` | `/api/v1/cost-governance/agent-throttling/{id}/status` | Get throttling status for agent |
| **Usage Forecasting** | `POST` | `/api/v1/cost-governance/usage-forecasting` | Generate usage forecast |
| | `GET` | `/api/v1/cost-governance/usage-forecasting` | List usage forecasts |
| | `GET` | `/api/v1/cost-governance/usage-forecasting/{id}` | Get usage forecast details |
| | `POST` | `/api/v1/cost-governance/usage-forecasting/{id}/export` | Export usage forecast |
| **Tenant Billing** | `POST` | `/api/v1/cost-governance/tenant-billing` | Create tenant billing record |
| | `GET` | `/api/v1/cost-governance/tenant-billing` | List tenant billing records |
| | `GET` | `/api/v1/cost-governance/tenant-billing/{id}` | Get tenant billing record |
| | `POST` | `/api/v1/cost-governance/tenant-billing/{id}/invoice` | Generate invoice for billing record |
| | `POST` | `/api/v1/cost-governance/tenant-billing/export` | Export billing records |
| **Dashboard** | `GET` | `/api/v1/cost-governance/cost-dashboard` | Get cost governance dashboard |
| **Health** | `GET` | `/api/v1/cost-governance/health` | Service health check |

See also `../../contracts/v1/openapi-17-cost-governance-engine.yaml` and `../../contracts/v1/schema-17-cost-governance-engine.json` for OpenAPI contract and JSON schemas.

---

## API Schemas

| Category | Schema | Description |
|----------|--------|-------------|
| Token Budgets | CreateTokenBudgetRequest, TokenBudget, UpdateTokenBudgetRequest, TokenBudgetUsage | Budget creation, tracking, and usage monitoring |
| Execution Quotas | CreateExecutionQuotaRequest, ExecutionQuota, UpdateExecutionQuotaRequest, ExecutionCheckRequest, ExecutionCheckResult | Quota management and enforcement |
| Agent Throttling | CreateThrottlingRuleRequest, ThrottlingRule, UpdateThrottlingRuleRequest, ThrottlingStatus | Rate limiting and throttling control |
| Usage Forecasting | GenerateForecastRequest, UsageForecast, ExportForecastRequest | Predictive cost and usage analysis |
| Billing | CreateBillingRecordRequest, BillingRecord, ExportBillingRequest | Billing records, invoicing, and cost tracking |
| Dashboard | CostDashboard, HealthStatus | Cost overview and service health |

---

## Data Flow

```
[Operan Services] ──► [Cost Governance Engine] ──► [Token Tracker]
                                │
                                ▼
                    [Quota & Throttle Engine]
                                │
                    ┌───────────┼───────────┐
                    ▼           ▼           ▼
              [Token Budgets] [Exec Quotas] [Rate Limits]
                    │           │           │
                    ▼           ▼           ▼
              [Alert System] [Queue/Delay] [Backoff]
                                │
                                ▼
                    [Usage Forecaster]
                                │
                                ▼
                    [Cost Dashboard & Billing]
```

---

## Example

```bash
# Create a monthly token budget for an agent
curl -X POST /api/v1/cost-governance/token-budgets \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Agent Monthly Budget",
    "limit_tokens": 10000000,
    "period": "monthly",
    "target_type": "agent",
    "target_id": "agent-123",
    "warning_threshold": 80,
    "alert_actions": ["notify", "throttle"]
  }'

# Check execution quota before running a workflow
curl -X POST /api/v1/cost-governance/execution-quotas/{id}/check \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"target_type": "workflow", "target_id": "wf-456", "requested_executions": 5}'

# Generate a cost forecast
curl -X POST /api/v1/cost-governance/usage-forecasting \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"forecast_type": "cost", "horizon_days": 90, "granularity": "daily"}'
```

---

## Cost Governance Rules

1. Token budgets enforce hard limits; requests exceeding the budget are rejected or throttled.
2. Execution quotas reset automatically based on their configured period (daily/weekly/monthly).
3. Agent throttling uses sliding window rate limiting with configurable backoff strategies.
4. Forecasts are generated using historical usage patterns with configurable prediction horizons.
5. Billing records are immutable once created; corrections require new credit entries.
6. Override roles (e.g., admin) can bypass quotas but still require audit logging.
7. All cost-related actions are logged in the Governance module's audit trail.

---

## Module 17 — Cost Governance Engine (Summary)

- **Category**: Operations & Cost Management
- **Status**: Ready
- **API**: `/api/v1/cost-governance`
- **OpenAPI**: `openapi-17-cost-governance-engine.yaml`
- **Schema**: `schema-17-cost-governance-engine.json`
- **Key Resources**: token-budgets, execution-quotas, agent-throttling, usage-forecasting, tenant-billing
- **Purpose**: Cost control, budget management, and operational spending prevention

---

*Next: Module 18 — Enterprise Connector Fabric*
