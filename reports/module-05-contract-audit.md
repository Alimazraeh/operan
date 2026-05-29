# Module 05 — Department Template Engine: Contract Audit & Remediation Plan

**Audit Date:** 2026-05-28  
**Auditor:** Qwen Code Architect Agent  
**Verdict:** **REJECT — Contract revision required before scaffolding**

---

## Executive Summary

Module 05's contracts are at approximately **26% coverage** of the architectural blueprint:
- **10 OpenAPI operations** implemented vs. **~38 expected** by the blueprint
- **5 AsyncAPI channels** implemented vs. **~22 expected** by the blueprint
- **8 field-level drifts** detected between blueprint model and contract schemas
- **3 platform standard violations** in error handling, pagination, and schema enforcement

The contracts require substantial revision before scaffolding or implementation can begin.

---

## 1. Platform Standards Violations (CRITICAL)

### 1.1 Error Schema Not RFC 7807 Compliant

**Current:**
```yaml
Error:
  type: object
  required: [code, message]
  properties:
    code: { type: integer }
    message: { type: string }
    details: { type: object, additionalProperties: true }
    trace_id: { type: string, format: uuid }
```

**Expected (per platform standard + Module 04):**
```yaml
Error:
  type: object
  required: [type, title, status, detail]
  properties:
    type: { type: string, format: uri, description: "About URL" }
    title: { type: string }
    status: { type: integer }
    detail: { type: string }
    instance: { type: string, format: uri }
    request_id: { type: string, format: uuid }
```

**Why:** RFC 7807 (Problem Details for HTTP APIs) is the platform-wide standard. `trace_id` is not a recognized field; `request_id` is.

**Fix:** Replace Error schema across all 3 contract files (OpenAPI, AsyncAPI payloads, JSON Schema).

---

### 1.2 Pagination Missing `has_more`

**Current:**
```yaml
PaginationMeta:
  type: object
  required: [total, page, page_size, total_pages]
  properties:
    total: { type: integer }
    page: { type: integer }
    page_size: { type: integer }
    total_pages: { type: integer }
```

**Expected:**
```yaml
PaginationMeta:
  type: object
  required: [total, page, page_size, has_more]
  properties:
    total: { type: integer }
    page: { type: integer }
    page_size: { type: integer }
    has_more: { type: boolean }
```

**Why:** `has_more` is the platform-standard pagination indicator (Module 04 contract). `total_pages` should be removed.

**Fix:** Replace `PaginationMeta` in all 3 contract files. Update all paginated response schemas.

---

### 1.3 Missing `additionalProperties: false` on OpenAPI Schemas

**Current:** 15+ object schemas in OpenAPI use `additionalProperties: true` or omit it entirely.

**Expected:** All domain schemas MUST have `additionalProperties: false` per platform standard.

**Affected schemas (OpenAPI only — JSON Schema is already correct):**
- `AgentDefinition` (missing)
- `WorkflowDefinition` (missing)
- `WorkflowStep` (missing)
- `MemoryTopology` (missing)
- `GovernanceRule` (missing)
- `KPIDefinition` (missing)
- `IntegrationDefinition` (missing)
- `OperationalPolicy` (missing)
- `Template` (missing)
- `TemplateCreate` (missing)
- `TemplatePatch` (missing)
- `TemplateDeployment` (missing)
- `TemplateDeployRequest` (missing)
- `CustomTemplate` (missing)
- `CustomTemplateCreate` (missing)
- `CustomTemplatePatch` (missing)
- `Error` (missing — already in JSON Schema)

**Why:** `additionalProperties: false` prevents clients from sending unknown fields, enforcing strict schema validation.

**Fix:** Add `additionalProperties: false` to every domain object schema in the OpenAPI contract.

---

### 1.4 Missing `tenant_id` on Domain Objects

**Expected per platform standard:** All domain objects that are tenant-scoped MUST include a `tenant_id` field in their response schemas.

**Affected schemas:**
- `Template` (missing `tenant_id`)
- `TemplateCreate` (missing `tenant_id` — should be derived from context, not request body)
- `TemplatePatch` (missing `tenant_id`)
- `CustomTemplate` (missing `tenant_id`)
- `TemplateDeployment` (missing `tenant_id`)
- `TemplateDeployRequest` (missing `tenant_id`)

**Why:** `tenant_id` is needed for tenant-isolation enforcement and audit logging.

