# Module 03 (Agent Orchestration) — Developer Remediation Instructions

> **Status:** REJECT → CONDITIONAL (after Phase 1) → APPROVE
> **Module:** 03-agent-orchestration — execution core for agent lifecycle management
> **Audited By:** ARCH review
> **Date Generated:** 2025-01-09

---

## Executive Summary

Module 03 is the execution engine for Operan — it manages workflows, agents, pipelines, human tasks, and multi-stack orchestration (LangGraph, Temporal, Ray, Celery). The module has **solid infrastructure scaffolding** (Dockerfile, Helm chart, README) but **critical security gaps** in handler-level tenant isolation and **significant test coverage deficits** that must be resolved before production deployment.

### Current State
| Category | Status |
|----------|--------|
| Build | ✅ PASSING |
| Tests | ✅ 57 tests pass |
| Handler coverage | 63.9% (target: 80%) |
| Middleware coverage | 95.9% |
| Store coverage | 90.3% |
| DAG engine coverage | 86.3% |
| Repository coverage | 0.0% |

---

## PHASE 1: CRITICAL BLOCKERS (Must Fix Before Any Review)

These issues prevent the module from passing any security or quality gate.

---

### P0-01: Handler-Level Tenant Isolation Bypass

**Severity:** CRITICAL
**Affected files:** `handler_workflows.go`, `handler_execution.go`, `handler_pipeline.go`, `handler_human_task.go`

#### Problem

Multiple handler methods read resources by ID **without verifying the requesting tenant owns the resource**. A malicious user with a valid JWT can request `GET /workflows/{any-id}` and receive data from any tenant.

**Evidence (handler_workflows.go):**
```go
func (h *WorkflowHandler) GetWorkflow(w, r) {
    id := extractIDFromPath(...)
    wf, _ := h.WorkflowStore.GetByID(id)  // ← No tenant check!
    h.WriteJSON(w, http.StatusOK, wf)     // ← Returns data to any authenticated user
}
```

**Evidence (handler_execution.go):**
```go
func (h *ExecutionHandler) CreateExecution(w, r) {
    tenantID := r.Header.Get("X-Tenant-ID")  // ← Reads from header, not context
    if tenantID == "" {
        tenantID = "default"  // ← "default" tenant is a security risk
    }
    execution := &store.PipelineExecution{TenantID: tenantID, ...}
}
```

All 12 methods in `handler_execution.go` follow this pattern. Same for `handler_pipeline.go` (12 methods) and `handler_human_task.go` (multiple methods).

#### Root Cause

Two distinct problems:

1. **No `GetByIDAndTenant` method** exists in any store interface. Stores only have `GetByID(id)` which is blind to tenant.
2. **Inconsistent tenant extraction**: Some handlers use `TenantIDFromContext()` (correct), others use `r.Header.Get("X-Tenant-ID")` directly (bypassable).

#### Fix

**Step 1: Add `GetByIDAndTenant` to all store interfaces.**

In each `repository` interface, add:
```go
type WorkflowStoreIface interface {
    // ... existing methods ...
    GetByIDAndTenant(id, tenantID string) (*store.Workflow, error)  // ← ADD THIS
}
```

Apply to: `WorkflowStoreIface`, `ExecutionStoreIface`, `PipelineStoreIface`, `HumanTaskStoreIface`, `EscalationStoreIface`, `ScheduleStoreIface`, `AgentStoreIface`.

**Step 2: Implement in-memory store layer.**

In `internal/store/store_*.go`, add tenant-scoped methods:
```go
func (s *WorkflowStore) GetByIDAndTenant(id, tenantID string) (*store.Workflow, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, wf := range s.data {
        if wf.ID == id && wf.TenantID == tenantID {
            copy := *wf
            return &copy, nil
        }
    }
    return nil, fmt.Errorf("workflow not found: %s", id)
}
```

**Step 3: Update ALL handler read methods.**

Replace `GetByID(id)` with `GetByIDAndTenant(id, tenantID)`:
```go
func (h *WorkflowHandler) GetWorkflow(w, r) {
    id := extractIDFromPath(r.URL.Path, "/workflows/")
    tenantID := middleware.TenantIDFromContext(r.Context())  // ← Must use context
    wf, err := h.WorkflowStore.GetByIDAndTenant(id, tenantID)  // ← Tenant-scoped
    if err != nil {
        h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
        return
    }
    h.WriteJSON(w, http.StatusOK, wf)
}
```

