# Module 05 (Department Template Engine) — Developer Remediation Instructions

> **Verdict Target:** CONDITIONAL → **APPROVED**
> **Module:** 05-department-template-engine — central registry for department templates, versions, deployments, and custom templates
> **Coverage Status:** 80.2% total ✅ — ABOVE 80% TARGET
> **Handler Coverage:** 85.1% ✅ — well above 80%

---

## Executive Summary

Module 05 is the most infrastructure-complete module reviewed so far (Dockerfile, Helm chart, README are all production-quality). Tenant isolation is correctly implemented on all stores. The remaining gaps are:

1. **Contract drift** (OpenAPI vs JSON Schema vs AsyncAPI) — 8 issues
2. **Event wiring gaps** (versioned, undeployed, custom template events) — 4 issues
3. **Security hardening** (clone error swallowing, JWT empty secret, rate limiting) — 4 issues
4. **Infrastructure completeness** (PROGRESS.md, HANDOFF.md) — 2 issues
5. **Dead code + bug fixes** — 3 issues

**Estimated effort:** 2–4 developer days

---

## PHASE 1: Contract Drift Fixes (P0)

### 1.1 Fix `TemplateDeployRequest` Required Field Mismatch

**Files:** `contracts/v1/schema-05-department-template-engine.json`

**Current state:**
- **OpenAPI** `TemplateDeployRequest` required: `[version, environment]`
- **JSON Schema** `TemplateDeployRequest` required: `[template_id, version, environment]`

`template_id` is redundant — it's already in the path parameter `/templates/{id}/deploy`.

**Fix:** Remove `template_id` from the required array in JSON Schema.

```json
// Before (line ~477):
"required": ["template_id", "version", "environment"],

// After:
"required": ["version", "environment"],
```

Also remove `template_id` from the properties section if it exists there (it shouldn't since it's path-scoped).

---

### 1.2 Add Undeclared Tags to OpenAPI

**File:** `contracts/v1/openapi-05-department-template-engine.yaml`

**Current state (lines 1281–1287):**
```yaml
tags:
  - name: Templates
    description: Standard department template CRUD operations
  - name: Deployments
    description: Template deployment lifecycle management
  - name: Custom Templates
    description: Free-form custom template operations
```

**Missing tags** that are used in operations:
- `Template Versions` — used by `listTemplateVersions`, `getTemplateVersion`
- `Template Operations` — used by `deployTemplate`, `cloneTemplate`

**Fix:** Add them after the existing tags:

```yaml
  - name: Template Versions
    description: Template versioning operations
  - name: Template Operations
    description: Template deployment and cloning operations
```

---

### 1.3 Remove Per-Operation Security Declarations

**File:** `contracts/v1/openapi-05-department-template-engine.yaml`

**Issue:** Every endpoint redundantly declares both `BearerAuth` and `TenantHeader` at the individual operation level, even though they're already set at the top-level `security` block.

**Fix:** Find all per-operation `security:` declarations (likely 15+) and remove them. The global security block handles authentication for all operations.

**Verification:**
```bash
grep -c "^      security:" contracts/v1/openapi-05-department-template-engine.yaml
# Should be 0 after cleanup
```

---

### 1.4 Reconcile `CustomTemplate.status` to Reference `agentStatus` Enum

**Files:**
- `contracts/v1/openapi-05-department-template-engine.yaml`
- `contracts/v1/schema-05-department-template-engine.json`

**Current state:** `CustomTemplate.status` has inline enum values `[draft, deprecated, published, archived]` which duplicate `agentStatus` enum.

**Fix in OpenAPI:** Replace the inline enum with a `$ref`:
```yaml
CustomTemplate:
  # ... other properties ...
  status:
    $ref: '#/components/schemas/agentStatus'  # or whatever the enum is called
```

**Fix in JSON Schema:** Replace the inline `enum` array with:
```json
"status": { "$ref": "#/definitions/agentStatus" }
```

---

### 1.5 Add `rolled_back` to AsyncAPI Deployment Stage Enum

**File:** `contracts/v1/asyncapi-05-department-template-engine.yaml`

**Current state (line ~328):**
```yaml
deployment_stage:
  type: string
  enum:
    - select
    - configure
    - connect_data
    - provision_memory
    - deploy_swarm
    - operational
```

**Missing:** `failed`, `rolled_back` (both exist in OpenAPI + JSON Schema).

