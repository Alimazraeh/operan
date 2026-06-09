# Module 02 (Identity & Access Management) — Developer Remediation Instructions

**Source Review:** `ARCH-reports/Module-02-Identity-Access.md` (Verdict: REJECT)
**Date:** 2026-06-18
**Estimated Effort:** P0 = 3–5 days | P1 = 5–7 days

---

## Important: Items Already Fixed Since Original Review

The following P0 items from the original review **have already been addressed** — verify they still work after your changes:

| Issue | Status | Evidence |
|-------|--------|----------|
| ABAC tenant isolation | ✅ FIXED | `handler_abac.go:137` — store uses `map[string]map[string]ABACPolicy` (tenantID-scoped) |
| `AuditStore.Create()` cross-tenant | ✅ FIXED | `CreateWithTenant()` exists at `audit.go:78`; handlers call `CreateWithTenant` (4 calls in `handler_users.go`) |
| `UserStore` tenant isolation | ✅ FIXED | `GetByTenantAndEmail()`, `List(tenantID)`, `GetByActorID()` |
| `RoleStore` tenant isolation | ✅ FIXED | `byName` key = `tenantID::name`; `List(tenantID)` filters by tenant |
| `ServiceIdentityStore` tenant isolation | ✅ FIXED | `idByTenantAndName` key = `tenantID::name`; `List(tenantID)` filters |
| `AgentIdentityStore` tenant isolation | ✅ FIXED | `Create()` checks tenant; `ListByTenant()` filters |
| `SSOConfigStore` tenant isolation | ✅ FIXED | `byTenant` map keyed by `tenantID` |
| `LDAPConfigStore` tenant isolation | ✅ FIXED | `byTenant` map keyed by `tenantID` |
| `ADConfigStore` tenant isolation | ✅ FIXED | `byTenant` map keyed by `tenantID` |
| `DelegationRoleStore` tenant isolation | ✅ FIXED | `byName` key = `tenantID::name`; `ListDelegations(tenantID)` |

---

## How to Use This Document

Each fix is self-contained with the **exact file**, **line range**, and **what to do**. Work through P0 first, verify `go test ./...` passes after each fix, then move to P1.

**Before starting:**
```bash
cd /Users/alimazraeh/ADRI/Operan/modules/02-identity-access
git checkout -b fix/module-02-arch-remediation
go test ./...    # baseline: note coverage output
```

---

## P0 — Critical Blockers (Must Complete Before Re-Review)

### P0-1: Remove JWT HMAC Fallback from AuthValidator

**File:** `internal/middleware/middleware.go`  
**Lines 41–130** — The `AuthValidator` function

**Current behavior (CRITICAL SECURITY BYPASS):**

```go
// Lines 109–122 — After JWKS/RS256 fails, silently falls back to HMAC
if err != nil || !tokenResult.Valid {
    if tokenSecret != "" {
        tokenResult, err = jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
            }
            return []byte(tokenSecret), nil
        })
    }
}
```

**What to do:**
1. Remove the entire fallback block (lines 109–127).
2. After the first JWKS/RS256 parse attempt fails, directly reject with 401.
3. Add a **separate code path** for service tokens if needed — use a DIFFERENT secret variable (`ServiceTokenSecret` or similar). Do not reuse `tokenSecret` for both RS256 and HMAC.
4. Also fix `ParseAndValidateToken()` in `internal/middleware/adduser_types.go:31–46` — this function uses `jwt.SigningMethodHS256` hardcoded. It should either be removed or use a separate internal secret.

**Verification tests (add to `middleware/auth_test.go`):**
```go
func TestAuthValidator_NoHmacFallback(t *testing.T) {
    // Create a token signed with HMAC
    hmacToken := middleware.GenerateToken("different-secret", "user-1", "user", "tenant-1", "", nil, 60)
    
    handler := middleware.AuthValidator(nil, "https://authentik.operan.internal", "my-jwks-secret", ...)
    
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req.Header.Set("Authorization", "Bearer "+hmacToken)
    
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)
    
    // Should be 401 — HMAC tokens must NOT pass validation
    if w.Code != http.StatusUnauthorized {
        t.Errorf("Expected 401 for HMAC token, got %v", w.Code)
    }
}
```

