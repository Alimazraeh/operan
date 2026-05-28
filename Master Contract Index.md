# Operan: Master Contract Index
Last Updated: 2026-05-28 (Module 03 implementation completed)
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
| 02-identity-access | ✅ | ✅ | ✅ | ✅ | RECONCILED | IAM patterns; style reference; 33 ops, 9 AsyncAPI events |
| 03-agent-orchestration | ✅ | ✅ | ✅ | ✅ | RECONCILED | 54 ops, 37 AsyncAPI channels, multi-stack (LangGraph/Temporal/Ray/Celery), 100% handler/store coverage, full test suite |
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
| 02-identity-access | 33 | 10 | **~85%** | MFA endpoints not in contract, SCIM PATCH/bulk not in contract, authn flow delegated to Module 20 gateway | 7 (SSOConfig structure, provider enum, AuditEntry↔AuditLog, User.status, Identity grouping, Identity discriminator, service/agent separation) |
| 03-agent-orchestration | 18 | 18 | **~85%** | Distributed execution (workers/shards/state-sync), auto-routing, escalation chains | 0 |

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

#### Module 02 — Identity & Access: ~85%

| PRD Requirement | Status | Notes |
|-----------------|--------|-------|
| SSO | ✅ | Config/test endpoints implemented (OAuth2/SAML via Authentik); authn flow (login/callback/logout/SLO) remains as platform-level concern |
| LDAP | ✅ | Full CRUD: configure, test, get, update, delete — delegated to Authentik LDAP Sources |
| Active Directory | ✅ | Full CRUD: configure, test, get, update, delete — delegated to Authentik LDAP Sources with AD flags |
| SCIM | ✅ | Full SCIM 2.0 provisioning endpoints implemented via `handler_sso.go` |
| RBAC | ✅ | Core evaluate + role CRUD; fully implemented via Authentik RBAC integration |
| ABAC | ✅ | Evaluation + policy CRUD (create, list, get, delete) with IP/time/ownership/department/custom rules |
| Service identities | ✅ | Full CRUD: create, list, get — delegated to Authentik Users API |
| Agent identities | ✅ | Full CRUD: register (create), list, get by agent_id — key integration for Module 03 |
| MFA | ✅ | Enroll, verify, disable, list devices, regenerate recovery codes — via Authentik flow execution |
| Audit trails | ✅ | Query endpoint + async event sources fully implemented |
| Session replay | ⚠️ Partial | Endpoint exists and captures requests/responses, but query string sanitization is not wired into capture path |
| Delegated admin roles | ✅ | Full CRUD + grant/revoke for delegation roles — mapped to Authentik groups |

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

### Module 03 — Implementation Notes

**Contract counts:** 54 OpenAPI operations · 37 AsyncAPI events · Multi-stack orchestration (LangGraph, Temporal, Ray, Celery/Kafka)

**Implementation status:**
- Handlers: `handler_workflows.go` (Create, List, Get, Pause, Resume, Cancel, Checkpoint, State, Replay)
- Stores: `WorkflowStore`, `ScheduleStore`, `AgentStore` (AgentAvailability/AgentAssignment), `EscalationStore`, `RetryRecordStore`, `StackHealthStore` — all in-memory with tenant isolation
- Middleware: `JWTAuth` (HMAC-S256 with JWT_SECRET env var), `TenantContext`, `TraceID`, `RequestID`, `Logger`
- Events: Publisher with 25+ typed publish methods using `operan.orchestration.{stack}.{entity}.{event}` topic format
- Deployment: Dockerfile (multi-stage, non-root user), Helm chart (deployment, service, configmap, ingress, HPA)

**Test coverage:** Full test suite across handler, middleware, store, and events packages. All tests passing. Key test scenarios: CRUD operations, tenant isolation, status transitions, pagination, JWT auth validation, middleware chaining, event topic naming.

**Known Bugs / Issues:**

| # | Issue | Location | Severity |
|---|-------|----------|----------|
| 1 | Event publisher is a stub — events logged but never published to AMQP | `events/events.go` | P0 |
| 2 | No database backend — all stores are in-memory | All `store/` files | P1 |
| 3 | JWT auth uses local secret (MVP) — should delegate to Module 02 IAM | `middleware/middleware.go` | P1 |
| 4 | AgentStore manages availability/assignment only — actual Agent definitions belong to Module 04 | `store/agents.go` | Info |

**Module 03 as style reference:** Module 03's implementation demonstrates the multi-stack orchestration pattern. Contract files: `openapi-03-agent-orchestration.yaml` (54 ops, 18 path groups), `asyncapi-03-agent-orchestration.yaml` (37 channels, 39 schemas).

