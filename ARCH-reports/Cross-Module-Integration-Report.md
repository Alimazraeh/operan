# ARCH — Cross-Module Integration Report: Waves 1–5

**Review Date:** 2026-06-18
**Reviewer:** ARCH (Platform Architect)
**Scope:** Modules 01–05 (Tenant Control Plane, Identity & Access, Agent Orchestration, Agent Registry, Department Template Engine)

---

## Executive Summary

| Module | Verdict | Coverage | Infrastructure | Security | Contract | Priority |
|--------|---------|----------|---------------|----------|----------|----------|
| **01 — Tenant Control Plane** | REJECT | 0% handlers tested | Dockerfile ✅ | ⚠️ JWT HMAC-only | 6 HIGH drifts | P0 |
| **02 — Identity & Access** | REJECT | 16% handler coverage | ❌ Missing | ❌ HMAC fallback, no tenant isolation on ABAC | 8 drifts | P0 |
| **03 — Agent Orchestration** | REJECT | 0% handler tests | Dockerfile ✅ Helm ✅ README ✅ | Tenant bypass on reads | 11 drifts | P0 |
| **04 — Agent Registry** | CONDITIONAL | 72.6% coverage | ❌ Missing | No RBAC middleware | 7 drifts | P1 |
| **05 — Department Template** | CONDITIONAL | ~50% coverage | Dockerfile ✅ Helm ✅ README ✅ | Empty JWT secret bypass | 7 drifts | P1 |

**Platform Overall: REJECT — Not production-ready**

Five modules reviewed, zero approved. Every module has critical security issues, missing tests, and contract drift.

---

## 1. Security Platform-Wide Assessment

### 1.1 JWT Authentication — Uniform Gap Across All Modules

| Module | Auth Method | JWT Secret | JWT Library |
|--------|------------|------------|-------------|
| 01 | HMAC-S256 (custom impl) | `"change-me-in-production"` (default) | None (hand-rolled) |
| 02 | HMAC-S256 (custom impl) + **RSA/JWKS fallback to HMAC** | `"change-me-in-production"` (default) | None (hand-rolled) |
| 03 | HMAC-S256 (hand-rolled) | `"change-me-in-production"` (default) | None (hand-rolled) |
| 04 | HMAC-S256 (hand-rolled) | `"change-me-in-production"` (default) | None (hand-rolled) |
| 05 | HMAC-S256 (hand-rolled) | `"change-me-in-production"` (default) | None (hand-rolled) |

**Platform-WIDE Findings:**

1. **All 5 modules use hand-rolled JWT validation** — no `golang-jwt` or equivalent. This is a major security risk: custom crypto implementations are prone to timing attacks, invalid encoding handling, and signature comparison bugs.
2. **All 5 modules default the JWT secret to `"change-me-in-production"`** — if this environment variable is not set, all modules accept tokens signed with this well-known secret.
3. **Module 02 has a catastrophic HMAC fallback** — if JWKS validation fails, it falls back to HMAC validation with the same secret. This means ANY token signed with a known HMAC secret passes validation, completely undermining RSA/JWKS authentication.
4. **No module implements JWKS-based RSA verification** — the architecture specifies asymmetric key verification, but no module implements it.

**Recommendation:** Standardize on `golang-jwt` across all modules. Module 02 should implement the JWKS client and connect to the IAM endpoint. All modules should fail startup if the JWT secret is the default value.

---

### 1.2 Tenant Isolation — Inconsistent Enforcement

| Module | Pattern | Issues |
|--------|---------|--------|
| 01 | `GetByTenant` on stores | ✅ Correct — all reads verify tenant |
| 02 | Store has `byTenant` maps | ❌ ABAC store is **global** (not tenant-scoped); AuditStore Create() has no tenant enforcement |
| 03 | `byTenant` maps on stores | ❌ Handlers call `GetByID` **without verifying** the retrieved resource's tenant ID |
| 04 | Tenant-scoped stores | ✅ Correct — all reads verified against tenant |
| 05 | `GetByIDAndTenant` pattern | ✅ Correct — all reads verify tenant |

**Platform-WIDE Findings:**

- **Modules 01, 04, 05** have correct tenant isolation patterns.
- **Module 02** has the most severe tenant bypass: the ABAC policy store is a **global map** accessible by any tenant. The AuditStore Create() does not enforce tenant from context.
- **Module 03** has the most common pattern: stores are tenant-scoped, but **handlers bypass tenant verification** by calling `GetByID` instead of `GetByIDAndTenant` and not checking the result.

**Recommendation:** Standardize on the `GetByIDAndTenant` pattern (used by Modules 01, 05) across all modules. This prevents handler-level bypasses by making the API impossible to misuse.

