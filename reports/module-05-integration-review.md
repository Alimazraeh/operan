# Module 05: Cross-Module Integration Review

**Review Type:** Integration audit — no implementation, only review
**Module:** `05-department-template-engine`
**Scope:** Contract alignment between Module 05 and implemented modules 01–04 (modules 06–20 are contracts-only, no code)
**Reference:** PRD Section 6, integration-graph.yaml, OpenAPI contracts, implementation

---

## Executive Summary

Module 05 is a **fully self-contained CRUD service** with zero outbound integration calls. The deployment workflow described in the PRD (6-step orchestration across modules 01–20) is entirely absent from implementation. The `deployTemplate` endpoint creates a deployment record with status `"select"` and never advances.

Of the 11 outgoing integration edges in the integration graph, only 4 target implemented modules (01, 02, 03, 04). Of those 4:

| Module | Edge Description | Target Contract Exists? | Contract Supports Module 05's Use Case? |
|--------|-----------------|----------------------|----------------------------------------|
| 01 | Validate tenant quota | Yes — `GET /tenants/{id}/quota` | Partial — generic quota, no template-aware check |
| 02 | RBAC/ABAC evaluation | Yes — `POST /rbac/evaluate` | Yes — accepts actor/action/resource/attributes |
| 03 | Deploy agent swarms | Yes — `POST /workflows` (DAG) | Partial — Module 05 must translate template → DAG |
| 04 | Register template agents | Yes — `POST /agents` + `department_id` | Partial — Module 05 must translate AgentDefinition → CreateAgentRequest |

**All 7 edges to modules 06–20 are deferred by design** (contracts-only, no code). Those are not actionable now.

---

## Finding 1: Config Scaffolding — Dead Code (MEDIUM)

**Location:** `internal/config/config.go:18-26, 39-47`

9 config fields declare cross-module endpoints:

| Field | Env Var | Target Module | Used? |
|-------|---------|--------------|-------|
| `Module03Endpoint` | `MODULE03_ENDPOINT` | Agent Orchestration (03) | ❌ |
| `Module04Endpoint` | `MODULE04_ENDPOINT` | Agent Registry (04) | ❌ |
| `Module07Endpoint` | `MODULE07_ENDPOINT` | Memory Fabric (07) | ❌ |
| `Module10Endpoint` | `MODULE10_ENDPOINT` | Policy Governance (10) | ❌ |
| `Module11Endpoint` | `MODULE11_ENDPOINT` | Observability (11) | ❌ |
| `Module18Endpoint` | `MODULE18_ENDPOINT` | Enterprise Connector (18) | ❌ |
| `SovereignEndpoint` | `MODULE20_ENDPOINT` | Sovereign Deployment (20) | ❌ |
| `PolicyEngineURL` | `MODULE05_POLICY_ENGINE_URL` | Policy Governance (10) — duplicate | ❌ |
| `JWKSURL` | `MODULE05_JWKS_URL` | Auth/JWKS | ❌ |

**Only 4 config fields are actually consumed:** `Port`, `JWTSecret`, `MaxPageSize`, `Validate()`.

**Impact:** Creates false confidence that integration is "configured." A future developer would look at these fields and assume HTTP clients exist.

---

## Finding 2: Module 01 — Quota Validation (PARTIAL SUPPORT)

**Integration graph says:** `POST /tenants/{id}/quota` — "Validate tenant quota before template instantiation"

**Module 01 actually provides:**
- `GET /tenants/{id}/quota` → `QuotaConfig` (max_agents, max_workflows_per_day, max_storage_gb, max_monthly_tokens, max_concurrent_workflows)
- `GET /tenants/{id}/resources/usage` → `ResourceUsageResponse` (current utilization)
- `GET /tenants/{id}/status` → active_agents, active_workflows, storage_used_gb, tokens_used_this_month

**Gap:** No dedicated "check quota for template deployment" endpoint. Module 05 would need to:
1. Call `GET /tenants/{id}/quota` to get limits
2. Call `GET /tenants/{id}/resources/usage` to get current usage
3. Manually compute whether deploying N agents + M workflows exceeds quota

This is doable but requires Module 05 to implement quota arithmetic that should be centralized.

**Risk:** LOW. The data Module 05 needs exists. The gap is convenience, not capability.

---

## Finding 3: Module 02 — RBAC/ABAC Evaluation (FULL SUPPORT)

**Integration graph says:** `POST /rbac/evaluate` — "RBAC/ABAC evaluation before state changes"

