# Module 01 — Tenant Control Plane: Drift Report

**Date:** 2025-07-12  
**Module ID:** 01 — Tenant Control Plane  
**Verdict:** 🔴 **DO NOT MERGE** — 4 P1 drifts + 5 missing handler tests

---

## 1. Drift Summary

| Severity | Count | Description |
|----------|-------|-------------|
| **P1** (Blocking) | 4 | Edge contract missing, PATCH /quota missing, subscription detail endpoints missing, 0% test coverage on 18 handlers |
| **P2** (Important) | 5 | Cross-spec field mismatch, 3 orphan files, secrets response leaks plaintext, handler store injection inconsistency |
| **P3** (Nice-to-have) | 2 | Unused handler store fields, no integration tests |

**Overall Test Coverage: 58.3%** (threshold: ≥80%)

---

## 2. P1 Drifts (Blocking)

### D-001: Edge Contract Missing from Disk
- **Contract side:** `contracts/v1/openapi-01-tenant-control-plane.yaml` defines the API spec
- **Edge contract:** `contracts/v1/edge-01-tenant-control-plane.yaml` is **not found on disk**
- **Index inconsistency:** `Master Contract Index.md` shows ✅ for Edge contract
- **Impact:** Cannot validate tenant-scoped routing convention end-to-end
- **Action:** Create Edge contract or remove checkmark from Index

### D-002: PATCH /tenants/{id}/quota — Endpoint Exists in Contract, Not Registered
- **Contract:** `openapi-01-tenant-control-plane.yaml` defines `updateTenantQuota` (PATCH /tenants/{id}/quota) at line 2426
- **Implementation:** No handler function, no route registration in `RegisterRoutes()`
- **Contract endpoint count:** 39 defined in OpenAPI
- **Routes registered in code:** 37 (see D-003 for others)
- **Handler coverage for `GetTenantQuota`:** 0% — function exists but never called

### D-003: Subscription Detail Endpoints Missing (3 endpoints)
- **Contract defines:**
  - `GET /tenants/{id}/subscriptions/{subscription_id}` → `getSubscription` (line 3450)
  - `PATCH /tenants/{id}/subscriptions/{subscription_id}` → `updateSubscription` (line 3508)
  - `POST /tenants/{id}/subscriptions/{subscription_id}/upgrade` → `upgradeSubscription` (line 3652)
- **Implementation:** Only singular `GetSubscription`, `PatchSubscription`, `CancelSubscription` registered under `/tenants/{id}/subscriptions` (collection paths)
- **Impact:** Cannot manage individual subscriptions by ID — missing key CRUD operations
- **Handler functions:** `GetSubscription` (80%), `PatchSubscription` (54.5%) exist in code but serve different paths than contract

### D-004: 18 Handler Functions Have 0% Test Coverage
These functions are **never invoked by any test**:

| Function | File | Coverage |
|----------|------|----------|
| `GetTenantQuota` | handler_billing.go | 0% |
| `GetBillingUsage` | handler_billing.go | 0% |
| `DownloadInvoice` | handler_billing.go | 0% |
| `ListPaymentMethods` | handler_billing.go | 0% |
| `CreatePaymentMethod` | handler_billing.go | 0% |
| `SetDefaultPaymentMethod` | handler_billing.go | 0% |
| `UpgradePlan` | handler_billing.go | 0% |
| `paymentMethodResponse` | handler_billing.go | 0% |
| `ListSecrets` | handler_secrets_status.go | 0% |
| `CreateSecret` | handler_secrets_status.go | 0% |
| `GetSecret` | handler_secrets_status.go | 0% |
| `UpdateSecret` | handler_secrets_status.go | 0% |
| `DeleteSecret` | handler_secrets_status.go | 0% |
| `RotateSecret` | handler_secrets_status.go | 0% |
| `GetModuleStatus` | handler_secrets_status.go | 0% |
| `parsePositiveInt` | handler_secrets_status.go | 0% |
| `RegisterRoutes` | response_types.go | 0% |
| `NewPaymentMethodStore` | payment_method.go (store) | 0% |

**Impact:** These 18 functions represent ~30% of all handler/store code. Even if fully implemented, bugs here go undetected.

---

## 3. P2 Drifts (Important)