---

### 1.3 Rate Limiting — Entirely Missing

**No module implements rate limiting.** All 5 modules directly expose their endpoints to unauthenticated (or just-authenticated) consumers with no throttle. This is a denial-of-service risk for all modules.

**Recommendation:** Implement a shared rate-limiting middleware package (sliding window or token bucket) with per-tenant configurable limits.

---

### 1.4 Request Validation — Entirely Missing

**No module validates incoming request bodies against JSON Schema.** All modules use `json.Decode` into Go structs with no schema-level validation. Malformed payloads silently produce runtime errors or partial data.

**Recommendation:** Implement a shared JSON Schema validation middleware (Module 04's blueprint specifies this requirement).

---

## 2. Cross-Spec Contract Harmonization

### 2.1 Security Scheme Naming — Platform-Wide Inconsistency

| Module | Tenant Header Scheme Name | BearerAuth bearerFormat |
|--------|--------------------------|----------------------|
| 01 | `TenantHeader` | `JWT` |
| 02 | `TenantHeader` | `JWT` |
| 03 | `X-Tenant-ID` | `JWT` |
| 04 | `TenantHeader` | `JWT` |
| 05 | `TenantHeader` | `JWT` |

**Finding:** 4 of 5 modules use `TenantHeader`. Module 03 uses `X-Tenant-ID`. Module 02 also defines a unique `X-Request-ID` security scheme. The AsyncAPI contracts use `TenantContext` (Modules 03, 04) — yet another variation.

**Recommendation:** Unify to `TenantHeader` across all modules (4/5 already agree). Rename Module 03's `X-Tenant-ID` security scheme to `TenantHeader`. Define a platform-level shared security scheme in a common OpenAPI extension file.

---

### 2.2 Event Broker Protocol — Four Different Protocols

| Module | Protocol | Broker Address |
|--------|----------|---------------|
| 01 | Kafka | `events.operan.internal:9092` |
| 02 | AMQP | `mq.operan.internal` |
| 03 | Kafka | `kafka.prod.operan.io:9092` |
| 04 | TLS/TCP | `broker.operan.internal:443` |
| 05 | MQTT | `events.operan.io` |

**Finding:** The platform uses **four different message broker protocols** across just 5 modules. There is no unified broker abstraction contract at the platform level. Each module implements its own broker client.

**Recommendation:**
1. Establish a platform-level `Broker` interface (Module 03 has the closest pattern with `Broker` interface + `InMemoryBroker` + `KafkaBroker`).
2. Standardize on Kafka as the platform broker protocol.
3. Provide a shared `operan/broker` Go package that all modules import.
4. Phase out AMQP, MQTT, and TLS/TCP broker implementations.

---

### 2.3 Event Channel Naming Convention — Two Different Patterns

| Module | Channel Pattern | Example |
|--------|----------------|---------|
| 01–04 | `operan/events/{domain}/{resource}/{action}` | `operan/events/tenant/provisioned` |
| 05 | `operan.templates.template.{action}` | `operan.templates.template.created` |

**Finding:** Module 05 uses dots instead of slashes, has no `events/` prefix, and repeats `template` in the channel name. This is a fundamental routing convention mismatch.

**Recommendation:** Rebrand all Module 05 channels to `operan/events/template/created` pattern.

---

### 2.4 Error Response Format — Inconsistent Across Modules

| Module | Error Format | Fields |
|--------|-------------|--------|
| 01 | RFC 7807 Problem Details | `type`, `title`, `status`, `detail`, `instance` |
| 02 | Custom `ErrorResponse` | `code` (string), `message` (string), `details` (object) |
| 03 | Inline error responses | Varies by endpoint |
| 04 | Custom `Error` | `code` (integer), `message` (string), `request_id` (uuid) |
| 05 | RFC 7807 Problem Details | `type`, `title`, `status`, `detail`, `instance` |

**Finding:** Modules 01 and 05 use RFC 7807. Module 02 uses a custom format with string error codes. Module 04 uses a custom format with integer error codes. No platform-wide error standardization exists.

**Recommendation:** Standardize on RFC 7807 Problem Details (used by Modules 01 and 05) with an additional `request_id` field. Define a shared `Error` schema in a platform-level OpenAPI extension.

---

### 2.5 Priority Range — Module 03 AsyncAPI vs OpenAPI/JSON Schema

| Contract | Module 03 Priority Range | Default |
|----------|------------------------|---------|
| OpenAPI | 1–10 | 5 |
| JSON Schema | 1–10 | 5 |
| AsyncAPI | 1–100 | 50 |

**Finding:** Module 03's AsyncAPI contract defines priority 1–100 while the REST contract defines 1–10. This is the only cross-module priority issue identified, but it highlights the pattern of async contracts being authored separately from REST contracts.

---

### 2.6 `AgentStatus` Enum — Harmonized Within Module 04, But Not Shared Across Modules

| Module | Entity | Enum Values |
|--------|--------|-------------|
| 04 | `AgentStatus` | `active`, `inactive`, `deprecated`, `archived` |
| 04 (AsyncAPI) | `AgentStatus` | `active`, `inactive`, `deprecated`, `archived` |
| 04 (JSON Schema) | `VersionStatus` | `active`, `beta`, `deprecated`, `archived` |

**Finding:** Module 04 harmonizes `AgentStatus` across its three contract formats. But `VersionStatus` has `beta` instead of `inactive`. Module 05's `agentStatus` has `draft` instead of `active`. No shared `AgentStatus` definition exists across modules.

---

### 2.7 JSON Schema `additionalProperties: false` — Module 05 Unique Pattern

Module 05 is the **only module** that defines `additionalProperties: false` on all JSON Schema object types. Modules 01–04 have zero instances.

**Finding:** This creates a structural inconsistency: Module 05 schemas are strict (reject unknown fields), while Modules 01–04 schemas are lax (accept unknown fields). Code generators will produce different struct tags.

**Recommendation:** Standardize on `additionalProperties: false` for all modules.

---

## 3. Infrastructure Platform-Wide Assessment

### 3.1 Deployment Artifacts

| Module | Dockerfile | Helm Chart | README | PROGRESS.md | HANDOFF.md |
|--------|-----------|-----------|--------|-------------|------------|
| 01 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 02 | ❌ | ❌ | ❌ | ❌ | ❌ |
| 03 | ✅ | ✅ | ✅ | ❌ | ❌ |
| 04 | ❌ | ❌ | ❌ | ❌ | ❌ |
| 05 | ✅ | ✅ | ✅ | ❌ | ❌ |

**Finding:** Only Module 01 has all five infrastructure artifacts. Modules 02 and 04 have **none**. Modules 03 and 05 have Dockerfiles, Helm charts, and READMEs but lack PROGRESS.md and HANDOFF.md.

**Recommendation:** Mandate all 5 artifacts as a release gate. Create templates for PROGRESS.md and HANDOFF.md.

---

### 3.2 PostgreSQL Adapters

| Module | PostgreSQL Adapter | Migration Files | Auto-Migration |
|--------|-------------------|----------------|----------------|
| 01 | ❌ In-memory only | ❌ None | ❌ |
| 02 | ❌ In-memory only | ❌ None | ❌ |
| 03 | ✅ `repository/` package with passthrough wrappers | ✅ 15 CREATE TABLE statements | ❌ |
| 04 | ❌ In-memory only | ❌ None | ❌ |
| 05 | ❌ In-memory only | ❌ None | ❌ |

**Finding:** Only Module 03 has a PostgreSQL adapter (though in-memory is the default mode). All 5 modules default to in-memory stores. None auto-apply migrations on startup.

**Recommendation:** Module 03's PostgreSQL adapter pattern is the best. Use it as the template for all modules. Add auto-migration on startup.

---

### 3.3 Event Broker Wiring

| Module | Broker Interface | Real Broker Impl | Default Broker | Wired in main() |
|--------|-----------------|-----------------|---------------|----------------|
| 01 | ❌ None | ❌ | ❌ | ❌ |
| 02 | ❌ None | ❌ | ❌ | ❌ |
| 03 | ✅ `Broker` interface | ✅ KafkaBroker | InMemoryBroker | ❌ (uses InMemoryBroker) |
| 04 | ✅ `Producer` interface | ✅ KafkaProducer (stub) | logBroker | ⚠️ Config-based but stub only |
| 05 | ✅ `Broker` interface | ❌ None | logBroker | ❌ (uses logBroker) |

**Finding:** No module wires a real event broker. Module 03 has the closest implementation (KafkaBroker with SASL/TLS), but it's not wired in `main.go`. Module 05 defines a `Broker` interface but only uses a no-op `logBroker`.

---

## 4. Test Coverage Platform-Wide

### 4.1 Coverage Summary

| Module | Test Files | Total Tests | Handler Coverage | Store Coverage | Overall |
|--------|-----------|-------------|-----------------|---------------|---------|
| 01 | 2 | ~15 | 0% | 0% | ❌ FAIL |
| 02 | 10 | 148 | 16% | 45% | 72.6% |
| 03 | 9 | ~100 | 0% | 30% | ~30% |
| 04 | 10 | 148 | 60% | 75% | 72.6% |
| 05 | 6 | ~50+ | 50% | 70% | ~55% |

**Finding:** No module reaches the 80% threshold. Module 02 has the most tests (148) but 16% handler coverage. Module 04 has 148 tests and 60% handler coverage — the best handler coverage. Module 03 has the worst handler coverage (0%).

### 4.2 Missing Test Categories Across All Modules

- **No tenant-isolation tests** — critical for all modules (Modules 02 and 03 have active tenant bypasses)
- **No integration tests** — no Dockerfile-based tests, no testcontainers for any module
- **No cross-module tests** — no tests that verify module-to-module integration
- **No security tests** — no JWT forgery tests, no HMAC fallback bypass tests

---

## 5. PRD Scope Expansion Summary

| Module | PRD Base Endpoints | Implemented Endpoints | Expansion |
|--------|-------------------|----------------------|-----------|
| 01 | ~10 | 16 | +60% |
| 02 | ~4 | ~51 | +1175% |
| 03 | ~10+ | 54 | +400% |
| 04 | ~14 | 17 | +21% |
| 05 | ~10 | 15 | +50% |

**Finding:** All modules expanded beyond PRD scope without documented change requests. Module 02's expansion is the most extreme (+1175%) — adding ABAC, SCIM 2.0, MFA, LDAP/AD, delegations, and session replay on top of 4 base IAM endpoints.

**Recommendation:** Implement a change request process. Document scope expansions in PROGRESS.md with ARCH approval notes.

---

## 6. Consolidated Remediation Priorities

### P0: Platform-Wide Blockers (Fix All Modules First)

| # | Issue | Modules Affected | Action |
|---|-------|-----------------|--------|
| 1 | Replace hand-rolled JWT with `golang-jwt` | 01, 02, 03, 04, 05 | Shared Go package |
| 2 | Fail startup if JWT secret is default | 01, 02, 03, 04, 05 | Shared config package |
| 3 | Remove Module 02 HMAC fallback | 02 only | CODER fix |
| 4 | Standardize `TenantHeader` naming | Module 03 only | ARCH contract fix |
| 5 | Standardize `Error` schema (RFC 7807 + request_id) | All modules | ARCH contract fix |
| 6 | Establish shared broker abstraction (Kafka) | All modules | ARCH + CODER |
| 7 | Standardize AsyncAPI channel naming | Module 05 only | ARCH contract fix |
| 8 | Standardize `additionalProperties: false` | Modules 01–04 only | ARCH contract fix |

### P1: Module-Specific Blockers

See individual module remediation plans for P0 items specific to each module.

### P2: Platform Enhancements

| # | Issue | Modules Affected | Action |
|---|-------|-----------------|--------|
| 1 | Implement shared rate-limiting middleware | All modules | Shared middleware package |
| 2 | Implement shared JSON Schema validation middleware | All modules | Shared middleware package |
| 3 | Standardize PostgreSQL adapter pattern | Modules 01, 02, 04, 05 | Based on Module 03 |
| 4 | Create PROGRESS.md and HANDOFF.md templates | All modules | ARCH standard |
| 5 | Add PostgreSQL auto-migration | Modules 01, 02, 04, 05 | Based on Module 03 |

---

## 7. Recommended Implementation Sequence

1. **Week 1–2:** P0 platform-wide fixes (JWT standardization, HMAC removal, naming unification)
2. **Week 2–3:** Module 02 tenant isolation fixes (ABAC scoping, AuditStore enforcement) + Module 03 tenant bypass fixes
3. **Week 3–4:** Module 02 and 03 handler tests (most critical coverage gaps)
4. **Week 4–5:** Module 04 and 05 contract drift fixes + infrastructure creation
5. **Week 5–6:** Shared broker abstraction + Kafka wiring
6. **Week 6–7:** PostgreSQL adapters for Modules 01, 02, 04, 05
7. **Week 7–8:** Integration tests + security hardening (rate limiting, request validation)

**Total estimate: 8–10 weeks for production readiness across all 5 modules.**

---

## 8. Verdict by Module

| Module | Status | Can Deploy? | Next Review |
|--------|--------|-------------|-------------|
| 01 | REJECT | ❌ No | After P0 handler tests + contract fixes |
| 02 | REJECT | ❌ No | After tenant isolation + contract fixes |
| 03 | REJECT | ❌ No | After handler tests + tenant bypass fixes |
| 04 | CONDITIONAL | ⚠️ Dev only | After contract fixes + infrastructure |
| 05 | CONDITIONAL | ⚠️ Dev only | After event wiring + contract fixes |

**All modules require remediation before production deployment.**

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18
**Platform Lead Acknowledgment:** _________________    **Date:** _________________
