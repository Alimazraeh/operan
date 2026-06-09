# Module 04 (Agent Registry) — Final Developer Remediation Instructions

> **Verdict Target:** CONDITIONAL → **APPROVED**
> **Module:** 04-agent-registry — central registry for reusable, versioned agents
> **Date:** 2026-05-31
> **Coverage Status:** 81.3% (handlers 80.0%) ✅ — ABOVE 80% TARGET

---

## Executive Summary

Module 04 has solid code quality and passed 80% handler coverage. The remaining gaps fall into three categories:

1. **Contract drift** (OpenAPI vs JSON Schema vs AsyncAPI) — 7 issues
2. **Security hardening** (RBAC, JWKS, rate limiting) — 4 issues
3. **Infrastructure completeness** (PROGRESS.md, HANDOFF.md, PostgreSQL adapter) — 3 issues

**Estimated effort:** 3–5 developer days

---

## PHASE 1: Contract Drift Fixes (P0)

### 1.1 Move `Error` Schema Under `components/schemas`

**File:** `contracts/v1/openapi-04-agent-registry.yaml`

**Current state (line ~1023):**
```yaml
  Error:
    type: object
    properties:
      code: { type: integer }
      message: { type: string }
      request_id: { type: string, format: uuid }
```

This is at the top level (sibling of `components`), not under `components/schemas`. But `$ref` references use `#/components/schemas/Error`, which is invalid.

**Fix:**
1. Delete the `Error` block at line ~1023
2. Add it under `components/schemas` (after the last schema definition, before `responses:` or `parameters:`)
3. Verify all `$ref: "#/components/schemas/Error"` references are correct

**Verification command:**
```bash
grep -n "Error" contracts/v1/openapi-04-agent-registry.yaml | grep '\$ref'
# All must point to #/components/schemas/Error
```

---

### 1.2 Reconcile `promoted_to` Type (OpenAPI vs JSON Schema)

**Files:**
- `contracts/v1/openapi-04-agent-registry.yaml`
- `contracts/v1/schema-04-agent-registry.json`
- `modules/04-agent-registry/internal/store/models.go`

**Current mismatch:**
- **OpenAPI:** `promoted_to` → `type: object, additionalProperties: { type: string }` — "Map of environment names to **version IDs**"
- **JSON Schema:** `promoted_to` → `type: object, additionalProperties: { type: string, format: date-time }` — "Map of environment names to **promotion timestamps**"
- **Go code (`store/models.go`):** `PromotedTo map[string]string` — stores **version IDs**

**Verdict:** The Go code is correct. The JSON Schema has the wrong format. Align both contracts to OpenAPI's version ID approach.

**Fix in JSON Schema (`schema-04-agent-registry.json`):**
1. Find `AgentVersion.promoted_to` definition
2. Remove `format: date-time` from the `additionalProperties`
3. Update description to say "Map of environment names to **version IDs**"

**Fix in OpenAPI (if needed):** Already correct — `type: object, additionalProperties: { type: string }`

---

### 1.3 Add Missing Fields to OpenAPI `Agent` Schema

**File:** `contracts/v1/openapi-04-agent-registry.yaml`

**Fields in JSON Schema + Go code but missing from OpenAPI:**
| Field | Type | Description |
|-------|------|-------------|
| `version` | `string` | Current agent version |
| `created_by` | `string, format: uuid` | Who created this agent |
| `dependencies` | `array of strings` | List of dependency agent IDs |

**Fix:** Add these three fields to the `Agent` schema in `components/schemas/Agent` in the OpenAPI file. Match the types from JSON Schema.

**Verification:**
```bash
# Check all three fields exist in OpenAPI Agent schema
grep -A 100 "Agent:" contracts/v1/openapi-04-agent-registry.yaml | grep -E "version|created_by|dependencies" | head -5
```

---

### 1.4 Add `runtime_constraints` to JSON Schema Request Types

**File:** `contracts/v1/schema-04-agent-registry.json`

**Current state:** `CreateAgentRequest` and `UpdateAgentRequest` are missing `runtime_constraints`.

**OpenAPI has it** — check `CreateAgentRequest` and `UpdateAgentRequest` schemas in `openapi-04-agent-registry.yaml`.

