# Wave 2 Integration — Remediation Plan

**Date:** 2026-05-28
**Author:** ARCH (Architectural Review)
**Scope:** Operan ADOS — Modules 01-04 (implemented), 05-20 (contract-only)
**Goal:** Bring implemented modules from current state to APPROVED level before Wave 2 release

---

## Executive Summary

All 4 implemented modules (01-04) compile and pass tests. Only **Module 04** has completed architectural review and holds an APPROVED verdict.

**Critical gaps blocking Wave 2 release:**
- Modules 01-03 have never undergone architectural review
- Average test coverage for 01-03 is **30.5%** (Module 04 approved at 72.6%)
- Core security, billing, and orchestration paths are untested in 01-03
- No automated coverage tracking (manifest.json) for 01-03

---

## 1. Module 01 — Tenant Control Plane

**Coverage:** 30.6% | **Tests:** 199 | **Review:** Not done

### Gaps

| Priority | Issue | Impact |
|----------|-------|--------|
| 🔴 Critical | `config/` and `events/` at 0% coverage | Config loading and event publishing untested |
| 🔴 Critical | `handler_billing.go` — all 10 functions at 0% | No billing write operations implemented or tested |
| 🔴 Critical | `handler/` overall at 21.0% | Core tenant CRUD tested but billing/quota/payment methods not |
| 🟡 Medium | `store/` at 42.9% | Many store methods have no test coverage |

### PRD Compliance

- Overall: **62.5%**
- Missing: Deployment manager, billing write operations

### Required Remediation

| Action | Effort |
|--------|--------|
| Add config tests (defaults, env vars, validation) — 7 tests | ~30 min |
| Add events tests (publisher, 4 publish methods) — 8 tests | ~30 min |
| Implement billing handler stubs from OpenAPI contract | ~2 days |
| Add handler tests for all implemented routes | ~1 day |
| Add store tests for uncovered methods | ~1 day |
| Architectural review (3-round process) | 2-3 days |

**Estimated total: 5-6 days**

---

## 2. Module 02 — Identity & Access

**Coverage:** 16.7% | **Tests:** 173 | **Review:** Not done

### Gaps

| Priority | Issue | Impact |
|----------|-------|--------|
| 🔴 Critical | Overall coverage only 16.7% | Core auth/billing logic largely untested |
| 🔴 Critical | `handler/` at 14.2% | User management, SSO, MFA, RBAC mostly untested |
| 🔴 Critical | `middleware/` at 30.7% | JWT validation, auth flow not fully tested |
| 🔴 Critical | `auth/client.go` at 0% | Authentik integration untested |
| 🔴 Critical | `config/` and `events/` at 0% | Config loading and identity events untested |
| 🟡 Medium | Session replay sanitization gap | PRD §9 partial compliance |

### PRD Compliance

- Overall: **~85%**
- Missing: Session replay sanitization

### Required Remediation

| Action | Effort |
|--------|--------|
| Add config tests (defaults, env vars, validation) — 7 tests | ~30 min |
| Add events tests (publisher, 4 publish methods) — 8 tests | ~30 min |
| Add middleware tests (JWTAuth, ExtractTenant, etc.) — 15 tests | ~2 hours |
| Add handler tests for all routes — 30+ tests | ~2 days |
| Add authentik client tests | ~4 hours |
| Fix session replay sanitization | ~1 hour |
| Architectural review (3-round process) | 3-4 days |

**Estimated total: 7-9 days**

---

## 3. Module 03 — Agent Orchestration

**Coverage:** 34.3% | **Tests:** 359 | **Review:** Not done

### Gaps

| Priority | Issue | Impact |
|----------|-------|--------|
| 🔴 Critical | `middleware/` JWTAuth at 0% | Security-critical auth path completely untested |
| 🔴 Critical | `store/pipeline_execution.go` at 0% | Core orchestration pipeline store untested |
| 🔴 Critical | `store/delegation.go` at 0% | Delegation management untested |
| 🔴 Critical | `middleware/` at 35.4% | JWT validation, tenant/user context extraction untested |
| 🟡 Medium | `events/` at 45.6% | Broker error path untested |
| 🟡 Medium | `store/workflow.go` — `UpdateCurrentNodes`, `AddVariable`, `SetVariables` at 0% | Workflow variable management untested |
| 🟡 Medium | `repository/` and `database/` at 0% | PostgreSQL adapters untested |