**Fix:** Add all values from the OpenAPI/JSON Schema enum:
```yaml
deployment_stage:
  type: string
  enum:
    - select
    - configure
    - connect_data
    - provision_memory
    - deploy_swarm
    - operational
    - failed
    - rolled_back
```

---

### 1.6 Standardize AsyncAPI Channel Names

**File:** `contracts/v1/asyncapi-05-department-template-engine.yaml`

**Current state:** Module 05 uses `operan.templates.template.{action}` pattern. Modules 01–04 use `operan/events/{domain}/{resource}/{action}`.

**Impact:** Events from Module 05 are not routable under a unified topic hierarchy.

**Fix:** Rename all 8 channel names:

| Current | New (standardized) |
|---------|-------------------|
| `operan.templates.template.created` | `operan/events/template/created` |
| `operan.templates.template.updated` | `operan/events/template/updated` |
| `operan.templates.template.deployed` | `operan/events/template/deployed` |
| `operan.templates.template.deployment_failed` | `operan/events/template/deployment_failed` |
| `operan.templates.template.undeployed` | `operan/events/template/undeployed` |
| `operan.templates.template.deleted` | `operan/events/template/deleted` |
| `operan.templates.template.versioned` | `operan/events/template/versioned` |
| `operan.templates.template.cloned` | `operan/events/template/cloned` |

Also update the `address` field for each channel and the `topic` constant in `events/events.go`:
```go
// Before:
const templateTopic = "operan.templates.template"

// After:
const templateTopic = "operan/events/template"
```

---

## PHASE 2: Event Wiring Gaps (P0)

### 2.1 Wire `PublishTemplateVersioned`

**File:** `modules/05-department-template-engine/internal/handlers/templates.go`

**Current state:** `PublishTemplateVersioned` is defined in events.go but **never called** by any handler.

**Fix:** Add event publishing in `CreateTemplate` (which also handles version creation) or in a dedicated version-creation path.

**Where:** After successful version creation in the handler — likely in the `CreateTemplate` handler or a version-specific handler. Check which handler creates versions.

```go
// After successful version creation:
h.EventPublisher.PublishTemplateVersioned(events.TemplateVersionedPayload{
    Event:            "template.versioned",
    TemplateID:       templateID,
    Version:          createdVersion.Version,
    PreviousVersion:  previousVersion,
    Name:             templateName,
    Category:         templateCategory,
    CreatedAt:        createdVersion.CreatedAt,
    CreatedBy:        middleware.UserIDFromContext(r.Context()),
    TenantID:         middleware.TenantIDFromContext(r.Context()),
})
```

---

### 2.2 Wire `PublishTemplateUndeployed`

**File:** `modules/05-department-template-engine/internal/handlers/nested.go`

**Current state:** `PublishTemplateUndeployed` is defined in events.go but **never called**.

**Fix:** Add event publishing when a deployment is deleted (status change to undeployed or DELETE on deployment).

**Where:** In `handleUpdateDeployment` when status changes to a final state that implies undeployment, or in a dedicated undeploy handler.

```go
// When deployment is successfully undeployed:
h.EventPublisher.PublishTemplateUndeployed(events.TemplateUndeployedPayload{
    Event:          "template.undeployed",
    DeploymentID:   deploymentID,
    TemplateID:     templateID,
    Version:        updated.Version,
    Environment:    updated.Environment,
    UndeployedAt:   time.Now(),
    UndeployedBy:   middleware.UserIDFromContext(r.Context()),
    Reason:         "manual_undeploy",  // or from request body
    TenantID:       middleware.TenantIDFromContext(r.Context()),
})
```

---

### 2.3 Add Custom Template Lifecycle Events

**Files:**
- `contracts/v1/asyncapi-05-department-template-engine.yaml` — add 4 new channels
- `modules/05-department-template-engine/internal/events/events.go` — add 4 new payload types + publish methods
- `modules/05-department-template-engine/internal/handlers/custom_templates.go` — wire publishing in CRUD handlers

**New AsyncAPI channels:**
| Channel | Trigger |
|---------|---------|
| `operan/events/custom_template/created` | Custom template created |
| `operan/events/custom_template/updated` | Custom template updated |
| `operan/events/custom_template/updated` | Custom template updated |
| `operan/events/custom_template/deleted` | Custom template deleted |
| `operan/events/custom_template/cloned` | Custom template cloned |

