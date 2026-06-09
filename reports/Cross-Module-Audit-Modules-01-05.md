# Cross-Module Audit: Modules 01-05 (Tenant Control Plane → Department Template Engine)

> **Date:** 2026-05-31
> **Scope:** All code, contracts, middleware, stores, and events across Modules 01-05
> **Modules 06-20:** SCAFFOLD ONLY (README.md files, no implementation code)

---

## Build & Test Status

| Module | Build | Tests | Coverage |
|--------|-------|-------|----------|
| **01 — Tenant Control Plane** | ✅ | ✅ Pass | 30.5% ❌ |
| **02 — Identity & Access** | ✅ | ✅ Pass | 44.2% ❌ |
| **03 — Agent Orchestration** | ✅ | **❌ 6 fail** | ~80% |
| **04 — Agent Registry** | ✅ | ✅ Pass | 85.8% ✅ |
| **05 — Department Template Engine** | ✅ | ✅ Pass | 80.6% ✅ |

---

## CRITICAL: Module 03 Test Failures

Module 03 has 6 test failures — all caused by the same root issue: **tests don't set up tenant context but handlers now enforce tenant isolation**.

| Test | Expected | Got | Root Cause |
|------|----------|-----|------------|
| `DeleteExecution/returns_500_for_missing` | 500 | 404 | Handler correctly returns 404 for missing execution; test expects wrong code |
| `GetWorkflowVariables/gets_workflow_variables` | 200 | 404 | Test adds variable to store but doesn't create the workflow; `GetByIDAndTenant` fails |
| `ExecuteWorkflow/success_without_DAG_engine` | 204 | 404 | No tenant context set; `GetByIDAndTenant(id, "")` fails |
| `ExecuteWorkflow/success_with_DAG_engine` | 204 | 404 | Same — no tenant context |
| `AcknowledgeEscalation/acknowledges_pending` | 200 | 403 | Handler checks `GetByIDAndTenant` on escalation's workflow; no workflow in store, no tenant |
| `ResolveEscalation/resolves_acknowledged` | 200 | 403 | Same — tenant mismatch on escalation workflow lookup |
| `ListWorkflowRetryRecords/lists_retry_records` | 200 | 404 | Workflow not in store, no tenant context |

**Fix:** Each test needs `req = setTenant(req)` (where defined) and must create the prerequisite entities (workflow, escalation) in the store before testing the handler.

---

## C1. ERROR RESPONSE INCONSISTENCY — CRITICAL

Five different error response patterns across 5 modules. API clients consuming any of these modules will get inconsistent error shapes.

| Module | Pattern | Signature | Response Shape |
|--------|---------|-----------|----------------|
| **01** | 5-param WriteError | `(w, status, code, message, details)` | `{ "code": 400, "message": "...", "details": "..." }` |
| **02** | Inline JSON via `http.Error` | N/A | `{ "error": "..." }` with `Content-Type: text/plain` ❌ |
| **03** | 4-param WriteError | `(w, status, code, message)` | `{ "code": 400, "message": "..." }` |
| **04** | 5-param writeError | `(w, status, type, title, detail)` | RFC 7807: `{ "type": "...", "title": "...", "detail": "..." }` |
| **05** | 5-param writeError | `(w, status, "about:blank", title, detail)` | Same RFC 7807 as 04 |

**Specific issues:**
- **Module 02** uses `http.Error(w, json, status)` which sets `Content-Type: text/plain; charset=utf-8` — clients will reject these as non-JSON
- **Module 01** includes a `details` field not present in any other module
- **Modules 04/05** use RFC 7807 Problem Details format — the most standards-compliant approach but different from all others
- **No shared error type** exists in any common package

**Recommendation:** Create a shared `internal/errors` package with a single `ErrorResponse` struct and `WriteError` method. Standardize on RFC 7807 format (Modules 04/05 are correct).

---

## C2. MIDDLEWARE PATTERN INCONSISTENCY

### JWT Authentication

| Module | Method | Source | Fallback |
|--------|--------|--------|----------|
| 01 | HMAC-S256 | `JWTSecret` env var | None |
| 02 | JWKS/RS256 + HMAC-S256 | `JWKSCache` from Authentik | HMAC fallback for internal tokens |
| 03 | HMAC-S256 | `JWTSecret` env var | None |
| 04 | HMAC-S256 → JWKS | `JWTAuth` then `JWKSAuth` | HMAC fallback |
| 05 | HMAC-S256 only | `JWTSecret` env var | None (empty secret rejected in Validate) |

