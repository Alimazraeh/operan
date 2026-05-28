# Module 02 → Module 03 Handover Document

> **From:** Module 02 (Identity & Access Management)  
> **To:** Module 03 (Agent Orchestration Engine)  
> **Date:** 2026-05-28  
> **Status:** Ready for handoff

---

## 1. What Module 02 Provides

Module 02 is the **Identity & Access Management** (IAM) module for the Operan platform. It provides:

- **User, Role, Service Identity, and Agent Identity management** — CRUD via REST API (`/api/v1/iam/*`)
- **Authentik integration** — full REST client wrapping Authentik API v3 (Users, Groups, RBAC, OAuth2, SAML, LDAP, SCIM)
- **JWT validation** — RSA via JWKS with HMAC fallback, issuer validation, claims extraction (`sub`, `user_type`, `tenant_id`, `roles`)
- **Tenant isolation** — `X-Tenant-ID` header injection into context on every request
- **ABAC policies** — time-based, IP/CIDR, ownership, department, and custom rule evaluation
- **Audit trails & session replay** — HTTP request/response capture, RBAC/ABAC evaluation logging
- **MFA enrollment & verification** — via Authentik flow execution API
- **SSO configuration** — OAuth2, SAML providers via Authentik
- **LDAP / Active Directory federation** — configure, test, CRUD directory connectors
- **Delegated admin roles** — maps Operan delegation roles to Authentik groups
- **SCIM 2.0** — automated user provisioning

### Module 02 API Surface (Base: `/api/v1/iam`)

| Group | Endpoints |
|-------|-----------|
| Users | `POST /users`, `GET /users`, `GET /users/{id}`, `PATCH /users/{id}`, `DELETE /users/{id}`, `PUT /users/{id}/roles` |
| Roles | `POST /roles`, `GET /roles`, `GET /roles/{id}`, `DELETE /roles/{id}` |
| Service Identities | `POST /service-identities`, `GET /service-identities`, `GET /service-identities/{id}` |
| **Agent Identities** | `POST /agent-identities`, `GET /agent-identities`, `GET /agent-identities/agent/{agent_id}` |
| RBAC | `POST /rbac/evaluate` — checkPermission |
| ABAC | `POST /abac/evaluate`, `POST /abac/policies`, `GET /abac/policies`, `GET /abac/policies/{id}`, `DELETE /abac/policies/{id}` |
| Audit | `GET /audit/trails`, `GET /audit/session-replay/{session_id}` |
| MFA | `POST /mfa/enroll`, `POST /mfa/verify`, `POST /mfa/disable`, `GET /mfa/enrolled`, `POST /mfa/recovery-codes` |
| SSO | `POST /auth/sso/configure`, `POST /auth/sso/test` |
| LDAP/AD | Configure, test, get, update, delete for both LDAP and AD |
| Delegations | Full CRUD + grant/revoke for delegation roles |
| SCIM | Full SCIM 2.0 provisioning endpoints |

All endpoints require `BearerAuth` (JWT) + `X-Tenant-ID` header.

---

## 2. Module 02 Models Relevant to Module 03

### AgentIdentity

This is the **key model** Module 03 needs to work with:

```go
type AgentIdentity struct {
    ID                string    // internal UUID
    TenantID          string    // tenant isolation
    AgentID           string    // Module 03's agent ID — the join key
    Capabilities      []string  // what the agent can do
    MemoryScope       []string  // which memory areas the agent can access
    AllowedTools      []string  // which external tools the agent may invoke
    EscalationTargets []string  // who to escalate to
    CreatedAt         time.Time
}
```

**How Module 03 uses this:**
- Module 03 agents **must register** with Module 02 via `POST /api/v1/iam/agent-identities` before they can be recognized by the IAM layer.
- Module 03 runtime resolves an agent's registered identity via `GET /api/v1/iam/agent-identities/agent/{agent_id}` to determine its permission scope and tool access.

### JWTToken (from Module 02 middleware)