**Step 4: Fix tenant extraction across ALL handlers.**

Replace every instance of:
```go
tenantID := r.Header.Get("X-Tenant-ID")  // ← WRONG
```

With:
```go
tenantID := middleware.TenantIDFromContext(r.Context())  // ← CORRECT
```

Affected locations:
- `handler_execution.go` lines 66, 92
- `handler_pipeline.go` lines 66, 117
- `handler_human_task.go` lines 76, 121, 196, 225

**Step 5: Remove "default" tenant fallback.**

There is no "default" tenant. If `X-Tenant-ID` is missing, the `TenantContext` middleware already returns 400. Handlers should never see an empty tenantID.

```go
// BEFORE (WRONG):
tenantID := middleware.TenantIDFromContext(r.Context())
if tenantID == "" {
    tenantID = "default"  // ← NEVER DO THIS
}

// AFTER (CORRECT):
tenantID := middleware.TenantIDFromContext(r.Context())
// If empty here, middleware already rejected the request — no fallback needed
```

**Step 6: Fix CreateWorkflow's CreatedBy field.**

In `handler_workflows.go` line ~84:
```go
// BEFORE:
CreatedBy: middleware.TenantIDFromContext(r.Context()),  // ← Bug: sets user=tenant

// AFTER:
CreatedBy: middleware.UserIDFromContext(r.Context()),    // ← Correct: user who created
```

---

### P0-02: UpdateStatus/Delete Without Tenant Verification

**Severity:** CRITICAL
**Affected files:** `handler_workflows.go` (CancelWorkflow, PauseWorkflow, etc.), `handler_execution.go` (DeleteExecution, StartExecution, StopExecution, RetryExecution)

#### Problem

Write operations (`UpdateStatus`, `Delete`) mutate resources without verifying the requesting tenant owns them.

```go
func (h *WorkflowHandler) CancelWorkflow(w, r) {
    id := extractIDFromPath(...)
    h.WorkflowStore.UpdateStatus(id, store.WorkflowStatusCancelled)  // ← Any tenant can cancel any workflow
}
```

#### Fix

Add `UpdateStatusAndTenant(id, status, tenantID string) error` to store interfaces. Implement with tenant check.

In `store/store_workflow.go`:
```go
func (s *WorkflowStore) UpdateStatusAndTenant(id, status, tenantID string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i, wf := range s.data {
        if wf.ID == id && wf.TenantID == tenantID {
            s.data[i].Status = store.WorkflowStatus(status)
            return nil
        }
    }
    return fmt.Errorf("workflow not found: %s", id)
}
```

Apply same pattern to `ExecutionStore`, `PipelineStore`, `HumanTaskStore`.

---

### P0-03: EscalationHandler Post-Get Check Pattern

**Severity:** HIGH → Should be resolved by P0-01

#### Current State (GOOD)

`handler_missing.go` `EscalationHandler` already does post-get tenant verification:
```go
func (h *EscalationHandler) ListWorkflowEscalations(w, r, workflowID) {
    tenantID := middleware.TenantIDFromContext(r.Context())
    wf, err := h.WorkflowStore.GetByID(workflowID)
    if err != nil || wf == nil {
        h.WriteError(w, http.StatusNotFound, 404, "workflow not found")
        return
    }
    if wf.TenantID != tenantID {  // ← Good check!
        h.WriteError(w, http.StatusForbidden, 403, "tenant mismatch")
        return
    }
}
```

#### Issue

