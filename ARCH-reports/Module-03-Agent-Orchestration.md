# ARCH — Module 03: Agent Orchestration — Production Maturity Review

**Review Date:** 2026-06-18
**Reviewer:** ARCH (Platform Architect)
**Verdict:** **REJECT — Zero handler tests, handler tenant isolation bypasses, contract drift, and missing routes prevent production deployment**

---

## 1. Summary Scorecard

| Category | Score | Notes |
|----------|-------|-------|
| Contract Compliance | ❌ FAIL | 54 operationIds in OpenAPI; 41 AsyncAPI channels; 13 JSON Schema defs — significant cross-spec inconsistencies |
| Test Coverage | ❌ FAIL | 0% handler coverage. 30% store coverage. Middleware and events are well-tested (~80%) |
| Integration | ❌ FAIL | In-memory broker default; Kafka broker exists but no broker wiring in main; all modules have 0 outbound events wired |
| Infrastructure | ⚠️ PARTIAL | Dockerfile, Helm chart, README exist; compiled binary missing |
| Security | ❌ FAIL | Hardcoded JWT default, tenant bypass on workflow reads, no rate limiting |
| Database | ⚠️ PARTIAL | PostgreSQL adapter exists (15 migration tables) but in-memory is default mode |
| PRD Alignment | ⚠️ PARTIAL | Scope expanded from 10+ base endpoints to ~54 operations including LangGraph, Temporal, Ray, Celery stack management |
| **Overall** | **REJECT** | **Not production-ready** |

---

## 2. Contract Drift Analysis

### 2.1 [CRITICAL] Security Scheme Naming — Module 03 Uses Different Tenant Header

| Module | Security Scheme Name | Reference Pattern |
|--------|---------------------|-------------------|
| **Module 01** | `TenantHeader` | `- TenantHeader: []` |
| **Module 02** | `TenantHeader` | `- TenantHeader: []` |
| **Module 03** | `X-Tenant-ID` | Inline security requirement with schema |

**Impact:** A gateway routing requests across modules will fail to apply consistent tenant extraction. Module 01 and 02 extract tenant from the `X-Tenant-ID` **header**, but Module 03's security scheme is named `X-Tenant-ID` with a different definition. This may work if the header name matches, but the OpenAPI spec's security requirements are not interoperable.

**AsyncAPI Module 03** uses `TenantContext` as the security scheme — yet another variation.

**Remediation:** Unify to `TenantHeader` across all modules, or define a shared `X-Tenant-ID` scheme in a platform-level OpenAPI extension.

---

### 2.2 [CRITICAL] Priority Range Mismatch — AsyncAPI vs OpenAPI/JSON Schema

| Contract | Field | Range | Default |
|----------|-------|-------|---------|
| **OpenAPI** `CreateWorkflowRequest.priority` | `minimum: 1, maximum: 10` | 1–10 | 5 |
| **JSON Schema** `CreateWorkflowRequest.priority` | `minimum: 1, maximum: 10` | 1–10 | 5 |
| **AsyncAPI** `WorkflowCreatedPayload.priority` | `minimum: 1, maximum: 100` | 1–100 | 50 |

**Impact:** A workflow created via OpenAPI (priority 5–10) that triggers an AsyncAPI event (priority 50) will appear to have been "upgraded" in priority mid-lifecycle. Downstream consumers that filter by priority range will silently drop or misclassify events.

**Remediation:** Align to a single range (recommend 1–100, default 50) across all three contracts.

---

### 2.3 [CRITICAL] `WorkflowGraph` Definition Discrepancy — OpenAPI `oneOf` vs JSON Schema Flat

| Contract | Structure |
|----------|-----------|
| **OpenAPI** `WorkflowGraph` | `oneOf`: references `LangGraphDefinition` OR has legacy `nodes/edges/error_strategy` object |
| **JSON Schema** `WorkflowGraph` | Flat object: `nodes` (array), `edges` (array), `error_strategy` (enum) — no `oneOf` |

