# Operan: Master Contract Index

## Last Updated: 2025-06-10

## Owner: You (Human Orchestrator)

### Module Contract Status

| Module | OpenAPI | Schema | AsyncAPI | Edge | Status | Notes |
|--------|---------|--------|----------|------|--------|-------|
| 01-tenant-control-plane | ✅ | ✅ | ✅ | ✅ | RECONCILED | Full spec; style reference |
| 02-identity-access | ✅ | ✅ | ✅ | ✅ | RECONCILED | IAM patterns; style reference |
| 03-agent-orchestration | ✅ | ✅ | ✅ | ✅ | RECONCILED | |
| 04-agent-registry | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI just created; had AsyncAPI + orphan schema |
| 05-department-template-engine | ✅ | ✅ | ✅ | ✅ | RECONCILED | Has both -engine and bare AsyncAPI/schema |
| 06-knowledge-ingestion | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI now created; 10 endpoints |
| 07-memory-fabric | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI now created; 12 endpoints |
| 08-tool-execution | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI now created; 10 endpoints |
| 09-human-supervision | ✅ | ✅ | ✅ | ✅ | RECONCILED | |
| 10-policy-governance | ✅ | ✅ | ✅ | ✅ | RECONCILED | Full spec; style reference |
| 11-observability | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI now created; 8 endpoints |
| 12-model-abstraction | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI now created; 11 endpoints |
| 13-workflow-orchestration | ✅ | ✅ | ✅ | ✅ | RECONCILED | Added openapi-13-workflow-orchestration.yaml from orphan |
| 13-multi-model-routing-engine | ✅ | ✅ | ✅ | ✅ | RECONCILED | Both 13 contracts coexist; routing + workflow |
| 14-agent-collaboration | ✅ | ✅ | ✅ | ✅ | RECONCILED | OpenAPI now created; 10 endpoints |
| 15-agent-marketplace | ✅ | ✅ | ✅ | ✅ | RECONCILED | Renamed from notification-routing; now has OpenAPI, AsyncAPI, schema |
| 16-execution-sandbox | ✅ | ✅ | ✅ | ✅ | RECONCILED | Renamed from billing-metering; now has OpenAPI, AsyncAPI, schema |
| 17-cost-governance-engine | ✅ | ✅ | ✅ | ✅ | RECONCILED | |
| 18-enterprise-connector-fabric | ✅ | ✅ | ✅ | ✅ | RECONCILED | |
| 19-arabic-language-core | ✅ | ✅ | ✅ | ✅ | RECONCILED | Confirmed as correct module 19 |
| 20-sovereign-deployment-fabric | ✅ | ✅ | ✅ | ✅ | RECONCILED | |

### PRD Compliance Audit (Modules 01–03)

| Module | Endpoints | Schemas | Compliance | Key Gaps | Cross-Spec Inconsistencies |
|--------|-----------|---------|------------|----------|---------------------------|
| 01-tenant-control-plane | 25 | 40 | **62.5%** | Deployment manager (0 endpoints), billing read-only, `GET /tenants` missing, no policy engine | 2 (contact_email↔admin_email, custom_policies in JSON only) |
| 02-identity-access | 15 | 9 | **~60%** | LDAP, AD, authn flow (login/token/MFA), SCIM write-only, session replay no response schema | 5 (SSOConfig structure, provider enums, AuditEntry↔AuditLog, User.status, Identity type) |
| 03-agent-orchestration | 18 | 18 | **~70%** | Delegation API, distributed execution (workers/shards/state-sync), auto-routing, escalation chains | 0 |

#### Module 01 — Tenant Control Plane: 62.5%

| PRD Requirement | Status | Notes |
|-----------------|--------|-------|
| Tenant onboarding | ✅ | `POST /tenants` fully implemented |
| Namespace creation | ⚠️ Partial | Baked into provisioning; no standalone endpoint |
| Quota allocation | ✅ | GET/PATCH `/tenants/{id}/quota` with all 5 quota fields |
| Billing integration | ⚠️ Partial | 5 endpoints but **all GET** — no invoice creation, payment processing, billing cycle |
| Deployment lifecycle | ⚠️ Partial | Status transitions on tenant object; no `/deployments` resource |
| Environment isolation | ✅ | Full `IsolationConfig` (namespace, encryption algo+rotation, network policy) |
| Tenant registry | ⚠️ Partial | `TenantListResponse` schema exists but `GET /tenants` endpoint missing |
| Tenant policy engine | ⚠️ Partial | Freeform `custom_policies` metadata only; no CRUD or evaluation API |
| Deployment manager | ❌ | Zero deployment endpoints |
| Subscription manager | ⚠️ Partial | Missing `POST /subscriptions`; downgrade not supported |
| Tenant secrets manager | ✅ | Full CRUD + rotation with versioning |