**Issue:** Modules 01/03/05 only support HMAC-S256 with a shared secret. Module 02 and 04 support JWKS/RS256. This means:
- Tenant admin tokens from Module 02's Authentik integration won't validate in Modules 01/03/05
- Module 05's JWT validation was recently hardened (empty secret rejected) but still uses HMAC-only

### Rate Limiting

| Module | Rate Limit | Window | Scope |
|--------|-----------|--------|-------|
| 01 | ❌ Not present | — | — |
| 02 | ✅ 100 req/s, 200 burst | Token bucket | Per-client (X-Forwarded-For) |
| 03 | ❌ Not present | — | — |
| 04 | ✅ 100 req/min | Sliding window | Per-tenant |
| 05 | ✅ 100 req/min | Sliding window | Per-tenant |

**Issue:** Modules 01 and 03 have no rate limiting. Module 02's rate limiter is per-client-IP and runs BEFORE auth (unauthenticated requests consume rate limit). Modules 04/05 rate limit per-tenant after auth (better).

### Middleware Chain Order

| Module | Chain (outer to inner) |
|--------|----------------------|
| 01 | JWTAuth → TenantContext → TraceID → RequestID → Logger → router |
| 02 | CORS → RequestID → Logger → TraceInjector → AuthValidator → TenantInjector → RateLimiter → router |
| 03 | JWTAuth → TenantContext → TraceID → RequestID → Logger → router |
| 04 | ChainJWTAuth → RateLimit → ExtractTenant → TraceID → RequestID → Logger → router |
| 05 | ChainJWTAuth → RateLimit → TenantContext → TraceID → RequestID → Logger → router |

**Issue:** Module 02's chain is significantly different (has CORS, different order). Rate limiting should come AFTER tenant context extraction, not before (Module 02 issue).

---

## C3. STORE TENANT ISOLATION — CRITICAL FOR MODULE 02

Three different tenant isolation patterns across modules:

| Module | Pattern | Enforcement Level | Coverage |
|--------|---------|-------------------|----------|
| **01** | `GetByIDAndTenant(id, tenantID)` methods | Store-level | 5 stores with tenant isolation, but NOT all stores have it |
| **02** | `GetByID(id)` only — NO tenant check | Handler-level only | ❌ **ALL stores are tenant-blind at store level** |
| **03** | `GetByIDAndTenant(id, tenantID)` | Store-level | 5 stores, not all |
| **04** | `byTenant[tenantID]` maps — all data partitioned | Store-level | ✅ All 4 stores use `byTenant` maps |
| **05** | `GetByIDAndTenant(id, tenantID)` | Store-level | 3 stores with tenant isolation |

**Module 02 is critically exposed:** Every `GetByID(id)` call is tenant-blind. A malicious request with a user ID from another tenant's scope would return data. The handler-level tenant checks are a safety net, but any handler that forgets to check will leak cross-tenant data.

**Module 01 has partial coverage:** `TenantStore.GetByIDAndTenant` exists but `BillingStore.GetByID` does not. Check which stores are missing the tenant-scoped variant.

---

## C4. EVENT PUBLISHING GAP

### Module 01 (Tenant Control Plane)

| Method | Called? |
|--------|---------|
| `TenantCreated` | ✅ In `Create` |
| `TenantUpdated` | ✅ In `Update` |
| `TenantDeleted` | ✅ In `Delete` |

### Module 02 (Identity & Access)

| Method | Called? |
|--------|---------|
| `UserCreated` | ✅ |
| `UserUpdated` | ❌ Never called |
| `UserSuspended` | ❌ Never called |
| `IdentityRotated` | ✅ |
| `PermissionGranted` | ✅ |
| `PermissionRevoked` | ❌ Never called |
| `SessionCreated` | ❌ Never called |
| `SessionExpired` | ❌ Never called |
| `SessionEnded` | ❌ Never called |
| `MfaEnrolled` | ✅ |
| `SsoLogin` | ❌ Never called |
| `SessionActive/Replay_*` | ❌ Never called |

**8 out of 16 event methods are dead code.** AsyncAPI defines 9 channels, but only 3 are actively published.

### Module 03 (Agent Orchestration)

Publishes events for workflows, executions, schedules, escalations, retries. Covers 30+ event types across LangGraph/Temporal/Ray/Celery stacks. All appear wired in their respective handlers.

### Module 04 (Agent Registry)

| Method | Called? |
|--------|---------|
| `PublishTemplateCreated` | ✅ |
| `PublishTemplateUpdated` | ✅ |
| `PublishTemplateDeployed` | ✅ |
| `PublishTemplateDeploymentFailed` | ✅ |
| `PublishTemplateUndeployed` | ✅ |
| `PublishTemplateDeleted` | ✅ |
| `PublishTemplateVersioned` | ✅ |
| `PublishTemplateCloned` | ✅ |
| `PublishCustomTemplateCreated` | ✅ |
| `PublishCustomTemplateUpdated` | ✅ |
| `PublishCustomTemplateDeleted` | ✅ |
| `PublishCustomTemplateCloned` | ✅ |

