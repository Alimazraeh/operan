# ARCH — Module 05: Department Template Engine — Production Maturity Review

**Review Date:** 2026-06-18
**Reviewer:** ARCH (Platform Architect)
**Verdict:** **CONDITIONAL — Strong infrastructure and tenant isolation, but event gaps, missing HANDOFF/PROGRESS docs, and contract drift require remediation**

---

## 1. Summary Scorecard

| Category | Score | Notes |
|----------|-------|-------|
| Contract Compliance | ⚠️ PARTIAL | 15 operationIds; 8 AsyncAPI channels; 35 JSON Schema defs — required field mismatch, undeclared tags, channel naming drift |
| Test Coverage | ⚠️ PARTIAL | ~50+ tests across 6 test files — good store and handler coverage but error paths are thin |
| Integration | ❌ FAIL | In-memory broker only (log-only); no Kafka/AMQP/MQTT broker wired; protocol is MQTT (unique) |
| Infrastructure | ✅ GOOD | Dockerfile (multi-stage, non-root), Helm chart (complete), README.md comprehensive |
| Security | ⚠️ PARTIAL | Tenant isolation on all stores, HMAC-S256 JWT, no rate limiting, empty secret bypass |
| Database | ❌ FAIL | In-memory only — no PostgreSQL adapter |
| PRD Alignment | ⚠️ PARTIAL | 15 endpoints implemented vs PRD ~10-base; clone missing for custom templates |
| **Overall** | **CONDITIONAL** | **Stable for development/testing; not production-ready** |

---

## 2. Contract Drift Analysis

### 2.1 [CRITICAL] `TemplateDeployRequest` Required Field Mismatch

| Contract | Required Fields |
|----------|----------------|
| **OpenAPI** `TemplateDeployRequest` | `[version, environment]` |
| **JSON Schema** `TemplateDeployRequest` | `[template_id, version, environment]` |

**Impact:** An OpenAPI-valid request (without `template_id`) will be rejected by JSON Schema validators. The `template_id` is redundant since the template ID is already in the path parameter, but the inconsistency must be resolved.

**Remediation:** Remove `template_id` from JSON Schema's required fields (it's path-scoped, not body-scoped).

---

### 2.2 [CRITICAL] AsyncAPI Channel Naming Convention — Module 05 Uses Different Pattern

| Module | Channel Pattern | Example |
|--------|----------------|---------|
| **Modules 01–04** | `operan/events/{domain}/{resource}/{action}` | `operan/events/tenant/provisioned` |
| **Module 05** | `operan.templates.template.{action}` | `operan.templates.template.created` |

**Impact:** This is a **fundamental routing convention mismatch**. Message brokers using a unified topic hierarchy (e.g., `operan/events/*`) will not route Module 05 events correctly. Subscription patterns, ACLs, and monitoring dashboards built for the `operan/events/` prefix will silently drop Module 05 events.

**Remediation:** Standardize all modules to `operan/events/template/created` pattern. This requires a contract update for Module 05 and a migration plan for existing subscribers.

---

### 2.3 [HIGH] Undeclared Tags in OpenAPI

The OpenAPI contract defines operations with tags `Template Versions` and `Template Operations`, but these are **not declared** in the top-level `tags` list (which only has `Templates`, `Deployments`, `Custom Templates`).

**Impact:** OpenAPI validators will flag this as non-compliant. Documentation generators (Swagger UI, Redoc) will fail or produce incomplete output.

**Remediation:** Add `Template Versions` and `Template Operations` to the `tags` list.

---

### 2.4 [HIGH] AsyncAPI Protocol Fragmentation

| Module | Protocol | Broker |
|--------|----------|--------|
| 01 | Kafka | `events.operan.internal:9092` |
| 02 | AMQP | `mq.operan.internal` |
| 03 | Kafka | `kafka.prod.operan.io:9092` |
| 04 | TLS/TCP | `broker.operan.internal:443` |
| **05** | **MQTT** | **`events.operan.io`** |

