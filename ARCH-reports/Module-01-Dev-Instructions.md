# Module 01 (Tenant Control Plane) — Developer Remediation Instructions

**Source Review:** `ARCH-reports/Module-01-Tenant-Control-Plane.md` (Verdict: REJECT)
**Date:** 2026-06-18
**Estimated Effort:** P0 = 3–5 days | P1 = 5–7 days

---

## How to Use This Document

Each fix is self-contained with the **exact file**, **line range**, and **what to do**. Work through P0 first, verify `go test ./...` passes after each fix, then move to P1.

**Before starting:**
```bash
cd /Users/alimazraeh/ADRI/Operan/modules/01-tenant-control-plane
git checkout -b fix/module-01-arch-remediation
go test ./...    # baseline: note coverage output
```

---

## P0 — Critical Blockers (Must Complete Before Re-Review)

### P0-1: Replace XOR Encryption with AES-256-GCM

**File:** `internal/store/secret.go`  
**Current code (line ~260):**
```go
func encryptValue(plaintext string) string {
    return fmt.Sprintf("ENC:%x", []byte(plaintext))
}
```

**What to do:**
1. Add `crypto/aes`, `crypto/cipher`, `crypto/rand` to imports.
2. Read the `JWTSecret` from config (`module/config/config.go`) as the encryption key.
3. Implement AES-256-GCM:
   ```go
   func encryptValue(key, plaintext string) (string, error) {
       block, err := aes.NewCipher([]byte(key))
       if err != nil { return "", err }
       aead, err := cipher.NewGCM(block)
       if err != nil { return "", err }
       nonce := make([]byte, aead.NonceSize())
       rand.Read(nonce)
       ciphertext := aead.Seal(nonce, nonce, []byte(plaintext), nil)
       return fmt.Sprintf("AES256:%x", ciphertext), nil
   }
   ```
4. Update every call site (`secret.go` lines 78, 233) to pass the key and handle the error.
5. Store `EncryptedValue` with `AES256:` prefix. The `Value` field still stores plaintext for in-memory use.

**Verification:** Write at least 2 unit tests in `secret_test.go`:
- `TestEncryptDecrypt_RoundTrip` — encrypt then decrypt matches original
- `TestEncrypt_DifferentNonces` — same plaintext produces different ciphertext

---

### P0-2: Implement JWT Validation Middleware

**File:** `internal/middleware/middleware.go`  
**Config field:** `module/config/config.go` has `JWTSecret string`

**What to do:**
1. Add `github.com/golang-jwt/jwt/v5` to `go.mod`:
   ```bash
   go get github.com/golang-jwt/jwt/v5
   ```

2. Add a new middleware function in `middleware.go` (insert **before** `TenantContext` in the chain):
   ```go
   func JWTValidator(secret, issuer string) func(next http.Handler) http.Handler {
       return func(next http.Handler) http.Handler {
           return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
               tokenStr := r.Header.Get("Authorization")
               if tokenStr == "" {
                   http.Error(w, "missing authorization header", http.StatusUnauthorized)
                   return
               }
               if !strings.HasPrefix(tokenStr, "Bearer ") {
                   http.Error(w, "invalid authorization scheme", http.StatusUnauthorized)
                   return
               }
               tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
               
               token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
                   if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                       return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                   }
                   return []byte(secret), nil
               })
               if err != nil || !token.Valid {
                   http.Error(w, "invalid token", http.StatusUnauthorized)
                   return
               }
               
               claims, ok := token.Claims.(jwt.MapClaims)
               if !ok {
                   http.Error(w, "invalid token claims", http.StatusUnauthorized)
                   return
               }
               
               // Validate issuer
               if iss, ok := claims["iss"].(string); !ok || iss != issuer {
                   http.Error(w, "invalid token issuer", http.StatusUnauthorized)
                   return
               }
               
               // Extract tenant_id and user_id from claims
               if tid, ok := claims["tenant_id"].(string); ok {
                   ctx := context.WithValue(r.Context(), ctxKeyTenantID, tid)
                   r = r.WithContext(ctx)
               }
               if uid, ok := claims["user_id"].(string); ok {
                   ctx := context.WithValue(r.Context(), ctxKeyUserID, uid)
                   r = r.WithContext(ctx)
               }
               
               next.ServeHTTP(w, r)
           })
       }
   }
   ```

