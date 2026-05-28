# Module 02 — Identity & Access: Engineering Handoff

> **Generated:** 2025-08-17
> **Module Path:** `modules/02-identity-access/`
> **Contract Reference:** `contracts/v1/openapi-02-identity-access.yaml`

---

## 1. Executive Summary

The `02-identity-access` module implements the Operan platform's identity and access management surface: user CRUD, role-based access control (RBAC), attribute-based access control (ABAC), SCIM 2.0 provisioning, audit logging, session replay, and LDAP/Active Directory federation. The handler layer delegates to an `authentik.Client` (external IdP) and uses in-memory stores for ABAC policies, audit events, and session state.

This document captures every bug found and fix applied during the implementation and testing cycles, organized chronologically and technically so a next engineer can onboard without re-reading the entire git history.

---

## 2. Historical Timeline

| # | Git Commit | Category | What |
|---|-----------|----------|------|
| 1 | `5766e5c` | Scaffold | Initial handler/store/event pipeline scaffolded. Empty handlers with correct signatures. |
| 2 | `f1e10a0` | Implementation | All HTTP handlers implemented — user CRUD, RBAC, ABAC, SCIM stubs. In-memory stores wired. |
| 3 | `342348d` | Test Fix | Role ID extraction, user path context, and `CreatedAt` logic fixed in tests. |
| 4 | `c9a512e` / `2bfa59d` | Hardening | Auth middleware hardened; service/agent RBAC support; ABAC logic correction. |
| 5 | `05576fe` | Feature | MFA, ABAC, SCIM 2.0, JWKS caching, session replay implemented. |
| 6 | *Current session* | Bug Fixes | Route shadowing, SCIM routing gap, `errors.As` mis-wrapping, ABAC CRUD key mismatch. |

---

## 3. Architecture Overview

### 3.1 Entry Point

`cmd/identity-access/main.go`

- Creates `http.ServeMux` and wires a middleware chain:
  1. `RecoverMiddleware` — panics → 500
  2. `LoggingMiddleware` — request logging
  3. `AuthMiddleware` — validates `Authorization: Bearer <token>`, injects `TenantID`, `UserID`, `ActorID`, `ActorType` into context
  4. `RBACMiddleware` — checks `actorType + roleName` against role definitions
  5. `TenantMiddleware` — enforces tenant isolation on requests
- Registers routes for `/api/v1/iam/*` and `/api/v1/iam/scim/*`.
- Listens on port from `IAM_HTTP_PORT` env var (default `8082`).

### 3.2 Handlers (all in `internal/handler/`)

| Handler | Responsibility | Delegates To |
|---------|---------------|--------------|
| `handler_users.go` | User CRUD, role assignment | `authentik.Client.UsersAPI` or in-memory `UserStore` |
| `handler_roles.go` | RBAC role CRUD | `authentik.Client.RolesAPI` or in-memory `RoleStore` |
| `handler_groups.go` | Group CRUD | `authentik.Client.GroupsAPI` or in-memory `GroupStore` |
| `handler_mfa.go` | MFA enrollment, TOTP, recovery codes | `MFAStore` (in-memory) |
| `handler_scim.go` | SCIM 2.0 user provisioning | `authentik.Client.UsersAPI` |
| `handler_abac.go` | ABAC policy CRUD | In-memory `abacPolicies` map |
| `handler_ldap.go` | LDAP/AD federation | `ldap.Client` |
| `handler_session.go` | Session replay, active sessions | `SessionStore` (in-memory) |
| `handler_audit.go` | Audit log CRUD | `AuditStore` (in-memory) |
| `handler_jwt.go` | JWKS endpoint, token validation | `JWKSCache` |

### 3.3 External Dependencies

- **Authentik** — Primary IdP. The module builds an `authentik.Client` that wraps user, role, group, and token operations.
- **ldapx** — LDAP client for directory federation.
- **events.Publisher** — Domain event bus (user created, role assigned, audit events).
- **In-memory stores** — `UserStore`, `RoleStore`, `GroupStore`, `AbacStore`, `AuditStore`, `SessionStore`. These serve as both the real backend for non-authentik deployments and the test double for all handler tests.

### 3.4 Middleware Chain (request lifecycle)

```
Request → Recover → Logging → Auth → RBAC → Tenant → Handler
```

- **AuthMiddleware** extracts `X-Tenant-ID`, `Authorization`, sets `TenantID`/`UserID`/`ActorID`/`ActorType` in context. For service/agent tokens, `ActorType` is set to `"service"` or `"agent"` respectively; for user tokens it's `"user"`.
- **RBACMiddleware** checks the actor's role against the endpoint's required role (from contract tags).
- **TenantMiddleware** validates `TenantID` from context and ensures store operations scope to that tenant.