**Fix:** Add `runtime_constraints` property to both request schemas in JSON Schema. Reference `$ref: "#/components/schemas/RuntimeConstraints"` or inline the definition.

**Verification:**
```bash
grep -A 20 "CreateAgentRequest" contracts/v1/schema-04-agent-registry.json | grep runtime
grep -A 20 "UpdateAgentRequest" contracts/v1/schema-04-agent-registry.json | grep runtime
# Both should contain "runtime_constraints"
```

---

### 1.5 Deduplicate `RuntimeConstraints` and `CostProfile` in OpenAPI

**File:** `contracts/v1/openapi-04-agent-registry.yaml`

**Current state:** These two schemas are defined twice (likely one inline and one under `components/schemas`).

**Fix:**
1. Find both definitions
2. Keep only the one under `components/schemas/RuntimeConstraints` and `components/schemas/CostProfile`
3. Replace all inline definitions with `$ref: "#/components/schemas/RuntimeConstraints"` and `$ref: "#/components/schemas/CostProfile"`
4. Verify the definitions are correct in `components/schemas`

**Verification:**
```bash
grep -c "RuntimeConstraints:" contracts/v1/openapi-04-agent-registry.yaml
grep -c "CostProfile:" contracts/v1/openapi-04-agent-registry.yaml
# Both should return exactly 1 (the definition under components/schemas)
```

---

### 1.6 Add Missing OpenAPI Schemas to JSON Schema

**File:** `contracts/v1/schema-04-agent-registry.json`

**Missing definitions that exist in OpenAPI:**
| Schema | Description |
|--------|-------------|
| `PromoteVersionRequest` | Environment promotion request |
| `AgentListResponse` | Paginated agent list response |
| `VersionList` | Versions list response |
| `DependencyList` | Dependencies list response |

**Fix:** Add these four schemas to the JSON Schema file. Use the same structure as the OpenAPI file.

---

### 1.7 Fix `RemoveDependency` to Use Path Parameter

**File:** `contracts/v1/openapi-04-agent-registry.yaml` + `modules/04-agent-registry/internal/handlers/router.go`

**Current state (OpenAPI):**
```yaml
removeDependency:
  delete:
    /registry/agents/{agent_id}/dependencies:
      parameters:
        - name: dependency_id
          in: query
          ...
```

**Current handler (router.go line ~95):**
```go
mux.HandleFunc("DELETE /registry/agents/", mw.ExtractTenant(func(w http.ResponseWriter, r *http.Request) {
    // ...
    case "dependencies":
        h.RemoveDependency(w, r)
```

Handler reads `dependency_id` from query param (line ~746):
```go
depID := r.URL.Query().Get("dependency_id")
```

**Fix:** Change the contract AND the handler to use path parameter:
1. OpenAPI: Change path to `/registry/agents/{agent_id}/dependencies/{dependency_id}` and move `dependency_id` from `query` to `path`
2. Handler (agent_registry.go ~line 746): Change from `r.URL.Query().Get("dependency_id")` to extracting from path
3. Router: Update the DELETE case path to match

**Verification:**
```bash
# Contract should show path param
grep -A 5 "removeDependency" contracts/v1/openapi-04-agent-registry.yaml | grep "in: path"
# Handler should extract from path
grep "Query.*dependency_id" modules/04-agent-registry/internal/handlers/agent_registry.go
# Should return 0 results
```

---

### 1.8 Add 401/403 Responses to Write Endpoints

**File:** `contracts/v1/openapi-04-agent-registry.yaml`

**Missing responses:**
| Endpoint | Missing |
|----------|---------|
| `updateAgentVersion` | 401, 403 |
| `createAgentVersion` | 401, 403 |
| `indexCapabilities` | 403 |

**Fix:** Add these `responses` blocks to the three endpoints in the OpenAPI file:
```yaml
401:
  description: Unauthorized — invalid or missing JWT token
  content:
    application/json:
      schema:
        $ref: "#/components/schemas/Error"
403:
  description: Forbidden — insufficient permissions
  content:
    application/json:
      schema:
        $ref: "#/components/schemas/Error"
```

---

## PHASE 2: Security Hardening (P1)

### 2.1 Wire RBAC Middleware

**Files:**
- `modules/04-agent-registry/internal/middleware/middleware.go` — `RequireRole` already exists (line ~137)
- `modules/04-agent-registry/internal/handlers/router.go` — needs per-endpoint role guards