3. Add `GetJWTSecret()` and `GetIssuer()` helper functions to the config package.
4. In `cmd/tenant-control-plane/main.go` (or wherever the middleware chain is built), insert `JWTValidator(config.JWTSecret, config.Issuer)` before `RequestID`.

**Verification:** Write tests in `middleware/middleware_test.go`:
- `TestJWTValidator_ValidToken` — 200 OK
- `TestJWTValidator_ExpiredToken` — 401
- `TestJWTValidator_WrongIssuer` — 401
- `TestJWTValidator_MissingHeader` — 401

---

### P0-3: Add Tenant-Scoped Store Methods

**Files:** All 12 store files in `internal/store/`

**Current pattern (every store):**
```go
func (s *TenantStore) GetByID(id string) (*Tenant, error)  // NO tenant check
```

**What to do for EACH of the 12 stores:**

1. Add a `GetByIDAndTenant` method:
   ```go
   func (s *TenantStore) GetByIDAndTenant(id, tenantID string) (*Tenant, error) {
       s.mu.RLock()
       defer s.mu.RUnlock()
       t, ok := s.tenants[id]
       if !ok {
           return nil, fmt.Errorf("tenant %s not found", id)
       }
       if t.TenantID != tenantID {
           return nil, fmt.Errorf("tenant %s does not belong to tenant context %s", id, tenantID)
       }
       cpy := *t
       return &cpy, nil
   }
   ```

2. Add a `GetByTenant` variant where the store method is currently called with just an ID:
   - `TenantStore.GetByTenant(id)` — already exists as `GetByTenant(tenantID)` ✅
   - `SecretStore.GetByTenant(id, secretID)` — add
   - `SubscriptionStore.GetByTenant(id, subID)` — add (currently only has `GetByTenant(tenantID)` for list)
   - `AgentStore.GetByIDAndTenant(id, tenantID)` — add
   - `NamespaceStore.GetByIDAndTenant(id, tenantID)` — add
   - `ResourceStore.GetByIDAndTenant(id, tenantID)` — add
   - `DeploymentStore.GetByIDAndTenant(id, tenantID)` — add
   - `PolicyStore.GetByIDAndTenant(id, tenantID)` — add
   - `EnvironmentStore.GetByIDAndTenant(id, tenantID)` — add
   - `InvoiceStore.GetByTenant(tenantID, invoiceID)` — add (currently `BillingStore.GetByID`)
   - `PaymentMethodStore.GetByTenant(tenantID, pmID)` — add

3. **Update every handler** to extract `tenantID` from context and pass it to store methods. The `TenantContext` middleware already puts it in context. Add a helper:
   ```go
   func GetTenantID(ctx context.Context) string {
       v := ctx.Value(ctxKeyTenantID)
       if v == nil {
           return ""
       }
       return v.(string)
   }
   ```

4. **Find and fix all `GetByID` calls in handlers.** There are 46 occurrences across:
   - `handler_secrets_status.go` lines 31, 122
   - `handler_deployments.go` lines 61, 94, 149, 328, 353, 381, 400, 425
   - `handler_billing.go` lines 25, 72, 159, 398
   - `handler_namespaces.go` lines 55, 88, 141, 203, 227
   - `handler_resources_billing.go` lines 24, 73, 145, 258, 628, 707, 802
   - `handler_policies.go` line 183
   - `handler_agents_status.go` lines 24, 104, 161, 227
   - `handler_tenants.go` line 160
   - `handler_environments.go` lines 240, 371

   Replace `h.TenantStore.GetByID(id)` with `h.TenantStore.GetByIDAndTenant(id, tenantID)`.

5. For cross-references (e.g., `SubscriptionStore.GetByID(subID)`), verify tenant ownership with `GetByTenant(tenantID, subID)`.