**Fix:** Add `tenant_id` to response schemas. Remove from request schemas where it will be derived from JWT/context.

---

### 1.5 Paginated Response Uses Inline Schema Instead of Typed Wrapper

**Current (listTemplates response):**
```yaml
'200':
  content:
    application/json:
      schema:
        type: object
        properties:
          data:
            type: array
            items: { $ref: '...' }
          meta:
            $ref: '#/components/schemas/PaginationMeta'
        required: [data, meta]
```

**Expected:**
```yaml
'200':
  content:
    application/json:
      schema:
        $ref: '#/components/schemas/PaginatedTemplateResponse'
```

**Fix:** Define typed `PaginatedTemplateResponse` and `PaginatedDeploymentResponse` wrapper schemas, reference them in all list endpoints.

---

## 2. Operations Gap (HIGH)

### 2.1 Current vs Expected Operations

| # | Expected Operation | Blueprint Purpose | Contract Status |
|---|-------------------|-------------------|-----------------|
| 1 | `createTemplate` | Create standard template | ✅ Implemented |
| 2 | `listTemplates` | List templates with filters | ✅ Implemented |
| 3 | `getTemplate` | Get template by ID | ✅ Implemented |
| 4 | `updateTemplate` | Update template | ✅ Implemented |
| 5 | `deleteTemplate` | Delete template | ✅ Implemented |
| 6 | `deployTemplate` | Deploy template | ✅ Implemented |
| 7 | `listDeployments` | List template deployments | ✅ Implemented |
| 8 | `createCustomTemplate` | Create free-form template | ✅ Implemented |
| 9 | `getCustomTemplate` | Get custom template by ID | ✅ Implemented |
| 10 | `updateCustomTemplate` | Update custom template | ✅ Implemented |
| — | `deleteCustomTemplate` | Delete custom template | ❌ **MISSING** |
| — | `listCustomTemplates` | List custom templates | ❌ **MISSING** |
| — | `getTemplateVersion` | Get specific template version | ❌ **MISSING** |
| — | `listTemplateVersions` | List all versions of a template | ❌ **MISSING** |
| — | `cloneTemplate` | Clone a template | ❌ **MISSING** |
| — | `instantiateTemplate` | Create runtime instance from template | ❌ **MISSING** |
| — | `listTemplateInstances` | List all runtime instances | ❌ **MISSING** |
| — | `getTemplateInstance` | Get instance by ID | ❌ **MISSING** |
| — | `bindAgent` | Bind agent to template | ❌ **MISSING** |
| — | `unbindAgent` | Unbind agent from template | ❌ **MISSING** |
| — | `listBoundAgents` | List agents bound to template | ❌ **MISSING** |
| — | `bindWorkflow` | Bind workflow to template | ❌ **MISSING** |
| — | `unbindWorkflow` | Unbind workflow from template | ❌ **MISSING** |
| — | `listBoundWorkflows` | List workflows bound to template | ❌ **MISSING** |
| — | `compileGovernancePolicy` | Compile template → OPA/Rego | ❌ **MISSING** |
| — | `validateTemplateCompliance` | Validate template against governance | ❌ **MISSING** |
| — | `publishToMarketplace` | Publish template to Module 15 | ❌ **MISSING** |
| — | `unpublishFromMarketplace` | Unpublish from Module 15 | ❌ **MISSING** |
| — | `getTemplateHealth` | Get deployment health metrics | ❌ **MISSING** |
| — | `rollBackTemplate` | Roll back a template deployment | ❌ **MISSING** |
| — | `validateMemoryTopology` | Validate memory → Module 07 mapping | ❌ **MISSING** |
| — | `validateIntegrations` | Validate connectors → Module 18 | ❌ **MISSING** |
| — | `getInstantiationStatus` | Check instantiation progress | ❌ **MISSING** |
| — | `cancelInstantiation` | Cancel a running instantiation | ❌ **MISSING** |

**Total: 10/37 implemented (27% coverage)**

### 2.2 Required New Operations by Phase

**Phase 1 (Sprint 1-2) — Core CRUD + Versioning:**
1. `deleteCustomTemplate`
2. `listCustomTemplates`
3. `getTemplateVersion` (GET `/templates/{id}/versions/{version}`)
4. `listTemplateVersions` (GET `/templates/{id}/versions`)
5. `cloneTemplate` (POST `/templates/{id}/clone`)

