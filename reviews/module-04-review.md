---
name: Module 04 architecture review — all findings
description: Comprehensive architectural review of Module 04 Agent Registry against PRD, contracts, and implementation
type: project
---

# Module 04 Agent Registry — Architectural Review

**Review Date:** 2026-05-28
**Reviewer:** Architect (AI Architect & Contract Manager)
**Status:** REJECT — 4 Critical, 8 High, 6 Medium, 3 Low issues found

---

## PRD Compliance Summary

Per PRD Section 5 (Module 04 — Agent Registry), the following capabilities are required:

| PRD Capability | Status | Gap |
|----------------|--------|-----|
| Agent versioning | ✅ | Fully implemented |
| Capability indexing | ⚠️ Partial | Structured in contracts, stored as `[]string` in impl |
| Permissions | ⚠️ Partial | Tenant isolation only; no RBAC/ABAC (Module 02) |
| Dependency management | ⚠️ Partial | CRUD only; no DAG resolution, no cycle detection |
| Runtime constraints | ⚠️ Partial | Stored but not enforced at runtime |
| Cost profiles | ⚠️ Partial | Stored but Module 17 integration missing |

PRD Section 8 (Agent Object Model) fields missing from implementation:
- `objectives` — array of Objective objects
- `supported_languages` — array of strings (Module 19)
- `current_version_id` — reference to active version
- `access_control` — object for per-agent ACL config

---

## BLOCKER-1: DependencyType Enum Mismatch [CRITICAL]

**Location:** `schema-04-agent-registry.json` vs `openapi-04-agent-registry.yaml` vs `asyncapi-04-agent-registry.yaml` vs Go code

| Contract | Enum Values |
|----------|-------------|
| OpenAPI | `[hard, soft, optional]` |
| JSON Schema | `[soft, optional]` — **MISSING `hard`** |
| AsyncAPI | `[soft, optional]` — **MISSING `hard`** |
| Go store | `[hard, soft, optional]` |

**Why:** Harmonization standard requires all three contracts agree. Go code is correct; JSON Schema and AsyncAPI are missing `hard`.

**How to fix:**
- Add `"hard"` to `DependencyType` enum in `schema-04-agent-registry.json`
- Add `"hard"` to `DependencyTypeEnum` in `asyncapi-04-agent-registry.yaml`

---

## BLOCKER-2: Search Agents Tenant Context Bypass [CRITICAL]

**Location:** `internal/handlers/agent_registry.go` — `SearchAgents` handler

The `SearchAgents` handler reads `tenant_id` from the request body instead of enforcing it from the middleware-set context. While `ExtractTenant` is wrapped on the route, the handler ignores it and uses `req.TenantID` from the body. This allows a tenant to search another tenant's agents by sending their tenant_id in the body — the handler never cross-checks against the middleware-extracted tenant.

**Why:** Multi-tenant isolation per PRD Section 9 and platform principle 4.2 is mandatory.

**How to fix:**
- Change `SearchAgents` to read `tenantID` from `r.Context().Value(middleware.TenantContextKey{}).(string)`
- Remove `tenant_id` from the request body struct (or keep as alias but validate against context)

---

## BLOCKER-3: Routes Missing Tenant Extraction [CRITICAL]

**Location:** `internal/handlers/router.go`

The following routes do NOT wrap handlers with `ExtractTenant`:

| Route | Handler |
|-------|---------|
| `GET /v1/agents/` | `AgentByID` |
| `PATCH /v1/agents/` | `AgentByID` |
| `GET /v1/agents/*/versions/` | `VersionByID` |
| `PATCH /v1/agents/*/versions/` | `VersionByID` |

Additionally, `AgentByID` internal dispatchers (`GetAgent`, `UpdateAgent`, `DeleteAgent`, `ListAgentCapabilities`, `ListAgentVersions`, `ListDependencies`) extract tenant from context in some cases but NOT in the sub-dispatch path. They read `r.Context().Value(middleware.TenantContextKey{})` which won't be set if the route didn't apply `ExtractTenant`.

**Why:** Every mutable or query path must enforce tenant context. Without `ExtractTenant`, these paths accept any request without tenant validation.

**How to fix:**
- Wrap `AgentByID` and `VersionByID` with `middleware.ExtractTenant()` in `RegisterRoutes`
- Alternatively, have `AgentByID` and `VersionByID` extract tenant themselves

---

## BLOCKER-4: No Event Publishing [CRITICAL]

**Location:** `internal/events/agent_registry.go` — event structs exist but are NEVER published

