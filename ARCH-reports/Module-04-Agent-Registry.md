# ARCH — Module 04: Agent Registry — Production Maturity Review

**Review Date:** 2026-06-18
**Reviewer:** ARCH (Platform Architect)
**Verdict:** **CONDITIONAL — Strong test coverage and tenant isolation, but missing infrastructure, incomplete PRD scope, and contract drift require remediation**

---

## 1. Summary Scorecard

| Category | Score | Notes |
|----------|-------|-------|
| Contract Compliance | ⚠️ PARTIAL | 17 operationIds; 8 AsyncAPI channels; 20 JSON Schema defs — several cross-spec inconsistencies |
| Test Coverage | ⚠️ PARTIAL | 148 tests, 72.6% coverage — below 80% threshold but zero failures |
| Integration | ❌ FAIL | Kafka broker is stub; no inter-module edges wired; 0 outbound events to real broker |
| Infrastructure | ❌ FAIL | No Dockerfile, Helm chart, README, PROGRESS.md, or HANDOFF.md |
| Security | ⚠️ PARTIAL | Tenant isolation on stores, HMAC-S256 auth (no JWKS), no rate limiting |
| Database | ❌ FAIL | In-memory only — no PostgreSQL adapter |
| PRD Alignment | ⚠️ PARTIAL | 5 of ~14 PRD endpoints implemented; core agent CRUD + versions + capabilities + dependencies + search |
| **Overall** | **CONDITIONAL** | **Usable for development/testing; not production-ready** |

---

## 2. Contract Drift Analysis

### 2.1 [CRITICAL] `Error` Schema Misplaced in OpenAPI

The OpenAPI contract defines `Error` schema at the top level under `components` (alongside `schemas`, `responses`, `parameters`) rather than under the `schemas` key. The schema is referenced as `#/components/schemas/Error` but since it is not under `schemas`, this is an invalid `$ref`.

**Impact:** OpenAPI validators will reject the contract as invalid. Clients generating code from the contract will fail.

**Remediation:** Move `Error` under `components/schemas/Error` in the OpenAPI file.

---

### 2.2 [HIGH] `promoted_to` Field — OpenAPI vs JSON Schema

| Contract | Type | Description |
|----------|------|-------------|
| **OpenAPI** `AgentVersion.promoted_to` | `map<string, string>` (no format) | "Map of environment names to **version IDs**" |
| **JSON Schema** `AgentVersion.promoted_to` | `map<string, string>` (`format: date-time`) | "Map of environment names to **promotion timestamps**" |

**Impact:** OpenAPI consumers send version IDs; JSON Schema consumers send timestamps. These are semantically different. A version promotion recorded as a timestamp will fail version ID validation and vice versa.