**Module 02 provides:**
```
POST /rbac/evaluate
Request: { actor_id, action, resource, attributes? }
Response: { allowed, reason, policy_match?, evaluated_at? }
```

**Suitability:** Full support. Module 05 can call this before any template CRUD or deployment action. The `attributes` field allows passing `department_id`, `template_version`, `environment`, etc.

**Gap:** Only evaluates one permission at a time. Module 05's 6-phase deployment would need 6 separate calls (or Module 05 could batch-check locally and only call Module 02 for the deployment action).

**Risk:** NONE. This is well-aligned.

---

## Finding 4: Module 03 — Agent Swarm Deployment (PARTIAL — FIELD MAPPING)

**Integration graph says:** "Deploy template-configured agent swarms"

**Module 03 provides:** `POST /workflows` — Creates and starts a DAG workflow.

```
CreateWorkflowRequest:
  tenant_id, department_id, name, version
  graph: DAG with nodes (type: agent, action, human_gate), depends_on, timeouts, retry
  variables: runtime variables
```

**Gap — Field Mapping:** Module 05's `Template` schema has:
```go
Agents        []AgentDefinition
Workflows     []WorkflowDefinition
```

But Module 03's `CreateWorkflowRequest` expects a flat DAG structure with typed nodes. Module 05 would need to:
1. Resolve each `AgentDefinition` → Module 04 agent by ID/capability
2. Convert `WorkflowDefinition.Steps[]` → Module 03 DAG nodes
3. Build `depends_on` relationships from `WorkflowStep` sequences
4. Map `WorkflowStep.type` (agent_call, api_call, data_fetch, etc.) → Module 03 node types (agent, action, human_gate)

**This is a significant translation layer** that Module 05 must implement. There is no "submit template for execution" endpoint in Module 03 — it only accepts workflows directly.

**Risk:** MEDIUM. Data exists but requires non-trivial translation logic.

---

## Finding 5: Module 04 — Agent Registration (PARTIAL — FIELD MAPPING)

**Integration graph says:** "Register template agents"

**Module 04 provides:**
- `POST /agents` — Register an agent (accepts `department_id`)
- `GET /agents?department_id={uuid}` — List agents by department
- `POST /agents/search` — Search by capabilities/constraints

**Module 04 is explicitly aware of Module 05:**
```yaml
# openapi-04-agent-registry.yaml
Agent.department_id: "UUID of the department this agent belongs to (Module 05). Optional."
CreateAgentRequest.department_id: "UUID of the parent department (Module 05). Optional."
AgentSearchRequest.department_id: "Filter by department (Module 05)."
```

**Gap — Field Mapping:** Module 05's `AgentDefinition` differs from Module 04's `CreateAgentRequest`:

| Module 05 `AgentDefinition` | Module 04 `CreateAgentRequest` | Match? |
|----------------------------|-------------------------------|--------|
| `role` | `role` | ✅ |
| `name` | `name` | ✅ |
| `capabilities` | `capabilities` | ✅ |
| `model` | (no direct equivalent) | ❌ Module 03 handles model routing |
| `system_prompt` | (no direct equivalent) | ❌ |
| `memory_profile` | `memory_access` | ⚠️ Different semantics |
| `tool_requirements` | `tools` | ✅ (name collision, different shape) |
| `constraints` | `runtime_constraints` | ✅ (nested structure) |
| `access_control` | (no direct equivalent) | ❌ |

Module 05 would need a mapping layer to convert `AgentDefinition` → `CreateAgentRequest`.

**Risk:** MEDIUM. Module 04's API is usable, but field translation is required.

---

## Finding 6: `department_id` — Loose Coupling (LOW)

Modules 03 and 04 both accept `department_id` as a plain `uuid` string, and both have it in their schemas with descriptions referencing "Module 05." However:

- **No cross-module `$ref`** exists. Module 04 does not import Module 05's `Department` schema.
- **No validation of `department_id` format/content** beyond UUID regex. Module 04 does not verify that the department exists in Module 05.
- **No bidirectional linkage.** Module 05 stores templates with agent/workflow references, but Module 03's workflow and Module 04's agent do not store a back-reference to Module 05.

**This is acceptable for now** — it's a loose identifier pattern, not a contract violation. But it means Module 05 is the single source of truth for department↔agent/workflow linkage. If Module 05's deployment creates agents via Module 04 without a back-reference, Module 04 cannot answer "what department does this agent belong to?" beyond what Module 05 told it.