The AsyncAPI contract defines 8 channels:
1. `agentRegistered` — No Go struct (`AgentCreated` exists but name mismatch)
2. `agentCapabilitiesUpdated` — No Go struct
3. `agentVersionCreated` — `AgentVersionCreated` exists
4. `agentPromoted` — `AgentVersionPromoted` exists but name mismatch
5. `agentDeprecated` — **No Go struct**
6. `agentArchived` — **No Go struct**
7. `dependencyAdded` — **No Go struct**
8. `dependencyRemoved` — **No Go struct**

**Why:** The PRD requires event-driven module communication. Module 04 events enable Module 03 orchestration to know about agent lifecycle changes.

**How to fix:**
- Add missing event structs: `AgentDeprecated`, `AgentArchived`, `DependencyAdded`, `DependencyRemoved`, `AgentCapabilitiesUpdated`
- Align struct names with AsyncAPI operationIds: `AgentRegistered`, `AgentPromoted`
- Implement `Publisher` interface with real Kafka/AMQP (following Module 03 pattern from `events/events.go`)
- Wire `Publish` calls into handler lifecycle methods (CreateAgent → AgentRegistered, etc.)

---

## HIGH-1: OpenAPI Base Path Missing `/registry` Prefix [HIGH]

**Location:** `openapi-04-agent-registry.yaml`

The architecture doc specifies base path `/api/v1/registry`, but the OpenAPI contract uses `/agents` paths. All operations are under `/agents` without a `/registry` namespace prefix.

**Why:** Platform routing convention per integration-graph.yaml uses `/api/v1/` prefix. Without `/registry`, paths collide with any other module using `/agents`.

**How to fix:**
- Prepend `/registry` to all path definitions in `openapi-04-agent-registry.yaml`
- Update server URL to `https://api.operan.io/v1/registry`
- Align implementation handlers to use `/v1/registry/agents` paths

---

## HIGH-2: MemoryAccess Structure Mismatch [HIGH]

**Location:** OpenAPI `MemoryAccess` schema vs Go `Agent.MemoryAccess []string`

OpenAPI and JSON Schema define `MemoryAccess` as a structured object with `scope`, `isolated_stores`, `allowed_types`, and `isolation_level` fields (mapping to PRD Section 9 multi-tenant isolation levels).

The Go implementation stores `MemoryAccess` as `[]string`.

**Why:** Structured memory access config is required for Module 07 Memory Fabric integration and proper isolation enforcement.

**How to fix:**
- Change `Agent.MemoryAccess` in Go to `*MemoryAccessConfig` (structured type)
- Add corresponding schema to contracts if not already unified

---

## HIGH-3: CostProfile Structure Mismatch [HIGH]

**Location:** `openapi-04-agent-registry.yaml` `CostProfile` vs Go `CostProfile` struct

OpenAPI defines 6 fields: `cost_per_execution`, `cost_per_token`, `estimated_monthly_cost`, `budget_limit`, `throttle_threshold`, `billing_tag`.

Go code defines 3 fields: `EstimatedCostPerRun`, `Currency`, `CostUnit`.

**Why:** Module 17 Cost Governance Engine integration requires the full 6-field profile.

**How to fix:**
- Align Go `CostProfile` struct to match OpenAPI schema (6 fields)
- Update DTO conversion functions

---

## HIGH-4: Dependency Request Schema Mismatch [HIGH]

**Location:** `openapi-04-agent-registry.yaml` `DependencyRequest` vs handler `AddDependency`

OpenAPI `DependencyRequest` requires `dependency_id` and has optional `dependency_type`, `version_constraint`, `description`.

Handler reads body with `AgentID`, `DependencyType`, `VersionConstraint` — missing `dependency_id`, `description`, and `DependencyRequest` is not referenced directly.

**Why:** Contract mismatch causes validation failures for clients using the OpenAPI spec.

**How to fix:**
- Align handler request struct with `DependencyRequest` schema

---

## HIGH-5: Missing Fields in Agent CRUD DTOs [HIGH]

**Location:** `AgentDTO`, `CreateAgentRequest` handler struct, `UpdateAgent` handler struct

The following fields exist in OpenAPI/JSON Schema but are absent from handler DTOs:

| Field | In OpenAPI/Schema | In Handler DTO |
|-------|-------------------|----------------|
| `objectives` | ✅ | ❌ |
| `supported_languages` | ✅ | ❌ |
| `current_version_id` | ✅ | ❌ |
| `access_control` | ✅ | Partial (map) |
| `description` (Agent object) | ✅ | ❌ |
| `department_id` | ✅ | ❌ |