**New payload types in events.go:**
```go
type CustomTemplateCreatedPayload struct {
    Event            string                 `json:"event"`
    CustomTemplateID string                 `json:"custom_template_id"`
    Name             string                 `json:"name,omitempty"`
    Category         string                 `json:"category,omitempty"`
    CreatedAt        time.Time              `json:"created_at"`
    CreatedBy        string                 `json:"created_by,omitempty"`
    TenantID         string                 `json:"tenant_id"`
}

// Same for Updated, Deleted, Cloned payloads
```

**Wire in handlers:**
- `CreateCustomTemplate` → `PublishCustomTemplateCreated`
- `UpdateCustomTemplate` → `PublishCustomTemplateUpdated`
- `DeleteCustomTemplate` → `PublishCustomTemplateDeleted`
- (clone for custom templates — see P1-7) → `PublishCustomTemplateCloned`

---

## PHASE 3: Security Hardening (P1)

### 3.1 Fix Clone Error Swallowing

**File:** `modules/05-department-template-engine/internal/handlers/nested.go`

**Current state (line ~151):**
```go
body, _ := io.ReadAll(r.Body)
defer r.Body.Close()
var req struct {
    Name string `json:"name"`
    Category string `json:"category"`
}
json.Unmarshal(body, &req)  // ERROR IGNORED
```

**Fix:** Add proper error handling:

```go
body, err := io.ReadAll(r.Body)
if err != nil {
    writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
        "Failed to read request body", r.URL.Path, reqID)
    return
}
defer r.Body.Close()

var req struct {
    Name     string                 `json:"name"`
    Category string                 `json:"category"`
    Metadata map[string]interface{} `json:"metadata"`
    Tags     []string               `json:"tags"`
}
if err := json.Unmarshal(body, &req); err != nil {
    writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
        "Invalid JSON body", r.URL.Path, reqID)
    return
}
```

---

### 3.2 Fix JWT Config to Reject Empty Secrets

**File:** `modules/05-department-template-engine/internal/config/config.go`

**Current state:** Default `JWTSecret` is `"change-me-in-production"`. `Config.Validate()` checks for this, but `validateJWT` in middleware accepts an empty string.

**Fix:** Ensure `Config.Validate()` rejects empty strings:

```go
func (c *Config) Validate() error {
    if c.JWTSecret == "change-me-in-production" || c.JWTSecret == "" {
        return fmt.Errorf("JWT_SECRET environment variable is required and must not be empty")
    }
    // ... other validations
}
```

Also fix `validateJWT` in middleware to reject empty secrets:

```go
func validateJWT(tokenStr string, secret string) (*jwt.MapClaims, error) {
    if secret == "" {
        return nil, errors.New("JWT secret is empty")
    }
    // ... existing logic
}
```

---

### 3.3 Add Rate-Limiting Middleware

**File:** `modules/05-department-template-engine/internal/middleware/ratelimit.go` (new)

**Fix:** Implement the same rate-limiting pattern as Module 04:

```go
package middleware

import (
    "net/http"
    "sync"
    "time"
)

type rateLimiter struct {
    mu         sync.Mutex
    buckets    map[string]*tokenBucket
    maxBuckets int
    maxTokens  float64
    refillRate float64
}

type tokenBucket struct {
    tokens     float64
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
    rl.mu.Lock()
    defer rl.mu.Unlock()

    bucket, exists := rl.buckets[tenantID]
    if !exists {
        if len(rl.buckets) >= rl.maxBuckets {
            return false
        }
        rl.buckets[tenantID] = &tokenBucket{tokens: rl.maxTokens}
        bucket = rl.buckets[tenantID]
    }

    now := time.Now()
    elapsed := now.Sub(bucket.lastRefill).Seconds()
    bucket.tokens += elapsed * rl.refillRate
    if bucket.tokens > rl.maxTokens {
        bucket.tokens = rl.maxTokens
    }
    bucket.lastRefill = now

    if bucket.tokens < 1 {
        return false
    }
    bucket.tokens--
    return true
}

func RateLimit(rl *rateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tenantID := TenantIDFromContext(r.Context())
            if tenantID == "" {
                tenantID = r.Header.Get("X-Tenant-ID")
            }
            if tenantID == "" {
                // Fall back to client IP
                tenantID = r.RemoteAddr
            }
            if !rl.allow(tenantID) {
                w.Header().Set("Retry-After", "1")
                WriteError(w, http.StatusTooManyRequests, "about:blank",
                    "Rate limit exceeded", "Too Many Requests", r.URL.Path, RequestIDFromContext(r.Context()))
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

**Wire in `main.go`:**
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

## PHASE 4: Infrastructure & Documentation (P0)

### 4.1 Create `PROGRESS.md`

**File:** `modules/05-department-template-engine/PROGRESS.md`

**Template:**
```markdown
# Module 05 — Department Template Engine — Development Progress

