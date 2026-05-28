# Module 03 Agent Orchestration — Complete Audit Report

**Date:** 2025-01-XX  
**Auditor:** Human Orchestrator (AI)  
**Module:** 03 — Agent Orchestration Engine  
**Contracts:** openapi-03-agent-orchestration.yaml, asyncapi-03-agent-orchestration.yaml  
**Deployment Wave:** Wave 2 (Modules 03, 04, 14, 16)  
**Phase:** Phase 1 — MVP Core (Sprints 1-8)

---

## Table of Contents

1. [Platform Context: Purpose and Final Destination](#1-platform-context-purpose-and-final-destination)
2. [Audit Summary](#2-audit-summary)
3. [OpenAPI Gap Analysis](#3-openapi-gap-analysis)
4. [AsyncAPI Gap Analysis](#4-asyncapi-gap-analysis)
5. [Implementation Health Assessment](#5-implementation-health-assessment)
6. [Critical Gaps Requiring Immediate Attention](#6-critical-gaps-requiring-immediate-attention)
7. [Step-by-Step Completion Plan](#7-step-by-step-completion-plan)
8. [Definition of Done Checklist](#8-definition-of-done-checklist)

---

## 1. Platform Context: Purpose and Final Destination

### What Operan (ADOS) Is

Operan — **Agentic Department Operating System (ADOS)** — is the foundational operating system for AI-native enterprises and governments. It enables deployable **digital departments** that operate with institutional memory, governance, explainability, and sovereign-grade control.

**Positioning:** "Enterprise Agentic Workforce Infrastructure" — competing with BPO firms, ERP vendors, and enterprise operating systems. NOT an "AI assistant platform."

### The 20-Module Platform

The platform consists of 20 interconnected modules across 6 horizontal layers:

| Layer | Modules |
|---|---|
| **Experience Layer** | 05 (Dept Templates), 15 (Marketplace), UI/SDK |
| **Orchestration Layer** | **03 (Agent Orchestration)**, 14 (Comm Bus) |
| **Memory Layer** | 06 (Knowledge), 07 (Memory Fabric) |
| **Execution Layer** | 08 (Tool Execution), 16 (Sandbox), 18 (Enterprise Connector) |
| **Governance Layer** | 09 (Human Supervision), 10 (Policy/Governance), 11 (Observability), 17 (Cost Governance) |
| **Infrastructure Layer** | 01 (Tenant Control Plane), 02 (IAM), 12 (Model Abstraction), 19 (Arabic), 20 (Sovereign Deploy) |

### Five Core Principles

1. **Sovereign by Design** — Full on-prem, regional isolation, air-gapped compatibility
2. **Multi-Tenant Isolation** — Every tenant gets isolated memory, orchestration, models, governance
3. **Human-Governed Autonomy** — Agents may recommend/execute; critical actions require approval
4. **Explainability First** — Traceability, replayability, provenance for every action
5. **Modular Runtime Architecture** — Independently deployable and replaceable modules

### Module 03's Role in the Platform

Module 03 is the **central execution engine**. It is Wave 2 (the second deployment wave after Tenant Control Plane and IAM). Every department template (Module 05) and every workflow across the platform funnels through Module 03's orchestration engine. Without Module 03:
- Departments cannot execute multi-step workflows
- Agent assignments are meaningless (Module 04 has no executor)
- Human supervision hooks (Module 09) have no execution context to attach to
- Observability (Module 11) has no execution traces to collect

### Success Metrics (from Platform DoD)

- Workflow success rate: >= 99.5%
- P95 agent response latency: < 2s
- MTTR: < 15 mins

---

## 2. Audit Summary

### Snapshot

| Dimension | Spec | Implemented | Coverage |
|---|---|---|---|
| **OpenAPI operations** | 54 | ~17 | **31%** |
| **AsyncAPI channels** | 37 | ~8 publishing paths | **22%** |
| **AsyncAPI schemas** | 39 | ~12 defined types | **31%** |
| **Execution stacks** | 4 (LangGraph, Temporal, Ray, Celery) | 0 | **0%** |
| **Config wired** | Yes (8 env vars) | No | **0%** |
| **Auth middleware** | BearerAuth + X-Tenant-ID | No | **0%** |
| **Persistent storage** | Implicit (multi-tenant, production) | In-memory only | **0%** |
| **Unit/integration tests** | >= 80% (DoD) | 0 tests | **0%** |
| **Helm charts** | Required (DoD) | Empty template dir | **0%** |
| **Event topic naming** | Multi-stack format | Legacy format | **0%** |

### Overall Verdict: **CONDITIONAL — REJECT**

The module has a solid skeleton (stores, basic CRUD handlers, middleware chain, event types defined) but is **not production-ready** and does **not implement the contracts** as specified. All critical platform standards (auth, config, persistence, tests, Helm) are missing.

---

## 3. OpenAPI Gap Analysis

### Contract: `contracts/v1/openapi-03-agent-orchestration.yaml`
- OpenAPI 3.0.3, 2817 lines
- 54 operations across 18 path groups
- Base path: `/api/v1/orchestration`
- Security: `BearerAuth` + `X-Tenant-ID`

### Operations Implemented vs Missing

#### 3.1 Workflows (`/workflows` + `/workflows/{id}`) — 11 operations | ~8 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| createWorkflow | POST /workflows | CreateWorkflow | DONE |
| listWorkflows | GET /workflows | ListWorkflows | DONE |
| getWorkflow | GET /workflows/{id} | GetWorkflow | DONE |
| cancelWorkflow | DELETE /workflows/{id} | CancelWorkflow | DONE |
| pauseWorkflow | POST /workflows/{id}/pause | PauseWorkflow | DONE |
| resumeWorkflow | POST /workflows/{id}/resume | ResumeWorkflow | DONE |
| getWorkflowState | GET /workflows/{id}/state | GetWorkflowState | DONE |
| createCheckpoint | POST /workflows/{id}/checkpoint | CreateCheckpoint | DONE |
| replayWorkflow | POST /workflows/{id}/replay | ReplayWorkflow | DONE |
| getWorkflowVariables | GET /workflows/{id}/variables | GetWorkflowVariables | DONE |
| updateWorkflowVariables | PATCH /workflows/{id}/variables | UpdateWorkflowVariables | DONE |

**Assessment:** Workflows are the best-covered area. All 11 operations exist. However, see Section 5 for quality issues (no real DAG execution, no tenant filtering in some handlers, no validation against JSON Schema).

#### 3.2 Schedules (`/schedules` + `/schedules/{id}`) — 6 operations | 5 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| scheduleWorkflow | POST /schedules | ScheduleWorkflow | DONE |
| getSchedule | GET /schedules/{id} | GetSchedule | DONE |
| updateSchedule | PATCH /schedules/{id} | UpdateSchedule | DONE |
| deleteSchedule | DELETE /schedules/{id} | DeleteSchedule | DONE |
| triggerSchedule | POST /schedules/{id}/trigger | TriggerSchedule | DONE |
| pauseSchedule | POST /schedules/{id}/pause | — | **MISSING** |
| resumeSchedule | POST /schedules/{id}/resume | — | **MISSING** |

**Missing:** `pauseSchedule` (operationId) and `resumeSchedule` (operationId).

#### 3.3 Agents — 4 operations | 2 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| assignAgent | POST /agents/assign | AssignAgent | DONE |
| getAgentAvailability | GET /agents/availability | GetAgentAvailability | DONE |
| delegateNodeTask | POST /agents/assign/{assignmentId}/delegate | — | **MISSING** |
| listAgents | GET /agents | — | **MISSING** |

**Missing:** `delegateNodeTask`, `listAgents`.

#### 3.4 Escalations — 3 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listWorkflowEscalations | GET /workflows/{id}/escalations | — | **MISSING** |
| createEscalation | POST /workflows/{id}/escalations | — | **MISSING** |
| acknowledgeEscalation | PUT /escalations/{id}/acknowledge | — | **MISSING** |

**Status:** No escalation store, no escalation handler. Entire feature is absent.

#### 3.5 Retries — 2 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listWorkflowRetryRecords | GET /workflows/{id}/retries | — | **MISSING** |
| retryNode | POST /workflows/{id}/retries/{retryId}/retry | — | **MISSING** |

**Status:** No retry record store, no retry handler. Entire feature is absent.

#### 3.6 Workflow Results & Nodes — 2 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listWorkflowNodes | GET /workflows/{id}/nodes | — | **MISSING** |
| listWorkflowResults | GET /workflows/{id}/results | — | **MISSING** |

**Status:** No node result tracking, no result aggregation.

#### 3.7 Agent Workers — 1 operation | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| getAgentWorkers | GET /agents/{agentId}/workers | — | **MISSING** |

#### 3.8 Stack Health — 1 operation | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| getStackHealth | GET /stacks/health | — | **MISSING** |

#### 3.9 LangGraph (5 operations) — 5 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listLangGraphs | GET /stacks/langgraph | — | **MISSING** |
| createLangGraph | POST /stacks/langgraph | — | **MISSING** |
| getLangGraph | GET /stacks/langgraph/{id} | — | **MISSING** |
| updateLangGraph | PATCH /stacks/langgraph/{id} | — | **MISSING** |
| deleteLangGraph | DELETE /stacks/langgraph/{id} | — | **MISSING** |

#### 3.10 Temporal (6 operations) — 6 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listTemporalWorkflows | GET /stacks/temporal | — | **MISSING** |
| createTemporalWorkflow | POST /stacks/temporal | — | **MISSING** |
| getTemporalWorkflow | GET /stacks/temporal/{id} | — | **MISSING** |
| updateTemporalWorkflow | PATCH /stacks/temporal/{id} | — | **MISSING** |
| deleteTemporalWorkflow | DELETE /stacks/temporal/{id} | — | **MISSING** |
| listTemporalCheckpoints | GET /stacks/temporal/{id}/checkpoints | — | **MISSING** |

#### 3.11 Ray (5 operations) — 5 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listRayPools | GET /stacks/ray | — | **MISSING** |
| createRayPool | POST /stacks/ray | — | **MISSING** |
| getRayPool | GET /stacks/ray/{id} | — | **MISSING** |
| deleteRayPool | DELETE /stacks/ray/{id} | — | **MISSING** |
| scaleRayPool | POST /stacks/ray/{id}/scale | — | **MISSING** |

#### 3.12 Celery (6 operations) — 6 operations | 0 implemented

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| listCeleryQueues | GET /stacks/celery | — | **MISSING** |
| createCeleryQueue | POST /stacks/celery | — | **MISSING** |
| getCeleryQueue | GET /stacks/celery/{id} | — | **MISSING** |
| updateCeleryQueue | PATCH /stacks/celery/{id} | — | **MISSING** |
| deleteCeleryQueue | DELETE /stacks/celery/{id} | — | **MISSING** |
| listCeleryConsumers | GET /stacks/celery/{id}/consumers | — | **MISSING** |

#### 3.13 Pipelines, Executions, Human Tasks

| Operation | Contract Path | Handler Method | Status |
|---|---|---|---|
| All pipeline CRUD | /pipeline, /pipeline/{id} | ListPipelines, CreatePipeline, GetPipeline, etc. | DONE (7 ops) |
| All execution CRUD | /executions, /executions/{id} | ListExecutions, CreateExecution, etc. | DONE (10 ops) |
| All human-tasks CRUD | /human-tasks, /human-tasks/{id} | ListHumanTasks, CreateHumanTask, etc. | DONE (8 ops) |

**Note:** Pipeline/execution/human-task handlers exist and cover their paths, but they operate on in-memory stores with no real execution logic.

### Summary: 36 of 54 operations missing (67% missing)

---

## 4. AsyncAPI Gap Analysis

### Contract: `contracts/v1/asyncapi-03-agent-orchestration.yaml`
- AsyncAPI 2.6.0, 1937 lines
- 37 channels, 39 schemas
- Multi-stack topic format: `operan.orchestration.{stack}.{entity}.{event}`

### Channels With No Go Publisher

| # | Channel | Event Type | Topic Format | Status |
|---|---|---|---|---|
| 1 | workflow.created | orchestration.workflow.created | `operan.orchestration.{stack}.workflow.created` | **NO PUBLISHER** |
| 2 | workflow.started | orchestration.workflow.started | — | **NO PUBLISHER** |
| 3 | workflow.completed | orchestration.workflow.completed | — | **NO PUBLISHER** |
| 4 | workflow.failed | orchestration.workflow.failed | — | **NO PUBLISHER** |
| 5 | workflow.paused | orchestration.workflow.paused | — | **NO PUBLISHER** |
| 6 | workflow.resumed | orchestration.workflow.resumed | — | **NO PUBLISHER** |
| 7 | workflow.cancelled | orchestration.workflow.cancelled | — | **NO PUBLISHER** |
| 8 | workflow.checkpointed | orchestration.workflow.checkpointed | — | **NO PUBLISHER** |
| 9 | workflow.replayed | orchestration.workflow.replayed | — | **NO PUBLISHER** |
| 10 | schedule.triggered | orchestration.schedule.triggered | — | **NO PUBLISHER** |
| 11 | agent.assigned | orchestration.agent.assigned | — | **NO PUBLISHER** |
| 12 | agent.unavailable | orchestration.agent.unavailable | — | **NO PUBLISHER** |
| 13 | node.started | orchestration.node.started | — | **NO PUBLISHER** |
| 14 | node.completed | orchestration.node.completed | — | **NO PUBLISHER** |
| 15 | node.failed | orchestration.node.failed | — | **NO PUBLISHER** |
| 16 | node.retry | orchestration.node.retry | — | **NO PUBLISHER** |
| 17 | escalation.created | orchestration.escalation.created | — | **NO PUBLISHER** |
| 18 | escalation.acknowledged | orchestration.escalation.acknowledged | — | **NO PUBLISHER** |
| 19 | escalation.resolved | orchestration.escalation.resolved | — | **NO PUBLISHER** |
| 20 | retry.recorded | orchestration.retry.recorded | — | **NO PUBLISHER** |
| 21 | retry.executed | orchestration.retry.executed | — | **NO PUBLISHER** |
| 22 | pipeline.created | orchestration.pipeline.created | — | **NO PUBLISHER** |
| 23 | pipeline.started | orchestration.pipeline.started | — | **NO PUBLISHER** |
| 24 | pipeline.completed | orchestration.pipeline.completed | — | **NO PUBLISHER** |
| 25 | pipeline.failed | orchestration.pipeline.failed | — | **NO PUBLISHER** |
| 26 | pipeline.step.completed | orchestration.pipeline.step.completed | — | **NO PUBLISHER** |
| 27 | human_task.pending | orchestration.human_task.pending | — | **NO PUBLISHER** |
| 28 | human_task.responded | orchestration.human_task.responded | — | **NO PUBLISHER** |
| 29 | agent.available | orchestration.agent.available | — | **NO PUBLISHER** |

### Event Topic Naming Mismatch

The AsyncAPI contract specifies multi-stack topics:
```
operan.orchestration.{stack}.workflow.created
operan.orchestration.{stack}.node.started
operan.orchestration.{stack}.escalation.created
```

But the Go publisher (`internal/events/events.go`) uses a legacy flat format:
```go
p.publish("orchestration.workflow.created", data)
p.publish("orchestration.workflow.started", data)
```

The `{stack}` placeholder is completely absent.

### Summary: 29 of 37 channels have no Go publishing call (78% missing)

---

## 5. Implementation Health Assessment

### 5.1 Config Package — **UNUSED**

`internal/config/config.go` defines 8 configuration values:

```go
type Config struct {
    ListenAddr    string  // Default: ":8080"
    OTLPEndpoint  string  // Default: "http://localhost:4318"
    LogEnv        string  // Default: "production"
    Version       string  // Default: "1.0.0"
    EventBusHost  string  // Default: "events.operan.internal"
    EventBusPort  string  // Default: "9092"
    EventBusProto string  // Default: "kafka"
    LogLevel      string  // Derived: "debug" or "info"
}
```

**But `ParseConfig()` is never called.** `main.go` hardcodes:
```go
port := 8003
log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), chain))
```

### 5.2 Authentication — **NOT IMPLEMENTED**

OpenAPI specifies:
```yaml
security:
  - BearerAuth: []
  - X-Tenant-ID:
      schema:
        type: string
```

The middleware chain in `main.go` is:
```go
chain = middleware.Logger(chain)
chain = middleware.TenantContext(chain)  // Only extracts X-Tenant-ID
chain = middleware.TraceID(chain)
chain = middleware.RequestID(chain)
```

**No JWT validation middleware exists.** Any client can call any endpoint.

### 5.3 Persistence — **IN-MEMORY ONLY**

All stores use `sync.RWMutex` + `map[string]*T` in RAM. Data is lost on process restart. No PostgreSQL, no Redis, no etcd.

Stores:
- `WorkflowStore` — map + tenant index + checkpoints + variables + history
- `ScheduleStore` — map + tenant index
- `AgentStore` — map + workflow index + availability
- `PipelineStore` — map + tenant index
- `ExecutionStore` — map + pipeline index + tenant index + step map
- `HumanTaskStore` — map + tenant index + execution index

### 5.4 Multi-Stack Execution — **NOT IMPLEMENTED**

The contract and README describe four execution backends:

| Stack | Contract Operations | Go Handler | Reality |
|---|---|---|---|
| LangGraph | 5 (CRUD) | 0 | No LangGraph client, no integration |
| Temporal | 6 (CRUD + checkpoints) | 0 | No Temporal client, no integration |
| Ray | 5 (CRUD + scale) | 0 | No Ray client, no integration |
| Celery | 6 (CRUD + consumers + publish) | 0 | No Celery client, no integration |

The README describes stack-aware execution routing (e.g., "LangGraph for DAG-based workflows, Temporal for long-running processes"). The Go code has zero stack integration. The `CreateWorkflow` handler simply persists a DAG definition in memory and returns it.

### 5.5 Handler Quality Issues

Several handlers have quality issues:

1. **`CreateCheckpoint`** — assigns the workflow ID as the node ID instead of reading from the request body:
   ```go
   cp := store.Checkpoint{
       NodeID:    id,  // BUG: should be from req.NodeID
   ```

2. **`GetWorkflowState`** — returns all nodes as `NodeStatusPending` regardless of actual state. No real state machine.

3. **`ReplayWorkflow`** — does not actually replay. Only updates variables and appends an event to history.

4. **`ListWorkflows`** — pagination uses `offset`/`limit` query params (legacy) instead of `page`/`page_size` (platform standard).

5. **`TenantIDFromContext`** — returns empty string when header is missing, and handlers don't reject requests without tenant context.

6. **Error responses** — inconsistent format. Some handlers use `middleware.ErrorResponse` (with Code, Message, RequestID), others use the raw `writeError` in main.go (with Code, Message only, no RequestID).

### 5.6 Tests — **ZERO**

The `tests/` directory is empty. The DoD requires >= 80% unit/integration test coverage.

---

## 6. Critical Gaps Requiring Immediate Attention

### P0 — Blockers (Must Fix Before Any Demo/Deploy)

| # | Gap | Impact | Effort |
|---|---|---|---|
| 1 | No JWT auth middleware | Any unauthenticated client can call all endpoints | 1-2 days |
| 2 | Config hardcoded (port 8003, no env vars) | Cannot deploy to different environments | 2 hours |
| 3 | No persistent storage | Data lost on restart | 1-2 days |
| 4 | No unit/integration tests | Fails DoD gate | Ongoing |
| 5 | 36 of 54 OpenAPI operations missing | Cannot fulfill Wave 2 contract commitments | 2-3 weeks |

### P1 — High Priority (Should Fix in MVP Phase)

| # | Gap | Impact | Effort |
|---|---|---|---|
| 6 | No multi-stack execution (LangGraph/Temporal/Ray/Celery) | README describes capabilities that do not exist | 4-6 weeks |
| 7 | Event topic naming mismatch | AsyncAPI consumers will not receive correctly formatted messages | 1 day |
| 8 | 29 of 37 AsyncAPI channels not published | Event-driven architecture is broken | 1-2 weeks |
| 9 | Escalations, retries, results features completely absent | Contract specifies these as core workflow features | 2-3 weeks |
| 10 | No Helm charts | Fails DoD deployment gate | 1 day |

### P2 — Medium Priority (Phase 2 or Later)

| # | Gap | Impact | Effort |
|---|---|---|---|
| 11 | Agent availability is manual-only | No heartbeat/health-check mechanism | 1 week |
| 12 | No DAG execution engine | Workflows are persisted but never actually executed | 3-4 weeks |
| 13 | No real-time state machine | Node statuses are always "pending" | 2-3 weeks |
| 14 | No observability integration | OTLP endpoint is configured but never used | 1-2 days |

---

## 7. Step-by-Step Completion Plan

### Phase 0: Foundation Fixes (Sprint 1-2, ~1 week)

#### Step 1: Wire the Config Package
**File:** `cmd/orchestration-engine/main.go`

Replace the hardcoded `port := 8003` with:
```go
cfg := config.ParseConfig()
port := cfg.ListenAddr
if !strings.Contains(port, ":") {
    port = ":" + port
}
log.Printf("Orchestration Engine starting on %s (version %s)", port, cfg.Version)
log.Printf("Event bus: %s:%s (%s)", cfg.EventBusHost, cfg.EventBusPort, cfg.EventBusProto)
log.Printf("OTLP endpoint: %s", cfg.OTLPEndpoint)
log.Printf("Log level: %s", cfg.LogLevel)
log.Fatal(http.ListenAndServe(port, chain))
```

Add env var documentation to `README.md`:
```markdown
## Environment Variables
| Variable | Default | Description |
|---|---|---|
| LISTEN_ADDR | :8080 | HTTP listen address |
| OTLP_ENDPOINT | http://localhost:4318 | OpenTelemetry collector endpoint |
| LOG_ENV | production | Log environment (debug/production) |
| MODULE_VERSION | 1.0.0 | Semantic version |
| EVENT_BUS_HOST | events.operan.internal | Event bus hostname |
| EVENT_BUS_PORT | 9092 | Event bus port |
| EVENT_BUS_PROTO | kafka | Event bus protocol |
```

#### Step 2: Add JWT Authentication Middleware
**File:** `internal/middleware/middleware.go` — add new function

```go
// JWTAuth validates Bearer tokens against Module 02 (IAM) or a local JWT signing key.
// For MVP, implement local HMAC verification. Later, refactor to call Module 02.
func JWTAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            writeError(w, http.StatusUnauthorized, 401, "Authorization header required")
            return
        }
        if !strings.HasPrefix(authHeader, "Bearer ") {
            writeError(w, http.StatusUnauthorized, 401, "Invalid authorization scheme")
            return
        }
        token := strings.TrimPrefix(authHeader, "Bearer ")
        
        claims, err := validateJWT(token)
        if err != nil {
            writeError(w, http.StatusUnauthorized, 401, "Invalid or expired token")
            return
        }
        
        ctx := r.Context()
        ctx = context.WithValue(ctx, "user_id", claims.Subject)
        ctx = context.WithValue(ctx, "user_roles", claims.Roles)
        r = r.WithContext(ctx)
        
        next.ServeHTTP(w, r)
    })
}
```

Update the middleware chain in `main.go`:
```go
var chain http.Handler = mux
chain = middleware.Logger(chain)
chain = middleware.JWTAuth(chain)    // NEW: authenticate first
chain = middleware.TenantContext(chain)
chain = middleware.TraceID(chain)
chain = middleware.RequestID(chain)
```

**NOTE:** For MVP, the JWT signing key can be read from `JWT_SECRET` env var (HMAC-S256). In production, the proper path is to forward requests to Module 02 for token validation. This should be documented as a TODO.

#### Step 3: Fix Error Response Format Inconsistency
Ensure ALL handlers use `middleware.ErrorResponse` consistently (with `Code`, `Message`, `Details`, `RequestID`). Remove the `writeError` helper from `main.go` and migrate it to a shared utility.

#### Step 4: Fix Pagination to Platform Standard
All `List*` handlers must use `page`/`page_size` query params (not `offset`/`limit`). The platform standard uses 1-based pagination with `has_more` flag.

**File:** `internal/handler/handler_workflows.go` — fix `ListWorkflows`:
```go
// OLD:
page, _ := strconv.Atoi(r.URL.Query().Get("offset"))
page++
pageSize, _ := strconv.Atoi(r.URL.Query().Get("limit"))

// NEW:
page, _ := strconv.Atoi(r.URL.Query().Get("page"))
if page < 1 { page = 1 }
pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
if pageSize < 1 { pageSize = 20 }
```

Repeat for all List handlers.

### Phase 1: Core Contract Coverage (Sprint 2-4, ~3 weeks)

#### Step 5: Implement Missing OpenAPI Operations (36 ops)

Group them by feature area. Implement in this order:

**Priority Order:**
1. **Schedule pause/resume** (2 ops) — simplest; just call `ScheduleStore.Patch` to toggle `Enabled`
2. **Agent list** (1 op) — `GET /agents` returns list from AgentStore with status filter
3. **Agent delegate** (1 op) — POST to reassign an existing assignment to a different agent
4. **Escalations** (3 ops) — create a new `EscalationStore` (similar pattern to ScheduleStore), implement list/create/acknowledge
5. **Retries** (2 ops) — create `RetryRecordStore`, implement list/retry
6. **Workflow nodes/results** (2 ops) — derive from existing workflow graph data
7. **Stack health** (1 op) — return mock/skeleton response until stacks are implemented

For each new handler:
- Create a new file in `internal/handler/` (e.g., `handler_escalations.go`)
- Define a handler struct with the appropriate store references
- Implement each operation as a method
- Register routes in `main.go`
- Add corresponding AsyncAPI event publishing
- Write unit tests (see Step 10)

#### Step 6: Implement Missing AsyncAPI Event Publishers

For each of the 29 missing channels:
1. Define the payload struct (check `asyncapi-03-agent-orchestration.yaml` for exact schema)
2. Add a `Publish*` method to `events.Publisher`
3. Call the publish method from the appropriate handler

**Critical: Fix the topic naming format.** Change from:
```go
p.publish("orchestration.workflow.created", data)
```
To:
```go
p.publishTopic("operan.orchestration."+stack+".workflow.created", data)
```

The `stack` parameter should come from the workflow definition's `execution_stack` field (default: `langgraph` if not specified).

#### Step 7: Fix Handler Bugs
- `CreateCheckpoint`: read `NodeID` from request body
- `GetWorkflowState`: implement actual state computation from store
- `ReplayWorkflow`: implement actual replay logic (at minimum, create a new workflow instance with same graph)

### Phase 2: Multi-Stack Execution (Sprint 3-6, ~4-6 weeks)

#### Step 8: Implement LangGraph Integration

1. Create `internal/stack/langgraph/client.go`
2. Client wraps the LangGraph REST API or gRPC interface
3. Methods: `CreateGraph`, `ListGraphs`, `GetGraph`, `UpdateGraph`, `DeleteGraph`, `ExecuteGraph`
4. Handler calls client, client calls LangGraph service
5. Publishes `workflow.started`, `workflow.completed`, `workflow.failed` events

#### Step 9: Implement Temporal Integration

1. Create `internal/stack/temporal/client.go`
2. Client wraps Temporal Go SDK
3. Methods: `StartWorkflow`, `DescribeWorkflow`, `ListWorkflows`, `SignalWorkflow`, `CancelWorkflow`, `GetCheckpoints`
4. Handler delegates to client

#### Step 10: Implement Ray Integration

1. Create `internal/stack/ray/client.go`
2. Methods: `CreatePool`, `ListPools`, `GetPool`, `DeletePool`, `ScalePool`
3. Ray is primarily for distributed compute; orchestration module manages pool lifecycle

#### Step 11: Implement Celery Integration

1. Create `internal/stack/celery/client.go`
2. Methods: `CreateQueue`, `ListQueues`, `GetQueue`, `DeleteQueue`, `ListConsumers`, `PublishTask`
3. Celery is for Python-based task execution; orchestrator manages queues and publishes tasks

### Phase 3: Data Persistence (Sprint 2-3, parallel with Step 5)

#### Step 12: Add Persistent Storage Backend

The DoD requires production-grade persistence. Choose one:
- **PostgreSQL** (recommended) — structured data, ACID, multi-tenant isolation via `tenant_id` column
- **Redis** (for hot path: sessions, checkpoints)

Refactor all stores to use a repository pattern:
```go
type WorkflowRepository interface {
    Create(ctx context.Context, wf *Workflow) error
    GetByID(ctx context.Context, id string) (*Workflow, error)
    UpdateStatus(ctx context.Context, id string, status WorkflowStatus) error
    List(ctx context.Context, tenantID string, page, pageSize int, status *string) ([]*Workflow, int, bool)
    Delete(ctx context.Context, id string) error
}

type workflowRepo struct {
    db *sql.DB  // or *gorm.DB
}
```

Keep in-memory stores as a cache layer on top of the repository for high-frequency reads.

### Phase 4: Testing (Ongoing, parallel with all phases)

#### Step 13: Write Unit Tests

Target: >= 80% coverage (DoD requirement).

**Test structure:**
```
tests/
  handler/
    handler_workflows_test.go
    handler_schedules_test.go
    handler_scheduling_test.go
    handler_pipeline_test.go
    handler_execution_test.go
    handler_human_task_test.go
  middleware/
    middleware_test.go
  store/
    workflow_test.go
    pipeline_execution_test.go
  events/
    events_test.go
```

**Test patterns (see operan-handler-testing and operan-middleware-testing skills):**
- Use `httptest.NewRequest` and `httptest.NewRecorder`
- Inject mock stores (or use in-memory stores with clean state per test)
- Test all HTTP methods on each endpoint
- Test error cases (404, 400, 401, 409, 500)
- Test validation (missing required fields)
- Test tenant isolation (requests with different X-Tenant-ID must not see each other's data)
- Test pagination (page, page_size, has_more)

#### Step 14: Write Integration Tests

- Test the full middleware chain (JWT + TenantContext + TraceID + RequestID)
- Test handlers that span multiple stores
- Test event publishing (verify publishTopic is called with correct topic format)

### Phase 5: Deployment (Sprint 2, ~1 day)

#### Step 15: Create Helm Chart

**File:** `helm/Chart.yaml`
```yaml
apiVersion: v2
name: operan-agent-orchestration
description: Agent Orchestration Engine — Module 03
version: 0.1.0
appVersion: "1.0.0"
type: application
```

**File:** `helm/templates/deployment.yaml`
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: operan-agent-orchestration
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: operan-agent-orchestration
  template:
    metadata:
      labels:
        app: operan-agent-orchestration
    spec:
      containers:
        - name: orchestration-engine
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          ports:
            - containerPort: 8080
          env:
            - name: LISTEN_ADDR
              value: ":8080"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: operan-secrets
                  key: jwt-secret
            {{- range $key, $val := .Values.env }}
            - name: {{ $key }}
              value: {{ $val | quote }}
            {{- end }}
```

**File:** `helm/templates/service.yaml` — ClusterIP service on port 8080

**File:** `helm/values.yaml` — default values

**File:** `Dockerfile` — multi-stage build (follow Operan platform convention):
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o server ./cmd/orchestration-engine

FROM alpine:3.19
RUN addgroup -S operan && adduser -S operan -G operan
USER operan
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
```

#### Step 16: Add Observability

- Initialize OpenTelemetry SDK using `cfg.OTLPEndpoint`
- Add spans for each handler method
- Export metrics (request count, error rate, latency percentiles)
- Configure Grafana dashboard (refer to `docs/README.md` for expected metrics)

---

## 8. Definition of Done Checklist

Before Module 03 can pass the platform DoD gates, verify ALL items:

### Code Gate
- [ ] >= 80% unit + integration test coverage
- [ ] SAST/DAST security scan passed (no hardcoded secrets, no SQL injection, etc.)
- [ ] Architecture review: all cross-module dependencies approved (esp. Module 02 JWT validation)

### Deployment Gate
- [ ] Helm chart complete (Chart.yaml, templates/, values.yaml)
- [ ] Dockerfile present (multi-stage, non-root user)
- [ ] Deployed to dev environment and validated
- [ ] Deployed to staging environment and validated
- [ ] Rollback procedure documented and tested

### Observability Gate
- [ ] OpenTelemetry traces exported to collector
- [ ] OpenTelemetry metrics exported (Prometheus-compatible)
- [ ] Structured logging (JSON) with trace_id, request_id, tenant_id
- [ ] Grafana dashboard for Module 03 operational metrics
- [ ] Alerting rules defined for SLOs (P95 < 2s, error rate < 0.5%)

### Governance Gate
- [ ] Audit log entries for all state-changing actions (create, update, delete, trigger)
- [ ] OPA policy compliance (Module 10) — workflows respect policy constraints
- [ ] Data residency rules enforced (tenant isolation in all queries)
- [ ] PII data handled per Module 02 (IAM) policies

### Documentation Gate
- [ ] Auto-generated API docs (Swagger/OpenAPI UI) published
- [ ] Operations runbook (startup, scaling, rollback, troubleshooting)
- [ ] User-facing guide (how to define workflows, schedule executions, manage agents)
- [ ] README.md updated with deployment instructions and environment variables

---

## Appendix A: File Tree (Current State)

```
modules/03-agent-orchestration/
├── go.mod
├── go.sum
├── README.md
├── cmd/
│   └── orchestration-engine/
│       └── main.go              ← 183 lines, hardcodes port, no config
├── helm/
│   └── templates/               ← EMPTY
├── internal/
│   ├── config/
│   │   └── config.go            ← Defines Config struct, ParseConfig() — NEVER CALLED
│   ├── events/
│   │   └── events.go            ← Publisher struct, 12 publish methods, logs-only
│   ├── handler/
│   │   ├── handler_execution.go       ← 10 methods (executions CRUD + analytics)
│   │   ├── handler_human_task.go      ← 8 methods (human-tasks CRUD)
│   │   ├── handler_pipeline.go        ← 8 methods (pipeline CRUD + analytics)
│   │   ├── handler_pipeline_execution_init.go  ← 1 function (pipeline creation helper)
│   │   ├── handler_schedules.go       ← 5 methods (schedule CRUD + trigger)
│   │   ├── handler_scheduling.go      ← 2 methods (assign + availability)
│   │   └── handler_workflows.go       ← 11 methods (workflow CRUD + state + checkpoint + replay + vars)
│   ├── middleware/
│   │   └── middleware.go              ← Logger, TenantContext, TraceID, RequestID — NO JWT
│   └── store/
│       ├── pipeline_execution.go      ← PipelineStore, ExecutionStore, HumanTaskStore
│       └── workflow.go                ← WorkflowStore, ScheduleStore, AgentStore
└── tests/                           ← EMPTY
```

## Appendix B: Contract Reference

### OpenAPI Contract
- Path: `contracts/v1/openapi-03-agent-orchestration.yaml`
- 2817 lines
- 54 operations
- Base path: `/api/v1/orchestration`
- Security: `BearerAuth` (JWT) + `X-Tenant-ID` header

### AsyncAPI Contract
- Path: `contracts/v1/asyncapi-03-agent-orchestration.yaml`
- 1937 lines
- 37 channels
- 39 schemas
- Topic format: `operan.orchestration.{stack}.{entity}.{event}`

### Module README (Architecture)
- Path: `modules/03-agent-orchestration/README.md`
- Documents 4 execution stacks, multi-stack routing, event bus, DAG execution
- README describes capabilities that do NOT exist in code

## Appendix C: Deployment Wave Context

Module 03 is **Wave 2** — it depends on Wave 1 modules being operational:
- **Wave 1:** Module 01 (Tenant Control Plane), Module 02 (IAM), Module 12 (Model Abstraction), Module 20 (Sovereign Deployment)
- **Wave 2:** Module 03 (Agent Orchestration), Module 04 (Agent Registry), Module 14 (Agent Communication Bus), Module 16 (Execution Sandbox)

Module 03 requires:
- Module 02 for JWT token validation (or local JWT signing key as MVP workaround)
- Module 04 for agent registry data (agent definitions, capabilities, status)
- Module 20 for sovereign deployment constraints (data residency, on-prem support)
- Module 10 (Policy/Governance) for policy enforcement on workflow execution

Module 03 enables:
- Module 05 (Department Templates) — templates define workflows executed by Module 03
- Module 06/07 (Knowledge/Memory) — workflows access knowledge and memory during execution
- Module 08 (Tool Execution) — workflow nodes call tools via Module 08
- Module 09 (Human Supervision) — human tasks flow through Module 03
- Module 11 (Observability) — Module 03 emits execution traces consumed by Module 11
- Module 17 (Cost Governance) — Module 03 reports token/cost per workflow execution

---

*End of Audit Report.*
*Next action: Review this audit with the development pod assigned to Wave 2. Prioritize P0 blockers (Phase 0 steps) before proceeding to contract coverage (Phase 1).*
