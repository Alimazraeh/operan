# 📋 ARCH → CODER Handover: Module 04 — Agent Registry Remediation

**Date:** 2026-05-28  
**Review Verdict:** REJECT  
**Priority:** P0 — Blocker for Wave 2  
**Architect:** ARCH  
**Assignee:** CODER  
**Module ID:** `04-agent-registry`  
**PRD Reference:** Sections 5, 8, 9

---

## 🤖 CODER PERSONA & CONTEXT

You are **CODER_B** — the implementation specialist in the Operan manual agentic workflow. Your role is to take architectural reviews and contract specifications and produce **fully implemented, tested, contract-compliant Go code**.

### Your Responsibilities
- Read and implement against **contracts/v1/** (OpenAPI, JSON Schema, AsyncAPI)
- Write production-quality Go code following platform standards
- Achieve **≥80% test coverage** on all handler, store, and middleware packages
- Follow all **DO NOT VIOLATE** rules (see Global Constants below)
- Output complete files — do not truncate or summarize code

### Platform Standards (NON-NEGOTIABLE)
| Standard | Implementation |
|----------|---------------|
| **Auth** | `BearerAuth` (RSA/JWKS + HMAC fallback) via `Authorization` header |
| **Tenant Isolation** | `X-Tenant-ID` header extracted by middleware; all queries scoped by `tenant_id` from context |
| **Errors** | RFC 7807 Problem Details: `{ type: string, title: string, status: int, detail: string, instance: string, request_id: string }` |
| **Pagination** | Query params: `page`, `page_size`; Response wrapper: `{ items: T[], total: int, page: int, page_size: int, has_more: bool }` |
| **Schemas** | `additionalProperties: false` in all JSON schemas |
| **Timestamps** | ISO 8601 UTC: `2026-05-28T14:30:00Z` |
| **IDs** | UUID v4 format |
| **Observability** | `X-Trace-Id`, `X-Request-Id` headers; structured JSON logging |

### Your Constraints
- ❌ NO imports from other module packages (e.g., `operan/module-02-iam`)
- ❌ NO assumed interfaces beyond what's defined in `contracts/v1/`
- ❌ NO hardcoded secrets, tokens, or tenant IDs
- ❌ NO in-memory stubs for production-critical components (event publisher, config)
- ❌ NO string context keys — use typed `contextKey` constants

---

## 📊 CURRENT STATE ASSESSMENT

**Overall Compliance: ~45%**  
**Review Report:** `reviews/module-04-review.md`  
**Architecture Blueprint:** `modules/04-agent-registry/temp/Architecture.md`

### State Comparison

| Layer | Target | Current | Status |
|-------|--------|---------|--------|
| **Base Path** | `/api/v1/registry` | `/v1/agents` | ❌ |
| **Auth Middleware** | JWTAuth (RSA/JWKS + HMAC) | None — header-only tenant extraction | ❌ |
| **Tenant Isolation** | All 4 stores tenant-scoped | Only AgentStore scoped; 3 others cross-tenant | ❌ |
| **Event Publishing** | Real AMQP/Kafka + 8 typed structs | Stub logger; 4 structs with wrong names | ❌ |
| **Config** | 10+ env vars via `ParseConfig()` | Struct defined, never used | ❌ |
| **DTOs vs Contracts** | 1:1 match | Significant divergence (see below) | ❌ |
| **Test Coverage** | ≥80% | ~55% (tests use invalid enum values) | ❌ |

### Critical Issues (Must Fix First)

| # | Issue | File | Impact |
|---|-------|------|--------|
| **C1** | `SearchAgents` reads `tenant_id` from request body instead of `tenant.TenantID()` from context | `handlers/search.go` | Data isolation bypass |
| **C2** | `AgentByID` and `VersionByID` routes not wrapped with `ExtractTenant` | `handlers/agents.go` | Unscoped data exposure |
| **C3** | 4 AsyncAPI events have no Go struct; 4 existing structs have wrong names | `events/events.go` | Event contract not implemented |
| **C4** | Event structs named `AgentCreated` and `AgentVersionPromoted` but AsyncAPI expects `AgentRegistered` and `AgentPromoted` | `events/events.go` | Contract mismatch |

### High Issues

| # | Issue | File | Fix Required |
|---|-------|------|-------------|
| **H1** | `CostProfile` has 3 fields vs 6 in OpenAPI schema | `handlers/dtos.go`, `store/models.go` | Add `currency`, `billing_tier`, `cost_center` |
| **H2** | `MemoryAccess` stored as `[]string` instead of structured object | `handlers/dtos.go`, `store/models.go` | Create `MemoryAccess` struct matching OpenAPI |
| **H3** | `DependencyRequest.Type` should be `Description` per OpenAPI | `handlers/dependencies.go` | Rename field |
| **H4** | Version/Capability/Dependency stores lack tenant isolation | `store/versions.go`, `store/capabilities.go`, `store/dependencies.go` | Add `byTenant` index pattern |
| **H5** | Tests use `dependency_type: "direct"` — not in enum `[hard, soft, optional]` | `handlers/dependencies_test.go`, `store/dependencies_test.go` | Replace with valid enum values |
| **H6** | Config struct never wired into `main.go` | `config/config.go`, `main.go` | Call `config.ParseConfig()` and pass to handlers |
| **H7** | Missing Agent fields from PRD §8: `objectives`, `supported_languages`, `current_version_id`, `access_control` | All DTO and store files | Add to struct definitions |
| **H8** | JWTAuth middleware not implemented | No file — needs creation | Implement RSA/JWKS + HMAC fallback auth |

### Medium Issues

| # | Issue | File | Fix Required |
|---|-------|------|-------------|
| **M1** | Missing middleware: TraceID, RequestID, Logger | `middleware/` | Create matching Module 03 patterns |
| **M2** | `config/config.go` defined but unused | `config/config.go` | Wire into `main()` |
| **M3** | Base path `/agents` should be `/registry/agents` | `main.go`, `handlers/*.go` | Update route registration |
| **M4** | Event topic format does not match platform standard `operan.registry.{entity}.{event}` | `events/events.go` | Update topic naming |

---

## 📁 FILE REFERENCE INDEX

### Contracts (SOURCE OF TRUTH)

| Contract | Path | Ops/Channels | Notes |
|----------|------|-------------|-------|
| OpenAPI | `contracts/v1/openapi-04-agent-registry.yaml` | 16 ops, 8 path groups | Primary API spec |
| JSON Schema | `contracts/v1/schema-04-agent-registry.json` | 20 definitions | Struct validation schema |
| AsyncAPI | `contracts/v1/asyncapi-04-agent-registry.yaml` | 8 channels | Event contract |

### Source Files to Modify

#### Handlers
| File | Purpose | Changes Required |
|------|---------|-----------------|
| `handlers/agents.go` | Agent CRUD | Wrap routes with `ExtractTenant`, add missing fields |
| `handlers/agents_test.go` | Agent tests | Fix tenant context injection, valid enum values |
| `handlers/versions.go` | Version CRUD | Add tenant isolation, align with contract |
| `handlers/versions_test.go` | Version tests | Update test expectations |
| `handlers/capabilities.go` | Capability CRUD | Add tenant isolation |
| `handlers/capabilities_test.go` | Capability tests | Update test expectations |
| `handlers/dependencies.go` | Dependency CRUD | Rename `Type` → `Description` |
| `handlers/dependencies_test.go` | Dependency tests | Replace `direct` with valid enum |
| `handlers/search.go` | Agent search | **CRITICAL:** Use context tenant, not body |
| `handlers/dtos.go` | Data transfer objects | Rewrite `CostProfile`, `MemoryAccess`, add missing fields |

#### Middleware
| File | Purpose | Changes Required |
|------|---------|-----------------|
| `middleware/middleware.go` | Auth + tenant extraction | Add JWTAuth, TraceID, RequestID, Logger |
| `middleware/middleware_test.go` | Middleware tests | Add tests for new middleware |

#### Stores
| File | Purpose | Changes Required |
|------|---------|-----------------|
| `store/models.go` | DB/models layer | Add missing Agent fields, fix `CostProfile`, `MemoryAccess` |
| `store/agents.go` | Agent store | Already has tenant isolation — verify |
| `store/agents_test.go` | Agent tests | Verify tenant isolation tests |
| `store/versions.go` | Version store | Add `byTenant` index |
| `store/versions_test.go` | Version tests | Add tenant isolation tests |
| `store/capabilities.go` | Capability store | Add `byTenant` index |
| `store/capabilities_test.go` | Capability tests | Add tenant isolation tests |
| `store/dependencies.go` | Dependency store | Add `byTenant` index |
| `store/dependencies_test.go` | Dependency tests | Add tenant isolation tests, fix enum |

#### Events
| File | Purpose | Changes Required |
|------|---------|-----------------|
| `events/events.go` | Event structs & publisher | Create 4 missing structs, rename 2 existing, implement real publisher stub |
| `events/events_test.go` | Event tests | Add tests for new event structs |

#### Config
| File | Purpose | Changes Required |
|------|---------|-----------------|
| `config/config.go` | Config struct | Already defined — wire into `main.go` |
| `main.go` | Entry point | Wire config, middleware chain, routes with correct base path |

### Reference Files (Module 03 Patterns)

Use these as implementation patterns for Module 04:

| Reference | Path | Pattern |
|-----------|------|---------|
| JWTAuth | `modules/03-agent-orchestration/middleware/middleware.go` | HMAC JWT validation pattern |
| Tenant Context | `modules/03-agent-orchestration/middleware/middleware.go` | Typed context keys + `ExtractTenant` |
| Pagination | `modules/03-agent-orchestration/handlers/handler_workflows.go` | `page`, `page_size`, `has_more` response pattern |
| Event Publisher | `modules/03-agent-orchestration/events/events.go` | Topic naming, typed publish methods |
| Store Pattern | `modules/03-agent-orchestration/store/` | Tenant isolation via `byTenant` index |
| Config | `modules/03-agent-orchestration/config/config.go` | `ParseConfig()` env var loading |

---

## 🔧 REMEDIATION PLAN

### Phase 1: Critical Fixes (Blockers)

#### 1.1 Fix Tenant Context Bypass
**File:** `handlers/search.go`  
**Change:** Replace `var req SearchRequest; req.TenantID = tenantID` with:
```go
tenantID := tenant.TenantID(ctx)
if tenantID == "" {
    // return 401/403
}
```

#### 1.2 Wrap Routes with ExtractTenant
**File:** `main.go`  
**Change:** Ensure all agent routes use the middleware chain:
```go
// Before (broken):
r.HandleFunc("/agents/{id}", agentHandler.GetAgent).Methods("GET")

// After (correct):
tenantHandler := tenantMiddleware.ExtractTenant(agentHandler.GetAgent)
r.HandleFunc("/agents/{id}", tenantHandler).Methods("GET")
```

#### 1.3 Create Missing Event Structs & Rename Existing
**File:** `events/events.go`  
**Required structs (match AsyncAPI operationIds exactly):**

| AsyncAPI operationId | Current Go Struct | Required Go Struct |
|---------------------|-------------------|-------------------|
| `AgentRegistered` | `AgentCreated` | Rename to `AgentRegistered` |
| `AgentCapabilitiesUpdated` | *(none)* | Create `AgentCapabilitiesUpdated` |
| `AgentVersionCreated` | `AgentVersionCreated` | ✅ Keep as-is |
| `AgentPromoted` | `AgentVersionPromoted` | Rename to `AgentPromoted` |
| `AgentDeprecated` | *(none)* | Create `AgentDeprecated` |
| `AgentArchived` | *(none)* | Create `AgentArchived` |
| `DependencyAdded` | *(none)* | Create `DependencyAdded` |
| `DependencyRemoved` | *(none)* | Create `DependencyRemoved` |

**Event topic format:** `operan.registry.{entity}.{event}`  
Example: `operan.registry.agent.registered`

#### 1.4 Implement Stub Event Publisher
**File:** `events/events.go`  
Follow Module 03 pattern — typed `PublishAgentRegistered(ctx, event)` methods that log with structured output. Real AMQP/Kafka is Phase 3.

### Phase 2: DTO & Contract Alignment

#### 2.1 Rewrite CostProfile
**Files:** `handlers/dtos.go`, `store/models.go`  
**Current:**
```go
type CostProfile struct {
    BaseRatePerMinute float64 `json:"base_rate_per_minute"`
    MaxBudget         float64 `json:"max_budget"`
    CostThreshold     float64 `json:"cost_threshold"`
}
```
**Required (from OpenAPI):**
```go
type CostProfile struct {
    BaseRatePerMinute float64 `json:"base_rate_per_minute"`
    MaxBudget         float64 `json:"max_budget"`
    CostThreshold     float64 `json:"cost_threshold"`
    Currency          string  `json:"currency"`            // NEW
    BillingTier       string  `json:"billing_tier"`        // NEW
    CostCenter        string  `json:"cost_center"`         // NEW
}
```

#### 2.2 Rewrite MemoryAccess
**Files:** `handlers/dtos.go`, `store/models.go`  
**Current:** `MemoryAccess []string`  
**Required (from OpenAPI):**
```go
type MemoryAccess struct {
    Scope        string `json:"scope"`          // "semantic" | "episodic" | "graph" | "institutional"
    AccessLevel  string `json:"access_level"`   // "read" | "read_write"
    VectorStore  string `json:"vector_store"`
    Index        string `json:"index"`
    TTLMinutes   *int   `json:"ttl_minutes,omitempty"`
}
```

#### 2.3 Fix DependencyRequest
**File:** `handlers/dependencies.go`  
**Change:** Rename `Type string` → `Description string` in handler struct.

#### 2.4 Add Missing Agent Fields
**Files:** All DTO and store files  
**From PRD §8 — Agent Object Model:**
```go
type Agent struct {
    // ... existing fields ...
    Objectives         []string       `json:"objectives"`                    // NEW - PRD §8
    SupportedLanguages []string       `json:"supported_languages"`           // NEW - PRD §8
    CurrentVersionID   *string        `json:"current_version_id,omitempty"`  // NEW - PRD §8
    AccessControl      *AccessControl `json:"access_control,omitempty"`      // NEW - PRD §8
}

type AccessControl struct {
    Scope          string   `json:"scope"`           // "tenant" | "department" | "global"
    AllowedRoles   []string `json:"allowed_roles"`
    RestrictedTo   []string `json:"restricted_to"`     // user/department IDs
}
```

### Phase 3: Tenant Isolation in Stores

#### 3.1 Add byTenant Index to Remaining Stores
**Files:** `store/versions.go`, `store/capabilities.go`, `store/dependencies.go`

**Pattern from `store/agents.go`:**
```go
type VersionStore struct {
    mu       sync.RWMutex
    versions map[string]*Version
    byTenant map[string]map[string]*Version  // NEW: tenant_id -> version_id -> Version
}

func (s *VersionStore) GetByID(ctx context.Context, id string) (*Version, error) {
    tenantID := tenant.TenantID(ctx)  // NEW
    if tenantID == "" {
        return nil, fmt.Errorf("tenant context required")
    }
    s.mu.RLock()
    defer s.mu.RUnlock()
    if v, ok := s.versions[id]; ok && v.TenantID == tenantID {
        return v, nil
    }
    return nil, fmt.Errorf("not found")
}

func (s *VersionStore) ListByTenant(ctx context.Context, tenantID string) ([]*Version, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if items, ok := s.byTenant[tenantID]; ok {
        result := make([]*Version, 0, len(items))
        for _, v := range items {
            result = append(result, v)
        }
        return result, nil
    }
    return []*Version{}, nil
}
```

**Apply this pattern to all three stores.**

### Phase 4: Middleware Implementation

#### 4.1 Implement JWTAuth Middleware
**File:** `middleware/middleware.go`  
**Pattern from Module 03** (`modules/03-agent-orchestration/middleware/middleware.go`):
- Validate `Authorization: Bearer {jwt}` header
- Support RSA (JWKS) + HMAC fallback
- Extract `sub` (user ID) and `tenant_id` from JWT claims
- Set typed context keys: `TenantIDKey`, `UserIDKey`, `UserTypeKey`
- Return 401 on invalid token

#### 4.2 Add TraceID, RequestID, Logger Middleware
**Pattern from Module 03:**
- `TraceIDMiddleware`: Generate or propagate `X-Trace-Id`
- `RequestIDMiddleware`: Generate `X-Request-Id`
- `LoggerMiddleware`: Structured JSON logging with trace/request/tenant IDs

### Phase 5: Config Wiring

**Files:** `config/config.go`, `main.go`

**In `main.go`:**
```go
cfg, err := config.ParseConfig()
if err != nil {
    log.Fatalf("config parse error: %v", err)
}

handler := NewHandler(
    agentStore,
    versionStore,
    capabilityStore,
    dependencyStore,
    cfg,
)

// Wire middleware chain
r := http.NewServeMux()
r.HandleFunc("/registry/agents", handler.CreateAgent).Methods("POST")
// ... all routes under /registry/agents, not /agents
```

### Phase 6: Test Fixes

#### 6.1 Fix Enum Values
**Files:** `handlers/dependencies_test.go`, `store/dependencies_test.go`  
**Change:** All occurrences of `dependency_type: "direct"` → `"hard"`, `"soft"`, or `"optional"`

#### 6.2 Add Tenant Context to Tests
**Pattern from Module 03 tests:**
```go
func TestGetAgent(t *testing.T) {
    ctx := context.WithValue(context.Background(), tenant.TenantIDKey{}, "test-tenant-id")
    req := httptest.NewRequest("GET", "/agents/test-id", nil)
    req = req.WithContext(ctx)
    // ...
}
```

#### 6.3 Add Tenant Isolation Tests for New Stores
**Files:** `store/versions_test.go`, `store/capabilities_test.go`, `store/dependencies_test.go`  
Add test cases verifying cross-tenant data isolation.

---

## 📝 OUTPUT REQUIREMENTS

When complete, output the following files:

### Source Files (Full Content)
- [ ] `handlers/agents.go`
- [ ] `handlers/dtos.go`
- [ ] `handlers/dependencies.go`
- [ ] `handlers/search.go`
- [ ] `handlers/versions.go`
- [ ] `handlers/capabilities.go`
- [ ] `middleware/middleware.go`
- [ ] `store/models.go`
- [ ] `store/agents.go`
- [ ] `store/versions.go`
- [ ] `store/capabilities.go`
- [ ] `store/dependencies.go`
- [ ] `events/events.go`
- [ ] `config/config.go`
- [ ] `main.go`

### Test Files (Full Content)
- [ ] `handlers/agents_test.go`
- [ ] `handlers/dependencies_test.go`
- [ ] `handlers/search_test.go`
- [ ] `handlers/versions_test.go`
- [ ] `handlers/capabilities_test.go`
- [ ] `middleware/middleware_test.go`
- [ ] `store/agents_test.go`
- [ ] `store/versions_test.go`
- [ ] `store/capabilities_test.go`
- [ ] `store/dependencies_test.go`
- [ ] `events/events_test.go`

### Documentation
- [ ] `manifest.json` — `{ "coverage": X.X, "contract_compliant": true }`

---

## ✅ DEFINITION OF DONE

**Before submitting for REVIEW:**

- [ ] All 4 Critical issues resolved
- [ ] All 8 High issues resolved
- [ ] All 6 Medium issues resolved
- [ ] `SearchAgents` uses `tenant.TenantID(ctx)` — no body reading
- [ ] All routes wrapped with `ExtractTenant` middleware
- [ ] 8 event structs exist, names match AsyncAPI operationIds exactly
- [ ] Event publisher uses typed `Publish*` methods
- [ ] `CostProfile` has all 6 OpenAPI fields
- [ ] `MemoryAccess` is a structured object (not `[]string`)
- [ ] `DependencyRequest.Type` → `Description`
- [ ] All 4 stores have tenant-scoped queries
- [ ] Tests use valid `DependencyType` enum values (`hard`, `soft`, `optional`)
- [ ] JWTAuth middleware implemented (RSA/JWKS + HMAC)
- [ ] TraceID, RequestID, Logger middleware added
- [ ] Config wired into `main.go`
- [ ] Base path is `/registry/agents`
- [ ] `additionalProperties: false` in all handler response validation
- [ ] RFC 7807 error format in all error responses
- [ ] Test coverage ≥80%
- [ ] `manifest.json` output with coverage metric

---

## 🔗 REFERENCE LINKS

| Resource | Path |
|----------|------|
| Architectural Review | `reviews/module-04-review.md` |
| Architecture Blueprint | `modules/04-agent-registry/temp/Architecture.md` |
| OpenAPI Contract | `contracts/v1/openapi-04-agent-registry.yaml` |
| JSON Schema | `contracts/v1/schema-04-agent-registry.json` |
| AsyncAPI Contract | `contracts/v1/asyncapi-04-agent-registry.yaml` |
| Module 03 Patterns (Auth) | `modules/03-agent-orchestration/middleware/middleware.go` |
| Module 03 Patterns (Events) | `modules/03-agent-orchestration/events/events.go` |
| Module 03 Patterns (Stores) | `modules/03-agent-orchestration/store/agents.go` |
| Module 03 Patterns (Config) | `modules/03-agent-orchestration/config/config.go` |
| Master Contract Index | `Master Contract Index.md` |
| Manual Runbook | `Operan Manual Runbook.md` |

---

## 🚨 COMMON PITFALLS

1. **Reading tenant_id from request body** — This is the #1 mistake. Always use `tenant.TenantID(ctx)`.
2. **Using string context keys** — Use typed `contextKey` structs (see Module 03 middleware).
3. **Hardcoding enum values** — Always validate against OpenAPI schema. `DependencyType` must be `hard`, `soft`, or `optional`.
4. **Forgetting tenant isolation in nested stores** — Version, Capability, and Dependency stores must all have `byTenant` indexes.
5. **Naming event structs differently from AsyncAPI** — Go struct names must match AsyncAPI operationIds exactly.
6. **Missing DTO fields** — Cross-reference every handler DTO against the OpenAPI schema field by field.
7. **Unwrapped routes** — Every route that serves data must pass through `ExtractTenant`.

---

**Handover Complete. Begin remediation.**