## Current Status: REMEDIATION

### Completed
- [x] Handler test coverage ≥ 80% (85.1%)
- [x] Total package coverage ≥ 80% (80.2%)
- [x] Dockerfile (multi-stage, non-root user, health check)
- [x] Helm chart (deployment, service, ingress, HPA, serviceaccount)
- [x] README.md (comprehensive)
- [x] manifest.json

### In Progress
- [ ] Contract drift fixes (P0-1 through P0-6)
- [ ] Event wiring for versioned/undeployed/custom events
- [ ] Clone error swallowing fix
- [ ] JWT empty secret rejection

### Pending
- [ ] Rate-limiting middleware
- [ ] PostgreSQL adapter
- [ ] Custom template clone endpoint
- [ ] Template search endpoint

### Metrics
| Package | Coverage | Target |
|---------|----------|--------|
| handlers | 85.1% | ≥80% ✅ |
| middleware | 94.1% | ≥80% ✅ |
| store | 75.5% | ≥80% ⚠️ |
| events | 77.3% | ≥80% ⚠️ |
| **Total** | **80.2%** | **≥80% ✅** |
```

### 4.2 Create `HANDOFF.md`

**File:** `modules/05-department-template-engine/HANDOFF.md`

**Template:**
```markdown
# Module 05 — Department Template Engine — Architecture Handoff Document

## Purpose
Central registry for department templates with versioning, multi-stage deployment lifecycle, custom template support, and tenant isolation.

## Architecture
- **Module Number:** 05
- **API Base Path:** `/templates`
- **Event Topics:** `operan/events/template.*`, `operan/events/custom_template.*`
- **Database:** PostgreSQL (schema: `template_engine`)
- **Cache:** In-memory LRU (100 entries, configurable)

## API Endpoints
| Method | Path | Description | Auth Required |
|--------|------|-------------|---------------|
| GET | /templates | List templates | ✅ |
| POST | /templates | Create template | ✅ |
| GET | /templates/{id} | Get template | ✅ |
| PATCH | /templates/{id} | Update template | ✅ |
| DELETE | /templates/{id} | Delete template | ✅ |
| POST | /templates/{id}/deploy | Deploy template | ✅ |
| GET | /templates/{id}/deployments | List deployments | ✅ |
| GET | /templates/{id}/deployments/{depId} | Get deployment | ✅ |
| PATCH | /templates/{id}/deployments/{depId} | Update deployment | ✅ |
| GET | /templates/{id}/versions | List versions | ✅ |
| GET | /templates/{id}/versions/{vid} | Get version | ✅ |
| POST | /templates/{id}/clone | Clone template | ✅ |
| GET | /templates/custom | List custom templates | ✅ |
| POST | /templates/custom | Create custom template | ✅ |
| GET | /templates/custom/{id} | Get custom template | ✅ |
| PATCH | /templates/custom/{id} | Update custom template | ✅ |
| DELETE | /templates/custom/{id} | Delete custom template | ✅ |

## Events (AsyncAPI)
- `template.created` — triggered on standard template creation
- `template.updated` — triggered on template update
- `template.deployed` — triggered on deployment creation
- `template.deployment_failed` — triggered on deployment failure
- `template.undeployed` — triggered on deployment undeployment
- `template.deleted` — triggered on template deletion
- `template.versioned` — triggered on version creation
- `template.cloned` — triggered on template cloning
- `custom_template.created` — triggered on custom template creation
- `custom_template.updated` — triggered on custom template update
- `custom_template.deleted` — triggered on custom template deletion

## Deployment Lifecycle
1. select → configure → connect_data → provision_memory → deploy_swarm → operational
2. Any stage can fail → failed → (manual retry → back to failed stage)
3. Failed deployments can be rolled_back

