# ARCH — Module 01: Tenant Control Plane — Production Maturity Review

**Review Date:** 2026-06-18
**Reviewer:** ARCH (Platform Architect)
**Verdict:** **REJECT — Contract drift, missing tenant isolation, and incomplete scope prevent production deployment**

---

## 1. Summary Scorecard

| Category | Score | Notes |
|----------|-------|-------|
| Contract Compliance | ❌ FAIL | 60 operationIds in OpenAPI; 58 routed in `RegisterRoutes`; 2 missing (`rollbackTenantDeployment`, `upgradeSubscription`) |
| Test Coverage | ⚠️ PARTIAL | ~40 handler tests (single file) + 81 store tests (91.1% store coverage). No middleware, config, or event package tests. |
| Integration | ❌ FAIL | No inter-module edges wired; events stubbed (log-only), no Kafka/AMQP broker |
| Infrastructure | ❌ FAIL | No Dockerfile, Helm chart, or README |
| Security | ❌ FAIL | XOR-based secret encryption, no JWT middleware, no tenant-scoped store getters |
| Database | ❌ FAIL | In-memory only — no PostgreSQL/Redis |
| PRD Alignment | ⚠️ PARTIAL | Massive scope expansion (4 PRD endpoints → 60 contract operations) without formal change request |
| **Overall** | **REJECT** | **Not production-ready** |

---

## 2. Contract Drift Analysis

### 2.1 [CRITICAL] `contact_email` vs `admin_email` — JSON Schema ≠ OpenAPI

| Contract | Field Name | Definition |
|----------|-----------|------------|
| **JSON Schema** (`schema-01-*.json`) | `contact_email` | `"type": "string", "format": "email"` |
| **OpenAPI** (`openapi-01-*.yaml`) | `admin_email` | `"type": "string", "format": "email", "description": "Primary admin email for the tenant"` |
| **Handler** (Go DTO) | **BOTH** | `ContactEmail string \`json:"contact_email"\`` AND `AdminEmail string \`json:"admin_email"\`` |

**Impact:** Callers validating against the JSON Schema will send `contact_email`. The handler accepts both fields. The OpenAPI contract only declares `admin_email`. This is a breaking contract mismatch.

**Remediation:** Reconcile to a single field name across all three contract types. Architectural recommendation: use `contact_email` (consistent with PRD terminology). Remove `admin_email` from OpenAPI and handler DTO.

---

### 2.2 [CRITICAL] Plan Enum Drift — AsyncAPI ≠ OpenAPI/JSON Schema

| Contract | Enum Values |
|----------|------------|
| **OpenAPI** (`PlanType` in OpenAPI contract) | `saas`, `enterprise`, `sovereign` |
| **JSON Schema** (corresponding definition) | `saas`, `enterprise`, `sovereign` |
| **AsyncAPI** (`PlanType` at `asyncapi-01-tenant-control-plane.yaml:129`) | `free`, `team`, `enterprise`, `custom` |

**Impact:** AsyncAPI events reference plan values (`free`, `team`, `custom`) that the OpenAPI contract never permits. A consumer validating events against the JSON Schema will reject valid AsyncAPI payloads, and vice versa.

**Remediation:** Align all three documents to a single plan enum. Architectural recommendation: `saas`, `enterprise`, `sovereign` (OpenAPI/JSON Schema set). Remove `free`, `team`, `custom` from AsyncAPI.

---

### 2.3 [HIGH] OpenAPI Defines 60 Operations; Implementation Missing 2

Per `operationId:` grep on the OpenAPI contract — **60 total operationIds**. Per `RegisterRoutes()` in `handler/response_types.go` — **58 routes registered**. Two operations have no implementation:

| Missing Operation | OpenAPI Path (tenant-scoped) |
|-------------------|-----------------------------|
| `rollbackTenantDeployment` | `POST /tenants/{id}/deployments/{deployment_id}/rollback` |
| `upgradeSubscription` | `POST /tenants/{id}/subscriptions/{subscription_id}/upgrade` |

The PROGRESS.md claims "37 endpoints implemented" (stale) and "25/39" (also stale — contract now has 60 operations). The handler layer has ~18 handler functions, but `RegisterRoutes` wires 58 routes via a mix of handler functions and some duplicate registrations.

**Remediation:** Either implement the 2 missing handlers or formally retract them from OpenAPI v1 via change request to ARCH.

---

### 2.4 [HIGH] `updateSubscription` (PATCH /subscriptions/{id}) — Contract Exists, No Handler

The OpenAPI contract defines `updateSubscription` at `PATCH /tenants/{id}/subscriptions/{subscription_id}` (operationId at line 5180). The route registration file defines `UpdateSubscriptionByID` but the actual handler function is not present in the handler source files — it's referenced in `RegisterRoutes` but undefined.

