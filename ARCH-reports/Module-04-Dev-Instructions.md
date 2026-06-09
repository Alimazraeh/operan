# Module 04 (Agent Registry) — Developer Remediation Instructions

> **Status:** CONDITIONAL → APPROVE
> **Module:** 04-agent-registry — central registry for reusable, versioned agents with capability indexing, permissions, and dependency management
> **Audited By:** ARCH review
> **Date Generated:** 2025-01-09

---

## Executive Summary

Module 04 is a **well-architected module** — tenant isolation is properly implemented via context-based `byTenant` maps in all stores, the JWT middleware chain is correct, event publishing is wired to Kafka, and the handler layer is fully implemented. However, **handler test coverage sits at 58.5%** (target: 80%), critical **infrastructure artifacts are missing** (Dockerfile, Helm chart, README, .gitignore), and there are **two P1 security issues** around secret defaults.

### Current State
| Category | Status |
|----------|--------|
| Build | ✅ PASSING |
| Tests | ✅ 148 tests pass |
| Handler coverage | 58.5% (target: 80%) |
| Middleware coverage | 83.8% |
| Store coverage | 77.7% |
| Broker coverage | 97.8% |
| Cache coverage | 98.2% |
| Events coverage | 96.0% |
| Repository coverage | 0.0% (no test file) |
| Infrastructure | ❌ Dockerfile, Helm chart, README, .gitignore — all MISSING |

---

## PHASE 1: P1 SECURITY FIXES (Must Fix Before APPROVE)

---

### P1-01: Database Password Default Has No Guard

**Severity:** HIGH
**File:** `internal/config/config.go`

#### Problem

```go
const DefaultDBPassword = "postgres"  // ← Hardcoded weak default
```

Unlike `JWT_SECRET` which has a `Validate()` check that fails startup if the default is used, `DB_PASSWORD` has **no validation guard**. If the env var is not set, the application connects to PostgreSQL with the default `postgres` password — which is trivially guessable.

#### Fix

Add validation in `config.Validate()`:
```go
func (c *Config) Validate() error {
    if c.JWTSecret == DefaultJWTSecret {
        return fmt.Errorf("JWT_SECRET is set to default value")
    }
    // ADD:
    if c.DBPassword == DefaultDBPassword {
        return fmt.Errorf("DB_PASSWORD is set to default value; set via DB_PASSWORD env var")
    }
    return nil
}
```

---

### P1-02: `getSecretFromEnv` Default Case Bug

**Severity:** MEDIUM
**File:** `internal/middleware/middleware.go`

#### Problem

```go
func getSecretFromEnv(envVar string) string {
    switch envVar {
    case "JWT_SECRET":
        return os.Getenv("JWT_SECRET")
    case "IAM_TOKEN_SECRET":
        return os.Getenv("IAM_TOKEN_SECRET")
    default:
        return envVar  // ← Returns the VARIABLE NAME as the secret!
    }
}
```

When `ChainJWTAuth(cfg.JWTSecret)` is called with `cfg.JWTSecret = "change-me-in-production"`, the switch falls through to `default` and uses the literal string `"change-me-in-production"` as the HMAC key. This **does work** (the hardcoded secret IS used), but the env var lookup silently fails — meaning if someone sets `JWT_SECRET=something` but the default config has `"change-me-in-production"`, the env var is ignored and the default is used instead.

This bug is **masked** by the `Validate()` check that rejects the default, so in practice if the default is fixed, `cfg.JWTSecret` will be a real secret value (not an env var name), and `getSecretFromEnv` falls through to `default` using that real value directly. So it works by accident.

#### Fix

Two options — both correct:

**Option A (simpler):** Remove the switch and just use the config value directly (no env var indirection):
```go
func ChainJWTAuth(secret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // secret is passed directly from config — no indirection needed
            ...
        })
    }
}
```

**Option B (explicit):** Fix the default case to handle unrecognized values properly:
```go
func getSecretFromEnv(envVar string) string {
    switch envVar {
    case "JWT_SECRET":
        return os.Getenv("JWT_SECRET")
    case "IAM_TOKEN_SECRET":
        return os.Getenv("IAM_TOKEN_SECRET")
    default:
        // envVar is a literal secret value, not a var name
        return envVar
    }
}
```

Either approach is fine. Option A is cleaner and removes dead code.

---

## PHASE 2: HANDLER TEST COVERAGE (58.5% → 80%+)

### P2-01: Cover Uncovered/Under-Covered Handlers

#### Priority Gap Analysis

| Handler | Coverage | Gap |
|---------|----------|-----|
| `GetAgentVersion` | **0.0%** | No test exists |
| `PromoteVersion` | 52.4% | Missing error paths, invalid environment |
| `SearchAgents` | 52.9% | Missing filter combinations |
| `ListDependencies` | 55.6% | Missing not-found paths |
| `AddDependency` | 57.1% | Missing duplicate detection |

#### Specific Tests to Write