**Current state:** `RequireRole` exists in code but is never called. All handlers are accessible to any authenticated user.

**Fix:**
1. Define a role map per endpoint:
   - Write operations (`CreateAgent`, `CreateAgentVersion`, `UpdateAgent`, `UpdateAgentVersion`, `DeprecateAgent`, `ArchiveAgent`, `AddDependency`, `UpdateAgentCapabilities`) → require `admin` or `registry_admin`
   - Read operations → `admin`, `registry_admin`, or `registry_reader`
   - Search → any authenticated user

2. Create a wrapper function in `router.go`:
```go
func authWithRole(allowedRoles []string, handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        mw.RequireRole(allowedRoles...)(http.HandlerFunc(handler)).ServeHTTP(w, r)
    }
}
```

3. Wrap handler registrations in `router.go`:
```go
// Before:
mux.HandleFunc("POST /registry/agents", h.CreateAgent)
// After:
mux.HandleFunc("POST /registry/agents", authWithRole([]string{"admin", "registry_admin"}, h.CreateAgent))
```

4. Apply to all write handlers.

---

### 2.2 Implement JWKS-Based JWT Validation

**Files:**
- `modules/04-agent-registry/internal/middleware/middleware.go`
- `modules/04-agent-registry/internal/config/config.go`

**Current state:** Only HMAC-S256 validation (hardcoded secret). No JWKS endpoint.

**Fix:**
1. Add JWKS config fields to `config.go`:
```go
type Config struct {
    // ... existing fields ...
    JWKSURL         string  // URL of JWKS endpoint (Module 02 or external IdP)
    JWKSRefreshRate int     // Refresh rate in seconds
}
```

2. In `middleware.go`, add JWKS-based JWT validation:
```go
func JWTAuthJWKS(jwksURL string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            // ... extract token ...
            
            // Try JWKS first, fall back to HMAC
            claims, err := validateJWKSJWT(jwksURL, tokenStr)
            if err != nil {
                // Fallback to HMAC for backward compatibility during migration
                claims, err = validateHMACJWT(cfg.JWTSecret, tokenStr)
                if err != nil {
                    WriteError(w, http.StatusUnauthorized, "invalid_token", ...)
                    return
                }
            }
            // ... extract claims ...
        })
    }
}
```

3. Update `main.go` to use the JWKS middleware chain.

**Note:** This requires Module 02's JWKS endpoint to be available. Wire it to the tenant control plane's JWKS endpoint.

---

### 2.3 Add Rate-Limiting Middleware

**Files:**
- `modules/04-agent-registry/internal/middleware/ratelimit.go` (new file)

**Fix:**
1. Create a new middleware file with an in-memory token bucket rate limiter:
```go
package middleware

import (
    "sync"
    "time"
    "net/http"
)

type rateLimiter struct {
    mu       sync.Mutex
    buckets  map[string]*tokenBucket
    maxBuckets int
}

type tokenBucket struct {
    tokens     float64
    maxTokens  float64
    refillRate float64 // tokens per second
    lastRefill time.Time
}

func NewRateLimiter(maxRequestsPerMinute int) *rateLimiter {
    return &rateLimiter{
        buckets:    make(map[string]*tokenBucket),
        maxBuckets: 10000,
        maxTokens:  float64(maxRequestsPerMinute),
        refillRate: float64(maxRequestsPerMinute) / 60.0,
    }
}

func (rl *rateLimiter) allow(tenantID string) bool {
    // ... token bucket algorithm ...
}

func RateLimit(rl *rateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tenantID := TenantIDFromContext(r.Context())
            if tenantID == "" {
                tenantID = r.Header.Get("X-Tenant-ID")
            }
            if !rl.allow(tenantID) {
                w.Header().Set("Retry-After", "1")
                WriteError(w, http.StatusTooManyRequests, "rate_limited", "Too Many Requests", "Rate limit exceeded", r.URL.Path)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

2. Wire it in `main.go`:
```go
rateLimiter := middleware.NewRateLimiter(100) // 100 req/min per tenant
handler := middleware.Chain(
    func(w http.ResponseWriter, r *http.Request) { router.ServeHTTP(w, r) },
    middleware.ChainJWTAuth(cfg.JWTSecret),
    middleware.RateLimit(rateLimiter),
    middleware.ExtractTenant,
    middleware.TraceID,
    middleware.RequestID,
    middleware.Logger,
)
```

---

## PHASE 3: Infrastructure & Documentation (P1)

### 3.1 Create `PROGRESS.md`

**File:** `modules/04-agent-registry/PROGRESS.md`

**Template:**
```markdown
# Module 04 — Agent Registry — Development Progress