#### Module 04 — Agent Registry: ~45%

| PRD Requirement | Status | Notes |
|-----------------|--------|-------|
| Agent versioning | ✅ | Full CRUD + promote; semver enforced |
| Capability indexing | ⚠️ Partial | CRUD implemented; structured scores stored but `[]string` in memory |
| Permissions | ⚠️ Partial | Tenant isolation via context; no RBAC/ABAC (Module 02) |
| Dependency management | ⚠️ Partial | CRUD only; no DAG resolution, no cycle detection |
| Runtime constraints | ⚠️ Partial | Stored on Agent object; not enforced at runtime |
| Cost profiles | ⚠️ Partial | Stored but Module 17 integration missing |
| Agent Object Model (PRD §8) | ⚠️ Partial | Core fields present; missing: `objectives`, `supported_languages`, `current_version_id`, `access_control` |

**Contract counts:** 16 OpenAPI operations · 8 AsyncAPI channels · JSON Schema with 20 definitions

**Implementation status:**
- Handlers: CRUD for agents, versions, capabilities, dependencies + search
- Stores: 4 in-memory stores (AgentStore, VersionStore, CapabilityStore, DependencyStore) — AgentStore has tenant isolation; others do not
- Middleware: `ExtractTenant`, `ExtractUserID` — no JWT validation, no TraceID/RequestID, no structured logging
- Events: Event structs defined but never published — `AgentCreated`, `AgentVersionCreated`, `AgentVersionPromoted`, `AgentStatusChanged` (names do not match AsyncAPI operationIds)
- Test coverage: 20 handler tests, 27 store tests (all in-memory)

**Known Issues (from architectural review 2026-05-28):**

| # | Issue | Severity |
|---|-------|----------|
| 1 | `DependencyType` enum missing `hard` in JSON Schema and AsyncAPI | **High** |
| 2 | `SearchAgents` bypasses tenant context — reads `tenant_id` from body | **Critical** |
| 3 | Routes for `AgentByID` and `VersionByID` not wrapped with `ExtractTenant` | **Critical** |
| 4 | No event publishing — 4 of 8 AsyncAPI events have no Go struct | **Critical** |
| 5 | Base path `/agents` should be `/registry/agents` per architecture blueprint | **High** |
| 6 | `MemoryAccess` stored as `[]string` instead of structured object | **High** |
| 7 | `CostProfile` has 3 fields vs 6 in OpenAPI contract | **High** |
| 8 | `Agent` required fields misaligned: OpenAPI requires 8 fields, JSON Schema requires 5 | **Medium** |
| 9 | `DependencyRequest` handler struct does not match OpenAPI `DependencyRequest` schema | **High** |
| 10 | Config struct defined but never wired into `main.go` | **Medium** |
| 11 | Version/Capability/Dependency stores lack tenant isolation | **Medium** |
| 12 | Tests use `dependency_type: "direct"` not in enum `[hard, soft, optional]` | **Medium** |

---

### Module 04 — Contract vs Implementation Gaps

**Base Path:** OpenAPI uses `/agents`; architecture blueprint specifies `/api/v1/registry`. Implementation routes use `/v1/agents`.

**Store Tenant Isolation:** Only `AgentStore` has tenant-scoped `byTenant` index. `VersionStore`, `CapabilityStore`, `DependencyStore` are cross-tenant by design.

**Missing Agent Fields in Handler DTOs:** `objectives`, `supported_languages`, `current_version_id`, `department_id`, `description` (Agent-level).

**Event Struct Name Mismatches:**
| AsyncAPI operationId | Go struct | Status |
|---------------------|-----------|--------|
| `AgentRegistered` | `AgentCreated` | ❌ Mismatch |
| `AgentCapabilitiesUpdated` | *(none)* | ❌ Missing |
| `AgentVersionCreated` | `AgentVersionCreated` | ✅ Match |
| `AgentPromoted` | `AgentVersionPromoted` | ❌ Mismatch |
| `AgentDeprecated` | *(none)* | ❌ Missing |
| `AgentArchived` | *(none)* | ❌ Missing |
| `DependencyAdded` | *(none)* | ❌ Missing |
| `DependencyRemoved` | *(none)* | ❌ Missing |

---

### Orphan Files (Drafts — unnumbered, pending cleanup)