AsyncAPI events: 4/9 covered (provisioned, suspended, deprovisioned, quota_exceeded). Missing: created, status_changed, subscription_changed, secret_rotated, quota_updated.

#### Module 02 — Identity & Access: ~60%

| PRD Requirement | Status | Notes |
|-----------------|--------|-------|
| SSO | ⚠️ Partial | Config/test endpoints; no authn flow (login/callback/logout/SLO) |
| LDAP | ❌ | No LDAP endpoints or config |
| Active Directory | ❌ | No AD sync or directory endpoints |
| SCIM | ⚠️ Partial | Read-only `GET /scim/users`; missing create/update/delete |
| RBAC | ✅ | Core evaluate + role CRUD; missing list/read for roles |
| ABAC | ⚠️ Partial | Evaluation exists; no policy CRUD or attribute store |
| Service identities | ⚠️ Partial | Create only; no read/update/delete/rotate |
| Agent identities | ⚠️ Partial | Register only; no read/update/delete/deregister |
| MFA | ⚠️ Partial | Flag + event exist; no enroll/verify/login endpoints |
| Audit trails | ✅ | Query endpoint + async event sources solid |
| Session replay | ⚠️ Partial | Endpoint exists but no response schema |
| Delegated admin roles | ⚠️ Partial | Role system exists; no delegation hierarchy |

#### Module 03 — Agent Orchestration: ~70%

| PRD Requirement | Status | Notes |
|-----------------|--------|-------|
| Workflow execution | ✅ | Full DAG CRUD, lifecycle, state management |
| Agent scheduling | ⚠️ Partial | Manual assign only; no auto-routing, pools, scaling |
| State management | ✅ | `WorkflowState` + variables comprehensive |
| Task routing | ⚠️ Partial | Conditional edges only; no routing engine |
| Delegation | ❌ | No delegation endpoints, schemas, or events |
| Retry logic | ✅ | `RetryPolicy` with constant/linear/exponential backoff |
| Timeout handling | ⚠️ Partial | Per-node only; no global timeout, override, or warnings |
| Escalation logic | ⚠️ Partial | Single human gate; no chains, policies, or severity |
| DAG execution | ✅ | Nodes, edges, conditions, error strategies |
| Async execution | ✅ | 201/202 async + polling + events |
| Resumable workflows | ✅ | Pause/resume + checkpoint persistence |
| Distributed execution | ❌ | No worker discovery, shards, state sync, or coordination |
| State checkpointing | ✅ | Manual checkpoint + integrity checksums |
| Workflow replay | ✅ | Replay from checkpoint with variable override |

### Orphan Files (Drafts — unnumbered, pending cleanup)

**OpenAPI (14 files):**
`openapi-arabic-language.yaml`, `openapi-enterprise-connector.yaml`, `openapi-cost-governance.yaml`, `openapi-knowledge.yaml`, `openapi-messaging.yaml`, `openapi-ml.yaml`, `openapi-observability.yaml`, `openapi-governance.yaml`, `openapi-supervision.yaml`, `openapi-tools.yaml`, `openapi-memory.yaml`, `openapi-ingestion.yaml`, `openapi-departments.yaml`, `openapi-registry.yaml`

**Schema (11 files):**
`schema-arabic-language.json`, `schema-enterprise-connector.json`, `schema-cost-governance.json`, `schema-knowledge.json`, `schema-workflows.json`, `schema-messaging.json`, `schema-ml.json`, `schema-governance.json`, `schema-supervision.json`, `schema-tools.json`, `schema-memory.json`