## Current Status: REMEDIATION

### Completed
- [x] Handler test coverage ≥ 80% (81.3%)
- [x] Dockerfile with multi-stage build
- [x] Helm chart with deployment, service, probes
- [x] README.md
- [x] .gitignore

### In Progress
- [ ] Contract drift fixes (P0-1 through P0-8)
- [ ] RBAC middleware wiring (P1-9)
- [ ] JWKS-based JWT validation (P1-10)
- [ ] Rate-limiting middleware (P1-15)

### Pending
- [ ] PostgreSQL adapter (P1-16)
- [ ] Event publishing to real Kafka (P1-14)

### Metrics
| Package | Coverage | Target |
|---------|----------|--------|
| handlers | 80.0% | ≥80% ✅ |
| middleware | 85%+ | ≥80% ✅ |
| store | 75%+ | ≥80% ⚠️ |
| **Total** | **81.3%** | **≥80% ✅** |
```

### 3.2 Create `HANDOFF.md`

**File:** `modules/04-agent-registry/HANDOFF.md`

**Template:**
```markdown
# Module 04 — Agent Registry — Architecture Handoff Document

## Purpose
Central registry for reusable, versioned agents with capability indexing, permissions, and dependency management.

## Architecture
- **Module Number:** 04
- **API Base Path:** `/registry/agents`
- **Event Topics:** `agent.registry.*`, `agent.version.*`, `agent.capabilities.*`, `agent.dependencies.*`
- **Database:** PostgreSQL (schema: `agent_registry`)
- **Cache:** In-memory LRU with eviction callbacks

## API Endpoints
| Method | Path | Description | Auth Required |
|--------|------|-------------|---------------|
| GET | /registry/agents | List agents | ✅ |
| POST | /registry/agents | Create agent | ✅ admin |
| GET | /registry/agents/{id} | Get agent | ✅ |
| PATCH | /registry/agents/{id} | Update agent | ✅ admin |
| DELETE | /registry/agents/{id} | Archive agent | ✅ admin |
| POST | /registry/agents/search | Search agents | ✅ |
| GET | /registry/agents/{id}/versions | List versions | ✅ |
| POST | /registry/agents/{id}/versions | Create version | ✅ |
| GET | /registry/agents/{id}/versions/{vid} | Get version | ✅ |
| PATCH | /registry/agents/{id}/versions/{vid} | Update version | ✅ admin |
| POST | /registry/agents/{id}/versions/{vid}/promote | Promote version | ✅ admin |
| GET | /registry/agents/{id}/capabilities | List capabilities | ✅ |
| PATCH | /registry/agents/{id}/capabilities | Update capabilities | ✅ admin |
| POST | /registry/agents/{id}/capabilities/index | Index capabilities | ✅ admin |
| GET | /registry/agents/{id}/dependencies | List dependencies | ✅ |
| POST | /registry/agents/{id}/dependencies | Add dependency | ✅ admin |
| DELETE | /registry/agents/{id}/dependencies/{depId} | Remove dependency | ✅ admin |

## Events (AsyncAPI)
- `agentRegistered` — triggered on agent creation
- `agentUpdated` — triggered on agent update
- `agentDeprecate` — triggered on agent deprecation
- `agentArchived` — triggered on agent archive
- `agentVersionCreated` — triggered on version creation
- `agentPromoted` — triggered on version promotion
- `agentCapabilitiesUpdated` — triggered on capability changes
- `agentDependencyAdded` — triggered on dependency addition
- `agentDependencyRemoved` — triggered on dependency removal

## Configuration
| Env Var | Default | Description |
|---------|---------|-------------|
| AGENT_REGISTRY_PORT | :8083 | Listen port |
| JWT_SECRET | (required) | HMAC-S256 JWT secret |
| JWKS_URL | | JWKS endpoint URL |
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_USER | postgres | PostgreSQL user |
| DB_PASSWORD | (required) | PostgreSQL password |
| ... | | ... |