**Impact:** A client sending a `LangGraphDefinition`-based graph (via OpenAPI) will produce an invalid payload per JSON Schema. Conversely, a client validating against JSON Schema will never include the `LangGraphDefinition` reference.

**Remediation:** Reconcile to a single definition. Architectural recommendation: use the OpenAPI's `oneOf` approach but ensure JSON Schema supports the same polymorphic structure.

---

### 2.4 [HIGH] `created_by` Field — OpenAPI Lacks UUID Format

| Contract | Field | Definition |
|----------|-------|-----------|
| **OpenAPI** `Workflow.created_by` | `type: string` (no format) | Bare string |
| **JSON Schema** `Workflow.created_by` | `type: string, format: uuid` | UUID formatted |

**Impact:** JSON Schema consumers will validate UUID format and may reject or transform values sent via OpenAPI handlers that accept arbitrary strings.

**Remediation:** Add `format: uuid` to OpenAPI's `Workflow.created_by`.

---

### 2.5 [HIGH] `NodeExecutionResult.status` — OpenAPI 4 Values vs AsyncAPI 2

| Contract | Enum Values |
|----------|-------------|
| **OpenAPI** `NodeExecutionResult.status` | `success`, `failed`, `skipped`, `cancelled` |
| **AsyncAPI** `NodeCompletedPayload.status` | `success`, `skipped` |

**Impact:** A node that completes with `failed` or `cancelled` status will produce an AsyncAPI event with an invalid status value. Consumers will reject or misinterpret the event.

**Remediation:** Add `failed` and `cancelled` to the AsyncAPI enum, or reduce the OpenAPI enum to match AsyncAPI.

---

### 2.6 [HIGH] `RetryRecord.status` — OpenAPI 4 Values vs AsyncAPI 2

| Contract | Enum Values |
|----------|-------------|
| **OpenAPI** `RetryRecord.status` | `pending`, `in_progress`, `success`, `exhausted` |
| **AsyncAPI** `RetryCompletedPayload.status` | `success`, `exhausted` |

**Remediation:** Add `pending` and `in_progress` to AsyncAPI enum.

---

### 2.7 [HIGH] `Delegation.status` — OpenAPI 4 Values vs AsyncAPI 2

| Contract | Enum Values |
|----------|-------------|
| **OpenAPI** `Delegation.status` | `pending`, `accepted`, `rejected`, `completed` |
| **AsyncAPI** `DelegationCompletedPayload.status` | `completed`, `rejected` |

**Remediation:** Add `pending` and `accepted` to AsyncAPI enum.

---

### 2.8 [HIGH] LangGraph Node Types — OpenAPI 9 vs JSON Schema 6

| Contract | Node Types |
|----------|-----------|
| **OpenAPI** `LangGraphNode.type` | `agent`, `action`, `human_gate`, `condition`, `parallel_branch`, `delay`, **`tool_call`**, **`state_update`**, **`subgraph`** |
| **JSON Schema** `WorkflowNode.type` | `agent`, `action`, `human_gate`, `condition`, `parallel_branch`, `delay` |

**Remediation:** Add `tool_call`, `state_update`, and `subgraph` to JSON Schema.

---

### 2.9 [MEDIUM] `WorkflowState.status` — OpenAPI Has Enum, JSON Schema Does Not

| Contract | Definition |
|----------|-----------|
| **OpenAPI** `WorkflowState.status` | `enum: [pending, running, paused, completed, failed, cancelled]` |
| **JSON Schema** `WorkflowState.status` | `type: string` (no enum) |

**Remediation:** Add the enum to JSON Schema.

---

### 2.10 [MEDIUM] Agent Status Enums — OpenAPI 4 Values vs AsyncAPI Multiple `reason` Enums