## Configuration
| Env Var | Default | Description |
|---------|---------|-------------|
| TEMPLATE_ENGINE_PORT | :8084 | Listen port |
| JWT_SECRET | (required) | HMAC-S256 JWT secret |
| LRU_CACHE_SIZE | 100 | In-memory LRU cache size |
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_USER | postgres | PostgreSQL user |
| DB_PASSWORD | (required) | PostgreSQL password |
| DB_NAME | template_engine | Database name |

## Known Issues & Deferrals
- PostgreSQL adapter exists at schema level but not wired as primary backend
- Custom template clone endpoint not implemented (standard template clone exists)
- Template search endpoint not implemented
- Event broker is log-only; Kafka/AMQP integration pending

## Testing
```bash
cd modules/05-department-template-engine
go build ./...
go test ./... -cover   # Target: ≥80% total
go test ./internal/handlers/... -cover  # Target: ≥80% handlers
```
```

---

## PHASE 5: Bug Fixes & Dead Code (P1)

### 5.1 Remove Dead `Update` Method from TemplateStore

**File:** `modules/05-department-template-engine/internal/store/templates.go`

**Issue (line ~129):**
```go
func (s *TemplateStore) Update(id string, patch map[string]interface{}) (*Template, error) {
    return s.UpdateByTenant(id, "", patch)  // tenantID is empty string
}
```

`UpdateByTenant` checks `if t.TenantID != tenantID` which will always fail for empty tenantID.

**Fix:** Either:
1. Remove the `Update` method entirely (since all handlers use `UpdateByTenant` with proper tenant), OR
2. Make `Update` fetch the template first to extract its tenantID, then call `UpdateByTenant`

**Recommended:** Remove `Update` — it's dead code. All handlers call `UpdateByTenant` directly.

---

### 5.2 Remove Dead `GetByIDAndTemplate` from DeploymentStore

**File:** `modules/05-department-template-engine/internal/store/deployments.go`

**Issue:** `GetByIDAndTemplate` has 0% coverage and is never called by any handler.

**Fix:** Remove it unless it's needed by a future feature (like cross-tenant deployment lookup).

---

### 5.3 Add `toJSONArray` and `toJSON` Helper Tests

**File:** `modules/05-department-template-engine/internal/store/models.go`

**Issue:** `toJSONArray` and `toJSON` have 0% coverage (never tested).

**Fix:** These are helper functions that convert store models to JSON arrays. Add a test that creates a store, populates it, and calls these functions.

---

## PHASE 6: Additional Clone Endpoint (P1)

### 6.1 Add Custom Template Clone Endpoint

**Files:**
- `contracts/v1/openapi-05-department-template-engine.yaml` — add `cloneCustomTemplate` operation
- `contracts/v1/asyncapi-05-department-template-engine.yaml` — add `operan/events/custom_template/cloned` channel
- `modules/05-department-template-engine/internal/handlers/custom_templates.go` — add `handleCloneCustom` handler
- `modules/05-department-template-engine/internal/handlers/router.go` — add route
- `modules/05-department-template-engine/internal/events/events.go` — add `PublishCustomTemplateCloned`

**Handler pattern (similar to standard clone in nested.go):**

```go
func (h *TemplateHandlers) handleCloneCustom(w http.ResponseWriter, r *http.Request, reqID string) {
    id := extractIDFromPath(r.URL.Path, "/templates/custom/")
    if id == "" {
        writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
            "Invalid custom template ID", r.URL.Path, reqID)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
            "Failed to read request body", r.URL.Path, reqID)
        return
    }
    defer r.Body.Close()

    var req struct {
        Name     string                 `json:"name"`
        Category string                 `json:"category"`
        Metadata map[string]interface{} `json:"metadata"`
        Tags     []string               `json:"tags"`
    }
    if err := json.Unmarshal(body, &req); err != nil {
        writeError(w, http.StatusBadRequest, "about:blank", "Bad Request",
            "Invalid JSON body", r.URL.Path, reqID)
        return
    }

    tenantID := middleware.TenantIDFromContext(r.Context())
    ct, err := h.CustomTemplateStore.GetByIDAndTenant(id, tenantID)
    if err != nil {
        writeError(w, http.StatusNotFound, "about:blank", "Not Found",
            "Custom template not found", r.URL.Path, reqID)
        return
    }

    clone := &store.CustomTemplate{
        TenantID:       tenantID,
        Name:           req.Name,
        Description:    ct.Description,
        Category:       req.Category,
        Version:        "1.0.0",
        Content:        ct.Content,
        Metadata:       req.Metadata,
        Tags:           req.Tags,
        CreatedBy:      middleware.UserIDFromContext(r.Context()),
        Status:         "draft",
    }

    created, err := h.CustomTemplateStore.Create(clone)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "about:blank", "Internal Server Error",
            "Failed to clone custom template", r.URL.Path, reqID)
        return
    }

    h.EventPublisher.PublishCustomTemplateCloned(events.CustomTemplateClonedPayload{
        Event:               "custom_template.cloned",
        SourceTemplateID:    id,
        ClonedTemplateID:    created.ID,
        Name:                created.Name,
        Category:            created.Category,
        CreatedAt:           created.CreatedAt,
        CreatedBy:           created.CreatedBy,
        TenantID:            middleware.TenantIDFromContext(r.Context()),
    })

    writeJSON(w, http.StatusCreated, toCustomTemplateResponse(created))
}
```

---

## VERIFICATION COMMANDS

```bash
cd modules/05-department-template-engine