This is a **defense-in-depth** pattern, not a primary guard. The primary guard should be `GetByIDAndTenant` which rejects at the query level (faster, can't be accidentally omitted). Once P0-01 is implemented, remove all post-get tenant checks and replace with tenant-scoped queries.

---

### P0-04: JWT Secret Validation Not Enforced in Production

**Severity:** MEDIUM → ALREADY FIXED

#### Current State

The `config.Validate()` function checks the JWT secret and returns an error:
```go
func (c *Config) Validate() error {
    if c.JWTSecret == DefaultJWTSecret {
        return fmt.Errorf("JWT_SECRET is set to default value; set a secure value via JWT_SECRET env var")
    }
    return nil
}
```

And `main.go` calls:
```go
if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid configuration: %v", err)  // ← Correct: exits on validation failure
}
```

**No action needed.** This is correctly implemented.

---

## PHASE 2: HANDLER TEST COVERAGE (63.9% → 80%+)

### P1-01: Cover Uncovered Handler Methods

**Current gap:** 36.1% of handler statements are uncovered.

#### Priority Coverage Targets (by method complexity):

1. **`handler_execution.go`** — 12 methods, ~0% coverage estimated
   - `CreateExecution`: Test with valid pipeline, invalid pipeline, missing pipeline_id
   - `ListExecutions`: Test pagination, tenant-scoped listing
   - `GetExecution`: Test 404 for missing execution
   - `StartExecution`, `StopExecution`, `RetryExecution`: Test state transitions and error paths
   - `GetExecutionAnalytics`: Test tenant-scoped aggregation

2. **`handler_pipeline.go`** — 12 methods, ~0% coverage estimated
   - `CreatePipeline`: Test with valid/invalid graph
   - `GetPipeline`: Test 404
   - `ListPipelines`: Test pagination
   - All CRUD operations for pipeline nodes, steps, outputs

3. **`handler_human_task.go`** — 7 methods, ~0% coverage estimated
   - `CreateHumanTask`: Test with valid/invalid inputs
   - `ListHumanTasks`: Test status filter, pagination
   - `GetPendingTasks`: Test tenant-scoped query
   - Task completion/acknowledgment flows

4. **`handler_workflows.go`** — partially covered
   - `CancelWorkflow`: Test cancellation state transition
   - `PauseWorkflow`: Test pause/resume
   - `GetWorkflowVariables`: Test variable retrieval
   - Error paths for all methods

5. **`handler_missing.go`** — EscalationHandler + DelegationHandler + AgentWorkersHandler
   - `ListWorkflowEscalations`: Test tenant isolation
   - `CreateEscalation`: Test severity levels
   - DelegationHandler: Test delegate/acknowledge/reject flows

#### Test File Structure

Use the existing pattern from `handler_workflows_test.go`:
```go
func TestGetWorkflow_TenantIsolation(t *testing.T) {
    store := store.NewWorkflowStore()
    // Create workflows for two tenants
    wf1 := &store.Workflow{ID: "wf-1", TenantID: "tenant-1", Name: "Workflow A"}
    wf2 := &store.Workflow{ID: "wf-2", TenantID: "tenant-2", Name: "Workflow B"}
    store.data["wf-1"] = wf1
    store.data["wf-2"] = wf2

    h := NewWorkflowHandler(store, nil, nil)

    // Request as tenant-1 should NOT return tenant-2's workflow
    req := httptest.NewRequest("GET", "/workflows/wf-2", nil)
    ctx := context.WithValue(req.Context(), middlewareCtxKeyTenantID, "tenant-1")
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()

    h.GetWorkflow(w, req)

    if w.Code != http.StatusNotFound {
        t.Errorf("expected 404, got %d", w.Code)
    }
}
```

---

### P1-02: Add Error Path Tests

Every handler method should have tests for:
- **400 Bad Request**: Missing required fields, malformed JSON
- **404 Not Found**: Non-existent resource ID
- **403 Forbidden**: Cross-tenant access attempt
- **409 Conflict**: Duplicate resource, invalid state transition
- **500 Internal Error**: Store failure simulation

---

## PHASE 3: CONTRACT RECONCILIATIONS

### P1-03: Priority Range Mismatch

**Severity:** HIGH
**Contracts affected:** OpenAPI `openapi-03-agent-orchestration.yaml`, AsyncAPI `asyncapi-03-agent-orchestration.yaml`

#### Problem

| Contract | Priority Range |
|----------|---------------|
| OpenAPI schema | `minimum: 1`, `maximum: 10` |
| AsyncAPI schema | `minimum: 1`, `maximum: 100` |
| Handler code | Clamps to max 10 (line 96-98 in `handler_workflows.go`) |

#### Fix

Align ALL contracts to `minimum: 1`, `maximum: 100` (use AsyncAPI's range as the canonical — 1-10 is too restrictive for a production system).

**Action:** Update OpenAPI priority schema `maximum` from 10 to 100. Handler clamping logic should also be removed or updated to match.

---

### P1-04: WorkflowGraph Definition Mismatch

**Severity:** MEDIUM
**Contracts affected:** OpenAPI + AsyncAPI

#### Problem

OpenAPI defines `WorkflowGraph` as:
```yaml
WorkflowGraph:
  type: object
  properties:
    nodes:
      type: array
      items:
        $ref: '#/components/schemas/WorkflowNode'
    edges:
      type: array
```

But the handler code expects `*store.WorkflowGraph` which may have additional fields or different nesting.

#### Fix

Audit `store.WorkflowGraph` struct against the OpenAPI definition. Ensure field names, types, and nesting match exactly. Any divergence must be fixed in the contract OR the code (prefer fixing code to match contract).

---

### P1-05: Enum Inconsistencies Across 3 Contracts

**Severity:** MEDIUM
**Contracts affected:** OpenAPI 03, AsyncAPI 03, JSON Schema (if exists)

#### Problem

Enum values for `WorkflowStatus`, `PriorityLevel`, `EscalationSeverity`, etc. may differ between OpenAPI, AsyncAPI, and JSON Schema files.

#### Fix

Create a single source of truth for each enum:
```yaml
# OpenAPI + AsyncAPI + JSON Schema — all must match:
WorkflowStatus:
  type: string
  enum:
    - pending
    - running
    - paused
    - completed
    - cancelled
    - failed
```

Cross-check all three contract types for every enum used in Module 03.

---

### P1-06: Add JSON Schema for Module 03

**Severity:** LOW
**Missing file:** `schema-03-agent-orchestration.json`

#### Fix

Generate JSON Schema from the OpenAPI contract using a tool like `oapi-codegen` or manual export. The schema should cover all request/response types used by Module 03.

---

## PHASE 4: MISSING INTEGRATION POINTS

### P2-01: Event Broker — AMQP/Kafka Integration

**Severity:** HIGH for production, LOW for MVP

#### Current State

- `events/broker.go` defines the `Broker` interface with `Publish` and `Subscribe`
- `events/broker.go` has `logBroker` (no-op) as the default — events are only logged
- `events/kafka_broker.go` exists but Kafka is not wired into the default config
- `events/broker.go` factory supports Kafka but **not AMQP** (despite AsyncAPI contracts mentioning RabbitMQ)
- `config/config.go` has no `EVENTS_BROKER` or `KAFKA_BROKERS` config fields

#### Fix Priority

For **MVP**, the log-only broker is acceptable. For **production**, implement:
1. Add `EVENTS_BROKER` config field (`log` | `kafka` | `amqp`)
2. Wire the broker factory based on config
3. If using AMQP (per AsyncAPI contracts), implement AMQP broker (currently only Kafka exists)

---

### P2-02: DelegationHandler Routes

**Severity:** MEDIUM → Check if routes are now wired

#### Current State

`DelegationHandler` exists in `handler_missing.go` but its routes (`/delegate`, `/acknowledge`, `/reject`) may not be registered in `main.go`.

#### Fix

Ensure all delegation endpoints are registered:
```go
// In main.go:
delegateHdlr := handler.NewDelegationHandler(...)
mux.HandleFunc(base+"/workflows/"+workflowIDPath+"/delegate", delegateHdlr.Delegate)
mux.HandleFunc(base+"/escalations/"+idPath+"/acknowledge", delegateHdlr.Acknowledge)
mux.HandleFunc(base+"/escalations/"+idPath+"/reject", delegateHdlr.Reject)
```

---

### P2-03: AgentWorkersHandler Stub

**Severity:** MEDIUM

#### Current State

`handler_missing.go` `GetAgentWorkers` returns an empty array:
```go
func (h *AgentWorkersHandler) GetAgentWorkers(w, r, agentID) {
    workers := h.AgentWorkerStore.ListByAgent(agentID)
    h.WriteJSON(w, http.StatusOK, map[string]interface{}{
        "workers": workers,
        "total":   len(workers),
    })
}
```

#### Fix

Implement the actual worker retrieval logic. If `AgentWorkerStore` is a stub, implement it as well. At minimum, return a meaningful response (empty is acceptable if no workers exist, but the code path must be tested).

---

## VERIFICATION COMMANDS

After implementing all P0 fixes:

```bash
cd modules/03-agent-orchestration

# Build
go build ./...

# Tests
go test ./...

# Coverage (target: handlers ≥ 80%)
go test -cover ./internal/handler/...

# Tenant isolation audit (grep for remaining bypasses)
grep -rn "GetByID(" internal/handler/ | grep -v "GetByIDAndTenant"
grep -rn "r\.Header\.Get.*X-Tenant-ID" internal/handler/

# Middleware chain verification
grep -A5 "Middleware chain" cmd/orchestration-engine/main.go
```

---

## DETAILED PHASED ASSIGNMENT

### Sprint 1: Security Hardening (P0 Fixes)
| Task | Files | Effort |
|------|-------|--------|
| Add `GetByIDAndTenant` to all store interfaces | `repository/*.go` | 2h |
| Implement in-memory store tenant-scoped methods | `store/store_*.go` | 4h |
| Fix all handler read methods | `handler_workflows.go`, `handler_execution.go`, `handler_pipeline.go`, `handler_human_task.go` | 6h |
| Add `UpdateStatusAndTenant` methods | `repository/*.go`, `store/store_*.go` | 2h |
| Fix all handler write methods | Same handler files | 4h |
| Fix tenant extraction consistency | All handler files | 2h |
| **Total** | | **~20h** |

### Sprint 2: Test Coverage (P1 Fixes)
| Task | Files | Effort |
|------|-------|--------|
| Coverage tests for `handler_execution.go` | `handler_execution_test.go` | 4h |
| Coverage tests for `handler_pipeline.go` | `handler_pipeline_test.go` | 4h |
| Coverage tests for `handler_human_task.go` | `handler_human_task_test.go` | 4h |
| Coverage tests for `handler_workflows.go` remaining methods | `handler_workflows_test.go` | 4h |
| Coverage tests for `handler_missing.go` | `handler_missing_test.go` | 4h |
| Error path tests for all handlers | All test files | 6h |
| **Total** | | **~26h** |

### Sprint 3: Contract Reconciliations (P1/P2)
| Task | Files | Effort |
|------|-------|--------|
| Priority range alignment | OpenAPI 03, AsyncAPI 03 | 1h |
| WorkflowGraph definition fix | OpenAPI 03, store types | 2h |
| Enum reconciliation | All 3 contract types | 2h |
| JSON Schema generation | `schema-03-agent-orchestration.json` | 2h |
| **Total** | | **~7h** |

### Sprint 4: Integration Points (P2)
| Task | Files | Effort |
|------|-------|--------|
| Event broker wiring | `events/broker.go`, `config/config.go`, `main.go` | 4h |
| DelegationHandler routes | `main.go` | 1h |
| AgentWorkersHandler implementation | `handler_missing.go`, `store/` | 3h |
| **Total** | | **~8h** |

---

## SIGN-OFF CHECKLIST

Before requesting a re-review, verify:

- [ ] All P0 fixes implemented and tested
- [ ] `GetByIDAndTenant` exists in every store interface
- [ ] Zero instances of `GetByID(id)` in handler read methods (all use `GetByIDAndTenant`)
- [ ] Zero instances of `r.Header.Get("X-Tenant-ID")` in handlers (all use `TenantIDFromContext`)
- [ ] Zero "default" tenant fallbacks in any handler
- [ ] Handler test coverage ≥ 80%
- [ ] All error paths tested (400, 403, 404, 409, 500)
- [ ] Tenant isolation tests written (cross-tenant access returns 404)
- [ ] Priority range consistent across OpenAPI + AsyncAPI
- [ ] WorkflowGraph definition matches store struct
- [ ] All enums reconciled across 3 contract types
- [ ] Build passes: `go build ./...`
- [ ] Tests pass: `go test ./...`