### D-005: Cross-Spec Field Mismatch — `contact_email` vs `admin_email`
- **JSON Schema:** Uses `contact_email` for tenant admin contact
- **OpenAPI:** Uses `admin_email` for the same concept (lines 124, 156, 181)
- **Implementation Go struct:** `TenantPatchRequest` has **both** fields (`contact_email` + `admin_email`)
- **Impact:** API consumers may send either field; behavior is undefined which one takes precedence
- **Resolution:** Align JSON Schema to use `admin_email`; deprecate `contact_email`

### D-006: Secrets Handler Returns Plaintext `value` Field in Responses
- **Files:** `handler_secrets_status.go` lines 102, 141; `SecretResponse` struct
- **Contract:** JSON Schema defines `SecretResponse` with a `value` field (plaintext for creation only)
- **Risk:** `GetSecret` handler returns `secret.Value` (plaintext) in every response
- **Impact:** Security exposure — secret values visible on every GET request
- **Recommendation:** `GetSecret` should return `value: null` or omit the field entirely; only `CreateSecret` should return the plaintext value once

### D-007: Handler Struct Has Unused Store Fields
- **File:** `internal/middleware/middleware.go` line 80 — `Handler` struct
- **Fields:** `PaymentMethodStore`, `EventPublisher`, `AgentStore`, `ResourceStore` are initialized but never injected
- **Impact:** NewHandler() creates these stores unnecessarily; increases memory footprint; masks whether they're actually wired to routes
- **Note:** AgentStore and ResourceStore ARE used by agent/resource handlers — but the initialization in NewHandler is wasteful if stores are created in main.go and passed separately

### D-008: Orphan Files in contracts/v1/
- **3 `.bak` files** — misassigned module 19 files (non-blocking for Module 01)
- **14 unnumbered OpenAPI** + **11 unnumbered JSON Schema** files
- **Impact:** Not Module 01 specific, but pollutes the contracts directory
- **Action:** Clean up as separate task

### D-009: AsyncAPI Events Not Tested
- **Contract:** `asyncapi-01-tenant-control-plane.yaml` — 4 Kafka event channels:
  - `tenant_provisioned`
  - `tenant_suspended`
  - `tenant_deprovisioned`
  - `tenant_quota_exceeded`
- **Implementation:** `events/events.go` — `Publisher` struct with publish methods
- **Test coverage:** 0% — no tests for event publishing
- **Impact:** No guarantee that events are emitted correctly on tenant lifecycle transitions

---

## 4. Non-Drift Items (Passed Validation)

### ✅ Build Pass
```
go build ./... → clean (exit 0, no output)
```

### ✅ No Hardcoded Secrets Scanned
- grep for `api-key`, `token`, `password`, `Bearer`, `PrivateKey` — only found in:
  - Test fixtures (e.g., `"api-key"` as test key name in `secret_test.go`)
  - Variable/parameter names (`secretID`, `secret_id`)
  - No actual hardcoded credentials, API keys, or tokens found

### ✅ Tenant Lifecycle State Machine Correct
- `canTransition()` validates all state transitions against `validTransitions` map
- `canCreate`, `canDelete`, `canUpdate` enforcement in `TenantStore`
- Status constants (`TenantActive`, `TenantSuspended`, `TenantDeleted`) properly defined

### ✅ Store-Level Test Coverage (78.5%)
| Store | Coverage |
|-------|----------|
| `TenantStore` | 95.8% |
| `SubscriptionStore` | 93.8% |
| `SecretStore` | 89.4% |
| `BillingStore` (Invoice) | 89.7% |
| `ResourceStore` | 79.2% |
| `AgentStore` | 83.3% |
| `PaymentMethodStore` | 0% (no tests) |

### ✅ Tenant Context Middleware
- `X-Tenant-ID` header → context injection verified in tests
- `X-Request-ID` and `X-Trace-Id` propagation working

### ✅ HTTP Server Configuration
- Graceful shutdown via signal handling
- Timeouts configured (Read: 15s, Write: 15s, Idle: 60s)

---

## 5. Coverage Gap Detail (Per-Function)