```go
type JWTToken struct {
    Subject, UserType, TenantID, Email string
    Roles     []string
    Claims    jwt.MapClaims
}
```

The `user_type` claim distinguishes `"user"`, `"service"`, and `"agent"` — Module 03 middleware should follow this pattern.

---

## 3. Integration Points: Module 03 ← Module 02

### Required Integrations

| # | Integration | Module 02 Endpoint | Module 03 Responsibility |
|---|-------------|-------------------|--------------------------|
| 1 | **Agent Registration** | `POST /api/v1/iam/agent-identities` | Module 03 agents call this on startup to register their capabilities, memory scope, allowed tools, escalation targets |
| 2 | **Agent Identity Lookup** | `GET /api/v1/iam/agent-identities/agent/{agent_id}` | Module 03 resolves agent identity to determine its permission scope |
| 3 | **RBAC Evaluation** | `POST /api/v1/iam/rbac/evaluate` | Module 03 should call this before executing any action to verify the caller has permission |
| 4 | **ABAC Policy Evaluation** | `POST /api/v1/iam/abac/evaluate` | Module 03 can use this for additional policy checks (IP, time, ownership, department) |
| 5 | **JWT Validation** | `middleware.go` — `AuthValidator` | Module 03 must adopt this pattern: RSA via JWKS + HMAC fallback, claims extraction |
| 6 | **Tenant Isolation** | `middleware.go` — `TenantInjector` | Every Module 03 endpoint must be tenant-scoped via `X-Tenant-ID`, validated through middleware |
| 7 | **User Type Discrimination** | JWT claim `user_type` | Module 03 agents authenticate via tokens issued with `user_type: "agent"` |
| 8 | **Audit Trail Pattern** | `events/events.go` — Publisher | Module 03 should follow the same event envelope format |

### Module 02 Middleware Pattern (Adopt This in Module 03)

Module 02's middleware chain (from `middleware/middleware.go`):

```go
// Context keys (typed, not string literals)
TenantIDKey contextKey = "tenant_id"
UserIDKey   contextKey = "user_id"
UserTypeKey contextKey = "user_type"
TraceIDKey  contextKey = "trace_id"

// JWT validation — two strategies
// 1. RSA via JWKS (for Authentik-issued tokens)
// 2. HMAC-SHA256 fallback (for internal/admin tokens)

// Tenant injection — extracts X-Tenant-ID from request header
// Returns empty string when missing (handlers decide whether to reject)

// Trace injection — uses existing X-Trace-ID or generates new one
```

**Key takeaway:** Module 02's middleware establishes the platform convention. Module 03's `JWTAuth` exists but is **NOT wired into the middleware chain** (see Section 5).

---

## 4. Event Publishing: Module 02 Pattern

Module 02's `events/events.go` defines the event envelope format Module 03 should follow:

```json
{
  "event_type": "user.created",
  "correlationId": "...",
  "tenantId": "...",
  "timestamp": "2026-...",
  "payload": { ... }
}
```

Module 02 publishes events for:
- `user.created`, `user.updated`, `user.suspended`
- `identity.rotated`
- `permission.granted`, `permission.revoked`
- `session.created`, `session.expired`, `session.ended`, `session.active`
- `session.replay_captured`, `session.replay_retrieved`, `session.replay_deleted`
- `mfa.enrolled`
- `sso.login`

**Current state:** Module 02's event publisher is a **stub** — it logs to stdout but does not publish to AMQP (`// TODO: Implement actual AMQP broker connection`). Module 03 should plan for real Kafka/broker integration from the start.

---

## 5. Critical Gaps in Module 02 (Module 03 Engineer Must Know)

### P0 — Must Fix Before Module 03 Can Reliably Integrate