| Contract | Field | Enum Values |
|----------|-------|-------------|
| **OpenAPI** `WorkerInfo.status` | `online`, `idle`, `busy`, `offline` |
| **AsyncAPI** `AgentUnavailablePayload.reason` | `overloaded`, `degraded`, `offline`, `deprecated` |
| **AsyncAPI** `AgentOfflinePayload.reason` | `user_initiated`, `heartbeat_timeout`, `system_shutdown` |

The OpenAPI agent status and AsyncAPI agent event reason enums serve different purposes but represent related concepts. An agent in `busy` status could generate an `overloaded` reason event — but there's no clear mapping.

**Remediation:** Either unify the enums into a single `AgentStatus` model referenced by both contracts, or document the mapping between status states and event reasons.

---

### 2.11 [MEDIUM] Missing AsyncAPI Events for Key Operations

The OpenAPI contract defines 54 operations, but the AsyncAPI contract defines only 41 channels. The following operations have **no corresponding AsyncAPI event**:

| OpenAPI Operation | Missing Event |
|-------------------|--------------|
| `getWorkflow`, `getWorkflowState`, `getWorkflowVariables` | No read events (acceptable — read events are often not published) |
| `listWorkflows`, `listAgents` | No list events (acceptable) |
| `getSchedule`, `updateSchedule`, `pauseSchedule`, `resumeSchedule` | No schedule lifecycle events except `triggered` |
| `createLangGraph`, `updateLangGraph`, `deleteLangGraph` | Partial: `registered`, `deployed`, `state/updated` exist; `deleted` is missing |
| `createTemporalWorkflow`, `updateTemporalWorkflow`, `deleteTemporalWorkflow` | Partial: `registered`, `checkpoint/created`, `replayed`; `deleted` is missing |
| `deleteRayPool` | Missing event |
| `deleteCeleryQueue` | Missing event |
| `scaleRayPool` | Missing event |
| `publishCeleryTask` | Missing event |
| `getCeleryQueue`, `updateCeleryQueue`, `deleteCeleryQueue` | Queue lifecycle events partially missing |
| `listCeleryConsumers` | Missing event |
| `assignAgent` | `agent/assigned` exists ✅ |
| `delegateNodeTask` | `workflow/delegate` exists but `workflow/delegate/acknowledged` / `workflow/delegate/rejected` are missing |
| `acknowledgeEscalation` | `escalation/acknowledged` exists ✅ |
| `retryNode` | `retry/requested` and `retry/completed` exist ✅ |

---

### 2.12 [MEDIUM] OpenAPI Operations Without JSON Schema Definition

| OpenAPI Schema | JSON Schema Definition |
|---------------|----------------------|
| `StackHealthStatus`, `StackHealthModule` | ❌ None |
| `WorkflowState`, `NodeState` | `WorkflowState`, `NodeState` ✅ |
| `Checkpoint`, `ExecutionEvent` | `Checkpoint`, `ExecutionEvent` ✅ |
| `WorkflowVariables`, `UpdateVariablesRequest` | `CreateWorkflowRequest` covers base but `WorkflowVariables` and `UpdateVariablesRequest` are not in JSON Schema |
| `ReplayRequest` | ❌ None |
| `WorkflowList`, `CreateScheduleRequest`, `UpdateScheduleRequest` | `CreateScheduleRequest` ✅; `WorkflowList` and `UpdateScheduleRequest` ❌ |
| `Escalation`, `EscalationRequest`, `EscalationList` | ❌ None |
| `Delegation`, `DelegationRequest`, `DelegationList` | ❌ None |
| `RetryRecord`, `RetryRecordList` | ❌ None |
| `TaskRoute`, `RouteTaskRequest`, `TaskRouteList` | ❌ None |
| `WorkerInfo`, `WorkerInfoList` | `AssignAgentRequest` covers base but `WorkerInfo` and `WorkerInfoList` ❌ |
| `NodeExecutionResult`, `NodeExecutionResultList` | ❌ None |
| `CreateLangGraphRequest`, `LangGraphDefinition`, `LangGraphNode`, `LangGraphEdge`, `LangGraphCheckpoint` | ❌ None |
| `TemporalWorkflowDefinition`, `TemporalCheckpointConfig`, `TemporalReplayConfig` | ❌ None |
| `RayExecutionConfig`, `RayDistributedTask`, `RayWorkerPool` | ❌ None |
| `CeleryTaskQueue`, `CeleryConsumerConfig` | ❌ None |