**Phase 2 (Sprint 2-3) — Binding & Resolution:**
6. `bindAgent` (POST `/templates/{id}/bindings/agents`)
7. `unbindAgent` (DELETE `/templates/{id}/bindings/agents/{agent_id}`)
8. `listBoundAgents` (GET `/templates/{id}/bindings/agents`)
9. `bindWorkflow` (POST `/templates/{id}/bindings/workflows`)
10. `unbindWorkflow` (DELETE `/templates/{id}/bindings/workflows/{workflow_id}`)
11. `listBoundWorkflows` (GET `/templates/{id}/bindings/workflows`)
12. `validateMemoryTopology` (POST `/templates/{id}/validate/memory`)
13. `validateIntegrations` (POST `/templates/{id}/validate/integrations`)

**Phase 3 (Sprint 3-4) — Governance & Instantiation:**
14. `compileGovernancePolicy` (POST `/templates/{id}/governance/compile`)
15. `validateTemplateCompliance` (GET `/templates/{id}/governance/compliance`)
16. `instantiateTemplate` (POST `/templates/{id}/instantiate`)
17. `listTemplateInstances` (GET `/templates/{id}/instances`)
18. `getTemplateInstance` (GET `/templates/{id}/instances/{instance_id}`)
19. `getInstantiationStatus` (GET `/templates/{id}/instances/{instance_id}/status`)
20. `cancelInstantiation` (POST `/templates/{id}/instances/{instance_id}/cancel`)

**Phase 4 (Sprint 4-5) — Marketplace & Production:**
21. `publishToMarketplace` (POST `/templates/{id}/marketplace/publish`)
22. `unpublishFromMarketplace` (POST `/templates/{id}/marketplace/unpublish`)
23. `rollBackTemplate` (POST `/templates/{id}/deployments/{deployment_id}/rollback`)
24. `getTemplateHealth` (GET `/templates/{id}/health`)

---

## 3. AsyncAPI Channels Gap (HIGH)

### 3.1 Current vs Expected Channels

| # | Expected Channel | Blueprint Topic Pattern | Contract Status |
|---|-----------------|------------------------|-----------------|
| 1 | `template.created` | `operan.templates.template.created` | ✅ Implemented |
| 2 | `template.updated` | `operan.templates.template.updated` | ✅ Implemented |
| 3 | `template.deployed` | `operan.templates.template.deployed` | ✅ Implemented |
| 4 | `template.deployment_failed` | `operan.templates.template.deployment_failed` | ✅ Implemented |
| 5 | `template.undeployed` | `operan.templates.template.undeployed` | ✅ Implemented |
| — | `template.deleted` | `operan.templates.template.deleted` | ❌ **MISSING** |
| — | `template.versioned` | `operan.templates.template.versioned` | ❌ **MISSING** |
| — | `template.cloned` | `operan.templates.template.cloned` | ❌ **MISSING** |
| — | `template.instantiated` | `operan.templates.instance.instantiated` | ❌ **MISSING** |
| — | `template.instantiation_failed` | `operan.templates.instance.instantiation_failed` | ❌ **MISSING** |
| — | `template.agent_bound` | `operan.templates.template.agent_bound` | ❌ **MISSING** |
| — | `template.agent_unbound` | `operan.templates.template.agent_unbound` | ❌ **MISSING** |
| — | `template.workflow_bound` | `operan.templates.template.workflow_bound` | ❌ **MISSING** |
| — | `template.workflow_unbound` | `operan.templates.template.workflow_unbound` | ❌ **MISSING** |
| — | `template.memory_provisioned` | `operan.templates.template.memory_provisioned` | ❌ **MISSING** |
| — | `template.policy_aligned` | `operan.templates.template.policy_aligned` | ❌ **MISSING** |
| — | `template.compliance_checked` | `operan.templates.template.compliance_checked` | ❌ **MISSING** |
| — | `template.marketplace_published` | `operan.templates.template.marketplace_published` | ❌ **MISSING** |
| — | `template.marketplace_unpublished` | `operan.templates.template.marketplace_unpublished` | ❌ **MISSING** |
| — | `template.rolled_back` | `operan.templates.template.rolled_back` | ❌ **MISSING** |
| — | `template.health_alert` | `operan.templates.template.health_alert` | ❌ **MISSING** |
| — | `template.integration_validated` | `operan.templates.template.integration_validated` | ❌ **MISSING** |

