# ARCH — Module 02: Identity & Access Management — Production Maturity Review

**Review Date:** 2026-06-18
**Reviewer:** ARCH (Platform Architect)
**Verdict:** **REJECT — Missing tenant isolation on critical stores, contract drift, and incomplete scope prevent production deployment**

---

## 1. Summary Scorecard

| Category | Score | Notes |
|----------|-------|-------|
| Contract Compliance | ❌ FAIL | 33 OpenAPI operations; 9 AsyncAPI channels; 9 JSON Schema defs — multiple cross-spec mismatches, missing role CRUD |
| Test Coverage | ⚠️ PARTIAL | 139 tests across 7 test files (~16% handler coverage). 9 of 11 handlers have zero tests |
| Integration | ❌ FAIL | Events are log-only stubs; no inter-module edges wired; Authentik is external dependency (not present in CI) |
| Infrastructure | ❌ FAIL | No Dockerfile, Helm chart, README; compiled binary at module root |
| Security | ❌ FAIL | Hardcoded JWT secret, HMAC fallback, no tenant isolation on ABAC/AuditStore, static ID generator |
| Database | ❌ FAIL | In-memory only — no PostgreSQL/Redis |
| PRD Alignment | ⚠️ PARTIAL | Scope expanded beyond 4-base endpoints to include ABAC, SCIM, MFA, LDAP, AD, delegations |
| **Overall** | **REJECT** | **Not production-ready** |

---

## 2. Contract Drift Analysis

### 2.1 [CRITICAL] Role Representation — OpenAPI vs JSON Schema

| Contract | Field | Type | Description |
|----------|-------|------|-------------|
| **OpenAPI** `User.roles` | array of **role name strings** | `["department_admin"]` | Human-readable role names |
| **JSON Schema** `User.role_ids` | array of **UUID strings** | `["550e8400-..."]` | Role UUID references |

**Impact:** Consumers validating against the JSON Schema will send role UUIDs. The handler expects role name strings. These types are incompatible — this is a breaking API contract mismatch.

**Remediation:** Align to a single representation. Architectural recommendation: `role_ids` (UUIDs) — more explicit, supports role renaming without breaking references.

---

### 2.2 [CRITICAL] SSOConfig Provider Enum Mismatch

| Contract | Provider Enum Values |
|----------|---------------------|
| **OpenAPI** | `azure_ad`, `okta`, `authentik`, `google_workspace` |
| **JSON Schema** | `azure_ad`, `google_workspace`, `okta`, `keycloak`, `custom` |