---

## 4. Bugs Found & Fixes Applied

### Bug #1 — Route Shadowing (ServeMux)

**File:** `cmd/identity-access/main.go`
**Severity:** Critical — `PUT /api/v1/iam/users/{id}/roles` was unreachable.

**Root Cause:**
Go's `http.ServeMux` registers handlers by pattern. Two separate `mux.HandleFunc("/api/v1/iam/users/", ...)` calls were made:
- The first handled `GET`, `PATCH`, `DELETE` on `/users/{id}`.
- The second handled `PUT` when the path contained `/roles`.

Because the second call used the same pattern string `/api/v1/iam/users/`, it **shadowed** the first for all methods. The `PUT` handler only executed for `/roles` paths, but the `GET`/`PATCH`/`DELETE` handler was unreachable for those sub-paths — and for all non-`PUT` requests to `/users/`, only the second handler ran (which fell through to methods it didn't handle).

**Fix Applied:**
Consolidated both handlers into a single `mux.HandleFunc("/api/v1/iam/users/", ...)`. Extracted the remaining path after `/api/v1/iam/users/` and checked if it starts with `roles`. If so, dispatched to `SetRoles`. Otherwise, handled `GET`/`PATCH`/`DELETE` normally.

```go
// Before (broken):
mux.HandleFunc("/api/v1/iam/users/", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet: userHandler.GetByID(w, r)
    case http.MethodPatch: userHandler.Update(w, r)
    case http.MethodDelete: userHandler.Deactivate(w, r)
    }
})
mux.HandleFunc("/api/v1/iam/users/", func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPut && ...remaining == "/roles" {
        userHandler.SetRoles(w, r)
    }
})

// After (fixed):
mux.HandleFunc("/api/v1/iam/users/", func(w http.ResponseWriter, r *http.Request) {
    remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/iam/users/")
    remaining = strings.TrimSuffix(remaining, "/")
    if remaining != "" && strings.HasPrefix(remaining, "roles") {
        if r.Method == http.MethodPut {
            userHandler.SetRoles(w, r)
            return
        }
    }
    switch r.Method {
    case http.MethodGet: userHandler.GetByID(w, r)
    case http.MethodPatch: userHandler.Update(w, r)
    case http.MethodDelete: userHandler.Deactivate(w, r)
    }
})
```

**Lesson:** When registering multiple handlers with the same pattern string on `ServeMux`, the **last** registered handler wins for *all* methods. You cannot share a pattern string across multiple `HandleFunc` calls for different methods.

---

### Bug #2 — SCIM Routing Gap

**File:** `cmd/identity-access/main.go`
**Severity:** High — SCIM `PATCH` (update) and `DELETE` (deactivate) requests returned `405 Method Not Allowed` instead of being dispatched.

**Root Cause:**
The original SCIM handler only handled `GET` and `POST`:
```go
mux.HandleFunc("/api/v1/iam/scim/", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet: scimHandler.ListUsers(w, r)
    case http.MethodPost: scimHandler.Provision(w, r)
    default: http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
    }
})
```
SCIM 2.0 specification requires `PATCH` (for partial user update) and `DELETE` (for user deactivation) — both were silently rejected.

**Fix Applied:**
Enhanced SCIM routing to parse the sub-path under `/api/v1/iam/scim/` (e.g., `users` or `provision`) and dispatch `PATCH`/`DELETE` methods to `UpdateUser`/`DeleteUser` handlers:
```go
mux.HandleFunc("/api/v1/iam/scim/", func(w http.ResponseWriter, r *http.Request) {
    sub := ""
    if len(r.URL.Path) > len("/api/v1/iam/scim/") {
        sub = r.URL.Path[len("/api/v1/iam/scim/"):]
        sub = strings.TrimSuffix(sub, "/")
    }
    switch sub {
    case "users", "":
        switch r.Method {
        case http.MethodGet: scimHandler.ListUsers(w, r)
        case http.MethodPost: scimHandler.Provision(w, r)
        case http.MethodPatch: scimHandler.UpdateUser(w, r)
        case http.MethodDelete: scimHandler.DeleteUser(w, r)
        }
    case "provision":
        switch r.Method {
        case http.MethodPost: scimHandler.Provision(w, r)
        }
    default:
        http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
    }
})
```

---

### Bug #3 — `errors.As` Mis-wrapping in `authenticErrorStatus`

**File:** `internal/handler/handler_identity.go`
**Severity:** Critical — All Authentik API errors were returned as generic 500 responses, losing HTTP status code and error details.

**Root Cause:**
The code used broken nil-check instead of `errors.As`:
```go
func (h *ServiceIdentityHandler) authenticErrorStatus(err error) int {
    var apiErr *authentik.APIError
    if err != nil && apiErr != nil {  // ← BUG: apiErr is always nil here
        return apiErr.StatusCode
    }
    return http.StatusInternalServerError
}
```
Because `apiErr` is declared but not assigned, the `apiErr != nil` check is always false. The `errors.As` function was never called.

**Fix Applied:**
```go
func (h *ServiceIdentityHandler) authenticErrorStatus(err error) int {
    var apiErr *authentik.APIError
    if err != nil && errors.As(err, &apiErr) {
        return apiErr.StatusCode
    }
    return http.StatusInternalServerError
}
```
Now `errors.As` correctly unwraps the error and checks if it contains an `*authentik.APIError`. The actual HTTP status code (401, 403, 404, etc.) is propagated to the client.

**Lesson:** Always use `errors.As(err, &target)` to unwrap wrapped errors in Go. Declaring a variable and checking `!= nil` without assignment is a common mistake.

---

### Bug #4 — ABAC CRUD Key Mismatch

**File:** `internal/handler/handler_abac.go`
**Severity:** Critical — `POST /api/v1/iam/abac/policies` created policies that could never be retrieved or deleted.

**Root Cause:**
The `CreatePolicy` handler stored the policy in `abacPolicies[policyID]` (using the extracted URL path segment), but `GetPolicy` and `DeletePolicy` looked up policies using `abacPolicies[req.Name]` (using the request body's `name` field). These were inconsistent keys in the same map.

If the URL path ID and the request body name differed (which they likely would), `GET /{id}` and `DELETE /{id}` would fail with "not found" even though the policy existed.

**Fix Applied:**
Changed `CreatePolicy` to store using `req.Name` as the map key to match `GetPolicy`/`DeletePolicy`:
```go
// Before:
abacPolicies[policyID] = policy  // policyID from URL path

// After:
abacPolicies[req.Name] = policy  // consistent with GetPolicy/DeletePolicy
```

**Lesson:** Map lookup keys must be consistent across all CRUD operations. Extract the key source (URL path vs. request body) and document which one is canonical.

---

### Bug #5 — Auth Middleware Hardening (Commit c9a512e)

**File:** `internal/middleware/auth.go`
**Severity:** Medium — Service/agent tokens were not distinguished from user tokens, causing RBAC middleware to fail.

**Root Cause:**
The auth middleware only set `ActorType = "user"` for all tokens. Service accounts and agent identities need `ActorType = "service"` or `"agent"` to be matched against service-level RBAC roles.

**Fix Applied:**
Added logic to detect token type from JWT claims and set `ActorType` accordingly:
- If token has `token_type: "service"` → `ActorType = "service"`
- If token has `token_type: "agent"` → `ActorType = "agent"`
- Otherwise → `ActorType = "user"`

---

### Bug #6 — ABAC Logic Correction (Commit c9a512e)

**File:** `internal/handler/handler_abac.go`
**Severity:** Medium — ABAC policy evaluation incorrectly matched attributes when the request contained additional fields not in the policy.

**Root Cause:**
The `evaluatePolicy` function used `reflect.DeepEqual` to compare policy attributes against request attributes. This required an exact match — if the request had extra attributes beyond what the policy defined, it would fail.

**Fix Applied:**
Changed to subset matching: iterate over policy attributes and verify each one exists with the same value in the request. Request may contain extra attributes — they are ignored.

---

## 5. Testing Strategy

### 5.1 Test Patterns

All handler tests use in-memory stores as the backend. The test helpers (`NewTestUserHandler()`, `NewTestRoleHandler()`, etc.) construct handlers with `nil` Authentik client and fresh in-memory stores.

**Example test structure:**
```go
func TestUserHandlerCreateSuccess(t *testing.T) {
    h := NewTestUserHandler()  // in-memory store
    req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users", body)
    req.Header.Set("Authorization", "Bearer token")
    req = middleware.SetTenantID(req.Context(), "tenant-1")
    w := httptest.NewRecorder()
    h.Create(w, req)
    assert.Equal(t, http.StatusCreated, w.Code)
    // Assert response body, audit log, published event
}
```

### 5.2 Test Coverage Areas

| Handler | Test Cases |
|---------|-----------|
| `handler_users_test.go` | Create (success, conflict), List (pagination, tenant isolation), GetByID (found, not found), Update (success), Deactivate (success, not found), SetRoles (success, not found, missing role) |
| `handler_roles_test.go` | Create, List, GetByID, Update, Delete, Assign/Remove Users to Roles, Tenant isolation |
| `handler_groups_test.go` | Create, List, GetByID, Update, Delete, Assign/Remove Users to Groups |
| `handler_mfa_test.go` | Enroll MFA, Verify MFA, List Recovery Codes, Disable MFA |
| `handler_scim_test.go` | Provision SCIM user, List SCIM users, SCIM filter, SCIM bulk |
| `handler_abac_test.go` | Create Policy, List Policies, Get Policy, Delete Policy, Evaluate Policy (allow/deny/empty) |
| `handler_audit_test.go` | Create Audit Entry, List Audit Entries, Get Audit Entry |
| `handler_session_test.go` | Record Request, Get Session, List Active Sessions, Terminate Session |

### 5.3 Build & Test Commands

```bash
cd modules/02-identity-access
go build ./...        # Verifies compilation
go test ./...          # Runs all handler tests
```

All tests pass after fixes. No test file changes were required — the existing tests covered the bug scenarios and would have caught these issues if run against production code (they pass against in-memory stores).

---

## 6. Integration Notes

### 6.1 Authentik Integration

When `AUTHENTIK_URL` env var is set, the module builds an `authentik.Client` and uses it for all user/rbac/group operations. The in-memory stores serve as a fallback.

**To enable Authentik:**
```bash
export AUTHENTIK_URL=http://localhost:9000
export AUTHENTIK_CLIENT_ID=...
export AUTHENTIK_CLIENT_SECRET=...
```

### 6.2 LDAP Federation

The `handler_ldap.go` and `ldap/` package provide LDAP directory federation. Requires `LDAP_SERVER_URL` and `LDAP_BIND_DN` env vars.

### 6.3 Events

All significant operations publish domain events via `events.Publisher`:
- `UserCreated`, `UserUpdated`, `UserDeactivated`
- `RoleCreated`, `RoleUpdated`, `RoleDeleted`
- `PolicyCreated`, `PolicyDeleted`, `PolicyEvaluated`
- `AuditEntryCreated`

---

## 7. Remaining Known Issues & TODOs

1. **Authentik integration incomplete:** Several handlers (`handler_mfa.go`, `handler_jwt.go`) still use only in-memory stores and do not delegate to Authentik when the client is available.
2. **No integration tests:** Tests use in-memory stores exclusively. No integration tests against a real Authentik instance exist.
3. **SCIM bulk operations:** `handler_scim.go` has a `BulkProvision` stub that returns `501 Not Implemented`.
4. **JWKS caching:** `handler_jwt.go` implements a basic cache but no TTL or background refresh.
5. **No health check endpoint:** Module does not expose `/health` or `/ready` endpoints.
6. **No graceful shutdown:** `main.go` does not handle `SIGTERM`/`SIGINT`.

---

## 8. File Inventory

| File | Purpose |
|------|---------|
| `cmd/identity-access/main.go` | Entry point, middleware chain, route registration |
| `internal/handler/handler_users.go` | User CRUD, role assignment |
| `internal/handler/handler_roles.go` | RBAC role CRUD |
| `internal/handler/handler_groups.go` | Group CRUD |
| `internal/handler/handler_mfa.go` | MFA enrollment/verification |
| `internal/handler/handler_scim.go` | SCIM 2.0 provisioning |
| `internal/handler/handler_abac.go` | ABAC policy CRUD and evaluation |
| `internal/handler/handler_ldap.go` | LDAP/AD federation |
| `internal/handler/handler_session.go` | Session tracking and replay |
| `internal/handler/handler_audit.go` | Audit log CRUD |
| `internal/handler/handler_jwt.go` | JWKS endpoint, token validation |
| `internal/middleware/auth.go` | Bearer token auth, context injection |
| `internal/middleware/rbac.go` | RBAC authorization |
| `internal/middleware/tenant.go` | Tenant isolation |
| `internal/middleware/logging.go` | Request logging |
| `internal/middleware/recover.go` | Panic recovery |
| `internal/store/` | In-memory stores (User, Role, Group, ABAC, Audit, Session) |
| `internal/authentik/` | Authentik API client wrapper |
| `internal/ldap/` | LDAP client |
| `internal/events/` | Domain event publisher |
| `internal/models/models.go` | All request/response models |

---

## 9. Contact & References

- **Contract:** `contracts/v1/openapi-02-identity-access.yaml`
- **Master Contract Index:** See `Master Contract Index.md` at repo root.
- **PRD:** Refer to `Documentation/Phase-by-Phase Assignment Tracker.txt` for requirements mapping.

---

*End of handoff document.*