**Impact:** Clients validating payloads against the JSON Schema will fail on most schema types. This suggests the JSON Schema file is incomplete — it was likely authored separately from the OpenAPI contract.

---

### 2.13 [MEDIUM] Missing Operation in OpenAPI vs HANDOFF

The HANDOFF document references `listAgents` as an implemented endpoint, but the OpenAPI contract's `/agents` GET endpoint uses operationId `listAgents`. However, the HANDOFF says it's "not found in the OpenAPI spec" — need to verify the exact count of operationIds (54 vs 56 from manual count).

---

## 3. Security Assessment

### 3.1 [CRITICAL] Handler Tenant Isolation Bypass

`handler_workflows.go` (and likely other handler files) calls `WorkflowStore.GetByID(id)` **without verifying the retrieved workflow's `TenantID` matches the context tenant**.

```go
// handler_workflows.go
tenantID := middleware.TenantIDFromContext(r.Context())
workflow, err := h.WorkflowStore.GetByID(id)
// No check: if workflow.TenantID != tenantID → should return 403
```

**Impact:** An authenticated user from Tenant A can read, pause, resume, cancel, checkpoint, replay, delegate, escalate, retry, or update variables for any workflow from Tenant B — simply by guessing the workflow ID. This is a full cross-tenant data leak.

**Remediation:** Add tenant verification to every `GetByID`-based handler. The store should either have a `GetByIDAndTenant` method (preferred, follows Module 01/02 pattern) or the handler should verify after retrieval.

---

### 3.2 [HIGH] Hardcoded Default JWT Secret

`internal/config/config.go` line 17: `DefaultJWTSecret = "change-me-in-production"`. The `Config.Validate()` method checks for this, but if validation fails, `main.go` does not exit — it continues. Any service starting with the default secret will accept any JWT.

**Remediation:** Fail hard on startup if the JWT secret is the default.

---

### 3.3 [MEDIUM] No Rate Limiting

No middleware exists for rate limiting. The module directly exposes 54 endpoints including workflow execution triggers, agent assignment, and stack health — all without throttle.

**Remediation:** Implement rate-limiting middleware (sliding window or token bucket) with per-tenant limits.

---

### 3.4 [MEDIUM] Duplicate `generateID()` Functions

The `generateID()` function is defined in **three files**:
- `internal/middleware/middleware.go`
- `internal/handler/handler_workflows.go`
- `internal/handler/handler_missing.go`

Each uses `crypto/rand` (good) but the duplication increases risk of inconsistency and makes it harder to audit.

**Remediation:** Extract to a shared `internal/utils/id.go`.

---

### 3.5 [MEDIUM] AMQP Broker is a Skeleton

`internal/events/kafka_broker.go` — `NewAMQPBroker` logs "skeleton" and `Publish()`/`Subscribe()` are no-ops. If the event broker config points to AMQP, events will be silently lost.

**Remediation:** Either implement AMQP properly or return an error from `BrokerFactory` when AMQP is configured.

---

## 4. Test Coverage Analysis

### 4.1 Coverage Summary

| Package | Test Files | Tests | Coverage | Status |
|---------|-----------|-------|----------|--------|
| `internal/middleware` | 1 | ~25 | ~80% | ✅ GOOD |
| `internal/events` | 2 | ~25 | ~75% | ✅ GOOD |
| `internal/config` | 1 | ~10 | ~60% | ⚠️ PARTIAL |
| `internal/store` | 4 | ~30 | ~30% | ❌ FAIL |
| `internal/handler` | 1 | 5 | ~0% | ❌ FAIL |
| `cmd/orchestration-engine` | 0 | 0 | 0% | ❌ FAIL |
| `internal/repository` | 0 | 0 | 0% | ❌ FAIL |
| `internal/database` | 0 | 0 | 0% | ❌ FAIL |