**Remediation:** Implement `UpdateSubscriptionByID` handler, or retract the operation from OpenAPI.

---

### 2.5 [MEDIUM] AsyncAPI Events Not Emitted on Lifecycle Transitions

The AsyncAPI contract defines 4 event channels:
- `operan/events/tenant/provisioned`
- `operan/events/tenant/suspended`
- `operan/events/tenant/deprovisioned`
- `operan/events/tenant/quota_exceeded`

**What the code does:**
- `events/events.go` defines all 4 event types and 4 publish methods — ✅
- `CreateTenant` handler calls `PublishTenantProvisioned` — ✅ (one event)
- **No handler calls** `PublishTenantSuspended` — ❌
- **No handler calls** `PublishTenantQuotaExceeded` — ❌
- `DeleteTenant` handler calls `PublishTenantDeprovisioned` — ✅
- The `publish()` method is a **log-only stub** — no Kafka/AMQP integration — ❌

**Remediation:** Wire all lifecycle events to their corresponding transitions. Replace log-only `publish()` with either Kafka (per AsyncAPI contract) or a documented stub limitation in the README.

---

## 3. Security Assessment

### 3.1 [CRITICAL] XOR "Encryption" for Secrets

`store/secret.go` implements:

```go
func encryptValue(value string) string {
    // XOR-based placeholder — NOT production-grade
}
```

**Impact:** Any attacker with read access to the in-memory store can trivially decrypt stored secrets. This directly violates the PRD requirement for "per-tenant encryption key generation" and the AsyncAPI contract's `encryption_key_management` field.