**Discrepancy:** OpenAPI includes `authentik` (the platform's own IdP); JSON Schema includes `keycloak` and `custom` but omits `authentik`. A consumer sending `keycloak` will fail OpenAPI validation.

**Remediation:** Reconcile to a single superset or agree on which providers are in-scope for Wave 1.

---

### 2.3 [CRITICAL] AuditEvent vs AuditLog — Structural Differences

| Field | OpenAPI `AuditEvent` | JSON Schema `AuditLog` |
|-------|----------------------|------------------------|
| `tenant_id` | **missing** | **present (required)** |
| `actor_type` | `enum: [user, service_identity, agent_identity]` | **missing** |
| `details` | `type: object` | **does not exist** |
| `metadata` | **does not exist** | `type: object` |
| `severity` | **missing** | `enum: [low, medium, high, critical]` |
| `result` enum | `success / failure / denied` | `success / failure` (no `denied`) |

**Impact:** These are fundamentally different event models. The OpenAPI `AuditEvent` is a flat log entry with actor type. The JSON Schema `AuditLog` includes tenant_id, severity, and metadata. An audit query returning JSON Schema payloads will fail OpenAPI validation.

**Remediation:** Reconcile to a single model. Architectural recommendation: use the JSON Schema's richer model (`tenant_id`, `severity`, `metadata`, `details`) and update OpenAPI to match.

---

### 2.4 [HIGH] PrincipalType Naming Drift — OpenAPI vs AsyncAPI

| Contract | Enum |
|----------|------|
| **OpenAPI** `AuditEvent.actor_type` | `user`, `service_identity`, `agent_identity` |
| **AsyncAPI** `PrincipalType` | `user`, `service`, `agent` |

**Impact:** The `service_identity`/`service` and `agent_identity`/`agent` naming drift means that code processing AsyncAPI events will send `service` as the principal type, which will fail OpenAPI validation against `AuditEvent.actor_type`.

**Remediation:** Align to a single naming convention. Architectural recommendation: `service` / `agent` (shorter, consistent with AsyncAPI).

---

### 2.5 [HIGH] Missing Role Endpoints in OpenAPI

The OpenAPI contract defines `createRole`, `deleteRole`, `listRoles`, and `getRole` endpoints — but:
- `listRoles` (GET `/roles`) and `getRole` (GET `/roles/{id}`) are listed in the HANDOFF as registered ✅, but they are **absent** from the OpenAPI contract's endpoint list (33 operations counted, neither present).
- The HANDOFF says "GET /roles/{id}" is implemented, but the OpenAPI contract does not define it.

**Remediation:** Either add `listRoles` and `getRole` to the OpenAPI contract, or remove them from the implementation.

---

### 2.6 [HIGH] SSOConfig Requires Client-Provided ID

The OpenAPI contract defines `configureSSO` (POST `/auth/sso/configure`) which takes an `SSOConfig` object as the request body. The `SSOConfig` schema has `id` as a **required** field. Normally, IDs are server-assigned. The endpoint should use a dedicated `CreateSSOConfigRequest` schema without the `id` field.

**Remediation:** Introduce a `CreateSSOConfigRequest` schema (subset of `SSOConfig` without `id`) for the POST body.

---

### 2.7 [MEDIUM] Missing AsyncAPI Events

The OpenAPI contract defines 33 operations, but the AsyncAPI contract defines only 9 event channels. The following significant operations have **no corresponding AsyncAPI event**:

| OpenAPI Operation | Missing Event Channel |
|-------------------|----------------------|
| `deactivateUser` (DELETE /users/{id}) | No `user/deactivated` event |
| `createRole` | No `role/created` event |
| `deleteRole` | No `role/deleted` event |
| `updateUser` | No `user/updated` event |
| `setUserRoles` (PUT /users/{id}/roles) | No `user/roles/set` event |
| `createServiceIdentity` | No `service-identity/created` event |
| `registerAgentIdentity` | No `agent-identity/registered` event |
| `createDelegationRole` / `updateDelegationRole` / `deleteDelegationRole` | No delegation lifecycle events |
| `grantDelegation` / `revokeDelegation` | No delegation grant/revoke events |
| `configureLDAP` / `updateLDAPConfig` / `deleteLDAPConfig` | No LDAP lifecycle events |
| `configureAD` / `updateADConfig` / `deleteADConfig` | No AD lifecycle events |

**Remediation:** Add missing event channels to the AsyncAPI contract OR document these as out-of-scope for Wave 1.

---

### 2.8 [MEDIUM] JSON Schema Schemas with No OpenAPI Equivalent

| JSON Schema Schema | OpenAPI Equivalent |
|-------------------|-------------------|
| `Identity` | ❌ None (covers service/agent/system identities) |
| `Permission` | ❌ None (full CRUD model with conditions) |
| `Session` | ❌ None (full session object) |
| `Role` | ❌ None (with `permission_ids`, `assignable_to`) |
| `SCIMUser` | ❌ None (SCIM-specific format) |
| `ErrorResponse` | ❌ None (OpenAPI uses inline error responses) |

**Remediation:** Either add these schemas to OpenAPI or remove them from JSON Schema.

---

## 3. Security Assessment

### 3.1 [CRITICAL] Hardcoded Default JWT Secret

`internal/config/config.go` defaults `IAM_TOKEN_SECRET` to `"change-me-in-production"`.

**Impact:** If this environment variable is not set in production, all JWT tokens are signed with a well-known default secret. Any attacker can forge valid tokens for any user, service, or agent.

**Remediation:** Fail startup if `IAM_TOKEN_SECRET` is not set, and generate a random default at build time (not runtime). Document the requirement in the README.

---

### 3.2 [CRITICAL] ABAC Policies — No Tenant Isolation

`handler_abac.go` stores ABAC policies in a global `map[string]ABACPolicy` with `sync.RWMutex`. The handler does **not** filter or scope policies by tenant.

**Impact:** Any tenant can read, modify, or delete another tenant's ABAC policies. This is a full security bypass for the attribute-based authorization layer.

**Remediation:** Scope the ABAC store to `tenantID`. Store policies as `abacPolicies[tenantID][key]` or use a tenant-aware store adapter.

---

### 3.3 [CRITICAL] AuditStore `Create()` Does Not Enforce Tenant

`internal/store/audit.go`'s `Create()` method accepts audit events without verifying tenant from context. The HANDOFF explicitly calls this out as BLOCKER-4: "Prevent cross-tenant audit event injection."

**Impact:** An attacker who can inject audit events into one tenant's namespace can pollute or delete audit trails for another tenant.

**Remediation:** Enforce tenant isolation in `AuditStore.Create()` by reading `tenantID` from context (via middleware).

---

### 3.4 [HIGH] JWT Validation Fallback to HMAC

`middleware/auth.go` (the AuthValidator) validates JWTs via JWKS (RS256), but when that fails, it **silently falls back to HMAC validation** using the same `tokenSecret`.

**Impact:** A token signed with a different HMAC secret (known to any attacker) will pass validation. This creates a backdoor that completely undermines RS256/JWKS-based authentication.

**Remediation:** Remove the HMAC fallback entirely. If internal service tokens use HMAC, sign them with a different secret and validate using a separate code path — do not use the same secret for both algorithms.

---

### 3.5 [HIGH] `generateID()` Returns Static ID

`middleware.go`'s `generateID()` always returns `"00000000-0000-0000-0000-000000000001"`.

**Impact:** Every request gets the same trace ID, breaking distributed tracing, log correlation, and debugging. A single ID collision across all requests in all tenants.

**Remediation:** Use `crypto/rand` to generate a proper UUID v4. See Module 01's `generateID()` as a reference.

---

### 3.6 [MEDIUM] No Rate Limiting or Throttling

No middleware exists for rate limiting. The module is directly exposed to all API consumers (including SCIM connectors, SSO clients, LDAP directories) with no throttle.

**Remediation:** Implement a rate-limiting middleware (sliding window or token bucket) with configurable per-tenant limits.

---

### 3.7 [MEDIUM] Session Replay Store — No Size Limits

`middleware/session_replay.go`'s `SessionReplayCapture` grows without bounds — no max session count, no per-session request limit, no TTL-based eviction.

**Remediation:** Implement a bounded LRU cache with TTL eviction (e.g., max 1000 sessions, 24h TTL).

---

### 3.8 [MEDIUM] TenantManager Cache — No Eviction

`internal/authentik/tenant_manager.go` caches tenant state indefinitely. No TTL, no max size, no eviction policy.

**Remediation:** Implement TTL-based eviction or a bounded LRU cache.

---

## 4. Test Coverage Analysis

### 4.1 Coverage Summary

| Package | Coverage | Target | Status |
|---------|----------|--------|--------|
| `cmd/identity-access` | 0.0% | ≥80% | ❌ FAIL |
| `internal/config` | 0.0% | ≥80% | ❌ FAIL |
| `internal/authentik` | 0.0% | ≥80% | ❌ FAIL |
| `internal/events` | 0.0% | ≥80% | ❌ FAIL |
| `internal/handler` | ~16% | ≥80% | ❌ FAIL |
| `internal/middleware` | ~25% | ≥80% | ❌ FAIL |
| `internal/store` | ~45% | ≥80% | ❌ FAIL |

### 4.2 Untested Handlers (9 of 11)

| Handler File | Tests | Coverage |
|--------------|-------|----------|
| `handler_users.go` | ✅ 22 tests | ~60% |
| `handler_roles.go` | ✅ 23 tests | ~55% |
| `handler_abac.go` | ✅ 18 tests | ~40% |
| `handler_mfa.go` | ❌ 0 tests | 0% |
| `handler_sso.go` (SSO + SCIM) | ❌ 0 tests | 0% |
| `handler_ldap.go` | ❌ 0 tests | 0% |
| `handler_ad.go` | ❌ 0 tests | 0% |
| `handler_delegations.go` | ❌ 0 tests | 0% |
| `handler_audit_rbac.go` | ❌ 0 tests | 0% |
| `handler_identity.go` | ❌ 0 tests | 0% |
| `handler_jwt.go` (in middleware) | — | N/A |

### 4.3 Untested Stores (6 of ~12)

| Store | Tests | Coverage |
|-------|-------|----------|
| `store/user.go` | ✅ 20 tests | ~50% |
| `store/role.go` | ✅ 13 tests | ~40% |
| `store/audit.go` | ❌ 0 tests | 0% |
| `store/ldap_config.go` | ❌ 0 tests | 0% |
| `store/ad_config.go` | ❌ 0 tests | 0% |
| `store/service_identity.go` | ✅ 13 tests | ~45% |
| `store/agent_identity.go` | ✅ 10 tests | ~35% |
| `store/delegation_role.go` | ❌ 0 tests | 0% |
| `store/sso_config.go` | ❌ 0 tests | 0% |

### 4.4 Missing Test Categories

- No integration tests (no Dockerfile, no testcontainers)
- No tenant-isolation tests (critical: ABAC policies are global, not tenant-scoped)
- No auth-fallback tests (JWT → HMAC fallback path)
- No rate-limiting tests (no rate limiter exists)
- No middleware chain tests (Recover → Logging → Auth → RBAC → Tenant)

---

## 5. Infrastructure & Deployment

### 5.1 [CRITICAL] No Dockerfile, No Helm Chart, No README

| Artifact | Status |
|----------|--------|
| `Dockerfile` | ❌ MISSING |
| `helm/Chart.yaml` + templates + values.yaml | ❌ MISSING |
| `README.md` | ❌ MISSING |
| Compiled binary `identity-access` | ⚠️ Present at module root (should not be) |

The compiled binary at the module root suggests manual `go build` usage, not a proper CI/CD pipeline. This binary should be in `.gitignore`.

### 5.2 [HIGH] Event Broker is a Stub

`internal/events/events.go`'s `Publish` method only logs. There is no Kafka/AMQP/RabbitMQ connection. All 12 event types (user created, suspended, identity rotated, permission granted/revoked, session created/expired, MFA enrolled, SSO login) are published to a log-only stub.

### 5.3 [HIGH] No Database — PostgreSQL + Redis Missing

All stores are in-memory. No PostgreSQL or Redis is configured. LDAP/AD configs, SSO configs, audit events, session data, ABAC policies, delegation roles, service identities, and agent identities are all lost on restart.

### 5.4 [MEDIUM] Provisioner Hardcodes External Commands

`internal/authentik/provisioner.go` uses `exec.CommandContext` to call `helm`, `docker compose`, and `curl`. These commands require specific tools to be installed on the host. In a containerized or minimal CI environment, these will fail.

---

## 6. PRD Alignment

### 6.1 Scope Expansion

The PRD specifies **4 base endpoints** for Module 02. The implementation provides **40+ route registrations** across:
- Users (6: CRUD + roles)
- Roles (4: create, list, get, delete)
- Service Identities (2: create, get)
- Agent Identities (2: register, get)
- SSO (3: configure, test, get config)
- SCIM (4: list, provision, update, delete)
- Audit (3: trails, trail-by-id, session-replay)
- RBAC (1: evaluate)
- LDAP (5: configure, get, update, delete, test)
- AD (5: configure, get, update, delete, test)
- Delegations (7: CRUD + grant + revoke + list)
- ABAC (5: policies CRUD + evaluate)
- MFA (5: enroll, verify, disable, list, recovery-codes)
- Session Replay (3: record, get, list)
- Health (1)
- **Total: ~51 route registrations**

**Architectural directive:** All scope expansions must be submitted as change requests to ARCH for approval against the integration graph and cross-module contracts before implementation.

**Decision:** The expanded scope is architecturally sound (ABAC + SCIM + MFA + LDAP + AD are all legitimate IAM capabilities). However, the following handlers exist but are **not wired** into `main.go`'s route table:
- ABAC handler (5 routes: policies CRUD + evaluate)
- MFA handler (5 routes: enroll, verify, disable, list, recovery-codes)
- Session Replay handler (3 routes: record, get, list)
- Service Identity list (1 route)
- SCIM PATCH/DELETE (2 routes — partially fixed per HANDOFF)

**Action:** Wire all missing routes, then re-review for contract compliance.

---

## 7. Remediation Plan

### Phase 1: Critical Blockers (Must Fix Before Re-Review)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 1 | Remove JWT HMAC fallback — use separate secret for service tokens | CODER | P0 |
| 2 | Fail startup if `IAM_TOKEN_SECRET` is the default value | CODER | P0 |
| 3 | Scope ABAC policies to `tenantID` — no global map | CODER | P0 |
| 4 | Enforce tenant on `AuditStore.Create()` — reject cross-tenant writes | CODER | P0 |
| 5 | Fix `generateID()` to use `crypto/rand` — no static IDs | CODER | P0 |
| 6 | Wire all missing routes (ABAC 5, MFA 5, Session Replay 3, SCIM PATCH/DELETE) | CODER | P0 |
| 7 | Reconcile `contact_email`/`admin_email` and plan enum drifts across OpenAPI/JSON Schema/AsyncAPI | ARCH → CODER | P0 |
| 8 | Reconcile AuditEvent/AuditLog structure across OpenAPI and JSON Schema | ARCH → CODER | P0 |
| 9 | Reconcile SSOConfig provider enum across OpenAPI and JSON Schema | ARCH → CODER | P0 |
| 10 | Create Dockerfile; verify `docker build` succeeds | CODER | P0 |

### Phase 2: High-Priority Gaps (Fix Before Production Sign-Off)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 11 | Achieve ≥80% handler test coverage (write tests for 9 untested handlers) | CODER | P1 |
| 12 | Write tenant-isolation tests for ABAC and AuditStore | CODER | P1 |
| 13 | Add rate-limiting middleware | CODER | P1 |
| 14 | Implement bounded LRU cache for SessionReplay and TenantManager | CODER | P1 |
| 15 | Wire event publishing to Kafka/AMQP or document stub limitation | CODER | P1 |
| 16 | Add PostgreSQL schema + migration files | CODER | P1 |
| 17 | Add graceful shutdown (SIGTERM/SIGINT) | CODER | P1 |
| 18 | Add `/ready` endpoint (health + dependency checks) | CODER | P1 |
| 19 | Create Helm chart and `values.yaml` | CODER | P1 |
| 20 | Write README (setup, config, deployment, Authentik integration) | CODER | P1 |
| 21 | Add integration tests with testcontainers | CODER | P1 |

### Phase 3: Medium-Priority Enhancements

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 22 | Test config package and event package (reach 80% coverage) | CODER | P2 |
| 23 | Implement `matchesScimFilter` fully (filter patterns) | CODER | P2 |
| 24 | Implement `evaluateIPPolicy` with CIDR parsing | CODER | P2 |
| 25 | Implement `evaluateCustomPolicy` with rule engine | CODER | P2 |
| 26 | Add OpenTelemetry instrumentation | CODER | P2 |
| 27 | Add `/status` module health endpoint | CODER | P2 |

---

## 8. Developer Sign-Off Checklist

Before resubmitting Module 02 for re-review, the CODER team must complete AND verify each item below:

- [ ] **P0-1:** JWT HMAC fallback removed; service tokens use separate secret
- [ ] **P0-2:** Startup fails if `IAM_TOKEN_SECRET` equals default value
- [ ] **P0-3:** ABAC policies scoped to `tenantID` — zero global map access
- [ ] **P0-4:** `AuditStore.Create()` enforces tenant from context — cross-tenant writes rejected
- [ ] **P0-5:** `generateID()` returns unique IDs (crypto/rand-based)
- [ ] **P0-6:** All 15 missing routes registered in `main.go` (ABAC 5, MFA 5, Session Replay 3, SCIM 2)
- [ ] **P0-7:** Cross-spec inconsistencies resolved (plan enum, AuditEvent structure, SSOConfig providers, PrincipalType naming)
- [ ] **P0-8:** Dockerfile created; `docker build` succeeds
- [ ] **P1-9:** Handler test coverage ≥ 80%
- [ ] **P1-10:** Tenant-isolation tests pass (ABAC, AuditStore, Delegations)
- [ ] **P1-11:** Rate-limiting middleware implemented
- [ ] **P1-12:** Session replay and TenantManager caches bounded with eviction
- [ ] **P1-13:** Event publishing wired to Kafka/AMQP (or stub with documented limitation)
- [ ] **P1-14:** PostgreSQL migration files exist
- [ ] **P1-15:** Graceful shutdown implemented
- [ ] **P1-16:** `/ready` endpoint with dependency checks
- [ ] **P1-17:** Helm chart created
- [ ] **P1-18:** README written
- [ ] **P1-19:** Integration tests pass
- [ ] **P2-20 through P2-27:** Phase 3 items completed (for production readiness)

---

## 9. Architect's Note

Module 02 is the **security layer** of the Operan platform. Every other module depends on tenant context, user identity, role-based permissions, and attribute-based policies. The current implementation demonstrates strong domain expertise — the Authentik client is comprehensive, the handler layer is rich with features (ABAC, SCIM 2.0, MFA, LDAP/AD federation, delegated admin), and the middleware chain is well-structured.

However, three critical security issues prevent production deployment:

1. **HMAC JWT fallback:** A token signed with any known HMAC secret bypasses JWKS validation. This is a complete authentication bypass.
2. **Global ABAC store:** Any tenant can read/modify/delete another tenant's authorization policies. This is a full security bypass for the attribute-based layer.
3. **Unenforced audit tenant:** Cross-tenant audit event injection is possible — an attacker can poison or erase another tenant's audit trail.

The static `generateID()` and hardcoded default secret compound these issues with basic operational failures.

The remediation effort is estimated at **3–5 developer days for P0 items** and **5–7 days for P1 items**.

**Next review date:** After all P0 items are checked off and verified by `go test ./...` and `docker build`.

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18
**CODER Acknowledgment:** _________________    **Date:** _________________