All wired. ✅

### Module 05 (Department Template Engine)

All events wired after remediation. ✅

---

## C5. ASYNCAPI CONTRACT ALIGMENT

| Module | AsyncAPI Channels | Active Events | Mismatch? |
|--------|-------------------|---------------|-----------|
| 01 | 4 (tenant.created/updated/deleted/deprecated) | 3 active | `tenant.deprecated` never published |
| 02 | 9 (iam/*) | 3 active | 5 events have no AsyncAPI channel; 6 events never published |
| 03 | 8+ (LangGraph/Temporal/Ray/Celery) | All published | ✅ |
| 04 | 8 (operan.events/template/*) | All published | ✅ |
| 05 | 8 (operan.events/template/*) | All published | ✅ |

---

## C6. DOCKERFILE & INFRASTRUCTURE CONSISTENCY

| Module | Dockerfile | Helm | README | PROGRESS.md | HANDOFF.md | manifest.json |
|--------|-----------|------|--------|-------------|------------|---------------|
| 01 | ✅ | ❌ (only Dockerfile) | ✅ | ✅ | ✅ | ❌ |
| 02 | ✅ | ❌ (no chart dir) | ✅ | ✅ | ✅ | ❌ |
| 03 | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| 04 | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| 05 | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |

**Issue:** Only Module 05 has both PROGRESS.md AND HANDOFF.md. Modules 03 and 04 are missing both. Module 02's PROGRESS.md is named `PROGRESS_REPORT.md` (inconsistent filename).

---

## C7. CONTRACT DUPLICATION

**Module 05:** Two sets of contract files exist:
- `openapi-05-department-template-engine.yaml` / `openapi-05-department-template.yaml`
- `schema-05-department-template-engine.json` / `schema-05-department-template.json`
- `asyncapi-05-department-template-engine.yaml` / `asyncapi-05-department-template.yaml`

The `-engine` suffixed versions are the active ones (post-remediation). The non-`-engine` versions are stale duplicates that should be removed.

---

## C8. MODULE 02 SPECIFIC ISSUES (Highest Risk)

### 8.1 Content-Type Violation
Handlers use `http.Error(w, jsonBody, status)` which sets `Content-Type: text/plain`. API clients will fail to parse these responses.

### 8.2 Tenant-Blind Stores
Every `GetByID` is tenant-blind. No `GetByIDAndTenant` exists in any store. This is a **critical tenant isolation bypass risk**.

### 8.3 Authentik Client Untested
9.5% coverage — all 18+ API client methods are untested. This is Module 02's primary external dependency.

### 8.4 Model Validations Untested
`internal/models` has 0% coverage — all 14 `Validate()` methods are never tested.

### 8.5 Config Untested
`internal/config` has 0% coverage — `ParseConfig()`, `Validate()`, `env()`, `envInt()` are never tested.

---

## SUMMARY: Issues by Priority

### 🔴 P0 — Security / Data Integrity (must fix before any module is "APPROVED")
1. **Module 02: Tenant-blind stores** — No `GetByIDAndTenant` in any store. Critical cross-tenant data leak risk.
2. **Module 02: Content-Type: text/plain on JSON errors** — API contract violation that could break clients.
3. **Module 03: 6 broken tests** — Handler tests don't set up tenant context properly.

### 🟠 P1 — Cross-Module Inconsistency (should fix before platform launch)
4. **5 different error response formats** — No shared error type. Creates consumer confusion.
5. **3 different middleware auth patterns** — JWT-only vs JWKS+HMAC. Tokens from one module won't work in others.
6. **2 different rate limiting implementations** — Per-client-IP (02) vs per-tenant (04/05).
7. **Only 1 module (05) has complete infra docs** — Missing PROGRESS.md and HANDOFF.md in 01-04.

### 🟡 P2 — Code Quality (should fix)
8. **Module 02: 8/16 event methods are dead code**
9. **Module 02: Authentik client 9.5% coverage**
10. **Module 02: Config and models 0% coverage**
11. **Module 01: Coverage 30.5%**
12. **Module 05: Contract duplication** — stale non-`-engine` contract files should be removed.

### 🟢 P3 — Minor
13. **Module 01: No rate limiting**
14. **Module 03: No rate limiting**
15. **Module 02: Rate limiting before auth** (brute-force DoS vector)