## Deployment
```bash
docker build -t operan/module-04-agent-registry:latest .
docker run -p 8083:8083 \
  -e JWT_SECRET=your-secret \
  -e DB_PASSWORD=your-password \
  operan/module-04-agent-registry:latest

helm install module-04 ./helm/ \
  --set image.repository=operan/module-04-agent-registry \
  --set image.tag=latest
```

## Known Issues & Deferrals
- Rate limiting is in-development (will be wired in remediation)
- JWKS validation is HMAC-only currently; JWKS endpoint will be added
- PostgreSQL adapter exists at schema level but not yet wired as primary backend

## Testing
```bash
cd modules/04-agent-registry
go build ./...
go test ./... -cover   # Target: ≥80% total
go test ./internal/handlers/... -cover  # Target: ≥80% handlers
```
```

### 3.3 Create PostgreSQL Adapter

**Files:**
- `modules/04-agent-registry/internal/db/` (new directory)
- `modules/04-agent-registry/internal/db/postgres.go` (new file)
- `modules/04-agent-registry/internal/store/agents.go` (update)
- `modules/04-agent-registry/internal/store/versions.go` (update)
- `modules/04-agent-registry/internal/store/capabilities.go` (update)
- `modules/04-agent-registry/internal/store/dependencies.go` (update)
- `modules/04-agent-registry/main.go` (update)

**Pattern:** Follow Module 03's PostgreSQL adapter pattern. Create a `Store` interface and a `PostgresStore` implementation that wraps the in-memory store.

1. Create `internal/db/postgres.go`:
```go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    "github.com/lib/pq"
    "github.com/operan/modules/04-agent-registry/internal/store"
)

type PostgresStore struct {
    db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    log.Println("connected to PostgreSQL for agent registry")
    return &PostgresStore{db: db}, nil
}

// Implement wrapper methods for AgentStore, VersionStore, etc.
// Each method delegates to the appropriate table with tenant isolation.
```

2. Create SQL migration file:
```sql
-- internal/db/migrations/001_create_tables.sql
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    description TEXT,
    tenant_id UUID NOT NULL,
    department_id UUID,
    status TEXT NOT NULL DEFAULT 'active',
    objectives JSONB,
    capabilities TEXT[],
    tools TEXT[],
    memory_access JSONB,
    escalation_rules TEXT[],
    governance_policies TEXT[],
    supported_languages TEXT[],
    runtime_constraints JSONB,
    cost_profile JSONB,
    execution_budget JSONB,
    access_control JSONB,
    current_version_id UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_agents_tenant ON agents(tenant_id);
CREATE INDEX idx_agents_status ON agents(tenant_id, status);

-- Similar tables for: agent_versions, agent_capabilities, agent_dependencies
```

3. Wire into `main.go` alongside the in-memory store:
```go
dbStore, err := db.NewPostgresStore(cfg.DatabaseDSN)
if err != nil {
    log.Printf("warning: PostgreSQL not available, using in-memory: %v", err)
}

h := handlers.NewAgentRegistryHandlers(
    agentStore, versionStore, capabilityStore, dependencyStore,
    dbStore,  // optional PostgreSQL backend
    cfg,
)
```

---

## PHASE 4: Bug Fixes (P2)

### 4.1 Remove Dead Code in Path Extraction

**File:** `modules/04-agent-registry/internal/handlers/agent_registry.go`

**Lines ~760-793:** `extractIDFromPath`, `extractAgentIDFromPath`, `extractVersionIDFromPath` have dead fallback code.

**Fix:** Simplify:
```go
func extractAgentIDFromPath(path string) string {
    s := strings.TrimPrefix(path, "/registry/agents/")
    parts := strings.SplitN(s, "/", 2)
    return parts[0]
}