**Renamed/removed files:**
- `openapi-19-knowledge-marketplace.yaml.bak`, `asyncapi-19-knowledge-marketplace.yaml.bak`, `schema-19-knowledge-marketplace.json.bak` — misassigned to module 19; marketplace belongs to module 15
- `openapi-marketplace.yaml` — renamed to `openapi-15-agent-marketplace.yaml`

### Global Constants (Copy-Paste These Everywhere)

- Tenant ID format: `uuid-v4`
- Auth header: `Authorization: Bearer {jwt}`
- Error format: RFC 7807 Problem Details
- Trace ID header: `X-Trace-Id: {uuid}`
- Timestamp format: ISO 8601 UTC

### Dependency Graph (Simplified)

Source: `contracts/v1/integration-graph.yaml`

```
01-tenant → [02, 03, 07, 09, 10, 11, 17, 20]
02-identity → [01, 03, 04, 05, 09, 10, 15, 16, 19, 20]
03-orchestration → [01, 04, 07, 09, 11, 14, 16]
04-registry → [01, 03, 07, 09, 10, 14]
05-dept-template → [01, 02, 03, 10, 13]
06-knowledge-ingest → [01, 07, 08, 10, 11]
07-memory → [01, 03, 04, 06, 10, 11]
08-tool-exec → [01, 04, 07, 10, 11, 17]
09-human-supervision → [01, 02, 03, 04, 05, 10, 15]
10-policy → [01, 02, 03, 04, 05, 06, 07, 08, 09, 11, 13, 17, 19, 20]
11-observability → [01, 02, 03, 04, 05, 06, 07, 08, 09, 10, 13, 14, 15, 16, 17, 19, 20]
12-model-abstraction → [01, 08, 10, 11, 13, 17, 19]
13-workflow → [01, 03, 05, 10, 11, 14, 17]
14-collaboration → [01, 03, 04, 11, 13, 15, 20]
15-agent-marketplace → [01, 02, 09, 11, 14, 16, 19, 20]
16-execution-sandbox → [01, 03, 08, 10, 11, 17, 20]
17-cost-governance → [01, 03, 08, 10, 11, 12, 13, 19, 20]
18-enterprise-connector → [01, 02, 05, 09, 10, 19, 20]
19-arabic-language → [01, 02, 10, 12, 13, 15, 18, 19, 20]
20-sovereign → [01, 02, 03, 09, 10, 14, 15, 18, 19]
```

### OpenAPI Style References

Use these as the gold standard for OpenAPI contract structure:
- **Module 01** (`openapi-01-tenant-control-plane.yaml`) — Most complete; 25 endpoints; 2385 lines
- **Module 02** (`openapi-02-identity-access.yaml`) — IAM patterns; 15 endpoints; 627 lines
- **Module 10** (`openapi-10-policy-governance.yaml`) — Policy patterns; 931 lines

### Cross-Spec Inconsistency Tracker

| Module | Issue | OpenAPI | JSON Schema | Severity |
|--------|-------|---------|-------------|----------|
| 01-tenant | `contact_email` vs `admin_email` | `contact_email` | `admin_email` | Low |
| 01-tenant | `custom_policies` missing | Not present | Present | Medium |
| 02-identity | SSOConfig structure | Flat (`issuer_url`, `scopes`) | Nested (`type`, `configuration`) | High |
| 02-identity | SSO provider enum | `azure_ad`, `okta`, `authentik` | `azure_ad`, `google_workspace`, `custom` | Medium |
| 02-identity | AuditEntry vs AuditLog | `actor_type` (enum), `details` | `actor_id`, `metadata` | Medium |
| 02-identity | User.status enum | `active`, `suspended`, `deactivated` | `active`, `inactive`, `suspended`, `pending` | Medium |
| 02-identity | Identity type grouping | Separate endpoints (service/agent) | Unified enum (`service`, `agent`, `system`) | Low |

### AsyncAPI Event Patterns

Use these as reference for AMQP/RabbitMQ event naming (`operan/events/{module}.{event}`):
- **Module 04** (`asyncapi-04-agent-registry.yaml`) — 8 events
- **Module 06** (`asyncapi-06-knowledge-ingestion.yaml`) — 7 events
- **Module 07** (`asyncapi-07-memory-fabric.yaml`) — 5 events
- **Module 08** (`asyncapi-08-tool-execution.yaml`) — 6 events