**Total: 5/22 implemented (23% coverage)**

### 3.2 Topic Naming Mismatch

**Current format:** `module05/template/created`  
**Blueprint format:** `operan.templates.template.created`  
**Platform standard format:** `operan.{module_name_singular}.{entity}.{event}`

**Fix:** Rename all 5 existing channels to use the `operan.templates.{entity}.{event}` pattern.

---

## 4. Field-Level Drift: Blueprint vs Contract (MEDIUM)

### 4.1 `memory_topology` — Structural Drift

| Field | Blueprint | Contract |
|-------|-----------|----------|
| Structure | `semantic_collections: [string]`, `episodic_retention_days: int`, `graph_schema_ref: string`, `vector_model_hint: string` | `semantic_enabled: bool`, `episodic_enabled: bool`, `procedural_enabled: bool`, `graph_enabled: bool` |
| Type | Detailed collection references | Boolean enable/disable flags |

**Fix:** Replace boolean flags with collection-level details matching Module 07 (Memory Fabric) conventions.

### 4.2 `agents` — Reference vs Embedded

| Aspect | Blueprint | Contract |
|--------|-----------|----------|
| Structure | `{ agent_ref: string, role: string, min_instances: int, max_instances: int, capabilities_required: [string], memory_access: [string] }` | Full embedded `AgentDefinition` object |

**Fix:** Replace embedded `AgentDefinition` with reference-based model. Agent detail should be resolved via Module 04 at runtime.

### 4.3 `workflows` — Reference vs Embedded

| Aspect | Blueprint | Contract |
|--------|-----------|----------|
| Structure | `{ workflow_ref: string, trigger: event|schedule|manual, priority: low|medium|high, execution_stack: langgraph|temporal|ray|celery }` | Full embedded `WorkflowDefinition` object |

**Fix:** Replace embedded `WorkflowDefinition` with reference-based model. Workflow detail should be resolved via Module 03 at runtime.

### 4.4 `governance_rules` — Reference vs Embedded

| Aspect | Blueprint | Contract |
|--------|-----------|----------|
| Structure | `{ policy_ref: string, enforcement: strict|advisory, scope: agent|workflow|department }` + escalation_chains | Full embedded `GovernanceRule` object |

**Fix:** Replace with reference-based model. Governance rules should be resolved via Module 10 at runtime.

### 4.5 `kpis` — Field Rename

| Field | Blueprint | Contract |
|-------|-----------|----------|
| Key field | `metric: string` (Module 11 metric path) | `metric_type: enum[...], unit: string` |
| Extra | `target: float, aggregation_window: 1h|24h|7d` | `aggregation_period: enum[...], thresholds: {warning, critical}` |

**Fix:** Align with Module 11 observability metric conventions.

### 4.6 `integrations` — Reference vs Embedded

| Aspect | Blueprint | Contract |
|--------|-----------|----------|
| Structure | `{ connector_ref: string, config: object, auth_method: oauth2|api_key|mtls, data_flow: inbound|outbound|bidirectional }` | Full embedded `IntegrationDefinition` object |

**Fix:** Replace with reference-based model. Connector details should be resolved via Module 18 at runtime.

### 4.7 Missing Blueprint Fields in Contract

| Blueprint Field | Present in Contract? | Action |
|----------------|---------------------|--------|
| `purpose` (string) | ❌ No | Add to `Template`/`TemplateCreate` |
| `deployment_flow` (list of steps) | ❌ No | Add to `Template` |
| `estimated_resources` (cpu, memory, storage, cost) | ❌ No | Add to `Template` |
| `observability_profile` | ❌ No | Add to `Template` |
| `marketplace_visible` (boolean) | ❌ No | Add to `Template` |
| `published_by` (user ID) | ❌ No | Add to `Template` |
| `tenant_id` | ❌ No (on Template/CustomTemplate) | Add to response schemas |

### 4.8 Status Enum Mismatch

| | Blueprint | Contract |
|---|-----------|----------|
| `Template.status` | `draft \| published \| deprecated \| archived` | `draft \| reviewed \| published \| archived` |
| `CustomTemplate.status` | — | `draft \| published \| archived` (no `deprecated`) |

**Fix:** Use `deprecated` instead of `reviewed` for standard templates. Add `deprecated` to custom template status enum.

### 4.9 `TemplateDeployRequest.version` Type

**Blueprint:** semver string (`^\\d+\\.\\d+\\.\\d+$`)  
**Contract:** generic string

