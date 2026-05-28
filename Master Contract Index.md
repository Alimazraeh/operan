# Operan: Master Contract Index
Last Updated: 2026-05-27
Owner: You (Human Orchestrator)
Project: Operan — Agentic Department Operating System (ADOS)

## Platform Standards Refactoring (In Progress)
Goal: Refactor all v1 OpenAPI contracts to adhere to strict Operan platform standards:
- Security: `BearerAuth` (JWT) + `X-Tenant-ID` (apiKey header)
- Pagination: `page`, `page_size`, `has_more` (replacing `limit`/`offset`)
- Error handling: Standard `Error` schema (`{ code: int, message: string, request_id: uuid }`) applied to all 4xx/5xx responses
- Schema integrity: `additionalProperties: false` on all request/response schemas
- Tags: Capitalized consistently (e.g., `Vectors`, `Search`, `Agents`)
**Progress:**

| Module | Status | Changes Applied |
|--------|--------|-----------------|
| 05-department-template-engine | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 08-tool-execution | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 11-observability | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 12-model-abstraction | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 13-multi-model-routing-engine | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 14-agent-collaboration | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 15-agent-marketplace | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |
| 16-execution-sandbox | ✅ Done | BearerAuth, X-Tenant-ID, pagination, Error schema, additionalProperties, tags |

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

### Cross-Spec Inconsistency Tracker (Modules 08, 11, 12, 14, 15, 16)

**Audit Date:** 2025-06-10

| Module | Issue | OpenAPI | JSON Schema | Severity |
|--------|-------|---------|-------------|----------|
| **15-agent-marketplace** | Wrong JSON Schema file entirely | Agent marketplace schemas | Notification schemas (`Notification`, `NotificationQueueRequest`, `NotificationRoute`, `BulkNotificationRequest`) | **Critical** |
| **16-execution-sandbox** | Wrong JSON Schema file entirely | Execution sandbox schemas | Cost-governance/billing schemas (`ResourceConsumption`, `Credit`, `SubscriptionPlan`, `QuotaStatus`, `BillingPeriod`) | **Critical** |
| **14-collaboration** | Schema name mismatch | `Message` | `CollaborationMessage` | **High** |
| **14-collaboration** | Schema name mismatch | `Conversation` | `CollaborationConversation` | **High** |
| **14-collaboration** | Schema name mismatch | `Task` | `CollaborationTask` | **High** |
| **14-collaboration** | Schema name mismatch | `ConversationContext` | `CollaborationContext` | **High** |
| **14-collaboration** | Required/nullable mismatch | `Message.conversation_id` required | `Message.conversation_id` nullable | **High** |
| **14-collaboration** | Required/nullable mismatch | `Message.expires_at` required | `Message.expires_at` nullable | **High** |
| **14-collaboration** | Required/nullable mismatch | `Task.assigned_to` required | `Task.assigned_to` nullable | **High** |
| **14-collaboration** | Required/nullable mismatch | `Task.due_at` required | `Task.due_at` nullable | **High** |
| **14-collaboration** | Required/nullable mismatch | `Task.result` required | `Task.result` nullable | **High** |
| **14-collaboration** | Required/nullable mismatch | `Task.conversation_id` required | `Task.conversation_id` nullable | **High** |
| **14-collaboration** | Missing fields | `Conversation.title` present | `Conversation.title` absent | **High** |
| **14-collaboration** | Missing fields | `Conversation.metadata` present | `Conversation.metadata` absent | **High** |
| **14-collaboration** | Required/nullable mismatch | `Conversation.created_at` required | `Conversation.created_at` nullable | **Medium** |
| **14-collaboration** | Required/nullable mismatch | `Conversation.updated_at` required | `Conversation.updated_at` nullable | **Medium** |
| **14-collaboration** | Structural mismatch | `ConversationContext.context_type, data` | `CollaborationContext.shared_context, metadata` | **High** |
| **14-collaboration** | Missing field | `Task.assigner_id` present | `Task.assigner_id` absent | **Medium** |
| **08-tool-execution** | Nullable mismatch | `ToolExecutionRecord.output` non-nullable | `ToolExecutionRecord.output` nullable | **Medium** |
| **08-tool-execution** | Nullable mismatch | `ToolExecutionRecord.execution_time_ms` non-nullable | `ToolExecutionRecord.execution_time_ms` nullable | **Medium** |
| **08-tool-execution** | Nullable mismatch | `ToolExecutionRecord.error_code` non-nullable | `ToolExecutionRecord.error_code` nullable | **Medium** |
| **08-tool-execution** | Nullable mismatch | `ToolExecutionRecord.error_message` non-nullable | `ToolExecutionRecord.error_message` nullable | **Medium** |
| **08-tool-execution** | Missing field | `ToolExecutionRecord` has no `execution_log` | `ToolExecutionRecord.execution_log` (array of strings) | **Medium** |
| **11-observability** | Extra field | `Alert.resolved_by` present | `Alert.resolved_by` absent | **Low** |
| **All 6** | Naming inconsistency | `Error` (code: int, message: string, request_id: uuid) | `ErrorResponse` (code: string, message: string, data: object) | **Medium** |
| **All 6** | Missing schemas | `Error` adds `details: string` in modules 11, 12 | Inconsistent across modules | **Low** |
| **All 6** | Missing `*List` schemas | `ToolList`, `MetricList`, `ModelProviderList`, `SandboxList`, `AgentList`, etc. (~20 total) | Absent | **High** |
| **08-tool-execution** | Missing schemas | `ToolRegisterRequest`, `ToolUpdateRequest`, `CostSummary` | Absent | **High** |
| **12-model-abstraction** | Missing schemas | `CostPerToken`, `CostPerCall`, `RateLimit`, `TokenUsage`, `CapacitySummary` | Absent | **High** |
| **12-model-abstraction** | Missing schemas | `ModelProvisionRequest`, `ModelUpdateRequest`, `RoutingRequest` | Absent | **High** |