### 4.2 Untested Handlers (9 of 10)

| Handler File | Tests | Coverage |
|--------------|-------|----------|
| `handler_workflows.go` | ❌ 0 tests | 0% |
| `handler_scheduling.go` | ❌ 0 tests | 0% |
| `handler_schedules.go` | ❌ 0 tests | 0% |
| `handler_pipeline.go` | ❌ 0 tests | 0% |
| `handler_pipeline_execution_init.go` | ❌ 0 tests | 0% |
| `handler_execution.go` | ❌ 0 tests | 0% |
| `handler_human_task.go` | ❌ 0 tests | 0% |
| `handler_missing.go` | ⚠️ 5 tests (delegation only) | ~2% |
| `dag_engine.go` | ❌ 0 tests | 0% |
| `cmd/orchestration-engine` | ❌ 0 tests | 0% |

### 4.3 Missing Test Categories

- **No handler tests for any core endpoint** — workflows, schedules, pipelines, executions, human tasks
- **No DAG engine tests** — topological sort, node execution, retry logic, error strategies, condition edges
- **No integration tests** — no Dockerfile-based tests, no testcontainers
- **No tenant-isolation tests** — critical given the bypass described in Section 3.1
- **No repository layer tests** — PostgreSQL adapter is completely untested
- **No database migration tests** — 15 CREATE TABLE statements verified only by compilation

---

## 5. Infrastructure & Deployment

### 5.1 Infrastructure Status

| Artifact | Status |
|----------|--------|
| `Dockerfile` | ✅ Present |
| `helm/Chart.yaml` | ✅ Present |
| `helm/templates/` | ✅ Present |
| `helm/values.yaml` | ✅ Present |
| `README.md` | ✅ Present |
| Compiled binary | ❌ Missing (not committed) |

Module 03 has complete infrastructure tooling — this is the best-in-class module for deployment readiness from an infrastructure perspective.

### 5.2 Event Broker Wiring

The event package provides three broker implementations:
- **InMemoryBroker** — log-based (default)
- **KafkaBroker** — full Kafka implementation via `segmentio/kafka-go`
- **AMQPBroker** — skeleton/no-op

**Issue:** `main.go` does not appear to wire the broker based on configuration. The default broker is used, which means all events are log-only in production.

### 5.3 Database Adapter

The repository package provides a unified `Store` wrapper with two modes:
- **ModeInMemory** (default) — in-memory stores
- **ModePostgreSQL** — PostgreSQL passthrough wrappers

**Issue:** Default mode is in-memory. PostgreSQL requires explicit `DB_MODE=postgres` or `DB_HOST` configuration. Migration SQL exists (15 tables) but is not auto-applied.

---

## 6. PRD Alignment

### 6.1 Scope Expansion

The PRD for Module 03 specifies agent orchestration with ~10+ base endpoints. The implementation provides **54 operations** including:
- Workflow CRUD + state management (21 endpoints)
- Schedule management (6 endpoints)
- Agent scheduling and availability (5 endpoints)
- Stack health (1 endpoint)
- LangGraph graph management (5 endpoints)
- Temporal workflow management (6 endpoints)
- Ray pool management (5 endpoints)
- Celery queue management (6 endpoints)

**Architectural directive:** All scope expansions must be submitted as change requests to ARCH for approval.

**Assessment:** The expansion into LangGraph, Temporal, Ray, and Celery stack management is architecturally sound — Module 03 is the orchestrator, and it needs visibility and management capabilities across execution stacks.

### 6.2 Unimplemented Features