**Why:** These are PRD Section 8 required fields for the Agent Object Model.

**How to fix:**
- Add missing fields to `AgentDTO` and handler request structs
- Wire through to/from store types

---

## HIGH-6: Route Shadowing Risk in Go ServeMux [HIGH]

**Location:** `internal/handlers/router.go`

```go
mux.HandleFunc("GET /v1/agents/", h.AgentByID)
mux.HandleFunc("GET /v1/agents/*/capabilities", middleware.ExtractTenant(h.ListAgentCapabilities))
```

Go's `http.ServeMux` (1.22+) matches routes using a tree algorithm. The pattern `GET /v1/agents/` has lower specificity than `GET /v1/agents/*/capabilities`, so the more specific route should take priority. However, the `AgentByID` handler uses string manipulation (`strings.TrimPrefix`) to extract IDs, which means it receives ALL requests to `/v1/agents/*` paths regardless of the sub-path. This creates confusion and potential for routing errors if new routes are added.

**Why:** Go ServeMux route matching changed in 1.22. Trailing-slash-only patterns match both `/v1/agents/` AND `/v1/agents/anything`. Combined with the manual path splitting in `AgentByID`, this creates fragile routing.

**How to fix:**
- Use Go 1.22+ `*` wildcard patterns for ALL agent sub-paths: `GET /v1/agents/*/`, `PATCH /v1/agents/*/`, `DELETE /v1/agents/*/`
- Remove string-path-dispatching from `AgentByID` and `VersionByID`
- Register separate route entries for each sub-path (capabilities, versions, dependencies)

---

## HIGH-7: No Middleware Chain for JWT Validation [HIGH]

**Location:** `internal/middleware/tenant.go`, `main.go`

The platform requires JWT validation via `BearerAuth` (per OpenAPI security scheme). Module 02 has `AuthValidator` with RSA/JWKS + HMAC. Module 04 only has `ExtractTenant` and `ExtractUserID` — no JWT signature validation, no JWKS fetching, no token expiry checking.

**Why:** PRD Section 10 (Security Requirements) mandates zero-trust, encrypted auth. Without JWT validation, any string can be passed as a user ID.

**How to fix:**
- Either import Module 02's `AuthValidator` middleware (via shared `pkg/auth` package) or create a local JWT validator
- Add JWT validation to the middleware chain before tenant extraction
- Follow Module 02 pattern: `JWTAuth → TenantContext → TraceID → Handler`

---

## MEDIUM-1: Config Struct Defined But Never Wired [MEDIUM]

**Location:** `internal/config/config.go` vs `main.go`

Config struct has `Port`, `DatabaseDSN`, `ModuleID` but `main.go` hardcodes port `:8083` and never instantiates `config.Config`. The `LoadEnv()` method has a dead code path (inner `if p != ""` with no parsing).

**Why:** Architecture doc requires 10+ env vars for DB, Cache, Broker, JWT, OTLP, Search, Rate Limits.

**How to fix:**
- Wire `config.DefaultConfig()` + `config.LoadEnv()` + `config.Validate()` into `main.go`
- Fix port parsing (missing `strconv` import)
- Expand config to match architecture doc requirements

---

## MEDIUM-2: No Store Tenant Isolation for Version/Capability/Dependency Stores [MEDIUM]

**Location:** `internal/store/agent_registry.go`

`AgentStore` has tenant-scoped `byTenant` index for O(1) tenant isolation. `VersionStore`, `CapabilityStore`, and `DependencyStore` have NO tenant field or tenant isolation.

**Why:** PRD Section 9 requires data isolation per tenant. Versions, capabilities, and dependencies belong to agents within tenants — they should be tenant-scoped.

**How to fix:**
- Add `TenantID` field to `AgentVersion`, `CapabilityEntry`, `AgentDependency`
- Add tenant-scoped indexes to each store
- Update all CRUD operations to filter/validate by tenant

---

## MEDIUM-3: Dependency Type Values Mismatch in Tests [MEDIUM]

**Location:** `internal/handlers/agent_registry_test.go` — `TestAddDependency_Success` and `internal/store/dependency_test.go`

Test uses `DependencyType: "direct"` but the OpenAPI/Go enum only allows `[hard, soft, optional]`.

**Why:** Tests should validate against the correct enum, not arbitrary values.

**How to fix:**
- Change `"direct"` to `"hard"` in tests

---

## MEDIUM-4: Delete vs Deprecate Confusion [MEDIUM]

**Location:** `internal/handlers/agent_registry.go` — `DeleteAgent`