# Build
go build ./...

# Tests (all must pass)
go test ./... -count=1

# Coverage (target: ≥80%)
go test ./... -cover
go test ./internal/handlers/... -cover  # Target: ≥80% handlers

# Contract validation
# Check TemplateDeployRequest required fields match
grep -A 2 "required:" contracts/v1/openapi-05-department-template-engine.yaml | grep -A 2 TemplateDeployRequest
grep -A 2 "required" contracts/v1/schema-05-department-template-engine.json | grep -A 2 TemplateDeployRequest
# Both should show: [version, environment] (no template_id)

# Check tags are declared
grep -c "^  - name:" contracts/v1/openapi-05-department-template-engine.yaml
# Should include "Template Versions" and "Template Operations"

# Check no per-operation security
grep -c "^      security:" contracts/v1/openapi-05-department-template-engine.yaml
# Should be 0

# Check AsyncAPI channels use slash notation
grep "address:" contracts/v1/asyncapi-05-department-template-engine.yaml | grep "operan/events/" | wc -l
# Should be 8 (all channels)

# Check files exist
test -f PROGRESS.md && echo "PROGRESS.md ✅" || echo "PROGRESS.md ❌"
test -f HANDOFF.md && echo "HANDOFF.md ✅" || echo "HANDOFF.md ❌"
test -f Dockerfile && echo "Dockerfile ✅" || echo "Dockerfile ❌"
test -f chart/Chart.yaml && echo "Helm chart ✅" || echo "Helm chart ❌"
```

---

## SIGN-OFF CHECKLIST

Before resubmitting for re-review, verify:

### Contract Drift (P0)
- [ ] `TemplateDeployRequest` required fields reconciled (`[version, environment]` in both)
- [ ] Undeclared tags added (`Template Versions`, `Template Operations`)
- [ ] Per-operation security declarations removed
- [ ] `CustomTemplate.status` references `agentStatus` enum via `$ref`
- [ ] `rolled_back` and `failed` added to AsyncAPI deployment_stage enum
- [ ] AsyncAPI channel names standardized to `operan/events/template/*`

### Event Wiring (P0)
- [ ] `PublishTemplateVersioned` called in version creation handler
- [ ] `PublishTemplateUndeployed` called in deployment undeploy handler
- [ ] Custom template lifecycle events added (4 new channels + handler wiring)

### Security (P1)
- [ ] Clone error handling fixed (json.Unmarshal error checked)
- [ ] JWT config rejects empty secrets
- [ ] Rate-limiting middleware implemented and wired

### Infrastructure (P0)
- [ ] `PROGRESS.md` created
- [ ] `HANDOFF.md` created

### Bug Fixes (P1)
- [ ] Dead `Update` method removed from TemplateStore
- [ ] Dead `GetByIDAndTemplate` removed from DeploymentStore (or tested)
- [ ] `toJSONArray`/`toJSON` helpers tested

### Clone Endpoint (P1)
- [ ] Custom template clone endpoint added (`/templates/custom/{id}/clone`)

### Build & Test
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] Coverage ≥ 80% total, ≥ 80% handlers