| # | Issue | Location | Impact on Module 03 |
|---|-------|----------|---------------------|
| 1 | **Event publisher is a stub** — events logged but never published to AMQP | `events/events.go` | Module 03's event integration will also be non-functional if built on this same pattern. Plan for real Kafka/broker from day one. |
| 2 | **`generateSecureToken` / `generateSecurePassword` not cryptographically secure** | `authentik/provisioner.go` | Module 03 agent registration tokens may be predictable. Do not use this for generating agent credentials. |
| 3 | **Query string sanitization defined but not used** in session replay capture | `middleware/session_replay.go` | If Module 03 logs requests with sensitive data (e.g., tool API keys), ensure sanitization is actually wired in. |
| 4 | **No database backend** — all stores are in-memory, data lost on restart | All `store/` files | Module 03 must design for persistent storage from the start (Module 02's in-memory stores are not production-ready). |

### P1 — Known Bugs / Issues

| # | Issue | Location | Workaround |
|---|-------|----------|------------|
| 1 | **Possible compilation errors** — missing accessor methods (`OAuth2API`, `SAMLAPI`, `Groups`, `Users`, `Call`) on Authentik client | Multiple handlers | The Authentik client (`client.go`) exposes sub-APIs as struct fields (e.g., `UsersAPI`, `GroupsAPI`). Use direct field access: `h.Auth.UsersAPI` not `h.Auth.Users()`. |
| 2 | **Two `AuditStore` types** — `store/audit.go` vs `handler/handler_audit_rbac.go` | Naming collision | Module 03: avoid reusing the `AuditStore` name. Use `AuditLogStore` or `AuditTrailStore` if defining your own. |
| 3 | **JWKS cache refresh ignores issuer URL parameter** after construction | `middleware/jwks.go` | If Module 03 uses JWKS validation for a multi-tenant setup with different Authentik instances, this may break. |
| 4 | **DelegationHandler.findUserUUID does full user list** for every lookup | `handler_delegations.go` | Performance concern for large tenants — Module 03 should cache agent identity lookups. |
| 5 | **MFA Disable handler has redundant/erroneous API call** | `handler_mfa.go` | Not directly relevant to Module 03 but signals that handler files in Module 02 have not been thoroughly vetted. |

---

## 6. Module 03 Current State (Summary from Module 02 Perspective)

Based on the audit report (`docs/AUDIT-03-agent-orchestration.md`), Module 03 is in the following state:

| Dimension | Spec | Implemented | Coverage |
|-----------|------|-------------|----------|
| OpenAPI operations | 54 | ~18 | **33%** |
| AsyncAPI channels | 37 | ~8 publishing paths | **22%** |
| AsyncAPI schemas | 39 | ~12 defined types | **31%** |
| Execution stacks | 4 (LangGraph, Temporal, Ray, Celery) | 0 | **0%** |
| Config wired | Yes (8 env vars) | No | **0%** |
| Auth middleware | BearerAuth + X-Tenant-ID | Partial | **JWT defined but NOT wired** |
| Persistent storage | Implicit (multi-tenant, production) | In-memory only | **0%** |
| Unit/integration tests | >= 80% (DoD) | ~36 tests | **Low for 22 Go files** |
| Helm charts | Required (DoD) | Skeleton exists | **~20%** |

### Module 03 Critical Blockers (From Audit)

1. **JWT auth NOT wired** — `main.go` middleware chain has no JWTAuth, any unauthenticated client can call all endpoints
2. **Config never called** — `main.go` hardcodes port 8003 instead of using `config.ParseConfig()`
3. **36 of 54 OpenAPI operations missing** — Escalations, Retries, Nodes/Results, Agent Workers, Stack Health, all multi-stack execution endpoints
4. **Event publishing never triggered** — publisher methods exist but no handler calls them
5. **Event topic naming mismatch** — AsyncAPI contract specifies `operan.orchestration.{stack}.workflow.created` but publisher uses flat format
6. **No DAG execution engine** — Workflows are persisted but never actually executed
7. **No multi-stack execution** — LangGraph, Temporal, Ray, Celery integration is completely absent

### Module 03 Known Bugs

| # | Bug | Location |
|---|-----|----------|
| 1 | `CreateCheckpoint` uses workflow ID as node ID instead of reading `NodeID` from request body | `handler_workflows.go` |
| 2 | `GetWorkflowState` returns all nodes as `pending` regardless of actual state | `handler_workflows.go` |
| 3 | `ReplayWorkflow` only creates a new workflow instance, does not actually replay from checkpoint | `handler_workflows.go` |
| 4 | `ListWorkflows` uses `offset`/`limit` (legacy) instead of platform standard `page`/`page_size` | `handler_workflows.go` |
| 5 | `TenantIDFromContext` returns empty string when header is missing; handlers don't reject | `middleware/middleware.go` |
| 6 | Inconsistent error response format across handlers | Various |

---

## 7. File-by-File Reference for Module 03 Engineer

### Module 02 Files (Source of Truth)

| Path | Purpose | Module 03 Should Reference |
|------|---------|---------------------------|
| `modules/02-identity-access/internal/models/models.go` | AgentIdentity, User, Role, ServiceIdentity models | Define agent identity schema matching Module 02's |
| `modules/02-identity-access/internal/middleware/middleware.go` | Context keys, AuthValidator, TenantInjector | Adopt `TenantIDKey`/`UserIDKey`/`UserTypeKey` pattern |
| `modules/02-identity-access/internal/middleware/auth.go` | JWT token generation/validation (HMAC) | Study for HMAC fallback pattern |
| `modules/02-identity-access/internal/middleware/jwks.go` | JWKS cache with RSA key parsing | Adopt for token validation against Authentik |
| `modules/02-identity-access/internal/handler/handler_identity.go` | Agent identity CRUD (key for Module 03) | **Primary integration point** — agent registration/lookup |
| `modules/02-identity-access/internal/handler/handler_abac.go` | ABAC evaluation | Study for how to handle RBAC→ABAC cascade |
| `modules/02-identity-access/internal/handler/handler_audit_rbac.go` | Audit trails, RBAC evaluation | Study for audit event format |
| `modules/02-identity-access/internal/authentik/client.go` | Full Authentik REST API v3 client | Study for understanding the identity platform |
| `modules/02-identity-access/internal/authentik/tenant_manager.go` | Per-tenant Authentik setup | Study for multi-tenant provisioning |
| `modules/02-identity-access/internal/events/events.go` | Event publisher, envelope format | Adopt event envelope format for Module 03 |
| `modules/02-identity-access/contracts/v1/openapi-02-identity-access.yaml` | OpenAPI contract | Reference for API patterns |
| `modules/02-identity-access/contracts/v1/asyncapi-02-identity-access.yaml` | AsyncAPI events contract | Reference for event topic naming |

### Module 03 Files (Starting Point)

| Path | Purpose |
|------|---------|
| `modules/03-agent-orchestration/internal/handler/handler_workflows.go` | Workflow CRUD (11 ops, all implemented) |
| `modules/03-agent-orchestration/internal/handler/handler_scheduling.go` | Agent assign/availability (2 ops) |
| `modules/03-agent-orchestration/internal/handler/handler_schedules.go` | Schedule CRUD (5 ops) |
| `modules/03-agent-orchestration/internal/handler/handler_pipeline.go` | Pipeline CRUD + analytics (8 ops) |
| `modules/03-agent-orchestration/internal/handler/handler_execution.go` | Execution CRUD (10 ops) |
| `modules/03-agent-orchestration/internal/handler/handler_human_task.go` | Human task CRUD (8 ops) |
| `modules/03-agent-orchestration/internal/handler/handler_missing.go` | Factory functions for missing handlers |
| `modules/03-agent-orchestration/internal/store/workflow.go` | WorkflowStore, ScheduleStore, AgentStore |
| `modules/03-agent-orchestration/internal/store/pipeline_execution.go` | PipelineStore, ExecutionStore, HumanTaskStore |
| `modules/03-agent-orchestration/internal/store/escalation_retry_health.go` | EscalationStore, RetryRecordStore, StackHealthStore |
| `modules/03-agent-orchestration/internal/middleware/middleware.go` | Logger, TenantContext, TraceID, RequestID, JWTAuth (NOT wired) |
| `modules/03-agent-orchestration/internal/events/events.go` | Publisher with 20+ publish methods (LOG-ONLY) |
| `modules/03-agent-orchestration/docs/AUDIT-03-agent-orchestration.md` | Full audit report (872 lines) — **read this first** |

---

## 8. Module 02's Perspective on Module 03's Top Priorities

Based on Module 02's experience building the identity layer, here's what Module 03 should tackle first:

### Phase 1 — Foundation (Blockers)
1. **Wire JWT auth into the middleware chain** — Study Module 02's `AuthValidator` pattern (RSA via JWKS + HMAC fallback)
2. **Wire config package** — Use `config.ParseConfig()` instead of hardcoded port
3. **Design persistent storage strategy** — Module 02's in-memory stores are NOT production-ready; Module 03 needs this from the start
4. **Fix the 6 known bugs** in workflow handlers

### Phase 2 — Completeness (36 missing operations)
5. **Implement missing handlers** — Escalations, Retries, Nodes/Results, Agent Workers, Stack Health, multi-stack execution endpoints
6. **Wire event publishing** — Move from LOG-ONLY to real broker integration
7. **Fix event topic naming** — Align with AsyncAPI contract: `operan.orchestration.{stack}.{entity}.{event}`

### Phase 3 — Execution Engine
8. **Build DAG execution engine** — Wire up LangGraph or Temporal as the runtime
9. **Implement multi-stack execution** — LangGraph, Temporal, Ray, Celery
10. **Implement agent heartbeat** — Replace manual availability with real-time health checks

### Phase 4 — Integration with Module 02
11. **Agent registration flow** — Module 03 agents call `POST /api/v1/iam/agent-identities` on startup
12. **RBAC/ABAC evaluation integration** — Module 03 calls Module 02's evaluate endpoints before executing actions
13. **Audit trail alignment** — Match Module 02's event envelope format for cross-module correlation

---

## 9. Module 02's `agent-identities` Endpoint (Deep Dive for Module 03)

This is the **most critical integration point** for Module 03.

### `POST /api/v1/iam/agent-identities` — Register Agent Identity

**Request body:**
```json
{
  "agent_id": "agent-abc123",           // Module 03's agent ID
  "capabilities": ["nlp", "code-gen"],  // What the agent can do
  "memory_scope": ["knowledge", "memory"], // Memory areas to access
  "allowed_tools": ["google-search", "slack"], // External tools
  "escalation_targets": ["supervisor-1"] // Who to escalate to
}
```

**Response:** `201 Created` — returns the `AgentIdentity` with internal UUID assigned.

### `GET /api/v1/iam/agent-identities/agent/{agent_id}` — Resolve Agent Identity

**Response:** `200 OK` — returns the `AgentIdentity` matching the external `agent_id`.

### Module 03 Integration Pattern

```
Module 03 Agent Startup
        │
        ▼
POST /api/v1/iam/agent-identities  ← Register with Module 02
        │
        ▼
Store returned AgentIdentity.ID locally
        │
        ▼
For each workflow execution:
    GET /api/v1/iam/agent-identities/agent/{agent_id}  ← Resolve identity
        │
        ▼
Check capabilities/memory_scope/allowed_tools
        │
        ▼
Post actions:
    POST /api/v1/iam/rbac/evaluate  ← Check permission
    POST /api/v1/iam/abac/evaluate  ← Check ABAC policies
```

---

## 10. Quick Reference: Module 02 → Module 03 Communication

### Authentication Flow

```
Module 03 receives request with:
  - Header: Authorization: Bearer <JWT>
  - Header: X-Tenant-ID: tenant-xyz

Module 03 middleware should:
  1. Validate JWT (RSA via JWKS from Authentik, HMAC fallback)
  2. Extract claims: sub, user_type, tenant_id, roles
  3. Inject into context: TenantIDKey, UserIDKey, UserTypeKey
  4. Handlers read from context, not headers directly
```

### Event Correlation

```
Module 03 publishes events using Module 02's envelope format:
{
  "event_type": "orchestration.workflow.started",
  "correlationId": "<shared-trace-id>",  // from X-Trace-ID header
  "tenantId": "<from-context>",
  "timestamp": "<RFC3339>",
  "payload": { ... }
}

Module 02 audit trail can then correlate across modules via correlationId.
```

---

## 11. Things Module 02 Got Wrong (So Module 03 Doesn't Repeat Them)

| Mistake | Module 02's Reality | Module 03 Should |
|---------|---------------------|-------------------|
| Stub event publisher | Logged to stdout, no broker | Build real Kafka/broker integration from day one |
| In-memory only stores | Data lost on restart | Design for PostgreSQL/MySQL or similar persistent store |
| No config wiring | Hardcoded port 8003 | Wire `config.ParseConfig()` from the start |
| JWT auth not wired | `JWTAuth` exists but not in chain | Wire `JWTAuth` into middleware chain before any handlers |
| String literal context keys | Used `"tenant_id"` directly | Use typed `contextKey` (Module 02's later pattern) |
| Inconsistent error responses | Some use `middleware.ErrorResponse`, others use inline `writeError` | Define and enforce a single error response format |
| Duplicate type names | Two `AuditStore` types in different packages | Avoid name collisions — use descriptive suffixes |
| Legacy pagination | `offset`/`limit` in some handlers | Use platform standard `page`/`page_size` consistently |
| Query string sanitization defined but unused | Session replay could log secrets | Wire sanitization in the capture path |
| Deterministic "secure" tokens | `generateSecureToken` is predictable | Use `crypto/rand` or `uuid` for all secrets |

---

## 12. Module 02 Module Status Summary

| Component | Status | Notes |
|-----------|--------|-------|
| User CRUD | ✅ Implemented | In-memory store, Authentik sync |
| Role CRUD | ✅ Implemented | In-memory store |
| Service Identity | ✅ Implemented | In-memory store, Authentik sync |
| Agent Identity | ✅ Implemented | In-memory store, key for Module 03 |
| RBAC Evaluate | ✅ Implemented | Calls Authentik API |
| ABAC Evaluate | ✅ Implemented | In-memory policy store |
| ABAC Policies | ✅ Implemented | CRUD + IP/time/ownership/department rules |
| Audit Trails | ✅ Implemented | In-memory store |
| Session Replay | ⚠️ Partially implemented | Captures requests, sanitization not wired |
| MFA | ⚠️ Partially implemented | Has compilation issues (empty struct call) |
| SSO | ⚠️ Partially implemented | Missing accessor methods (`OAuth2API`, `SAMLAPI`) |
| LDAP | ⚠️ Partially implemented | Missing accessor methods (`LDAPSources`) |
| AD | ⚠️ Partially implemented | Missing accessor methods |
| Delegations | ✅ Implemented | Maps to Authentik groups |
| SCIM | ✅ Implemented | In handler_sso.go |
| Event Publisher | ❌ Stub | Logs to stdout, no broker |
| JWKS Cache | ✅ Implemented | Auto-refresh every 55 min |
| Auth Validator | ✅ Implemented | RSA + HMAC fallback |
| Test Coverage | ✅ Good | 100+ tests across handlers |

---

## 13. Contact & Support

Module 02 is available to answer questions about:
- Agent identity registration flow
- JWT validation patterns (RSA/HMAC/JWKS)
- Tenant isolation conventions
- ABAC policy evaluation patterns
- Event envelope format
- Authentik API integration

Module 02 does **not** own or maintain Module 03's code — this handover document is the single source of truth for Module 02 → Module 03 knowledge transfer.

---

*Document end.*