**Remediation:** Replace with AES-256-GCM or integrate with a KMS (AWS KMS, HashiCorp Vault, or the platform's sovereign deployment fabric).

---

### 3.2 [CRITICAL] No Tenant Isolation on Store or Handler Layer

**Middleware layer:** The `TenantContext` middleware extracts `X-Tenant-ID` from the request header and stores it in context.

**Handler layer:** Handlers call `store.GetByID(id)` which takes only an `id` parameter. **None of the 58 routes read tenantID from context before calling a store method.** There is no `GetByIDAndTenant(id, tenantID)` variant on any store.

**Impact:** Cross-tenant data leakage is trivial. An attacker who knows a valid tenant UUID can access any other tenant's data by enumeration. The `X-Tenant-ID` header is captured but never enforced.

**Remediation:**
1. Add `GetByTenantID` / `GetByIDAndTenant` variants to all 12 store types.
2. Require handlers to extract `tenantID` from context (via `middleware.GetTenantID(ctx)`) and pass it to store methods.
3. Write tenant-isolation tests that verify a request from "tenant A" cannot access "tenant B" data.

---

### 3.3 [HIGH] No JWT Validation Middleware

The OpenAPI contract declares `BearerAuth` (JWT Bearer token) as the global security scheme. The config struct has a `JWTSecret` field, but **no JWT validation middleware exists anywhere in the middleware chain**. The middleware chain is: `RequestID → TraceID → TenantContext → [Business Handler]`.

**Impact:** The API has zero authentication. Any client can call any endpoint (assuming they provide a `X-Tenant-ID` header, which is also not enforced on the route level).

**Remediation:** Implement `JWTValidator` middleware using `golang-jwt/jwt/v5`. Validate issuer, expiration, and tenant-scoped claims. Insert into middleware chain before `TenantContext`.

---

### 3.4 [MEDIUM] No Plan Upgrade Downgrade Guard

The `UpgradePlan` handler exists but performs no tier-order validation. There is no enforcement that upgrades go `saas → enterprise → sovereign` and not in reverse.

**Remediation:** Define a plan tier ordering (`saas < enterprise < sovereign`) and validate that `req.TargetPlan > currentPlan`.

---

## 4. Test Coverage Analysis

### 4.1 Coverage Summary

| Package | Coverage | Target | Status |
|---------|----------|--------|--------|
| `cmd/tenant-control-plane` | 0.0% | ≥80% | ❌ FAIL |
| `internal/config` | 0.0% | ≥80% | ❌ FAIL |
| `internal/events` | 0.0% | ≥80% | ❌ FAIL |
| `internal/handler` | ~25% | ≥80% | ❌ FAIL |
| `internal/middleware` | 0.0% | ≥80% | ❌ FAIL |
| `internal/store` | 91.1% | ≥80% | ✅ PASS |

### 4.2 Handler Test Distribution

The handler test file `handler/handler_test.go` contains **~40 tests** in a single file, covering:
- Tenant CRUD + status transitions (22 tests)
- Agent CRUD (7 tests)
- Resource CRUD (6 tests)
- Billing: invoices + subscriptions (11 tests)

**Untested handler files** (~1,800 lines total):
| Handler File | Lines | Tests | Status |
|--------------|-------|-------|--------|
| `handler_billing.go` | ~316 | ❌ None | ❌ FAIL |
| `handler_deployments.go` | ~200 | ❌ None | ❌ FAIL |
| `handler_environments.go` | ~150 | ❌ None | ❌ FAIL |
| `handler_namespaces.go` | ~150 | ❌ None | ❌ FAIL |
| `handler_policies.go` | ~200 | ❌ None | ❌ FAIL |
| `handler_secrets_status.go` | ~470 | ❌ None | ❌ FAIL |
| `handler_resources_billing.go` | ~525 | (partial) | ⚠️ PARTIAL |
| `response_types.go` | ~290 | ❌ None | N/A (DTOs only) |

**Missing test categories:**
- No middleware tests (`RequestID`, `TraceID`, `TenantContext`)
- No config package tests (`ParseConfig` with env vars)
- No event package tests (event marshalling, publish logic)
- No integration tests (no Dockerfile or testcontainers)
- No tenant-isolation tests (critical gap)

---

## 5. Infrastructure & Deployment

### 5.1 [CRITICAL] No Dockerfile, No Helm Chart, No README

The module directory contains source code, `go.mod`, and `go.sum` — but:

| Artifact | Status |
|----------|--------|
| `Dockerfile` | ❌ MISSING |
| `helm/Chart.yaml` + `helm/templates/` + `helm/values.yaml` | ❌ MISSING |
| `README.md` | ❌ MISSING |
| `manifest.json` | ❌ MISSING |

**Remediation:** Implement multi-stage Dockerfile (Alpine builder → scratch runtime, non-root user), Helm chart with Deployment + Service + Ingress templates, and a README with setup, config reference, and deployment instructions.

---

### 5.2 [HIGH] No Database — PostgreSQL + Redis Missing

The PRD specifies PostgreSQL + Redis from Day 1. The implementation uses **entirely in-memory stores** (`store.Store` types with `sync.Map` and `map` backends). There are zero references to:
- `database/sql` or `pgx`
- Redis client initialization
- SQL migration files
- Connection configuration for database

**Remediation:** Define PostgreSQL schema, migration files, and a `pgx`-based store adapter. Implement Redis for quota tracking / rate limiting.

---

### 5.3 [MEDIUM] Module Status Endpoint Not Fully Specified

The OpenAPI contract defines `GET /v1/status` with a `ModuleStatusResponse` schema containing `uptime` and `health` fields. The handler `GetModuleStatus` exists but:
- No health check against database or Redis connectivity
- `uptime` field requires tracking start time in handler struct (not yet implemented)

---

## 6. PRD Alignment

### 6.1 Scope Expansion Without Architectural Approval

The PRD (section 12) specifies **4 endpoints**:
```
POST   /tenants
GET    /tenants/{id}
PATCH  /tenants/{id}
DELETE /tenants/{id}
```

The current implementation provides **60 operations** spanning:
- Tenants (5 CRUD + quota + status = 7)
- Agents (5 CRUD)
- Resources (5 CRUD)
- Namespaces (5 CRUD + quota check = 7)
- Deployments (5 CRUD + 4 lifecycle = 9)
- Policies (5 CRUD + 3 utility = 8)
- Environments (5 CRUD + 3 lifecycle = 8)
- Billing/Invoices (4)
- Payment Methods (4)
- Subscriptions (6)
- Secrets (6 CRUD/rotate)
- Health/status (1)
- **Total: ~68 route registrations in RegisterRoutes**

This is a **15–17x scope expansion** (4 PRD endpoints → 60+ contract operations) that introduces 11+ new domain models, 47+ schemas, and 6 distinct integration patterns.

**Architectural directive:** All scope expansions must be submitted as change requests to ARCH for approval against the integration graph and cross-module contracts before implementation.

**Decision:** The implementation scope will be retained IF: all 60 contract operations are implemented, all contract drifts are resolved, security issues are remediated, and test coverage reaches ≥80%. Otherwise, the module must be rolled back to the PRD-specified scope.

---

## 7. Remediation Plan

### Phase 1: Critical Blockers (Must Fix Before Re-Review)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 1 | Replace XOR encryption with AES-256-GCM or KMS integration | CODER | P0 |
| 2 | Implement JWT validation middleware | CODER | P0 |
| 3 | Add tenant-scoped store methods (`GetByIDAndTenant`, `ListByTenantID`) to all 12 stores; refactor all handlers to use them | CODER | P0 |
| 4 | Reconcile `contact_email` vs `admin_email` across OpenAPI, JSON Schema, and handler types | ARCH → CODER | P0 |
| 5 | Fix plan enum drift — align AsyncAPI to `saas | enterprise | sovereign` | ARCH → CODER | P0 |
| 6 | Implement `rollbackTenantDeployment` and `upgradeSubscription`, OR retract from OpenAPI | CODER/ARCH | P0 |
| 7 | Create Dockerfile; verify `docker build` succeeds | CODER | P0 |
| 8 | Implement `updateSubscription` handler OR retract from OpenAPI | CODER/ARCH | P0 |
| 9 | Add tenant-isolation integration tests (cross-tenant data leakage prevention) | CODER | P0 |

### Phase 2: High-Priority Gaps (Fix Before Production Sign-Off)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 10 | Achieve ≥80% handler test coverage (split tests into per-handler files) | CODER | P1 |
| 11 | Add PostgreSQL schema + migration files | CODER | P1 |
| 12 | Wire event publishing to Kafka/AMQP or document stub limitation in README | CODER | P1 |
| 13 | Implement Redis client for quota/rate-limit tracking | CODER | P1 |
| 14 | Implement `/health` endpoint with DB + Redis connectivity checks | CODER | P1 |
| 15 | Create Helm chart and `values.yaml` | CODER | P1 |
| 16 | Write README (setup, config reference, deployment) | CODER | P1 |
| 17 | Add integration tests with testcontainers | CODER | P1 |

### Phase 3: Medium-Priority Enhancements

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 18 | Test config package and event package (reach 80% coverage) | CODER | P2 |
| 19 | Add plan upgrade tier validation | CODER | P2 |
| 20 | Wire `PublishTenantSuspended` and `PublishTenantQuotaExceeded` to handler transitions | CODER | P2 |
| 21 | Add OpenTelemetry instrumentation to all handlers | CODER | P2 |

---

## 8. Developer Sign-Off Checklist

Before resubmitting Module 01 for re-review, the CODER team must complete AND verify each item below:

- [ ] **P0-1:** XOR encryption replaced with production-grade encryption (AES-256-GCM or KMS)
- [ ] **P0-2:** JWT validation middleware implemented and tested
- [ ] **P0-3:** All store `GetByID` methods replaced with tenant-scoped variants (`GetByIDAndTenant`, `ListByTenantID`) — zero calls bypass tenant isolation
- [ ] **P0-4:** `contact_email` / `admin_email` reconciled across OpenAPI, JSON Schema, and handler DTOs
- [ ] **P0-5:** Plan enum unified to `saas | enterprise | sovereign` in all 3 contract files (OpenAPI, JSON Schema, AsyncAPI)
- [ ] **P0-6:** All 60 OpenAPI operations implemented OR formally retracted from OpenAPI v1 (with ARCH approval)
- [ ] **P0-7:** Dockerfile created; `docker build` succeeds; `docker run` starts HTTP server on configured port
- [ ] **P0-8:** Tenant-isolation integration tests pass (cross-tenant access denied)
- [ ] **P1-9:** Handler test coverage ≥ 80% (verified by `go test -cover`)
- [ ] **P1-10:** PostgreSQL migration files exist and are applied on startup
- [ ] **P1-11:** Event publishing wired to Kafka/AMQP (or stub with documented limitation in README)
- [ ] **P1-12:** Redis client initialized for quota/rate-limit tracking
- [ ] **P1-13:** Integration tests using testcontainers pass
- [ ] **P1-14:** `GET /health` returns 200 with `{ status: "ok" }` including DB + Redis health checks
- [ ] **P1-15:** Helm chart and values.yaml created
- [ ] **P1-16:** README written (setup, config, deployment)
- [ ] **P2-17 through P2-21:** Phase 3 items completed (for production readiness, not blocking re-review)

---

## 9. Architect's Note

Module 01 is the **foundation** of the entire Operan platform. Every other module depends on tenant context (`X-Tenant-ID`), tenant validation, and the tenant lifecycle. The current implementation demonstrates strong engineering instincts — the store layer is well-structured, the middleware chain is clean, the directory layout is sensible, and the `RegisterRoutes` function is comprehensive.

However, three fundamental issues prevent production readiness:

1. **Security:** XOR encryption, missing JWT, and zero tenant isolation on the store layer represent unacceptable risk. A single ID enumeration attack would expose all tenant data.

2. **Contract Drift:** The JSON Schema, OpenAPI, and AsyncAPI contracts disagree on field names (`contact_email` vs `admin_email`) and plan enum values (`free/team/custom` vs `saas/enterprise/sovereign`). These are not edge cases — they are core domain concepts that all consumers will rely on.

3. **Incomplete Scope:** The module was scoped to 4 endpoints in the PRD but expanded to 60 operations without formal architectural review. Two operations remain unimplemented in the handler layer.

The remediation effort is estimated at **3–5 developer days for P0 items** and **5–7 days for P1 items**.

**Next review date:** After all P0 items are checked off and verified by `go test ./...` and `docker build`.

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18
**CODER Acknowledgment:** _________________    **Date:** _________________