### PRD Compliance

- Overall: **~70%**
- Missing: Delegation API, distributed execution, some pipeline operations

### Required Remediation

| Action | Effort |
|--------|--------|
| Add middleware tests (JWTAuth, ExtractTenant, etc.) — 15 tests | ~2 hours |
| Add pipeline store tests (Create, GetByID, Update, List, AddStep, GetSteps) — 15 tests | ~3 hours |
| Add delegation store tests — 10 tests | ~1 hour |
| Add events broker error path tests — 3 tests | ~30 min |
| Add workflow store tests for variable methods | ~1 hour |
| Implement delegation handler from OpenAPI contract | ~1 day |
| Add handler tests for untested routes | ~1 day |
| Architectural review (3-round process) | 3-4 days |

**Estimated total: 8-10 days**

---

## 4. Module 04 — Agent Registry (APPROVED ✅)

**Coverage:** 72.6% | **Tests:** 148 | **Verdict:** APPROVED

### Status: No action required

This module has passed all 3 architectural review rounds, has 148 tests across 8 packages, and config/events are at 100% coverage. Minor residual gaps (handlers at 58.5%, store at 77.7%) are non-blocking.

**Residual gaps (nice-to-have, not blocking release):**

| Issue | Impact |
|-------|--------|
| Handlers: 58.5% | `GetAgentVersion` untested — add 1 test |
| Store: 77.7% | `Delete`, `Exists` methods untested — add 3 tests |

---

## 5. Common Actions (Apply to All 01-03)

| Action | Target | Effort |
|--------|--------|--------|
| Add `manifest.json` with coverage tracking | All modules | ~15 min each |
| Add `config/` tests | All 3 modules | ~30 min each |
| Add `events/` tests | All 3 modules | ~30 min each |
| Add `middleware/` JWTAuth tests | Modules 02, 03 | ~2 hours each |

---

## 6. Contract-Only Modules (05-20)

No action needed for implementation. All 16 modules have complete contracts:
- **OpenAPI** YAML (REST endpoints)
- **AsyncAPI** YAML (event schemas)
- **JSON Schema** (request/response payloads)
- Integration graph edges defined

Wave 2 release will ship these as contracts. Wave 3+ implementation follows when dev capacity allows.

---

## 7. Recommended Execution Order

| Wave | Modules | Priority | Total Effort |
|------|---------|----------|-------------|
| **Wave 2A** | Module 04 (APPROVED) | Ship now | ✅ Ready |
| **Wave 2B** | Module 01 | High — foundation module | 5-6 days |
| **Wave 2C** | Module 02 | High — security-critical | 7-9 days |
| **Wave 2D** | Module 03 | High — core orchestration | 8-10 days |
| Wave 3 | Modules 05-20 | Implementation phase | TBD |

---

## 8. Acceptance Criteria for Each Module Before APPROVE

Each module must achieve:

| Criterion | Target |
|-----------|--------|
| All tests pass | 100% |
| Overall coverage | ≥ 70% (Module 04 baseline) |
| Config/events tested | 100% |
| Middleware tested | 100% (especially JWTAuth) |
| PRD compliance | ≥ 85% |
| Contract compliance | ≥ 95% |
| Architectural review | CONDITIONAL → APPROVE |
| manifest.json | Present with accurate coverage |

---

## 9. Current Metrics Summary

| Metric | Module 01 | Module 02 | Module 03 | Module 04 |
|--------|-----------|-----------|-----------|-----------|
| Go files | 32 | 38 | 42 | 24 |
| Tests | 199 | 173 | 359 | 148 |
| Coverage | 30.6% | 16.7% | 34.3% | 72.6% |
| Config | 0% | 0% | 69.7% | 100% |
| Events | 0% | 0% | 45.6% | 96% |
| Middleware | 97.1% | 30.7% | 35.4% | 83.8% |
| Handlers | 21.0% | 14.2% | 63.9% | 58.5% |
| Store | 42.9% | 39.3% | 45.6% | 77.7% |
| Review status | ❌ Not done | ❌ Not done | ❌ Not done | ✅ APPROVED |
| Manifest.json | ❌ Missing | ❌ Missing | ❌ Missing | ✅ Present |