| Function | Coverage | File |
|----------|----------|------|
| `main()` | 0% | cmd/tenant-control-plane/main.go |
| `ParseConfig()` | 0% | internal/config/config.go |
| `getEnvOrDefault()` | 0% | internal/config/config.go |
| `NewPublisher()` + all publish methods | 0% | internal/events/events.go |
| `NewHandler()` | 0% | internal/middleware/middleware.go |
| `NewPaymentMethodStore()` + all methods | 0% | internal/store/payment_method.go |
| **Handler functions with tests** | | |
| `CreateTenant` | 67.5% | handler_tenants.go |
| `ListTenants` | 100% ✅ | handler_tenants.go |
| `GetTenant` | 80% | handler_tenants.go |
| `PatchTenant` | 72.7% | handler_tenants.go |
| `DeleteTenant` | 81.8% | handler_tenants.go |
| `GetTenantStatus` | 85.7% | handler_agents_status.go |
| `TransitionTenantStatus` | 71.4% | handler_agents_status.go |
| `ListAgents` | 90.9% | handler_agents_status.go |
| `CreateAgent` | 73.9% | handler_agents_status.go |
| `GetAgent` | 80% | handler_agents_status.go |
| `PatchAgent` | 55.6% | handler_agents_status.go |
| `DeleteAgent` | 60% | handler_agents_status.go |
| `ListResources` | 85.7% | handler_resources_billing.go |
| `CreateResource` | 65.2% | handler_resources_billing.go |
| `GetResource` | 80% | handler_resources_billing.go |
| `PatchResource` | 55.6% | handler_resources_billing.go |
| `DeleteResource` | 60% | handler_resources_billing.go |
| `ListInvoices` | 85.7% | handler_resources_billing.go |
| `GetInvoice` | 80% | handler_resources_billing.go |
| `UpdateInvoice` | 66.7% | handler_resources_billing.go |
| `GetSubscription` | 80% | handler_resources_billing.go |
| `PatchSubscription` | 54.5% | handler_resources_billing.go |
| `CancelSubscription` | 54.5% | handler_resources_billing.go |

---

## 6. Contract vs Implementation Endpoint Comparison