| Handler | Status | Description |
|---------|--------|-------------|
| `AgentWorkersHandler` | ⚠️ STUB | Always returns empty workers array |
| `StackHealthHandler` | ⚠️ PARTIAL | Full CRUD exists but no real health checks |
| `DelegationHandler` | ⚠️ NOT WIRED | Handler exists but routes not registered in main.go |
| `dag_engine.go` | ⚠️ PARTIAL | DAG engine works but `NodeHandler` implementations are not provided |
| `InitialNodes` in DAG | ⚠️ PLACEHOLDER | Hardcoded `["node-1"]` instead of computed from DAG |

---

## 7. Implementation Quality Observations

### 7.1 Code Structure — Good

- Clean layered architecture: `main.go` → handlers → Store → (in-memory or PostgreSQL) → events.Publisher
- Tenant-indexed in-memory stores (`byTenant` maps)
- Comprehensive event schema (41 AsyncAPI channels, all typed payloads)
- Kafka broker with SASL/TLS
- Working DAG execution engine with topological sort, retry, and error strategies

### 7.2 Code Issues

| # | Location | Issue | Severity |
|---|----------|-------|----------|
| 1 | `handler_workflows.go` | `GetByID` without tenant verification | CRITICAL |
| 2 | `handler_missing.go` | `AgentWorkersHandler` returns empty array | HIGH |
| 3 | `handler_missing.go` | `DelegationHandler` not wired into router | HIGH |
| 4 | `handler_missing.go` | Off-by-one retry counting (`recordsLen + 1`) | MEDIUM |
| 5 | `handler_missing.go` | Hardcoded stack type `events.StackLangGraph` | MEDIUM |
| 6 | `handler_scheduling.go` | Hardcoded `MaxConcurrency: 5` | LOW |
| 7 | `database/database.go` | `HumanTaskList` bug: `append(params)` adds nothing | LOW |
| 8 | `middleware/middleware.go` | Duplicate `generateID()` in 3 files | LOW |
| 9 | `dag_engine.go` | Condition edges always return true (no evaluation) | MEDIUM |
| 10 | `events/kafka_broker.go` | AMQPBroker is skeleton (no-op) | MEDIUM |

---

## 8. Remediation Plan

### Phase 1: Critical Blockers (Must Fix Before Re-Review)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 1 | Fix tenant isolation: add `GetByIDAndTenant` or verify tenant in all handler reads | CODER | P0 |
| 2 | Fail startup if JWT secret is the default value | CODER | P0 |
| 3 | Wire `DelegationHandler` routes into `main.go` | CODER | P0 |
| 4 | Implement `AgentWorkersHandler` stub (or remove handler) | CODER | P0 |
| 5 | Reconcile `WorkflowGraph` definition across OpenAPI and JSON Schema | ARCH → CODER | P0 |
| 6 | Reconcile priority range (AsyncAPI 1–100 vs OpenAPI 1–10) | ARCH → CODER | P0 |
| 7 | Reconcile `created_by` format (OpenAPI vs JSON Schema) | ARCH → CODER | P0 |
| 8 | Reconcile security scheme naming across Modules 01/02/03 | ARCH → ALL | P0 |
| 9 | Implement DAG engine tests (topological sort, retry, error strategies) | CODER | P0 |
| 10 | Add handler tests for core workflows (5+ endpoints minimum) | CODER | P0 |

### Phase 2: High-Priority Gaps (Fix Before Production Sign-Off)

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 11 | Achieve ≥80% handler test coverage (write tests for all 9 untested handlers) | CODER | P1 |
| 12 | Write tenant-isolation tests (verify cross-tenant reads are rejected) | CODER | P1 |
| 13 | Add PostgreSQL integration tests with testcontainers | CODER | P1 |
| 14 | Implement AMQPBroker or error on AMQP config | CODER | P1 |
| 15 | Implement `evaluateCondition` in DAG engine | CODER | P1 |
| 16 | Wire event publishing to Kafka or document stub limitation | CODER | P1 |
| 17 | Add rate-limiting middleware | CODER | P1 |
| 18 | Extract `generateID()` to shared utility | CODER | P1 |
| 19 | Add `/ready` endpoint with database and event broker checks | CODER | P1 |
| 20 | Implement `InitialNodes` computation from DAG topology | CODER | P1 |
| 21 | Add PostgreSQL auto-migration on startup | CODER | P1 |