---

### P0-2: Fail Startup If JWT Secret Is Default

**File:** `internal/config/config.go`  
**Line 30:**
```go
TokenSecret: getEnvString("IAM_TOKEN_SECRET", "change-me-in-production"),
```

**What to do:**
1. Change the default to empty string:
   ```go
   TokenSecret: getEnvString("IAM_TOKEN_SECRET", ""),
   ```
2. Add validation after config loading:
   ```go
   if cfg.TokenSecret == "" {
       fmt.Fprintln(os.Stderr, "FATAL: IAM_TOKEN_SECRET must be set. A default is not acceptable.")
       os.Exit(1)
   }
   if cfg.TokenSecret == "change-me-in-production" {
       fmt.Fprintln(os.Stderr, "FATAL: IAM_TOKEN_SECRET still has default value")
       os.Exit(1)
   }
   ```

---

### P0-3: Fix generateID() to Use crypto/rand

**File:** `internal/middleware/middleware.go`  
**Lines 215–217:**
```go
func generateID() string {
    return "00000000-0000-0000-0000-000000000001"
}
```

**What to do:**
```go
func generateID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
```

Also fix `GenerateToken()` in `adduser_types.go:20–27` — it accepts a hardcoded secret. Either add a `GenerateServiceToken()` variant that reads from config, or make the secret a parameter that `main.go` passes in.

---

### P0-4: Create Dockerfile

**Location to create:** `/Users/alimazraeh/ADRI/Operan/modules/02-identity-access/Dockerfile`

**What to do:**
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /identity-access ./cmd/identity-access