### Cross-Spec Inconsistency Tracker (Modules 01–03)

| Module | Issue | OpenAPI | JSON Schema | Severity |
|--------|-------|---------|-------------|----------|
| 01-tenant | `contact_email` vs `admin_email` | `contact_email` | `admin_email` | Low |
| 01-tenant | `custom_policies` missing | Not present | Present | Medium |
| 02-identity | SSOConfig structure | Flat (`issuer_url`, `scopes`) | Nested (`type`, `configuration`) | High |
| 02-identity | SSO provider enum | `azure_ad`, `okta`, `authentik` | `azure_ad`, `google_workspace`, `custom` | Medium |
| 02-identity | AuditEntry vs AuditLog | `actor_type` (enum), `details` | `actor_id`, `metadata` | Medium |
| 02-identity | User.status enum | `active`, `suspended`, `deactivated` | `active`, `inactive`, `suspended`, `pending` | Medium |
| 02-identity | Identity type grouping | Separate endpoints (service/agent) | Unified enum (`service`, `agent`, `system`) | Low |

### Cross-Spec Action Items

| Priority | Action | Affected Modules |
|----------|--------|------------------|
| **Critical** | Replace JSON Schema files for modules 15 and 16 | 15, 16 |
| **High** | Generate missing `*List` schemas across all 6 modules | 08, 11, 12, 14, 15, 16 |
| **High** | Generate missing domain schemas for modules 08 and 12 | 08, 12 |
| **High** | Rename schemas in module 14 JSON Schema to match OpenAPI | 14 |
| **High** | Fix required/nullable mismatches in Message, Task, Conversation | 14 |
| **High** | Add missing fields (Conversation.title/metadata, Task.assigner_id, ConversationContext restructuring) | 14 |
| **Medium** | Align nullable fields in ToolExecutionRecord (module 08) | 08 |
| **Medium** | Resolve `Error` vs `ErrorResponse` naming inconsistency across all modules | All |
| **Medium** | Add `execution_log` to module 08 OpenAPI | 08 |
| **Low** | Add `resolved_by` to module 11 JSON Schema | 11 |
| **Low** | Standardize `Error` schema (remove `details` in modules 11, 12) | 11, 12 |

### AsyncAPI Event Patterns

Use these as reference for AMQP/RabbitMQ event naming (`operan/events/{module}.{event}`):
- **Module 04** (`asyncapi-04-agent-registry.yaml`) — 8 events
- **Module 06** (`asyncapi-06-knowledge-ingestion.yaml`) — 7 events
- **Module 07** (`asyncapi-07-memory-fabric.yaml`) — 5 events
- **Module 08** (`asyncapi-08-tool-execution.yaml`) — 6 events