| Contract Path | HTTP Method | Operation ID | Implemented? | Tested? |
|---------------|-------------|--------------|--------------|---------|
| `/tenants` | GET | listTenants | ✅ | ✅ |
| `/tenants` | POST | createTenant | ✅ | ✅ |
| `/tenants/{id}/agents` | GET | listTenantAgents | ✅ | ✅ |
| `/tenants/{id}/agents` | POST | createTenantAgent | ✅ | ✅ |
| `/tenants/{id}/agents/{agent_id}` | GET | getTenantAgent | ✅ | ✅ |
| `/tenants/{id}/agents/{agent_id}` | PATCH | updateTenantAgent | ✅ | ✅ |
| `/tenants/{id}/agents/{agent_id}` | DELETE | deleteTenantAgent | ✅ | ✅ |
| `/tenants/{id}/resources` | GET | listTenantResources | ✅ | ✅ |
| `/tenants/{id}/resources` | POST | createTenantResource | ✅ | ✅ |
| `/tenants/{id}/resources/{resource_id}` | GET | getTenantResource | ✅ | ✅ |
| `/tenants/{id}/resources/{resource_id}` | DELETE | deleteTenantResource | ✅ | ✅ |
| `/tenants/{id}/resources/{resource_id}` | PATCH | **updateTenantResource** | ⚠️ Route exists as `PatchResource` | ✅ |
| `/tenants/{id}/billing/upgrade-plan` | POST | upgradePlan | ✅ | ❌ |
| `/tenants/{id}` | GET | getTenant | ✅ | ✅ |
| `/tenants/{id}` | PATCH | patchTenant | ✅ | ✅ |
| `/tenants/{id}` | DELETE | deleteTenant | ✅ | ✅ |
| `/tenants/{id}/quota` | GET | getTenantQuota | ✅ | ❌ |
| `/tenants/{id}/quota` | PATCH | **updateTenantQuota** | ❌ MISSING | ❌ |
| `/tenants/{id}/status` | GET | getTenantStatus | ✅ | ✅ |
| `/tenants/{id}/status` | POST | transitionTenantStatus | ✅ | ✅ |
| `/tenants/{id}/billing/invoices` | GET | listBillingInvoices | ✅ | ✅ |
| `/tenants/{id}/billing/invoices/{invoice_id}` | GET | getBillingInvoice | ✅ | ✅ |
| `/tenants/{id}/billing/invoices/{invoice_id}` | PATCH | updateBillingInvoice | ✅ | ✅ |
| `/tenants/{id}/billing/invoices/{invoice_id}/download` | GET | downloadBillingInvoice | ✅ | ❌ |
| `/tenants/{id}/billing/usage` | GET | getBillingUsage | ✅ | ❌ |
| `/tenants/{id}/billing/payment-methods` | GET | listBillingMethods | ✅ | ❌ |
| `/tenants/{id}/billing/payment-methods` | POST | createBillingMethod | ✅ | ❌ |
| `/tenants/{id}/billing/payment-methods/{method_id}` | GET | getBillingMethod | ❌ MISSING | ❌ |
| `/tenants/{id}/billing/payment-methods/{method_id}` | PATCH | updateBillingMethod | ❌ MISSING | ❌ |
| `/tenants/{id}/billing/payment-methods/{method_id}/set-default` | POST | setDefaultBillingMethod | ✅ | ❌ |
| `/tenants/{id}/secrets` | GET | listSecrets | ✅ | ❌ |
| `/tenants/{id}/secrets` | POST | createSecret | ✅ | ❌ |
| `/tenants/{id}/secrets/{secret_id}` | GET | getSecret | ✅ | ❌ |
| `/tenants/{id}/secrets/{secret_id}` | PATCH | updateSecret | ✅ | ❌ |
| `/tenants/{id}/secrets/{secret_id}` | DELETE | deleteSecret | ✅ | ❌ |
| `/tenants/{id}/secrets/{secret_id}/rotate` | POST | rotateSecret | ✅ | ❌ |
| `/tenants/{id}/subscriptions` | GET | listSubscriptions | ✅ | ✅ |
| `/tenants/{id}/subscriptions` | PATCH | updateSubscription | ⚠️ Route exists as `PatchSubscription` | ✅ |
| `/tenants/{id}/subscriptions/{subscription_id}` | GET | getSubscription | ❌ MISSING | ❌ |
| `/tenants/{id}/subscriptions/{subscription_id}` | PATCH | updateSubscription | ❌ MISSING | ❌ |
| `/tenants/{id}/subscriptions/{subscription_id}/cancel` | POST | cancelSubscription | ⚠️ Route exists as `CancelSubscription` | ✅ |
| `/tenants/{id}/subscriptions/{subscription_id}/upgrade` | POST | upgradeSubscription | ❌ MISSING | ❌ |
| `/status` | GET | GetModuleStatus | ✅ | ❌ |

**Totals:**
- **39 contract endpoints** (counting all HTTP methods per path)
- **37 registered** in `RegisterRoutes()` (2 unique paths missing: PATCH quota + GET/PATCH subscription detail)
- **2 missing handlers** (updateTenantQuota, getBillingMethod, updateBillingMethod, getSubscription detail, updateSubscription detail, upgradeSubscription detail)
- **20 endpoints not tested** (including all missing handlers)

---

## 7. Recommended Merge Decision

### Current State: 🔴 **DO NOT MERGE**

The module passes compilation and has decent core tenant CRUD coverage, but the following must be resolved before merge:

#### Must-Fix (P1)
1. **Register or remove the PATCH /tenants/{id}/quota endpoint** — either implement `UpdateTenantQuota` handler or remove from OpenAPI contract
2. **Add subscription detail endpoints** (`GET`, `PATCH`, `POST /upgrade`) OR remove from contract
3. **Add `GET` and `PATCH` handlers for billing methods by ID** — these are in the contract but not implemented
4. **Write tests for the 18 zero-coverage handler functions** — minimum: success path + error path for each

#### Should-Fix (P2)
5. **Resolve `contact_email` vs `admin_email` cross-spec mismatch**
6. **Prevent plaintext `value` from being returned on `GetSecret` responses**
7. **Add tests for AsyncAPI event publishing**
8. **Fix Edge contract / Index inconsistency**

#### Estimated Effort
- P1 fixes: ~2-3 days (implementing 6 missing handlers + writing tests)
- P2 fixes: ~1 day (field alignment, security fix, event tests)

---

*Report generated by Module 01 Validation Gate — Deterministic Review*