**OpenAPI (14 files):**
`openapi-arabic-language.yaml`, `openapi-enterprise-connector.yaml`, `openapi-cost-governance.yaml`, `openapi-knowledge.yaml`, `openapi-messaging.yaml`, `openapi-ml.yaml`, `openapi-observability.yaml`, `openapi-governance.yaml`, `openapi-supervision.yaml`, `openapi-tools.yaml`, `openapi-memory.yaml`, `openapi-ingestion.yaml`, `openapi-departments.yaml`, `openapi-registry.yaml`

**Schema (11 files):**
`schema-arabic-language.json`, `schema-enterprise-connector.json`, `schema-cost-governance.json`, `schema-knowledge.json`, `schema-workflows.json`, `schema-messaging.json`, `schema-ml.json`, `schema-governance.json`, `schema-supervision.json`, `schema-tools.json`, `schema-memory.json`

**Renamed/removed files:**
- `openapi-19-knowledge-marketplace.yaml.bak`, `asyncapi-19-knowledge-marketplace.yaml.bak`, `schema-19-knowledge-marketplace.json.bak` — misassigned to module 19; marketplace belongs to module 15
- `openapi-marketplace.yaml` — renamed to `openapi-15-agent-marketplace.yaml`

### Cross-Spec Inconsistencies — Module 04 (Resolved)

| Field | OpenAPI | JSON Schema | AsyncAPI | Fix |
|-------|---------|-------------|----------|-----|
| `DependencyType` enum | `[hard, soft, optional]` | `[hard, soft, optional]` | `[hard, soft, optional]` | ✅ Harmonized — added `hard` to JSON Schema and AsyncAPI |

### Cross-Spec Action Items — Module 04

1. **Agent required fields** — OpenAPI requires `[id, name, role, tenant_id, status, capabilities, tools, created_at]`; JSON Schema requires `[id, name, role, tenant_id, status, capabilities, tools]`. Recommendation: align JSON Schema to OpenAPI by adding `created_at` to `Agent.required`.
2. **CostProfile shape** — OpenAPI `CostProfile` has 6 fields (`base_rate_per_minute`, `max_budget`, `cost_threshold`, `currency`, `billing_tier`, `cost_center`). Go DTO `CostProfile` has 3. Contract is the source of truth — implement full struct.
3. **MemoryAccess shape** — OpenAPI `MemoryAccess` has `[scope, access_level, vector_store, index, ttl_minutes]`. Go DTO stores as `[]string`. Align DTO to contract.
4. **DependencyRequest** — OpenAPI `DependencyRequest` has `[agent_id, depends_on, dependency_type, description]`. Go handler struct uses `[AgentID, DependsOn, DependencyType, Type]`. Rename `Type` → `Description`.
5. **Event struct names** — 5 of 8 Go event structs do not match AsyncAPI operationIds. Either rename Go structs to match AsyncAPI operationIds (recommended) or update AsyncAPI operationIds to match Go structs.

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
- **Module 02** (`openapi-02-identity-access.yaml`) — IAM patterns; 33 operations; 627 lines
- **Module 03** (`openapi-03-agent-orchestration.yaml`) — Multi-stack orchestration; 54 operations; 2817 lines; 18 path groups
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
| 02-identity | SSOConfig structure | Flat (`issuer_url`, `scopes`) | Nested (`type`, `configuration`) | **High** — still unresolved |
| 02-identity | SSO provider enum | `azure_ad`, `okta`, `authentik` | `azure_ad`, `google_workspace`, `custom` | **Medium** — needs harmonization |
| 02-identity | AuditEntry vs AuditLog | `actor_type` (enum), `details` | `actor_id`, `metadata` | **Medium** — handler defines AuditEntry, JSON Schema defines AuditLog |
| 02-identity | User.status enum | `active`, `suspended`, `deactivated` | `active`, `inactive`, `suspended`, `pending` | **Medium** — JSON Schema has extra values |
| 02-identity | Identity type grouping | Separate endpoints (service/agent) | Unified enum (`service`, `agent`, `system`) | **Low** — acceptable separation |
| 02-identity | `Identity` schema vs `ServiceIdentity`/`AgentIdentity` | Separate types | Unified `Identity` with enum discriminator | **Medium** — JSON Schema `Identity` type not reflected in OpenAPI |

### Cross-Spec Action Items