**Fix:** Add pattern validation to `TemplateDeployRequest.version`.

---

## 5. JSON Schema Audit

The JSON Schema (`schema-05-department-template-engine.json`) is **better aligned** with platform standards than the OpenAPI contract:
- ✅ All domain objects have `additionalProperties: false`
- ✅ All domain objects use proper `$ref` patterns
- ✅ Error schema has `additionalProperties: false`

**Remaining JSON Schema issues:**
1. Error schema: missing RFC 7807 fields (`type`, `title`, `status`, `detail`)
2. PaginationMeta: uses `total_pages` instead of `has_more`
3. Missing `tenant_id` on domain objects
4. Same field-level drift as OpenAPI (agents, workflows, memory_topology, etc.)

---

## 6. Integration Graph

Module 05 is correctly registered in `integration-graph.yaml` but needs edge definitions for its downstream dependencies:
- Module 01 (Tenant Control Plane)
- Module 02 (IAM)
- Module 03 (Agent Orchestration)
- Module 04 (Agent Registry)
- Module 07 (Memory Fabric)
- Module 10 (Policy/Governance)
- Module 11 (Observability)
- Module 13 (Multi-Model Routing)
- Module 15 (Agent Marketplace)
- Module 18 (Enterprise Connector Fabric)
- Module 20 (Sovereign Deployment)

---

## 7. Remediation Priority Order

### P0 — Blockers (Must Fix Before Scaffolding)

1. **Fix Error schema to RFC 7807** (all 3 contracts)
2. **Replace `PaginationMeta` with `has_more`** (all 3 contracts)
3. **Add `additionalProperties: false`** to all 17 OpenAPI schemas
4. **Rename AsyncAPI topics** to `operan.templates.{entity}.{event}` format
5. **Add missing operations**: `deleteCustomTemplate`, `listCustomTemplates`, `getTemplateVersion`, `listTemplateVersions`, `cloneTemplate`

### P1 — High Priority (Before Implementation)

6. **Align `memory_topology`**: replace boolean flags with collection details
7. **Switch `agents`/`workflows`/`integrations` to reference-based** models
8. **Add `tenant_id`** to all response schemas
9. **Add missing blueprint fields**: `purpose`, `deployment_flow`, `estimated_resources`, `observability_profile`, `marketplace_visible`
10. **Fix status enum**: `reviewed` → `deprecated`
11. **Add `deployments/{id}/rollback` operation**

### P2 — Medium Priority (Can Defer to Phased Implementation)

12. **Add remaining AsyncAPI channels** (17 missing)
13. **Add binding operations** (6 operations)
14. **Add instantiation operations** (5 operations)
15. **Add governance operations** (2 operations)
16. **Add marketplace operations** (2 operations)
17. **Add health monitoring operations** (2 operations)

### P3 — Low Priority (Post-Implementation)

18. **Refine KPI schema** to match Module 11 conventions
18. **Add `Paginated*` typed response wrappers** (currently inline schemas)
19. **Add version pattern validation** on `TemplateDeployRequest.version`
20. **Add `Deprecated` status to `CustomTemplate`**

---

## 8. Recommended Contract Revision Process

1. **Generate updated OpenAPI contract** with P0 fixes
2. **Run contract validation** against platform standards (RFC 7807, pagination, additionalProperties)
3. **Update JSON Schema** to match OpenAPI changes
4. **Update AsyncAPI** with renamed topics + 17 new channels
5. **Cross-spec validation**: ensure OpenAPI responses reference AsyncAPI event schemas consistently
6. **Architectural review** of revised contracts before scaffolding

---

## 9. Scaffolding Readiness

| Dimension | Status | Notes |
|-----------|--------|-------|
| Contract completeness | ❌ 27% (10/37 ops) | P0-P1 fixes needed first |
| Platform standards compliance | ❌ 3 violations | Error, pagination, additionalProperties |
| Blueprint alignment | ❌ 8 drifts | memory_topology, reference vs embedded |
| AsyncAPI coverage | ❌ 23% (5/22 channels) | Topic naming + 17 missing channels |
| Integration graph | ⚠️ Registered | Dependencies need explicit edges |
| Blueprint document | ✅ Complete | Detailed architecture, phased plan |

**Recommendation:** Block scaffolding until P0 issues are resolved. P1-P2 items can be addressed in the phased implementation plan.