# Runtime stage
FROM scratch
WORKDIR /app
COPY --from=builder /identity-access /app/identity-access
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8002
USER nobody
ENTRYPOINT ["/app/identity-access"]
```

**Verification:** `docker build -t operan-identity-access .` — must succeed.

---

### P0-5: Create README.md

**Location to create:** `/Users/alimazraeh/ADRI/Operan/modules/02-identity-access/README.md`

**Contents (at minimum):**
- Module purpose and scope
- Configuration reference (all env vars from `config.go`)
- Authentik integration setup
- Docker run instructions
- List of all endpoints (or link to OpenAPI contract)
- How to run tests
- How to run locally (no Docker)

---

### P0-6: Add Helm Chart

**Location to create:**
- `helm/Chart.yaml`
- `helm/templates/deployment.yaml`
- `helm/templates/service.yaml`
- `helm/values.yaml`

**What to do:** Follow the same pattern as other modules. At minimum, create a valid `Chart.yaml` with name `operan-identity-access`, version `0.1.0`, description, and a `values.yaml` with image, replicas, port (8002), and env var overrides.

---

### P0-7: Reconcile Cross-Spec Contract Drifts

**Files involved:**
- `contracts/v1/openapi-02-identity-access.yaml`
- `contracts/v1/schema-02-identity-access.json`
- `contracts/v1/asyncapi-02-identity-access.yaml`

#### 7a: SSOConfig Provider Enum

| Contract | Current Values |
|----------|----------------|
| **OpenAPI** | `azure_ad`, `okta`, `authentik`, `google_workspace` |
| **JSON Schema** | `azure_ad`, `google_workspace`, `okta`, `keycloak`, `custom` |

**Action:** Reconcile to a single superset: `["azure_ad", "google_workspace", "okta", "keycloak", "custom", "authentik"]` in all three contracts.

#### 7b: AuditEvent vs AuditLog Structure

The OpenAPI `AuditEvent` and JSON Schema `AuditLog` have fundamentally different fields:

| Field | OpenAPI | JSON Schema |
|-------|---------|-------------|
| `tenant_id` | ❌ missing | ✅ present (required) |
| `actor_type` | ✅ `user`/`service_identity`/`agent_identity` | ❌ missing |
| `details` | ✅ `type: object` | ✅ exists |
| `metadata` | ❌ missing | ✅ `type: object` |
| `severity` | ❌ missing | ✅ enum `[low, medium, high, critical]` |
| `result` | ✅ `success/failure/denied` | ✅ `success/failure` |

**Action:** Reconcile to the richer JSON Schema model. Add `tenant_id`, `actor_type`, `metadata`, `severity` to OpenAPI `AuditEvent`. Keep OpenAPI's `denied` in result enum (useful for RBAC context).

#### 7c: PrincipalType Naming — OpenAPI vs AsyncAPI

| Contract | Enum |
|----------|------|
| **OpenAPI** `AuditEvent.actor_type` | `user`, `service_identity`, `agent_identity` |
| **AsyncAPI** `PrincipalType` | `user`, `service`, `agent` |

**Action:** Align to `["user", "service", "agent"]` across all three contracts (shorter names, consistent with AsyncAPI). Update the Go models/handlers to match.

#### 7d: Role Representation — OpenAPI vs JSON Schema

OpenAPI uses string arrays `["department_admin"]` for `User.roles`; JSON Schema uses UUID arrays for `User.role_ids`.

**Action:** Architecturally recommended: use `role_ids` (UUIDs) everywhere. Update OpenAPI `User.roles` to `role_ids: [string (format: uuid)]` and update the handler to accept/return UUIDs.

---

## P1 — High-Priority Gaps (Fix Before Production Sign-Off)

### P1-8: Session Replay Cache — Add Size Limits & TTL

**File:** `internal/middleware/session_replay.go`  
**Current state:** `sessions map[string]*ReplaySession` — no max size, no TTL eviction. `CleanupOldSessions()` exists but is never called by any goroutine.

**What to do:**
1. Add max size constant: `const maxSessions = 10000`
2. Add max requests per session: `const maxRequestsPerSession = 500`
3. In `Capture()`, enforce max requests per session:
   ```go
   if len(session.Requests) >= maxRequestsPerSession {
       session.Requests = session.Requests[len(session.Requests)-maxRequestsPerSession+1:]
   }
   ```
4. In `SaveSession()`, evict oldest sessions if count exceeds max:
   ```go
   if len(c.sessions) > maxSessions {
       // Remove oldest session by StartedAt
       var oldestID string
       var oldestTime time.Time
       for id, s := range c.sessions {
           if oldestID == "" || s.StartedAt.Before(oldestTime) {
               oldestID = id
               oldestTime = s.StartedAt
           }
       }
       if oldestID != "" {
           delete(c.sessions, oldestID)
       }
   }
   ```
5. Start a cleanup goroutine or use a timer-based eviction.

---

### P1-9: TenantManager Cache — Add TTL Eviction

**File:** `internal/authentik/tenant_manager.go`  
**Current state:** `cache map[string]*TenantState` with no TTL, no max size, no eviction.

**What to do:**
1. Add TTL and max size to `TenantState`:
   ```go
   type TenantState struct {
       // ... existing fields
       CachedAt     time.Time
       TTL          time.Duration
   }
   ```
2. Add a TTL field to the TenantManager:
   ```go
   type TenantManager struct {
       // ... existing fields
       defaultTTL time.Duration
   }
   ```
3. In `GetTenantState()`, check if cached state is expired:
   ```go
   if state, ok := tm.cache[tenantID]; ok {
       if time.Since(state.CachedAt) > state.TTL {
           delete(tm.cache, tenantID)
           state = nil
       }
   }
   ```
4. Use a periodic cleanup goroutine or add a max cache size with LRU eviction.

---

### P1-10: Wire Event Publishing to AMQP

**File:** `internal/events/events.go`  
**Current state:** `Publish()` only logs. 14 event types defined but none published to broker.

**What to do:**
1. Add `github.com/rabbitmq/amqp091-go` dependency.
2. Update `NewPublisher()` to accept connection params and establish a connection/channel:
   ```go
   type Publisher struct {
       channel *amqp091.Channel
       conn    *amqp091.Connection
   }
   func NewPublisher(brokerURL string) (*Publisher, error) {
       conn, err := amqp091.Dial(brokerURL)
       if err != nil { return nil, err }
       ch, err := conn.Channel()
       if err != nil { return nil, err }
       return &Publisher{conn: conn, channel: ch}, nil
   }
   ```
3. Replace log-only stub in `Publish()` with:
   ```go
   err := p.channel.PublishWithContext(ctx, "", exchange, false, false, amqp091.Publishing{
       ContentType: "application/json",
       Body:        data,
   })
   if err != nil {
       p.logger.Printf("[IAM Events] publish error: %v", err)
   }
   ```
4. Add `Close()` method for graceful shutdown.

---

### P1-11: Add Rate-Limiting Middleware

**File to create:** `internal/middleware/rate_limiter.go`

**What to do:**
1. Implement a sliding-window rate limiter per tenant:
   ```go
   type RateLimiter struct {
       requests map[string][]time.Time
       mu       sync.Mutex
       maxReq   int
       window   time.Duration
   }
   ```
2. Create middleware:
   ```go
   func RateLimit(limiter *RateLimiter) func(next http.Handler) http.Handler
   ```
3. Integrate into `main.go`'s middleware chain (after `TenantInjector`, before `AuthValidator`).
4. Configurable limits via env vars (e.g., `IAM_RATE_LIMIT_MAX=100`, `IAM_RATE_LIMIT_WINDOW=1m`).

---

### P1-12: Achieve ≥80% Handler Test Coverage

**Current state:** 139 tests across 7 test files, ~16% handler coverage.

**Untested handler files (9 of 11):**

| Handler File | Lines | Priority | What to Test |
|--------------|-------|----------|-------------|
| `handler_mfa.go` | ~400 | HIGH | Enroll, Verify, Disable, List, RecoveryCodes |
| `handler_sso.go` | ~1300 | HIGH | ConfigureSSO, TestSSO, SCIM CRUD operations |
| `handler_ldap.go` | ~400 | MEDIUM | ConfigureLDAP, TestLDAP |
| `handler_ad.go` | ~400 | MEDIUM | ConfigureAD, TestAD |
| `handler_delegations.go` | ~700 | HIGH | CRUD delegation roles, Grant/Revoke |
| `handler_identity.go` | ~500 | MEDIUM | CreateServiceIdentity, CreateAgentIdentity |
| `handler_audit_rbac.go` | ~777 | HIGH | RBACEvaluate, AuditTrail endpoints |
| `handler_abac.go` | ~708 | MEDIUM | EvaluateABAC (partial) |

**Strategy:**
1. Split tests into per-handler files: `handler_mfa_test.go`, `handler_sso_test.go`, etc.
2. For each handler function, write: success test + 2–3 error path tests.
3. Use `httptest.NewRequest` and `httptest.NewRecorder` pattern (see existing `handler_users_test.go` for conventions).
4. Mock the Authentik client interface for SSO/SCIM/MFA tests.

---

### P1-13: Write Tenant-Isolation Tests

**Files to create:**
- `internal/handler/handler_abac_isolation_test.go` — verify tenant-A cannot read tenant-B ABAC policies
- `internal/store/audit_tenant_test.go` — verify `CreateWithTenant()` enforces tenant

**Example test:**
```go
func TestABACTenantIsolation_CannotReadCrossTenant(t *testing.T) {
    store := handler_abac.NewABACStore()
    store.Create("tenant-A", ABACPolicy{ID: "p1", Name: "A-Policy"})
    
    policy, ok := store.GetByID("tenant-B", "p1")
    if ok {
        t.Errorf("tenant-B should not see tenant-A's policy")
    }
}
```

---

### P1-14: Add PostgreSQL Migration Files

**Location to create:** `db/migrations/`

**What to do:**
1. Create `db/migrations/001_create_tables.up.sql`:
   ```sql
   CREATE TABLE users (
       id UUID PRIMARY KEY,
       tenant_id UUID NOT NULL,
       email VARCHAR(255) NOT NULL,
       ...
   );
   CREATE TABLE roles (
       id UUID PRIMARY KEY,
       tenant_id UUID NOT NULL,
       name VARCHAR(255) NOT NULL,
       ...
   );
   CREATE TABLE audit_events (
       id UUID PRIMARY KEY,
       tenant_id UUID NOT NULL,
       ...
   );
   -- And so on for: service_identities, agent_identities,
   -- sso_configs, ldap_configs, ad_configs, delegation_roles
   ```
2. Create corresponding `.down.sql` files.
3. Add migration runner code in `main.go`.

---

### P1-15: Remove Compiled Binary from Module Root

**Action:** Add `identity-access` to `.gitignore` in the module root directory, or delete it. This binary suggests a manual build step outside of CI/CD.

---

### P1-16: Add `/ready` Endpoint with Dependency Checks

**Current state:** `/health` exists at `main.go:535` but returns `{"status":"healthy"}` with no dependency checks. `/ready` is registered at line 568 but likely also empty.

**What to do:**
- Check database connectivity (if PostgreSQL is wired)
- Check RabbitMQ connection status
- Check Authentik API connectivity (ping)
- Return: `{"status":"ready","db":"ok","rabbitmq":"ok","authentik":"ok","uptime":"5m30s"}`

---

## P2 — Medium-Priority Enhancements

### P2-17: Test Config and Event Packages

### P2-18: Implement `matchesScimFilter` Fully
Current implementation ignores filter patterns. Implement proper SCIM filter parsing (e.g., `userName co "john"`).

### P2-19: Implement `evaluateIPPolicy` with CIDR Parsing
Currently a stub. Parse CIDR ranges and check against client IP.

### P2-20: Implement `evaluateCustomPolicy` with Rule Engine
Currently returns `allow`. Implement CEL or similar rule engine.

### P2-21: Add OpenTelemetry Instrumentation
Add request tracing, span propagation.

### P2-22: Add `/status` Module Health Endpoint
Similar to Module 01's `GetModuleStatus` — return version, startup time, feature flags.

---

## Developer Sign-Off Checklist

**Before requesting re-review, verify each item:**

**P0 (All must pass):**
- [ ] P0-1: JWT HMAC fallback removed; service tokens use separate secret (or removed)
- [ ] P0-2: Startup fails if `IAM_TOKEN_SECRET` is empty or default
- [ ] P0-3: `generateID()` returns unique IDs (crypto/rand)
- [ ] P0-4: Dockerfile created; `docker build` succeeds
- [ ] P0-5: README.md exists
- [ ] P0-6: Helm chart exists (Chart.yaml + values.yaml at minimum)
- [ ] P0-7: Cross-spec drifts resolved (SSO providers, AuditEvent structure, PrincipalType naming, role representation)

**P1 (Verify before production sign-off):**
- [ ] P1-8: Session replay cache bounded (max sessions, max requests per session)
- [ ] P1-9: TenantManager cache has TTL eviction
- [ ] P1-10: Event publishing wired to AMQP (or documented as stub in README)
- [ ] P1-11: Rate-limiting middleware implemented
- [ ] P1-12: Handler test coverage ≥ 80%
- [ ] P1-13: Tenant-isolation tests pass (ABAC, AuditStore)
- [ ] P1-14: PostgreSQL migration files exist
- [ ] P1-15: Compiled binary removed from `.gitignore`
- [ ] P1-16: `/ready` endpoint with dependency checks

**P2 (For production readiness):**
- [ ] P2-17 through P2-22: Phase 3 items completed

---

## Post-Fix Commands

After completing all P0 items:
```bash
cd /Users/alimazraeh/ADRI/Operan/modules/02-identity-access
go build ./...
go test ./... -cover
docker build -t operan-identity-access .
```

Then notify ARCH for re-review.

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18  
**CODER Acknowledgment:** _________________    **Date:** _________________  
**Re-Review Requested:** _________________    **Date:** _________________  
**Re-Review Verdict:** _________________    **Date:** _________________