| Priority | Action | Affected Modules |
|----------|--------|------------------|
| **Critical** | Replace JSON Schema files for modules 15 and 16 | 15, 16 |
| **High** | Generate missing `*List` schemas across all 6 modules | 08, 11, 12, 14, 15, 16 |
| **High** | Generate missing domain schemas for modules 08 and 12 | 08, 12 |
| **High** | Rename schemas in module 14 JSON Schema to match OpenAPI | 14 |
| **High** | Fix required/nullable mismatches in Message, Task, Conversation | 14 |
| **High** | Add missing fields (Conversation.title/metadata, Task.assigner_id, ConversationContext restructuring) | 14 |
| **Medium** | Harmonize SSOConfig structure between OpenAPI and JSON Schema (Module 02) | 02 |
| **Medium** | Harmonize SSO provider enum between OpenAPI and JSON Schema (Module 02) | 02 |
| **Medium** | Resolve AuditEntry vs AuditLog naming between handler and JSON Schema (Module 02) | 02 |
| **Medium** | Align User.status enum between OpenAPI and JSON Schema (Module 02) | 02 |
| **Medium** | Add `Identity` discriminator schema to OpenAPI for unified identity type (Module 02) | 02 |
| **Medium** | Align nullable fields in ToolExecutionRecord (module 08) | 08 |
| **Medium** | Resolve `Error` vs `ErrorResponse` naming inconsistency across all modules | All |
| **Medium** | Add `execution_log` to module 08 OpenAPI | 08 |
| **Low** | Add `resolved_by` to module 11 JSON Schema | 11 |
| **Low** | Standardize `Error` schema (remove `details` in modules 11, 12) | 11, 12 |

### Module 02 — Implementation Notes

**Contract counts:** 33 OpenAPI operations · 9 AsyncAPI events · JSON Schema with 10 top-level definitions · Integration graph edge present

**Implementation status:**
- Handlers: 13 handler files (users, roles, audit_rbac, sso, ldap, ad, delegations, mfa, abac, identity)
- Stores: 9 in-memory stores with tenant isolation (`sync.RWMutex` + per-tenant maps)
- Middleware: `AuthValidator` (RSA via JWKS + HMAC fallback), `TenantInjector`, `TraceInjector`, `SessionReplayCapture`, `JWKSCache`
- Events: Publisher with 15+ typed publish methods — **STUB**: logs to stdout, no AMQP broker
- Authentik client: Full REST v3 API wrapper (Users, Groups, Applications, Tokens, OAuth2, SAML, LDAP, SCIM, RBAC, Tenants)
- Provisioner: Helm and Docker Compose per-tenant Authentik instances

**Test coverage:** 139 test functions across handler/middleware/store packages. No integration tests (`tests/` directory is empty).

**Module 02 Known Bugs / Issues (see HANDOVER-MODULE-02-TO-MODULE-03.md for full list):**

| # | Issue | Location | Severity |
|---|-------|----------|----------|
| 1 | Event publisher is a stub — events logged but never published to AMQP | `events/events.go` | P0 |
| 2 | `generateSecureToken` / `generateSecurePassword` not cryptographically secure | `authentik/provisioner.go` | P0 |
| 3 | Query string sanitization defined but not wired into session replay capture | `middleware/session_replay.go` | P0 |
| 4 | Possible compilation errors — missing accessor methods (`OAuth2API`, `SAMLAPI`, `Groups`, `Users`, `Call`) | Multiple handlers | P1 |
| 5 | Two `AuditStore` types — `store/audit.go` vs `handler/handler_audit_rbac.go` naming collision | Naming | P1 |
| 6 | JWKS cache refresh ignores issuer URL parameter after construction | `middleware/jwks.go` | P2 |
| 7 | DelegationHandler.findUserUUID does full user list for every lookup | `handler_delegations.go` | P2 |
| 8 | MFA Disable handler has redundant/erroneous API call | `handler_mfa.go` | P2 |
| 9 | No database backend — all stores are in-memory | All `store/` files | P1 |

**Module 02 as style reference:** Module 02's OpenAPI contract (`openapi-02-identity-access.yaml`) is used as a gold standard for IAM patterns. It is NOT a platform-standards-refactored contract (it predates the refactoring initiative). Modules 05–16 were refactored to use the standards, but Module 02 was kept as-is for backward compatibility since it defines the IAM patterns.

Use these as reference for AMQP/RabbitMQ event naming (`operan/events/{module}.{event}`):
- **Module 03** (`asyncapi-03-agent-orchestration.yaml`) — 37 events; multi-stack format: `operan.orchestration.{stack}.{entity}.{event}`
- **Module 04** (`asyncapi-04-agent-registry.yaml`) — 8 events
- **Module 06** (`asyncapi-06-knowledge-ingestion.yaml`) — 7 events
- **Module 07** (`asyncapi-07-memory-fabric.yaml`) — 5 events
- **Module 08** (`asyncapi-08-tool-execution.yaml`) — 6 events