**`GetAgentVersion` — full test file needed:**
```go
func TestGetAgentVersion_Success(t *testing.T) {
    store := store.NewVersionStore()
    version := &store.AgentVersion{ID: "v-1", AgentID: "agent-1", Version: "1.0.0"}
    store.data["agent-1"]["v-1"] = version
    
    h := handlers.NewAgentRegistryHandlers(store, nil, nil, nil, nil)
    
    req := httptest.NewRequest("GET", "/agents/agent-1/versions/v-1", nil)
    ctx := context.WithValue(req.Context(), ctxkeys.TenantID, "tenant-1")
    ctx = ctxkeys.WithTenantID(ctx, "tenant-1")
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()
    
    h.GetAgentVersion(w, req, "agent-1", "v-1")
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

**`PromoteVersion` — error paths:**
- Invalid `to_environment` (not in enum: staging/production)
- Version not found (404)
- Version already promoted (409)
- Cross-tenant access (404)

**`SearchAgents` — filter combinations:**
- Search by capability
- Search by language
- Search by status + department
- Empty result set
- Invalid search params (400)

**`AddDependency` — duplicate detection:**
- Same `dependent_agent_id` already exists → 409 Conflict
- Missing `dependent_agent_id` → 400 Bad Request

---

### P2-02: Store `Exists` Methods at 0% Coverage

**Severity:** LOW

All three store `Exists` methods have no test coverage:
- `AgentStore.Exists` — 0% (actually 85.7% per some metrics, inconsistent)
- `CapabilityStore.Exists` — 0%
- `DependencyStore.Exists` — 0%
- `VersionStore.Exists` — 0%

Add simple tests:
```go
func TestVersionStore_Exists(t *testing.T) {
    s := store.NewVersionStore()
    s.Create("tenant-1", &store.AgentVersion{ID: "v-1", AgentID: "a-1"})
    
    found, err := s.Exists("v-1")
    assert.NoError(t, err)
    assert.True(t, found)
    
    found, err = s.Exists("v-missing")
    assert.NoError(t, err)
    assert.False(t, found)
}
```

---

### P2-03: Missing Tenant Isolation Tests at Handler Level

**Current state:** Only `TestListAgents_TenantIsolation` tests handler-level tenant isolation.

**Missing:**
- `GetAgent` — cross-tenant access returns 404
- `UpdateAgent` — cross-tenant update blocked
- `DeprecateAgent` — cross-tenant deprecation blocked
- `ArchiveAgent` — cross-tenant archive blocked
- `GetAgentVersion` — cross-tenant version access blocked

Add handler-level isolation tests following the pattern in `TestListAgents_TenantIsolation`:
```go
func TestGetAgent_TenantIsolation(t *testing.T) {
    store := store.NewAgentStore()
    store.Create("tenant-1", &store.Agent{ID: "a-1", Name: "Agent A"})
    store.Create("tenant-2", &store.Agent{ID: "a-2", Name: "Agent B"})
    
    h := handlers.NewAgentRegistryHandlers(store, nil, nil, nil, nil)
    
    // Request as tenant-2 for tenant-1's agent
    req := httptest.NewRequest("GET", "/agents/a-1", nil)
    ctx := ctxkeys.WithTenantID(context.Background(), "tenant-2")
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()
    
    h.GetAgent(w, req, "a-1")
    
    assert.Equal(t, http.StatusNotFound, w.Code)
}
```

---

## PHASE 3: BUG FIXES

### P2-04: PromoteVersion Hardcoded `"none"` for `from_env`

**Severity:** MEDIUM
**File:** `internal/handlers/agent_registry.go` (line ~557)

#### Problem

```go
h.EventPublisher.PublishAgentPromoted(version.AgentID, version.Version, mw.UserIDFromContext(r.Context()), "none", req.Environment, now())
```

The `fromEnv` parameter is always the literal string `"none"` instead of determining which environment the version was previously promoted from.

#### Fix

Determine the current promoted environment from the version list:
```go
// Find the currently promoted version in this agent's versions
currentEnv := "none"
if versions, _ := h.VersionStore.ListByAgentAndStatus(version.AgentID, "promoted"); len(versions) > 0 {
    currentEnv = versions[0].Environment  // Assuming versions track their environment
}
h.EventPublisher.PublishAgentPromoted(version.AgentID, version.Version, mw.UserIDFromContext(r.Context()), currentEnv, req.Environment, now())
```

---

### P2-05: AsyncAPI Enum Drift — `to_environment`

**Severity:** MEDIUM
**Contracts affected:** `openapi-04-agent-registry.yaml`, `asyncapi-04-agent-registry.yaml`

#### Problem

| Source | Allowed Values |
|--------|---------------|
| AsyncAPI contract | `["staging", "production"]` |
| Handler code | `"dev"`, `"staging"`, `"production"` |
| OpenAPI contract | Check what's defined (verify alignment) |

The handler accepts `"dev"` but the AsyncAPI contract only defines `"staging"` and `"production"`.

#### Fix

Determine which is correct:
- If `"dev"` is a valid environment: add it to the AsyncAPI `to_environment` enum
- If `"dev"` should NOT be allowed: add validation in the handler to reject it

Update both OpenAPI and AsyncAPI contracts to match. If JSON Schema exists, update it too.

---

## PHASE 4: INFRASTRUCTURE ARTIFACTS

### P2-06: Missing Dockerfile, Helm Chart, README, .gitignore

**Severity:** HIGH for production deployment

#### Current State

| Artifact | Status |
|----------|--------|
| `Dockerfile` | ❌ MISSING |
| `charts/` (Helm chart) | ❌ MISSING |
| `README.md` | ❌ MISSING |
| `.gitignore` | ❌ MISSING |
| `manifest.json` | ✅ EXISTS (pre-computed metrics) |

#### Fix

Create using the pattern from Modules 01, 02, 03, or 05 (which have all these artifacts):

**Dockerfile** (multi-stage build, non-root user):
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/identity-access ./cmd/identity-access

# Run stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
RUN addgroup -S app && adduser -S app -G app
USER app
COPY --from=builder /app/identity-access /usr/local/bin/identity-access
ENTRYPOINT ["identity-access"]
```