### Phase 3: Medium-Priority Enhancements

| # | Issue | Owner | Priority |
|---|-------|-------|----------|
| 22 | Reconcile all remaining enum mismatches (NodeExecutionResult, RetryRecord, Delegation status) | ARCH → CODER | P2 |
| 23 | Add missing AsyncAPI event channels (schedule lifecycle, pool deletion, scale, publish, delegation ack/reject) | ARCH → CODER | P2 |
| 24 | Add JSON Schema definitions for all OpenAPI schemas missing in JSON Schema (18+ schemas) | ARCH → CODER | P2 |
| 25 | Implement `StackHealthHandler` with real health checks | CODER | P2 |
| 26 | Add config and repository package tests (reach 80%) | CODER | P2 |
| 27 | Add OpenTelemetry instrumentation | CODER | P2 |
| 28 | Fix off-by-one retry counting | CODER | P2 |

---

## 9. Developer Sign-Off Checklist

Before resubmitting Module 03 for re-review, the CODER team must complete AND verify each item below:

- [ ] **P0-1:** Tenant isolation enforced on all handler read operations (GetByIDAndTenant or post-retrieval tenant verification)
- [ ] **P0-2:** Startup fails if JWT secret equals default value
- [ ] **P0-3:** `DelegationHandler` routes registered in `main.go`
- [ ] **P0-4:** `AgentWorkersHandler` either implemented or removed
- [ ] **P0-5:** `WorkflowGraph` definition reconciled across OpenAPI and JSON Schema
- [ ] **P0-6:** Priority range reconciled across all three contracts
- [ ] **P0-7:** `created_by` UUID format reconciled across OpenAPI and JSON Schema
- [ ] **P0-8:** Security scheme naming unified across Modules 01/02/03
- [ ] **P0-9:** DAG engine tests pass (topological sort, retry, error strategies)
- [ ] **P0-10:** Handler tests for core workflows (5+ endpoints, ≥50% coverage)
- [ ] **P1-11 through P1-18:** Phase 1–2 remediation items completed
- [ ] **P2-22 through P2-28:** Phase 3 items completed (for production readiness)

---

## 10. Architect's Note

Module 03 is the **execution core** of the Operan platform. Every workflow, schedule, agent assignment, and stack operation flows through this module. The infrastructure tooling is excellent (Dockerfile, Helm, README) — the most complete of all modules reviewed so far. The layered architecture is clean, the event schema is comprehensive (41 channels), and the DAG execution engine demonstrates solid algorithmic thinking.

However, **zero handler tests** is the most alarming finding. 54 endpoints with 5 test methods covering only delegation is unacceptable for a system that will orchestrate agent workloads across a multi-tenant enterprise platform. The tenant isolation bypass in handler reads is equally critical — any authenticated user could read or modify another tenant's workflows.

The DAG engine is the module's unique value proposition, yet it has **zero tests**. The topological sort, retry logic, error strategies, and condition evaluation are all untested. This is where confidence should be highest — instead, it's the largest blind spot.

**Remediation effort estimate:** **5–7 developer days for P0 items** (primarily tenant isolation and handler tests), **7–10 days for P1 items** (DAG tests, integration tests, enum reconciliations).

The architectural review has flagged these issues. The CODER team should prioritize tenant isolation first (it's a production security risk), then tests (it's a confidence risk).

---

**ARCH Sign-Off:** _________________    **Date:** 2026-06-18
**CODER Acknowledgment:** _________________    **Date:** _________________