**Verification:** Write tenant-isolation integration tests:
```go
func TestTenantIsolation_CannotAccessCrossTenant(t *testing.T) {
    // Create tenant-A and tenant-B data
    // Call GET with tenant-A context and tenant-B ID
    // Verify 404 is returned
}
```

---

### P0-4: Register `RollbackDeployment` in `response_types.go`

**File:** `internal/handler/response_types.go`  
**Current state:** `RollbackDeployment` function exists at `handler_deployments.go:241` but is **NOT registered** in the route map.

**What to do:**
Add this line after the other deployment routes (around line 442):
```go
mux.HandleFunc("POST /v1/tenants/{id}/deployments/{deployment_id}/rollback", RollbackDeployment(h))
```

**Contract alignment:** The OpenAPI contract (line 3747 of `openapi-01-tenant-control-plane.yaml`) uses path parameter `deploymentId` (camelCase). The handler uses `deployment_id` (snake_case). **Use the handler's snake_case** — it matches all other registered routes. If the contract team agrees, they should update the OpenAPI path from `deploymentId` to `deployment_id`.

**Verify:** `RollbackDeployment` should also extract tenantID from context and verify tenant ownership before calling `DeploymentStore.Rollback()`.

---

### P0-5: Reconcile `contact_email` vs `admin_email`

**Files:**
- `internal/handler/response_types.go` line 25: `AdminEmail string \`json:"admin_email,omitempty"\``
- `internal/handler/handler_tenants.go` line 37: local `AdminEmail` field
- `internal/handler/handler_tenants.go` lines 92, 199, 256: references
- `internal/handler/handler_billing.go` line 119: `AdminEmail` in response

**What to do:**
1. Rename ALL occurrences of `AdminEmail` / `admin_email` to `ContactEmail` / `contact_email` across ALL handler files.
2. In `internal/store/tenant.go`, verify the `Tenant` struct has `ContactEmail` (it should already — this was the source of truth).
3. **Update contracts:** Reconcile `openapi-01-tenant-control-plane.yaml` and `schema-01-tenant-control-plane.json` to use `contact_email` everywhere.

**Verification:** Run `grep -r "admin_email" internal/` — should return zero results.

---

### P0-6: Fix Plan Enum Drift — AsyncAPI ≠ OpenAPI/JSON Schema

**Files:**
- `asyncapi-01-tenant-control-plane.yaml` line 129: defines `PlanType` as `["free", "team", "enterprise", "custom"]`
- `openapi-01-tenant-control-plane.yaml`: defines PlanType as `["saas", "enterprise", "sovereign"]`
- `schema-01-tenant-control-plane.json`: defines PlanType as `["saas", "enterprise", "sovereign"]`

**What to do:**
1. Update `asyncapi-01-tenant-control-plane.yaml` to change the `PlanType` enum from `["free", "team", "enterprise", "custom"]` to `["saas", "enterprise", "sovereign"]`.
2. Update any event payloads in the AsyncAPI file that reference `free`, `team`, or `custom` plan values.
3. In the Go store (`internal/store/tenant.go`), verify the `Plan` type enum matches — it should be `saas`, `enterprise`, `sovereign`.

**Verification:** After the fix, all three contract files should agree on the same 4 plan values.

---

### P0-7: Register `PatchSubscription` Handler

**File:** `internal/handler/response_types.go` line 405
```go
mux.HandleFunc("PATCH /v1/tenants/{id}/subscriptions", PatchSubscription(h))
```

**Status:** ✅ Already implemented — `PatchSubscription` exists at `handler_resources_billing.go:490` and IS registered. No action needed.

---

### P0-8: Create Dockerfile

**Location to create:** `/Users/alimazraeh/ADRI/Operan/modules/01-tenant-control-plane/Dockerfile`

**What to do:**
Create a multi-stage Dockerfile (Alpine builder → scratch runtime, non-root user):
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tenant-control-plane ./cmd/tenant-control-plane