`DeleteAgent` sets status to `deprecated` instead of performing a hard delete. The OpenAPI contract says "Deprecate an agent" which is correct, but the handler name `DeleteAgent` is misleading — should be `DeprecateAgent`.

**Why:** Developer mental model should match contract semantics.

**How to fix:**
- Rename `DeleteAgent` to `DeprecateAgent`
- Update test names accordingly

---

## MEDIUM-5: JSON Schema Missing `agent_id` as Required on Agent [MEDIUM]

**Location:** `schema-04-agent-registry.json` — `Agent` definition

JSON Schema requires `[id, name, role, tenant_id, status]` but OpenAPI requires `[id, name, role, tenant_id, status, capabilities, tools, created_at]`.

**Why:** Cross-spec harmonization standard requires aligned required fields.

**How to fix:**
- Add `capabilities`, `tools`, `created_at` to JSON Schema Agent required array

---

## MEDIUM-6: Missing `*List` Schemas in JSON Schema [MEDIUM]

**Location:** `schema-04-agent-registry.json`

Missing schemas that should exist per platform standards:
- `AgentList` (replaces `AgentListResponse`)
- `VersionList` — defined but uses different structure than OpenAPI's `VersionList`
- `DependencyList` — defined but missing from JSON Schema

**Why:** Platform standards require consistent `*List` schemas.

**How to fix:**
- Add missing list schemas, align structures with OpenAPI

---

## LOW-1: JSON Schema `DependencyType` Description Outdated [LOW]

**Location:** `schema-04-agent-registry.json`

Description says "Harmonized: soft, optional" but should say "Harmonized: hard, soft, optional".

**Why:** Documentation accuracy.

**How to fix:**
- Update description text

---

## LOW-2: No Structured Logging / Trace ID [LOW]

**Location:** `internal/middleware/tenant.go`, `main.go`

No `TraceID` or `RequestID` middleware. No structured JSON logging.

**Why:** Architecture doc requires OpenTelemetry traces, structured JSON logs with `trace_id`, `request_id`, `tenant_id`.

**How to fix:**
- Add `TraceID` and `RequestID` middleware (follow Module 03 pattern)
- Add structured JSON logger middleware

---

## LOW-3: Missing `GET /agents/search` Tenant Context Enforcement [LOW]

**Location:** `router.go` — `POST /v1/agents/search`

The search route wraps with `ExtractTenant` but the handler ignores it (see BLOCKER-2). This is a medium-severity issue but the handler-level fix is already noted as BLOCKER-2.

**How to fix:**
- See BLOCKER-2

---

## Integration Graph Alignment

**Edge `agent_orchestration → agent_registry`** (required, REST lookup, SLA 50ms)
- Contract path: `/agents/{agent_id}` in OpenAPI
- Base path mismatch: OpenAPI uses `/agents` but integration graph implies `/api/v1/registry`
- **Fix needed:** Align base path before Module 03 can reliably call Module 04

**Edge `department_template_engine → agent_registry`** (required, REST register, SLA 200ms)
- Same base path issue
- **Fix needed:** Align base path

**Module 04 outgoing edges:**
- `agent_registry → observability` (AsyncEvent, publish) — Not yet implemented (BLOCKER-4)

---

## Master Contract Index Updates Needed

1. Module 04 row needs updated notes: "RECONCILED" but with detailed gap findings
2. Cross-spec inconsistency tracker needs Module 04 entry for DependencyType enum
3. PRD compliance audit section needs Module 04 row
4. Missing `*List` schemas section should include Module 04

---

## Verdict

**REJECT — Cannot proceed to development delegation.**

The three contracts (OpenAPI, JSON Schema, AsyncAPI) must be harmonized first, then implementation can be adjusted. The architecture document (temp/Architecture.md) is aspirational but needs updates to reflect actual implementation state vs. target state.

### Pre-Implementation Checklist (Must Complete Before Delegation)

| # | Item | Type |
|---|------|------|
| 1 | Harmonize DependencyType enum across all 3 contracts | BLOCKER-1 |
| 2 | Add missing events to AsyncAPI + Go structs | BLOCKER-4 |
| 3 | Fix SearchAgents tenant context bypass | BLOCKER-2 |
| 4 | Add ExtractTenant to all routes | BLOCKER-3 |
| 5 | Align base path to `/registry` | BLOCKER-5 |
| 6 | Add missing Agent fields (objectives, supported_languages, etc.) | BLOCKER-3 |
| 7 | Wire config into main.go | BLOCKER-1 |
| 8 | Add tenant isolation to Version/Capability/Dependency stores | BLOCKER-2 |