**Remediation:** Align to a single representation. Architectural recommendation: version IDs (OpenAPI's approach).

---

### 2.3 [HIGH] Promote Environment Enum — OpenAPI vs AsyncAPI

| Contract | Enum Values |
|----------|-------------|
| **OpenAPI** `PromoteVersionRequest.environment` | `dev`, `staging`, `production` |
| **AsyncAPI** `AgentPromotedPayload.to_environment` | `staging`, `production` (no `dev`) |

**Impact:** If a version is promoted to `dev`, the AsyncAPI event `AgentPromotedPayload.to_environment` will contain a value (`dev`) that is not in the AsyncAPI enum. Consumers will reject or misclassify the event.

**Remediation:** Add `dev` to the AsyncAPI enum, or remove it from the OpenAPI enum if dev promotions should not trigger events.

---

### 2.4 [HIGH] JSON Schema `Agent` Has Fields Missing in OpenAPI

| Field | JSON Schema | OpenAPI |
|-------|-------------|---------|
| `version` | ✅ `type: string` | ❌ Missing |
| `created_by` | ✅ `type: string, format: uuid` | ❌ Missing |
| `dependencies` | ✅ `type: array` | ❌ Missing |

**Impact:** JSON Schema consumers will expect these fields. If the handler does not return them, clients will get incomplete data. If the handler does return them, they will fail OpenAPI validation.

**Remediation:** Add `version`, `created_by`, and `dependencies` to the OpenAPI `Agent` schema.

---

### 2.5 [HIGH] JSON Schema `CreateAgentRequest` and `UpdateAgentRequest` Missing `runtime_constraints`

| Contract | `runtime_constraints` |
|----------|----------------------|
| **OpenAPI** `CreateAgentRequest` | ✅ Present |
| **OpenAPI** `UpdateAgentRequest` | ✅ Present |
| **JSON Schema** `CreateAgentRequest` | ❌ Missing |
| **JSON Schema** `UpdateAgentRequest` | ❌ Missing |

**Impact:** OpenAPI clients sending `runtime_constraints` will produce payloads that fail JSON Schema validation.

**Remediation:** Add `runtime_constraints` to both JSON Schema request types.

---

### 2.6 [HIGH] Duplicate Schemas in OpenAPI

| Schema | Occurrences |
|--------|-------------|
| `RuntimeConstraints` | Defined twice (lines ~481 and ~875) |
| `CostProfile` | Defined twice (lines ~452 and ~897) |

**Impact:** While functionally harmless (OpenAPI allows duplicate definitions), this is bad practice and may confuse code generators and documentation tools.

**Remediation:** Deduplicate — define once under `components/schemas` and `$ref` from all uses.

---

### 2.7 [MEDIUM] `RemoveDependency` Uses Query Parameter Instead of Path

The `removeDependency` (DELETE `/agents/{agent_id}/dependencies`) takes `dependency_id` as a **query parameter** rather than a path parameter (`/agents/{agent_id}/dependencies/{dependency_id}`). This breaks REST convention.

**Remediation:** Change to `/agents/{agent_id}/dependencies/{dependency_id}` DELETE with `dependency_id` as a path parameter.

---

### 2.8 [MEDIUM] Inconsistent Auth Response Codes

| Endpoint | 401 | 403 | 404 |
|----------|-----|-----|-----|
| `createAgent` | ✅ | ✅ | ✅ |
| `getAgent` | ✅ | ✅ | ✅ |
| `updateAgentVersion` | ❌ | ❌ | ✅ |
| `createAgentVersion` | ❌ | ❌ | ✅ |
| `indexCapabilities` | ✅ | ❌ | ✅ |
| `searchAgents` | ✅ | ✅ | ❌ |

**Impact:** API documentation is incomplete. Clients cannot know which endpoints require authentication vs authorization vs existence checks.

**Remediation:** Add missing 401/403 responses to all write endpoints.

---

### 2.9 [MEDIUM] Search Uses POST with No Pagination

All other list endpoints (agents, versions, capabilities, dependencies) use GET with `PageParam`/`PageSizeParam` query parameters. `searchAgents` uses POST with no pagination support.

**Impact:** Large registries will return unbounded result sets, causing memory issues and slow responses.

**Remediation:** Either add pagination to `searchAgents` or document it as intentionally unbounded (not recommended).

---

### 2.10 [MEDIUM] Missing OpenAPI Schemas in JSON Schema

| OpenAPI Schema | JSON Schema Definition |
|---------------|----------------------|
| `AgentVersion` (with `promoted_to` map) | ✅ Present |
| `CreateVersionRequest`, `UpdateVersionRequest` | ✅ Present |
| `CapabilityEntry`, `CapabilityList`, `CapabilityUpdate` | ✅ Present |
| `RuntimeConstraints`, `CostProfile` | ✅ Present |
| `ExecutionBudget` | ✅ Present |
| `AgentSearchRequest`, `AgentSearchResponse` | ✅ Present |
| `PromoteVersionRequest` | ❌ Missing |
| `AgentListResponse` | ❌ Missing |
| `VersionList` | ❌ Missing |
| `DependencyList` | ❌ Missing |
| `ErrorResponse` | ✅ Present |

---

## 3. Security Assessment

### 3.1 [MEDIUM] JWT Auth Uses HMAC-S256 Only

The middleware implements HMAC-S256 JWT validation only. The architecture blueprint specifies RSA via JWKS. No JWKS endpoint is configured, and there is no RSA key loading.

**Impact:** If an attacker obtains the HMAC secret (which defaults to `"change-me-in-production"` if not set), they can forge any JWT. RSA/JWKS provides asymmetric verification — the registry only needs the public key, not the private signing key.

**Remediation:** Implement JWKS-based RSA verification. This can be done by connecting to Module 02's JWKS endpoint or an external IdP.

---

### 3.2 [MEDIUM] Missing RBAC Middleware

The middleware chain does not include role-based access control. The `RequireRole` middleware exists in code but is not wired into the chain. No handler checks if the authenticated user has the correct role for the requested operation.

**Impact:** Any authenticated user can perform any operation on any agent, regardless of their role or tenant permissions.

**Remediation:** Wire `RequireRole` into the middleware chain for write operations. Define role requirements per endpoint.

---

### 3.3 [MEDIUM] Missing JSON Schema Request Validation

No middleware validates incoming request bodies against JSON Schema. Handlers simply decode JSON into structs. Invalid payloads will silently produce runtime errors or partial data.

**Impact:** API consumers can send malformed requests that bypass all validation, potentially corrupting store data.

**Remediation:** Add a JSON Schema validation middleware that validates request bodies against the corresponding schema before the handler processes them.

---

### 3.4 [MEDIUM] No Rate Limiting

No middleware exists for rate limiting. The search endpoint is particularly vulnerable — a malicious client could flood the registry with search requests.

**Remediation:** Implement rate-limiting middleware with per-tenant limits.

---

### 3.5 [LOW] `WriteError` Never Populates `RequestID`

The `WriteError` function sets `request_id` to an empty string in the `ErrorResponse`. This breaks request tracing and debugging.

**Remediation:** Populate `request_id` from context (the `RequestID` key set by middleware).

---

### 3.6 [INFO] Unused `eventPublisher` Parameter in `JWTAuth`

The `JWTAuth(secretEnvVar string, eventPublisher interface{})` function takes an `eventPublisher` parameter that is never used. This is dead code.

**Remediation:** Remove the unused parameter.

---

## 4. Test Coverage Analysis

### 4.1 Coverage Summary

| Package | Test Files | Tests | Coverage | Status |
|---------|-----------|-------|----------|--------|
| `internal/handlers` | 1 | 23 | ~60% | ⚠️ PARTIAL |
| `internal/store` | 3 | 32 | ~75% | ⚠️ PARTIAL |
| `internal/middleware` | 1 | 22 | ~85% | ✅ GOOD |
| `internal/events` | 1 | 18 | ~70% | ⚠️ PARTIAL |
| `internal/config` | 1 | 10 | ~65% | ⚠️ PARTIAL |
| `internal/cache` | 1 | 14 | ~70% | ⚠️ PARTIAL |
| `internal/broker` | 1 | 10 | ~60% | ⚠️ PARTIAL |
| `internal/ctxkeys` | 1 | 5 | ~80% | ✅ GOOD |
| `main.go` | 0 | 0 | 0% | ❌ FAIL |
| **Overall** | 10 | 148 | **72.6%** | Below 80% target |

### 4.2 Handler Coverage Gaps

The handler tests cover core CRUD for agents, search, deprecate, archive, and middleware chain. But:
- No tests for version CRUD (create, update, promote)
- No tests for capability CRUD (update, index)
- No tests for dependency CRUD (add, remove)
- No tenant-isolation tests for handlers
- No tests for error paths (400, 401, 403, 404 handling)

### 4.3 Store Coverage Gaps

- `AgentStore`: 13 tests — CRUD + tenant isolation ✅
- `VersionStore`: 7 tests — CRUD + list by status ✅
- `DependencyStore`: 12 tests — CRUD + tenant isolation ✅
- No integration/store-level tests with real database

### 4.4 Missing Test Categories

- **No handler-level tests for versions, capabilities, or dependencies**
- **No tenant-isolation tests for handler layer**
- **No integration tests** (no database, no testcontainers)
- **No end-to-end tests**

---

## 5. Infrastructure & Deployment

### 5.1 Infrastructure Status

| Artifact | Status |
|----------|--------|
| `Dockerfile` | ❌ MISSING |
| `helm/Chart.yaml` + templates + values.yaml | ❌ MISSING |
| `README.md` | ❌ MISSING |
| `PROGRESS.md` | ❌ MISSING |
| `HANDOFF.md` | ❌ MISSING |
| `manifest.json` | ✅ Present (claims 72.6% coverage) |
| `go.mod` | ✅ Present (single dep: `uuid`) |
| `temp/Architecture.md` | ✅ Present (ASCII architecture diagrams) |

Module 04 has no infrastructure tooling. This is the worst of all modules reviewed so far in this regard — Modules 01 and 03 have Dockerfiles and Helm charts.

### 5.2 Broker/Event Publishing

The `broker` package provides:
- `Producer` interface
- `KafkaProducer` — stub (only logs messages)
- `MockProducer` — test utility

`events.NewPublisherWithConfig(cfg)` creates a `KafkaProducer` from config, but the producer only logs. No real Kafka connection.

### 5.3 Empty Directories

`cmd/agent-registry/` and `internal/handler/` both exist but are empty. These are scaffolding artifacts from a prior generation phase.

---

## 6. PRD Alignment

### 6.1 Scope

The PRD specifies ~14 endpoints for Module 04. **17 endpoints are implemented** (including some beyond the PRD). However, the PRD indicates this module should:
- Support agent lifecycle (register, update, deprecate, archive) — ✅ Partial (CRUD + deprecate/archive exist)
- Support versioning (semver, promote, diff) — ✅ Partial (CRUD exists, promote exists)
- Support capability indexing — ✅ Partial (upsert + index exist)
- Support dependency tracking — ✅ Partial (CRUD exists)
- Support agent search — ✅ Implemented

### 6.2 Unimplemented Features

| PRD Feature | Status |
|------------|--------|
| Agent lifecycle management (full) | ✅ CRUD + deprecate/archive |
| Version management (full) | ✅ CRUD + promote |
| Capability indexing (full) | ✅ Upsert + index |
| Dependency tracking (full) | ✅ CRUD |
| Agent search (full) | ✅ Implemented |
| Cost profile management | ⚠️ Schema exists but no dedicated endpoint |
| Execution budget tracking | ⚠️ Schema exists but no dedicated endpoint |
| Memory access policies | ⚠️ Schema exists but no dedicated endpoint |
| Runtime constraints management | ⚠️ Schema exists but no dedicated endpoint |
| Multi-region deployment support | ❌ Not implemented |
| Agent health monitoring | ❌ Not implemented |

---

## 7. Implementation Quality Observations

### 7.1 Strengths

- **Tenant isolation on stores** — AgentStore, VersionStore, DependencyStore all use tenant-scoped maps
- **Comprehensive test suite** — 148 passing tests across 10 files, good variety of scenarios
- **Clean DTO pattern** — Handler models are separate from store models
- **Cache layer** — In-memory LRU cache with eviction callbacks
- **Context keys** — Typed context keys (TenantID, UserID, TraceID, RequestID, UserRole)
- **Graceful shutdown** — main.go handles SIGTERM/SIGINT

### 7.2 Issues

| # | Location | Issue | Severity |
|---|----------|-------|----------|
| 1 | `main.go` | Creates publisher via `NewPublisher()` then immediately overwrites it | MEDIUM |
| 2 | `handlers/agent_registry.go` | `extractIDFromPath` has dead fallback code | LOW |
| 3 | `store/capabilities.go` | `ListAll` returns last upserted per agent, not all capabilities | HIGH (design flaw) |
| 4 | `handlers/router.go` | Wildcard route matching — potential route shadowing | MEDIUM |
| 5 | `middleware/middleware.go` | `ChainJWTAuth` and `JWTAuthWithSecret` are duplicate paths | LOW |
| 6 | `store/agents.go` | `SearchAgents` filters in-memory after store.List() — inefficient | LOW |

---

## 8. Remediation Plan

### Phase 1: Critical Blockers (Must Fix Before Re-Review)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 1 | Fix `Error` schema placement in OpenAPI (move under `schemas` key) | ARCH → CODER | P0 |
| 2 | Reconcile `promoted_to` field type (OpenAPI string vs JSON Schema date-time) | ARCH → CODER | P0 |
| 3 | Reconcile promote environment enum (AsyncAPI missing `dev`) | ARCH → CODER | P0 |
| 4 | Add missing fields to OpenAPI `Agent` (`version`, `created_by`, `dependencies`) | ARCH → CODER | P0 |
| 5 | Add `runtime_constraints` to JSON Schema request types | ARCH → CODER | P0 |
| 6 | Deduplicate `RuntimeConstraints` and `CostProfile` in OpenAPI | ARCH → CODER | P0 |
| 7 | Create Dockerfile, Helm chart, README.md | CODER | P0 |
| 8 | Create PROGRESS.md and HANDOFF.md | CODER | P0 |

### Phase 2: High-Priority Gaps (Fix Before Production Sign-Off)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 9 | Wire RBAC middleware into handler chain | CODER | P1 |
| 10 | Implement JWKS-based JWT validation (replace HMAC-only) | CODER | P1 |
| 11 | Add JSON Schema request validation middleware | CODER | P1 |
| 12 | Add handler tests for versions, capabilities, dependencies (reach ≥80% coverage) | CODER | P1 |
| 13 | Implement CapabilityStore.ListAll properly (return all capabilities per agent) | CODER | P1 |
| 14 | Wire event publishing to Kafka or document stub limitation | CODER | P1 |
| 15 | Add rate-limiting middleware | CODER | P1 |
| 16 | Add PostgreSQL adapter (at minimum, same pattern as Module 03) | CODER | P1 |
| 17 | Fix `RemoveDependency` to use path parameter | ARCH → CODER | P1 |
| 18 | Add 401/403 responses to all write endpoints in OpenAPI | ARCH → CODER | P1 |
| 19 | Add pagination to `searchAgents` | ARCH → CODER | P1 |

### Phase 3: Medium-Priority Enhancements

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 20 | Remove unused `eventPublisher` parameter from `JWTAuth` | CODER | P2 |
| 21 | Populate `RequestID` in `WriteError` responses | CODER | P2 |
| 22 | Remove empty directories (`cmd/agent-registry/`, `internal/handler/`) | CODER | P2 |
| 23 | Fix off-by-one and dead code in path extraction | CODER | P2 |
| 24 | Add OpenTelemetry instrumentation | CODER | P2 |
| 25 | Add `/ready` endpoint with database and event broker checks | CODER | P2 |
| 26 | Add PostgreSQL auto-migration on startup | CODER | P2 |

---

## 9. Developer Sign-Off Checklist

Before resubmitting Module 04 for re-review, the CODER team must complete AND verify each item below:

- [ ] **P0-1:** `Error` schema moved under `components/schemas` in OpenAPI
- [ ] **P0-2:** `promoted_to` type reconciled across OpenAPI and JSON Schema
- [ ] **P0-3:** Promote environment enum reconciled across OpenAPI and AsyncAPI
- [ ] **P0-4:** Missing fields added to OpenAPI `Agent` schema
- [ ] **P0-5:** `runtime_constraints` added to JSON Schema request types
- [ ] **P0-6:** Duplicate schemas deduplicated in OpenAPI
- [ ] **P0-7:** Dockerfile created; `docker build` succeeds
- [ ] **P0-8:** Helm chart created; `README.md`, `PROGRESS.md`, `HANDOFF.md` written
- [ ] **P1-9 through P1-18:** Phase 2 remediation items completed
- [ ] **P2-20 through P2-26:** Phase 3 items completed (for production readiness)

---

## 10. Architect's Note

Module 04 has a **surprisingly solid code foundation** compared to its peers. With 148 passing tests and 72.6% coverage, it is the most test-covered module so far. The tenant isolation on all stores is correctly implemented, the DTO pattern is clean, and the cache layer with eviction callbacks shows operational maturity.

However, **no infrastructure tooling** (Dockerfile, Helm, README) is the most concerning gap — a module that has 72.6% test coverage should be able to produce a Dockerfile. The missing RBAC middleware and HMAC-only JWT validation are significant security gaps for a module that will store agent capability data and version information for the entire platform.

The contract drift (6 HIGH/MEDIUM issues) suggests that the JSON Schema was authored independently from the OpenAPI contract. Both should be generated from a common source of truth, or at least be reviewed together before release.

**Remediation effort estimate:** **2–3 developer days for P0 items** (mostly contract fixes and infrastructure), **5–7 days for P1 items** (security hardening, tests, PostgreSQL adapter).

Module 04 is the **closest to production** of all modules reviewed so far. With infrastructure tooling and security hardening, it could be deployed for internal testing.

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18
**CODER Acknowledgment:** _________________    **Date:** _________________