func extractVersionIDFromPath(path string) string {
    s := strings.TrimPrefix(path, "/registry/agents/")
    // s is now "versionID/..." or "versionID"
    parts := strings.SplitN(s, "/", 2)
    return parts[0]
}
```

Remove the redundant `/agents/` fallback path — all routes go through `/registry/agents/`.

---

### 4.2 Fix `ListAll` in CapabilityStore

**File:** `modules/04-agent-registry/internal/store/capabilities.go`

**Issue:** `ListAll` returns only the last upserted capability per agent, not all capabilities.

**Fix:**
```go
func (s *CapabilityStore) ListAll(ctx context.Context, agentID string) ([]*CapabilityEntry, error) {
    // ...
    // Return ALL capabilities for the agent, not just last upsert
    var result []*CapabilityEntry
    for _, cap := range s.capabilities {
        if cap.AgentID == agentID {
            result = append(result, cap)
        }
    }
    return result, nil
}
```

---

### 4.3 Remove Unused `eventPublisher` Parameter from `JWTAuth`

**File:** `modules/04-agent-registry/internal/middleware/middleware.go`

**Line ~245:**
```go
func JWTAuth(secretEnvVar string, eventPublisher interface{}) func(http.Handler) http.Handler {
```

**Fix:** Remove the `eventPublisher interface{}` parameter. Update `main.go` to call `JWTAuth(cfg.JWTSecret)` instead of `JWTAuth(cfg.JWTSecret, h.EventPublisher)`.

---

## VERIFICATION COMMANDS

```bash
cd modules/04-agent-registry

# Build
go build ./...

# Tests (all must pass)
go test ./... -count=1

# Coverage (target: ≥80%)
go test ./... -cover
go test ./internal/handlers/... -cover  # Target: ≥80%

# Contract validation
# Ensure Error schema is under components/schemas
grep -A 3 "^  schemas:" contracts/v1/openapi-04-agent-registry.yaml | head -5
grep "Error:" contracts/v1/openapi-04-agent-registry.yaml
# Should be: "    Error:" (under schemas: block)

# Ensure no duplicates
grep -c "RuntimeConstraints:" contracts/v1/openapi-04-agent-registry.yaml  # Should be 1
grep -c "CostProfile:" contracts/v1/openapi-04-agent-registry.yaml         # Should be 1

# Verify RemoveDependency uses path param
grep -A 3 "removeDependency" contracts/v1/openapi-04-agent-registry.yaml | grep "in: path"
# Should return: in: path

# Verify files exist
test -f PROGRESS.md && echo "PROGRESS.md ✅" || echo "PROGRESS.md ❌"
test -f HANDOFF.md && echo "HANDOFF.md ✅" || echo "HANDOFF.md ❌"
test -f Dockerfile && echo "Dockerfile ✅" || echo "Dockerfile ❌"
test -f helm/Chart.yaml && echo "Helm chart ✅" || echo "Helm chart ❌"
```

---

## SIGN-OFF CHECKLIST

Before resubmitting for re-review, verify:

### Contract Drift (P0)
- [ ] `Error` schema under `components/schemas` in OpenAPI
- [ ] `promoted_to` type aligned (OpenAPI = JSON Schema = Go code: `map<string,string>`)
- [ ] `version`, `created_by`, `dependencies` added to OpenAPI `Agent` schema
- [ ] `runtime_constraints` added to JSON Schema request types
- [ ] Duplicate `RuntimeConstraints` and `CostProfile` deduplicated in OpenAPI
- [ ] Missing OpenAPI schemas (`PromoteVersionRequest`, `AgentListResponse`, `VersionList`, `DependencyList`) added to JSON Schema
- [ ] `RemoveDependency` uses path parameter in both contract and handler
- [ ] 401/403 responses added to all write endpoints in OpenAPI

### Security (P1)
- [ ] RBAC middleware wired into router (write ops require admin/registry_admin)
- [ ] JWKS-based JWT validation implemented with HMAC fallback
- [ ] Rate-limiting middleware implemented and wired
- [ ] Event publishing to Kafka documented (stub limitation noted)

### Infrastructure (P1)
- [ ] `PROGRESS.md` created
- [ ] `HANDOFF.md` created
- [ ] PostgreSQL adapter scaffolded (at minimum, same pattern as Module 03)

### Build & Test
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes (200+ tests)
- [ ] Coverage ≥ 80% total, ≥ 80% handlers

### Code Quality (P2)
- [ ] Dead code in path extraction removed
- [ ] `CapabilityStore.ListAll` returns all capabilities
- [ ] Unused `eventPublisher` parameter removed from `JWTAuth`
- [ ] Empty directories (`cmd/agent-registry/`, `internal/handler/`) cleaned up