**Impact:** The platform requires **four different message broker implementations** (Kafka, AMQP, TLS, MQTT) across just five modules. There is no unified broker abstraction at the platform level. This increases operational complexity, monitoring overhead, and cost.

**Remediation:** Establish a platform-level broker abstraction (see Module 03's broker pattern is closest). Standardize on a single protocol (recommend Kafka) for all modules.

---

### 2.5 [HIGH] Missing Events for Custom Template Lifecycle

The AsyncAPI contract defines 8 event channels, but **none correspond to custom template operations**:
- No `onCustomTemplateCreated`
- No `onCustomTemplateUpdated`
- No `onCustomTemplateDeleted`
- No `onCustomTemplateCloned`

The handler also does not publish any events for custom template CRUD operations.

**Remediation:** Add 4 new event channels to the AsyncAPI contract and wire event publishing in the custom template handlers.

---

### 2.6 [HIGH] Missing Clone Endpoint for Custom Templates

Standard templates have `cloneTemplate` (POST `/templates/{id}/clone`). Custom templates have no equivalent at `/templates/custom/{id}/clone`.

**Remediation:** Add clone endpoint for custom templates, or document why it is intentionally excluded.

---

### 2.7 [MEDIUM] Deployment Status Enum — AsyncAPI Missing `rolled_back`

| Contract | Enum Values |
|----------|-------------|
| **OpenAPI/JSON Schema** `deploymentStatus` | `select, configure, connect_data, provision_memory, deploy_swarm, operational, failed, rolled_back` |
| **AsyncAPI** `TemplateDeploymentFailedPayload.deployment_stage` | `select, configure, connect_data, provision_memory, deploy_swarm, operational` (no `rolled_back`) |

**Remediation:** Add `rolled_back` to the AsyncAPI enum, or clarify that `rolled_back` is a post-failure state that does not generate a separate event.

---

### 2.8 [MEDIUM] `CustomTemplate.status` Duplicates `agentStatus`

Both enums have the same values: `[draft, deprecated, published, archived]`. The `CustomTemplate` should reference `agentStatus` via `$ref` instead of redefining it inline.

---

### 2.9 [MEDIUM] Module 05 JSON Schema Pattern Differs from Modules 01–04

Module 05 extracts **16 enums into named definitions** (`templateCategory`, `agentStatus`, etc.) and uses `additionalProperties: false` on all schemas. Modules 01–04 JSON Schemas do neither — they define zero enums as named definitions and lack `additionalProperties: false`.

**Impact:** This is a structural inconsistency. If a code generator processes all module schemas, it will produce different output for Module 05 types vs the others.

**Remediation:** Either standardize on Module 05's approach (named enums + strict validation) for all modules, or document why Module 05 is intentionally different.

---

### 2.10 [MEDIUM] Missing AsyncAPI Event Channels

| AsyncAPI Channel | Handler Calls It? |
|------------------|------------------|
| `onTemplateCreated` | ✅ `PublishTemplateCreated` called in handler |
| `onTemplateUpdated` | ✅ `PublishTemplateUpdated` called in handler |
| `onTemplateDeleted` | ✅ `PublishTemplateDeleted` called in handler |
| `onTemplateDeployed` | ✅ `PublishTemplateDeployed` called in handler |
| `onTemplateDeploymentFailed` | ✅ `PublishTemplateDeploymentFailed` called in handler |
| `onTemplateUndeployed` | ❌ `PublishTemplateUndeployed` defined but **never called** |
| `onTemplateVersioned` | ❌ `PublishTemplateVersioned` defined but **never called** |
| `onTemplateCloned` | ✅ `PublishTemplateCloned` called in handler |

**Impact:** The contract promises `undeployed` and `versioned` events but the code never produces them. Consumers subscribing to these channels will wait indefinitely for messages that never arrive.

**Remediation:** Wire event publishing for versioned (triggered by version creation) and undeployed (triggered by deployment deletion) events, or remove these channels from the AsyncAPI contract.

---

### 2.11 [MEDIUM] Redundant Security Declarations

Every endpoint redundantly declares both `BearerAuth` and `TenantHeader` at the individual operation level, even though they are already set at the top-level `security` block.

**Remediation:** Remove per-operation security declarations and rely on the global security block.

---

## 3. Security Assessment

### 3.1 [MEDIUM] Hardcoded Default JWT Secret

`internal/config/config.go` defaults `JWTSecret` to `"change-me-in-production"`. The `Config.Validate()` method checks for this, but the middleware (`validateJWT`) would accept an empty secret string, which is a weaker attack surface.

**Impact:** If the environment variable is not set and validation passes (due to a code path), or if the config allows an empty secret, JWT validation will use a blank secret — trivially forgeable.

**Remediation:** Fail hard on startup. Reject empty secrets in config validation.

---

### 3.2 [MEDIUM] No Rate Limiting

No middleware exists for rate limiting. Template operations (especially clone and deploy) can be resource-intensive — a malicious client could flood these endpoints.

**Remediation:** Implement rate-limiting middleware with per-tenant limits.

---

### 3.3 [MEDIUM] Clone Error Swallowing

`handlers/nested.go` — `handleClone` ignores the error from `json.Unmarshal`:

```go
_ = json.Unmarshal(body, &req) // error ignored
```

A malformed clone body silently uses zero values, potentially creating an empty clone or causing a panic.

**Remediation:** Return a 400 Bad Request with the unmarshaling error.

---

### 3.4 [LOW] Dead `Update` Method in TemplateStore

`store/templates.go`'s `Update` method calls `UpdateByTenant` with an empty tenant ID. Since `UpdateByTenant` checks the tenant ID, this will always return `ErrNotFound`. The method is effectively dead code.

**Remediation:** Either remove `Update` or make it verify the resource's tenant ID matches the stored record's tenant.

---

### 3.5 [LOW] No JSON Schema Request Validation

No middleware validates incoming request bodies against JSON Schema. The handlers simply decode JSON into structs.

**Remediation:** Add a JSON Schema validation middleware (similar to what Module 04's blueprint specifies).

---

## 4. Test Coverage Analysis

### 4.1 Coverage Summary

| Package | Test Files | Tests | Status |
|---------|-----------|-------|--------|
| `internal/handlers` | 1 | ~30+ | ⚠️ PARTIAL — main paths covered, error paths thin |
| `internal/store` | 1 | ~20+ | ✅ GOOD — CRUD, tenant isolation, pagination tested |
| `internal/middleware` | 1 | ~15+ | ✅ GOOD — JWT auth, tenant context, request/trace ID |
| `internal/events` | 1 | ~10+ | ✅ GOOD — all publish methods + broker swap |
| `internal/config` | 1 | ~8+ | ✅ GOOD — defaults, env vars, validation |
| `internal/ctxkeys` | 1 | ~5+ | ✅ GOOD |
| `main.go` | 0 | 0 | ❌ FAIL |

**Missing:**
- No integration tests (no database, no testcontainers)
- No handler error-path tests for nested.go
- No tenant-isolation tests for nested operations (clone, deploy)
- No custom-template handler tests

### 4.2 Handler Coverage Gaps

- `nested.go` has 7+ operations handled by a single function — only the happy paths are tested
- The `method-not-allowed` default path in nested.go (line 334–341) has **zero** test coverage
- Clone error swallowing path is untested
- Deploy failure and rollback paths are not tested

### 4.3 Store Coverage Gaps

- No concurrency/lock tests
- No data corruption tests (concurrent creates with same UUID should fail)
- No pagination boundary tests (page_size > total, page=0)

---

## 5. Infrastructure & Deployment

### 5.1 Infrastructure Status

| Artifact | Status |
|----------|--------|
| `Dockerfile` | ✅ Multi-stage build, non-root user `operan`, health check |
| `chart/Chart.yaml` | ✅ Present |
| `chart/templates/` | ✅ Complete (deployment, service, ingress, HPA, serviceaccount) |
| `chart/values.yaml` | ✅ Present (env vars, probes, autoscaling) |
| `README.md` | ✅ Comprehensive documentation |
| `manifest.json` | ✅ Present (module_id, dependencies, events, env vars, security) |
| `PROGRESS.md` | ❌ Missing |
| `HANDOFF.md` | ❌ Missing |

Module 05 has the **best infrastructure tooling** of all modules reviewed so far. The Dockerfile follows multi-stage best practices with a non-root user and health check. The Helm chart includes HPA and ingress. The README is comprehensive.

---

## 6. PRD Alignment

### 6.1 Scope

15 endpoints implemented vs the PRD's expected ~10 base endpoints:
- Template CRUD (5)
- Deployments (2)
- Custom template CRUD (5)
- Versions (2)
- Clone (1)

### 6.2 Unimplemented Features

| PRD Feature | Status |
|------------|--------|
| Template CRUD (standard) | ✅ Full |
| Template deploy lifecycle | ✅ Full (select → configure → connect_data → provision_memory → deploy_swarm → operational/failed/rolled_back) |
| Custom template CRUD | ✅ Full |
| Versioning | ✅ Full |
| Clone | ⚠️ Partial (standard templates only, not custom) |
| Governance rule management | ⚠️ Schema exists but no dedicated endpoint (stored inline in template) |
| KPI definition management | ⚠️ Schema exists but no dedicated endpoint |
| Integration management | ⚠️ Schema exists but no dedicated endpoint |
| Operational policy management | ⚠️ Schema exists but no dedicated endpoint |
| Template search | ❌ Not implemented (no search endpoint for templates) |
| Template import/export | ❌ Not implemented |

---

## 7. Implementation Quality Observations

### 7.1 Strengths

- **Tenant isolation** — All stores use `GetByIDAndTenant` and `UpdateByTenant` — correct pattern
- **Deep copies on read** — Store read operations return deep copies to prevent mutation
- **Rich domain model** — Template, CustomTemplate, Version, Deployment models are comprehensive
- **Multi-stage deployment lifecycle** — 8-stage deployment flow is well-structured
- **Clean DTO pattern** — Request/Response types are separate from store models
- **Event publishing** — Most lifecycle events are wired and published

### 7.2 Issues

| # | Location | Issue | Severity |
|---|----------|-------|----------|
| 1 | `handlers/nested.go` | Mega-handler (380+ lines, 7+ operations) | MEDIUM |
| 2 | `handlers/nested.go` | Clone error swallowing | MEDIUM |
| 3 | `store/templates.go` | Dead `Update` method | LOW |
| 4 | `store/templates.go` | Comment says `Update` does not verify tenant | LOW |
| 5 | `events/events.go` | No events for custom template lifecycle | HIGH |
| 6 | `events/events.go` | `PublishTemplateUndeployed` defined but never called | MEDIUM |
| 7 | `events/events.go` | `PublishTemplateVersioned` defined but never called | MEDIUM |
| 8 | `handlers/router.go` | All nested ops go through single POST route | LOW (known limitation) |

---

## 8. Remediation Plan

### Phase 1: Critical Blockers (Must Fix Before Re-Review)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 1 | Fix `TemplateDeployRequest` required field mismatch (`template_id` in JSON Schema only) | ARCH → CODER | P0 |
| 2 | Reconcile AsyncAPI channel naming convention with modules 01–04 | ARCH → ALL | P0 |
| 3 | Add undeclared tags (`Template Versions`, `Template Operations`) to OpenAPI | ARCH → CODER | P0 |
| 4 | Wire `PublishTemplateVersioned` and `PublishTemplateUndeployed` in handlers | CODER | P0 |
| 5 | Create PROGRESS.md and HANDOFF.md | CODER | P0 |
| 6 | Add async events for custom template lifecycle (4 new channels) | ARCH → CODER | P0 |

### Phase 2: High-Priority Gaps (Fix Before Production Sign-Off)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 7 | Add clone endpoint for custom templates | CODER | P1 |
| 8 | Standardize event broker protocol (recommend Kafka) | ARCH → ALL | P1 |
| 9 | Add handler error-path tests for nested.go (clone, deploy, method-not-allowed) | CODER | P1 |
| 10 | Add tenant-isolation tests for nested operations | CODER | P1 |
| 11 | Implement PostgreSQL adapter (same pattern as Module 03) | CODER | P1 |
| 12 | Add rate-limiting middleware | CODER | P1 |
| 13 | Fix JWT config to reject empty secrets | CODER | P1 |
| 14 | Fix clone error swallowing | CODER | P1 |
| 15 | Remove dead `Update` method from TemplateStore | CODER | P1 |
| 16 | Add `/ready` endpoint with dependency checks | CODER | P1 |

### Phase 3: Medium-Priority Enhancements

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 17 | Standardize enum definition pattern across all modules | ARCH → ALL | P2 |
| 18 | Add `additionalProperties: false` to Modules 01–04 JSON Schemas | ARCH → ALL | P2 |
| 19 | Implement template search endpoint | CODER | P2 |
| 20 | Add integration tests with testcontainers | CODER | P2 |
| 21 | Add OpenTelemetry instrumentation | CODER | P2 |
| 22 | Add PostgreSQL auto-migration on startup | CODER | P2 |

---

## 9. Developer Sign-Off Checklist

Before resubmitting Module 05 for re-review, the CODER team must complete AND verify each item below:

- [ ] **P0-1:** `TemplateDeployRequest` required fields reconciled across OpenAPI and JSON Schema
- [ ] **P0-2:** AsyncAPI channel naming standardized with modules 01–04
- [ ] **P0-3:** Undeclared tags added to OpenAPI
- [ ] **P0-4:** `PublishTemplateVersioned` and `PublishTemplateUndeployed` wired in handlers
- [ ] **P0-5:** `PROGRESS.md` and `HANDOFF.md` created
- [ ] **P0-6:** Custom template lifecycle events added (4 new channels + handler wiring)
- [ ] **P1-7 through P1-16:** Phase 2 remediation items completed
- [ ] **P2-17 through P2-22:** Phase 3 items completed (for production readiness)

---

## 10. Architect's Note

Module 05 is the **most infrastructure-complete** module reviewed so far. The Dockerfile, Helm chart, and README are all production-quality — far ahead of Modules 02, 03, and 04. The tenant isolation on all stores is correctly implemented, and the domain model is rich and well-structured.

The most surprising finding is the **AsyncAPI protocol fragmentation** — Module 05 uses MQTT while Modules 01 and 03 use Kafka, Module 02 uses AMQP, and Module 04 uses TLS/TCP. This is the worst platform-level integration issue identified across all five modules reviewed. There needs to be a **platform-level broker abstraction** that all modules use, with a single protocol (recommend Kafka) and module-specific topic prefixes.

The **channel naming convention drift** (dots vs slashes, `events/` prefix) is equally alarming. A broker administrator setting up topic hierarchies will find Module 05 events completely non-routable alongside the others.

The **missing event wiring** (versioned, undeployed) and **absent custom template events** are developer oversights — the event types are defined in the AsyncAPI contract, but no handler calls the corresponding publish methods. This is a common pattern across all modules reviewed (async events defined but not always wired).

**Remediation effort estimate:** **2–3 developer days for P0 items** (contract fixes and event wiring), **4–6 days for P1 items** (PostgreSQL adapter, tests, security hardening).

Module 05 is the **closest to production** of all modules reviewed so far, thanks to its infrastructure completeness and correct tenant isolation. With contract reconciliation and event wiring, it could be deployed for internal testing.

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18
**CODER Acknowledgment:** _________________    **Date:** _________________