**Helm chart:** Create `charts/` directory with:
- `charts/Chart.yaml` — apiVersion, name, version
- `charts/values.yaml` — replica count, image, env vars, resources
- `charts/templates/` — deployment, service, configmap

**README.md:** Module overview, architecture diagram, configuration reference, deployment instructions

**.gitignore:** Go-specific (`*.exe`, `*.test`, `vendor/`, `*.pid`)

---

## PHASE 5: CONTRACT COMPLETION

### P2-07: Missing JSON Schema Contract

**Severity:** LOW

No `jsonschema-04-agent-registry.yaml` (or `.json`) file exists for Module 04.

#### Fix

Generate JSON Schema from the OpenAPI contract. Use the same tooling that Modules 01-03 use. The schema should cover all request/response types used by Module 04.

---

## VERIFICATION COMMANDS

```bash
cd modules/04-agent-registry

# Build
go build ./...

# Tests
go test ./...

# Coverage (target: handlers ≥ 80%)
go test -cover ./internal/handlers/...

# Security audit
grep -rn 'DBPassword.*"postgres"' internal/config/
grep -rn 'default:.*return envVar' internal/middleware/
grep -rn '"none"' internal/handlers/  # Check hardcoded from_env
grep -rn '"dev"' internal/handlers/  # Check enum drift

# Contract alignment
grep -A5 'to_environment' contracts/v1/openapi-04-agent-registry.yaml
grep -A5 'to_environment' contracts/v1/asyncapi-04-agent-registry.yaml
```

---

## DETAILED PHASED ASSIGNMENT

### Sprint 1: Security Hardening (P1 Fixes)
| Task | Files | Effort |
|------|-------|--------|
| Add DB_PASSWORD validation guard | `config/config.go` | 0.5h |
| Fix `getSecretFromEnv` default case | `middleware/middleware.go` | 1h |
| **Total** | | **~1.5h** |

### Sprint 2: Test Coverage (P2-01 to P2-03)
| Task | Files | Effort |
|------|-------|--------|
| `GetAgentVersion` test (full coverage) | `handler/agent_registry_test.go` | 2h |
| `PromoteVersion` error path tests | `handler/agent_registry_test.go` | 1.5h |
| `SearchAgents` filter combination tests | `handler/agent_registry_test.go` | 2h |
| `AddDependency` duplicate detection tests | `handler/agent_registry_test.go` | 1h |
| Store `Exists` method tests | `store/*_test.go` | 1.5h |
| Handler-level tenant isolation tests | `handler/agent_registry_test.go` | 3h |
| **Total** | | **~12h** |

### Sprint 3: Bug Fixes + Infrastructure (P2-04 to P2-07)
| Task | Files | Effort |
|------|-------|--------|
| Fix `PromoteVersion` `from_env` logic | `handler/agent_registry.go` | 1.5h |
| Fix AsyncAPI enum drift | `openapi-04-*.yaml`, `asyncapi-04-*.yaml` | 1h |
| Create Dockerfile | `Dockerfile` | 1h |
| Create Helm chart | `charts/` | 2h |
| Create README.md | `README.md` | 1h |
| Create .gitignore | `.gitignore` | 0.5h |
| Generate JSON Schema | `jsonschema-04-agent-registry.json` | 2h |
| **Total** | | **~9h** |

---

## SIGN-OFF CHECKLIST

Before requesting a re-review, verify:

- [ ] P1 security fixes implemented
- [ ] `DB_PASSWORD` validation in `config.Validate()`
- [ ] `getSecretFromEnv` default case fixed or removed
- [ ] Handler test coverage ≥ 80%
- [ ] `GetAgentVersion` has full test coverage
- [ ] `PromoteVersion` error paths tested
- [ ] `SearchAgents` filter combinations tested
- [ ] All store `Exists` methods tested
- [ ] Handler-level tenant isolation tests for GET/PATCH/DELETE operations
- [ ] `PromoteVersion` `from_env` derives from actual promoted version
- [ ] AsyncAPI `to_environment` enum matches handler accepted values
- [ ] Dockerfile, Helm chart, README.md, .gitignore all created
- [ ] JSON Schema contract generated
- [ ] Build passes: `go build ./...`
- [ ] Tests pass: `go test ./...` (148+ tests, all pass)