---

## Finding 7: Deployment Lifecycle (HIGH — DEFERRED)

**PRD workflow:**
```
Select Template → Configure Policies → Connect Data Sources
→ Provision Memory → Deploy Swarm → Begin Operations
```

**Actual `handleDeploy` implementation:**
```go
deployment := &store.TemplateDeployment{
    Status: "select",  // Never advances from "select"
}
h.DeploymentStore.Create(deployment)
```

The `TemplateDeployment` schema includes statuses `select`, `configure`, `connect_data`, `provision_memory`, `deploy_swarm`, `operational`, `failed`, `rolled_back` — matching the PRD's 6-step workflow. But the code never advances between states. The `handleUpdateDeployment` handler accepts a PATCH to set the status, but that's manual state mutation, not orchestration.

**What's actually deployed when a user calls `POST /templates/{id}/deploy`:**
- A `TemplateDeployment` record is created in Module 05's in-memory store
- An `operan.templates.template.deployed` event is published
- That's it. No calls to Module 01, 02, 03, or 04.

**Risk:** This is expected at the current stage. Module 05's integration is deferred by design. The deployment contract promises behavior that doesn't exist yet. Consumers calling the deploy endpoint will get a record with status `"select"` and no further progress.

---

## Finding 8: Integration Graph vs Contract Alignment

The integration graph (`contracts/v1/integration-graph.yaml`) defines 11 outgoing edges from Module 05. Of the 4 edges targeting implemented modules:

| Edge | Target Contract Has Operation? | OperationId in Contract? | Contract Mentions Module 05? |
|------|------------------------------|------------------------|----------------------------|
| 05 → 01 (quota) | Yes — `getTenantQuota` | No | No |
| 05 → 02 (RBAC) | Yes — `evaluatePermission` | No | No |
| 05 → 03 (swarm) | Yes — `createWorkflow` | No | No (but has `department_id`) |
| 05 → 04 (agents) | Yes — `createAgent` | No | Yes — `department_id` descriptions |

**Gap:** None of the target module contracts mention Module 05 by operationId or explicitly reference it. Module 04 is the only module that acknowledges Module 05 in field descriptions.

**Risk:** LOW. The operations exist and can be called. The missing explicit references are documentation gaps, not functional blockers.

---

## Summary

| # | Finding | Severity | Actionable? | Notes |
|---|---------|----------|-------------|-------|
| 1 | ~~Config scaffolding — 7 unused endpoint fields~~ | **FIXED** | — | Removed 2026-06-XX (Module03Endpoint through SovereignEndpoint) |
| 2 | Module 01 — quota validation no specialized endpoint | LOW | No (data exists) | Module 05 must do quota arithmetic |
| 3 | Module 02 — RBAC/ABAC full support | OK | No | Well-aligned |
| 4 | Module 03 — DAG translation required | MEDIUM | Deferred | AgentDefinition → DAG conversion |
| 5 | Module 04 — field mapping required | MEDIUM | Deferred | AgentDefinition → CreateAgentRequest |
| 6 | `department_id` — loose coupling, no back-ref | LOW | No | Acceptable, Module 05 is source of truth |
| 7 | Deploy is no-op (deferred by design) | LOW* | Deferred | Expected at this stage |
| 8 | Integration graph edges unacknowledged in contracts | LOW | No | Documentation gap |

*\* Severity LOW for "deploy is no-op" because modules 06–20 have no code and the full orchestration requires all downstream modules. This is architectural scope, not a bug.*

---

## Recommendations

### Completed
1. ~~**Remove the 7 unused endpoint config fields**~~ ✅ Removed (Module03Endpoint through SovereignEndpoint)

### Deferred (when modules 06–20 are implemented)
2. **Define integration contracts** for modules 06–20 edges (memory topology, governance, observability, model routing, marketplace, connectors, sovereign deployment).
3. **Implement deploy orchestration** — convert `Template` → call Module 01 (quota), Module 02 (RBAC), Module 03 (DAG), Module 04 (agents), etc.
4. **Add back-references** in Module 03/04 schemas so agents/workflows can query Module 05 for department context.

---

*Report generated by REVIEW — integration audit only, no implementation*
*Files audited: 30 implementation files, 4 contract files (01–05), integration-graph.yaml, PRD*
*Scope: Contract alignment between Module 05 and modules 01–04 (implemented); modules 06–20 deferred*