# Runtime stage
FROM scratch
WORKDIR /app
COPY --from=builder /tenant-control-plane /app/tenant-control-plane
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/tenant-control-plane"]
```

**Verification:** Run `docker build -t operan-tenant-control-plane .` — must succeed. Then `docker run --rm -p 8080:8080 operan-tenant-control-plane` — must start the HTTP server.

---

### P0-9: Create README.md

**Location to create:** `/Users/alimazraeh/ADRI/Operan/modules/01-tenant-control-plane/README.md`

**Contents (at minimum):**
- Module purpose and scope
- Configuration reference (all env vars from config package)
- Docker run instructions
- Health endpoint documentation (`GET /v1/status`)
- How to run tests
- How to run locally (no Docker)
- List of all endpoints (or link to OpenAPI contract)

---

## P1 — High-Priority Gaps (Fix Before Production Sign-Off)

### P1-9: Achieve ≥80% Handler Test Coverage

**Current state:** ~25% coverage, all tests in one file (`handler/handler_test.go`).

**Untested handler files (total ~1,800 lines):**

| Handler File | Lines | Priority | What to Test |
|--------------|-------|----------|-------------|
| `handler_billing.go` | ~316 | HIGH | GetTenantQuota, PatchTenantQuota, GetBillingUsage, DownloadInvoice, plan upgrade |
| `handler_deployments.go` | ~200 | HIGH | Deployment lifecycle (deploy, stop, deprecate, scale, rollout) |
| `handler_environments.go` | ~150 | MEDIUM | Create/Activate/Deactivate environments, isolation config |
| `handler_namespaces.go` | ~150 | MEDIUM | Create/Delete namespaces, quota check |
| `handler_policies.go` | ~200 | HIGH | Create/EvaluatePolicies/CheckPolicyCompliance, GetPolicyStats |
| `handler_secrets_status.go` | ~470 | HIGH | Rotate, Update, Delete secrets, GetModuleStatus |
| `handler_resources_billing.go` | ~525 | MEDIUM | (Partial) Billing endpoints already tested |

**Strategy:**
1. Split tests into per-handler files matching handler file names (e.g., `handler_billing_test.go`, `handler_deployments_test.go`).
2. For each handler function, write: success test + 2–3 error path tests (missing params, invalid body, not-found, tenant not found).
3. Use `httptest.NewRequest` and `httptest.NewRecorder` pattern (see existing `handler_test.go` for conventions).

---

### P1-10: Add PostgreSQL Migration Files

**What to do:**
1. Create `db/migrations/` directory.
2. Create at least one migration file: `001_create_tenants.up.sql`:
   ```sql
   CREATE TABLE tenants (
       id UUID PRIMARY KEY,
       tenant_id UUID NOT NULL,
       name VARCHAR(255) NOT NULL,
       display_name VARCHAR(255),
       status VARCHAR(50) NOT NULL,
       plan VARCHAR(50) NOT NULL,
       region VARCHAR(100),
       isolation_level VARCHAR(50),
       contact_email VARCHAR(255),
       custom_metadata JSONB,
       quota_config JSONB,
       created_at TIMESTAMPTZ DEFAULT NOW(),
       updated_at TIMESTAMPTZ DEFAULT NOW()
   );
   ```
3. Create corresponding `.down.sql` files.
4. Add migration runner code in `cmd/tenant-control-plane/main.go`.

---

### P1-11: Wire Event Publishing to Kafka/AMQP

**File:** `internal/events/events.go`  
**Current:** `publish()` is a log-only stub (line ~135)

**What to do:**
1. Add a Kafka or AMQP client dependency (use `github.com/confluentinc/confluent-kafka-go/kafka` or `github.com/rabbitmq/amqp091-go`).
2. Update `NewPublisher()` to accept connection parameters from config.
3. Replace `log.Printf` in `publish()` with actual broker publish:
   ```go
   func (p *Publisher) publish(topic string, data []byte) {
       producer.Publish(&kafka.Message{
           TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
           Value:          data,
       })
   }
   ```
4. Document in README whether this is wired or stubbed.

---

### P1-12: Initialize Redis Client for Quota/Rate-Limit Tracking

**What to do:**
1. Add Redis client dependency (`github.com/redis/go-redis/v9`).
2. Initialize in `cmd/tenant-control-plane/main.go`.
3. Wire Redis into the `GetTenantQuota` and `PatchTenantQuota` handlers for real-time usage tracking.

---

### P1-13: Integration Tests with Testcontainers

**What to do:**
1. Add `github.com/testcontainers/testcontainers-go`.
2. Create `internal/handler/integration_test.go` with Dockerized PostgreSQL/Redis tests.
3. Test at least: tenant CRUD with real DB, tenant isolation (cross-tenant query returns 404).

---

### P1-14: `/health` Endpoint with DB + Redis Connectivity Checks

**File:** `internal/handler/handler_secrets_status.go`  
**Current:** `GetModuleStatus` (line 266) exists but only returns uptime and "ok" status.

**What to do:**
1. Add DB ping and Redis ping checks.
2. Return detailed health: `{"status": "ok", "db": "ok", "redis": "ok", "uptime": "5m30s"}`.

---

### P1-15: Create Helm Chart

**What to do:**
1. Create `helm/Chart.yaml` with module name, version, description.
2. Create `helm/templates/deployment.yaml`, `service.yaml`, `ingress.yaml`.
3. Create `helm/values.yaml` with configurable image, replicas, port, env vars.

---

## P2 — Medium-Priority Enhancements

### P2-17: Test Config and Event Packages

### P2-18: Add Plan Upgrade Tier Validation
In `UpgradePlan` handler (`handler_billing.go:370`) and `UpgradeSubscription` handler (`handler_resources_billing.go:764`), validate that the new plan tier is higher than the current one:
```go
var planTiers = map[store.Plan]int{
    "saas": 1, "enterprise": 2, "sovereign": 3,
}
if planTiers[req.NewPlan] <= planTiers[tenant.Plan] {
    h.WriteError(w, http.StatusBadRequest, 400, "validation failed", "new plan must be higher tier")
    return
}
```

### P2-19: Wire Missing Events
- `PublishTenantSuspended` → call from `TransitionTenantStatus` handler when status changes to `suspended`
- `PublishTenantQuotaExceeded` → call from quota check logic when usage exceeds limit
- `PublishTenantDeprovisioned` → already called in `DeleteTenant` ✅

---

## Developer Sign-Off Checklist

Before requesting re-review, verify each item:

**P0 (All must pass):**
- [ ] P0-1: XOR → AES-256-GCM (verified by `go test ./internal/store/... -run Encrypt`)
- [ ] P0-2: JWT middleware implemented and tested
- [ ] P0-3: All `GetByID` calls updated to tenant-scoped variants
- [ ] P0-4: No `admin_email` references in codebase
- [ ] P0-5: All 3 contracts agree on plan enum
- [ ] P0-6: `RollbackDeployment` registered in response_types.go
- [ ] P0-7: `docker build` succeeds
- [ ] P0-8: Tenant-isolation tests pass
- [ ] P0-8.5: `PatchSubscription` verified as registered ✅ (no action)
- [ ] P0-9: README.md exists

**P1 (Verify before production sign-off):**
- [ ] P1-9: `go test -cover ./internal/handler/...` ≥ 80%
- [ ] P1-10: Migration files exist in `db/migrations/`
- [ ] P1-11: Event publishing wired to broker (or documented as stub in README)
- [ ] P1-12: Redis client initialized
- [ ] P1-13: Integration tests pass
- [ ] P1-14: `/health` returns 200 with DB + Redis status
- [ ] P1-15: Helm chart exists

**P2 (For production readiness):**
- [ ] P2-17: Config and event packages tested
- [ ] P2-18: Plan upgrade tier validation in place
- [ ] P2-19: All lifecycle events wired

---

## Post-Fix Commands

After completing all P0 items:
```bash
cd /Users/alimazraeh/ADRI/Operan/modules/01-tenant-control-plane
go build ./...
go test ./... -cover
docker build -t operan-tenant-control-plane .
```

Then notify ARCH for re-review.

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18  
**CODER Acknowledgment:** _________________    **Date:** _________________  
**Re-Review Requested:** _________________    **Date:** _________________  
**Re-Review Verdict:** _________________    **Date:** _________________
