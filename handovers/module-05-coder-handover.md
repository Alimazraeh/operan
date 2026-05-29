# 📋 ARCH → CODER Handover: Module 05 — Department Template Engine

**Date:** 2026-05-29
**Review Verdict:** APPROVED — Ready for implementation
**Priority:** P0 — Next module in Wave 2 pipeline
**Architect:** ARCH
**Assignee:** CODER
**Module ID:** `05-department-template-engine`
**PRD Reference:** Section 5 (Module 05), Section 7 (Department Object Model)

---

## 🤖 CODER PERSONA & CONTEXT

You are **CODER_B** — the implementation specialist in the Operan manual agentic workflow. Your role is to take architectural reviews and contract specifications and produce **fully implemented, tested, contract-compliant Go code**.

### Your Responsibilities
- Read and implement against `contracts/v1/` (OpenAPI, JSON Schema, AsyncAPI)
- Write production-quality Go code following platform standards
- Achieve **≥80% test coverage** on all handler, store, and middleware packages
- Follow all **DO NOT VIOLATE** rules (see Global Constants below)
- Output complete files — do not truncate or summarize code

### Platform Standards (NON-NEGOTIABLE)
| Standard | Implementation |
|----------|---------------|
| **Auth** | `BearerAuth` (RSA/JWKS + HMAC fallback) via `Authorization` header |
| **Tenant Isolation** | `X-Tenant-ID` header extracted by middleware; all queries scoped by `tenant_id` from context |
| **Errors** | RFC 7807 Problem Details: `{ type, title, status, detail, instance, request_id }` |
| **Pagination** | Query params: `page`, `page_size`; Response wrapper: `{ data: T[], meta: { total, page, page_size, has_more } }` |
| **Schemas** | `additionalProperties: false` in all JSON schemas |
| **Timestamps** | ISO 8601 UTC |
| **IDs** | UUID v4 format |
| **Observability** | `X-Trace-Id`, `X-Request-Id` headers; structured JSON logging |

### Your Constraints
- ❌ NO imports from other module packages
- ❌ NO assumed interfaces beyond what's defined in contracts
- ❌ NO hardcoded secrets, tokens, or tenant IDs
- ❌ NO in-memory stubs for production-critical components
- ❌ NO string context keys — use typed `contextKey` constants

---

## 📊 CURRENT STATE ASSESSMENT

**Modules 01-04 are complete and serve as implementation references.**
**Module 05 has zero implementation — starting from scratch.**

### Contract Status (P0 FIXED)

| Layer | Status | Path |
|-------|--------|------|
| **OpenAPI** | ✅ Updated — RFC 7807, has_more, additionalProperties: false, 15 ops | `contracts/v1/openapi-05-department-template-engine.yaml` |
| **JSON Schema** | ✅ Updated — RFC 7807, has_more, deprecated enum | `contracts/v1/schema-05-department-template-engine.json` |
| **AsyncAPI** | ✅ Updated — `operan.templates.*` topics, 8 channels | `contracts/v1/asyncapi-05-department-template-engine.yaml` |
| **Blueprint** | ✅ Complete | `modules/05/Module 05 Department Template Engine.md` |

### Contract Coverage (P0 Phase)

| Category | Operations | Status |
|----------|-----------|--------|
| **Standard Template CRUD** | createTemplate, listTemplates, getTemplate, updateTemplate, deleteTemplate | ✅ Implemented in contract |
| **Deployment** | deployTemplate, listDeployments | ✅ Implemented in contract |
| **Custom Template CRUD** | createCustomTemplate, listCustomTemplates, getCustomTemplate, updateCustomTemplate, deleteCustomTemplate | ✅ Implemented in contract |
| **Versioning** | listTemplateVersions, getTemplateVersion | ✅ Implemented in contract |
| **Template Operations** | cloneTemplate | ✅ Implemented in contract |
| **AsyncAPI Channels** | created, updated, deployed, deployment_failed, undeployed, deleted, versioned, cloned | ✅ Implemented in contract |

**Total: 15 OpenAPI operations, 8 AsyncAPI channels — P0 scope covered.**

**P2+ operations to be phased in later:** bindAgent/unbindAgent, listBoundAgents, bindWorkflow/unbindWorkflow, listBoundWorkflows, compileGovernancePolicy, validateTemplateCompliance, instantiateTemplate, listTemplateInstances, getTemplateInstance, getInstantiationStatus, cancelInstantiation, publishToMarketplace, unpublishFromMarketplace, rollBackTemplate, getTemplateHealth, validateMemoryTopology, validateIntegrations.

---

## 📁 FILE REFERENCE INDEX

### Contracts (SOURCE OF TRUTH)

| Contract | Path | Details |
|----------|------|---------|
| OpenAPI | `contracts/v1/openapi-05-department-template-engine.yaml` | 15 ops, 5 path groups |
| JSON Schema | `contracts/v1/schema-05-department-template-engine.json` | 18 definitions |
| AsyncAPI | `contracts/v1/asyncapi-05-department-template-engine.yaml` | 8 channels |

### Architecture Blueprint

| Resource | Path |
|----------|------|
| Blueprint | `modules/05/Module 05 Department Template Engine.md` |
| Integration Graph | `contracts/v1/integration-graph.yaml` |

### Reference Files (Module 03/04 Patterns)

Use these as implementation patterns for Module 05:

| Reference | Path | Pattern |
|-----------|------|---------|
| JWTAuth | `modules/03-agent-orchestration/middleware/middleware.go` | HMAC JWT validation |
| Tenant Context | `modules/03-agent-orchestration/middleware/middleware.go` | Typed context keys |
| Pagination | `modules/03-agent-orchestration/handlers/handler_workflows.go` | `has_more` response |
| Event Publisher | `modules/03-agent-orchestration/events/events.go` | Topic naming, typed publish |
| Store Pattern | `modules/03-agent-orchestration/store/` | Tenant isolation |
| Config | `modules/03-agent-orchestration/config/config.go` | `ParseConfig()` env loading |
| DTO/Handler | `modules/04-agent-registry/handlers/dtos.go` | Schema-aligned DTOs |
| Test Patterns | `modules/04-agent-registry/handlers/*_test.go` | Table-driven tests |

---

## 🏗️ IMPLEMENTATION PLAN (Per Blueprint)

### Phase 1: Foundation (Sprint 1-2) — THIS SCOPE

#### 1.1 Project Structure
```
modules/05-department-template-engine/
├── main.go
├── config/
│   └── config.go
├── middleware/
│   └── middleware.go
├── handlers/
│   ├── templates.go          # Standard template CRUD
│   ├── custom_templates.go   # Custom template CRUD
│   ├── deployments.go        # Deploy lifecycle
│   ├── versions.go           # Version list/get
│   ├── clone.go              # Clone operation
│   └── dtos.go               # Shared DTOs
├── store/
│   ├── models.go             # DB models
│   ├── templates.go          # Standard template store
│   ├── custom_templates.go   # Custom template store
│   ├── deployments.go        # Deployment store
│   └── versions.go           # Version store
├── events/
│   └── events.go             # Event structs + publisher
└── handler/
    └── templates_test.go
    └── custom_templates_test.go
    └── deployments_test.go
    └── versions_test.go
    └── clone_test.go
    └── middleware_test.go
    └── store_test.go
    └── events_test.go
```

#### 1.2 PostgreSQL Schema (Core Tables)
```sql
-- templates table
CREATE TABLE templates (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    version VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    agents JSONB NOT NULL DEFAULT '[]',
    workflows JSONB NOT NULL DEFAULT '[]',
    memory_topology JSONB,
    governance_rules JSONB NOT NULL DEFAULT '[]',
    kpis JSONB NOT NULL DEFAULT '[]',
    integrations JSONB NOT NULL DEFAULT '[]',
    operational_policies JSONB NOT NULL DEFAULT '[]',
    metadata JSONB,
    tags TEXT[],
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_template_status CHECK (status IN ('draft', 'deprecated', 'published', 'archived'))
);

-- custom_templates table
CREATE TABLE custom_templates (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    content JSONB NOT NULL DEFAULT '{}',
    owner_id UUID NOT NULL,
    shared_with UUID[],
    version VARCHAR(20),
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_custom_status CHECK (status IN ('draft', 'deprecated', 'published', 'archived'))
);

-- template_versions table (immutable published artifacts)
CREATE TABLE template_versions (
    id UUID PRIMARY KEY,
    template_id UUID NOT NULL REFERENCES templates(id),
    version VARCHAR(20) NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- deployments table
CREATE TABLE deployments (
    id UUID PRIMARY KEY,
    template_id UUID NOT NULL REFERENCES templates(id),
    version VARCHAR(20) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'select',
    environment VARCHAR(20) NOT NULL,
    configuration JSONB,
    provisioned_entities JSONB,
    error_message TEXT,
    deployed_by UUID,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_deploy_status CHECK (status IN ('select', 'configure', 'connect_data', 'provision_memory', 'deploy_swarm', 'operational', 'failed', 'rolled_back'))
);

-- indexes
CREATE INDEX idx_templates_tenant ON templates(tenant_id);
CREATE INDEX idx_templates_tenant_status ON templates(tenant_id, status);
CREATE INDEX idx_custom_templates_tenant ON custom_templates(tenant_id);
CREATE INDEX idx_deployments_template ON deployments(template_id);
CREATE INDEX idx_deployments_tenant ON deployments(tenant_id);
CREATE INDEX idx_versions_template ON template_versions(template_id);
```

#### 1.3 Redis Cache Layer
- `template:{id}` — Hot template lookup cache (TTL: 5min)
- `template:version:{id}:{version}` — Versioned template cache
- `deployment:{id}` — Active deployment state

---

## 🔧 REMEDIATION PLAN

### Phase 1: Core Implementation

#### 1.1 Config + Middleware (Foundation)
**Files:** `config/config.go`, `middleware/middleware.go`

- `config.ParseConfig()` — Load from env: `DB_URL`, `REDIS_URL`, `JWT_SECRET`, `EVENT_BROKER_URL`, `OTLP_ENDPOINT`, `TEMPLATE_CACHE_TTL`, `MAX_PAGE_SIZE`
- JWT middleware — RSA/JWKS + HMAC fallback (pattern from Module 03)
- Typed context keys: `TenantIDKey`, `UserIDKey`, `UserTypeKey`, `TraceIDKey`, `RequestIDKey`
- TraceID middleware: generate or propagate `X-Trace-Id`
- RequestID middleware: generate `X-Request-Id`
- Logger middleware: structured JSON with trace/request/tenant IDs
- `ExtractTenant` middleware: inject `tenant_id` from JWT + `X-Tenant-ID` header into context

#### 1.2 Store Layer (PostgreSQL + Redis)
**Files:** `store/models.go`, `store/templates.go`, `store/custom_templates.go`, `store/deployments.go`, `store/versions.go`

**Pattern from Module 03/04 store:**
- All stores have `byTenant` index for tenant isolation
- `Create(ctx, item)`, `GetByID(ctx, id)`, `List(ctx, tenantID, page, pageSize)`, `Update(ctx, item)`, `Delete(ctx, id)`
- Redis cache layer: `Get` reads cache first, `Create/Update/Delete` invalidate

#### 1.3 Handlers (Contract-Aligned)
**Files:** `handlers/templates.go`, `handlers/custom_templates.go`, `handlers/deployments.go`, `handlers/versions.go`, `handlers/clone.go`, `handlers/dtos.go`

**CRITICAL RULES:**
- **NEVER** read `tenant_id` from request body — always use `tenant.TenantID(ctx)`
- All DTOs must match OpenAPI schema field-by-field
- All responses use RFC 7807 Error format
- All list responses use `data` + `meta` wrapper with `has_more`
- All route handlers wrapped with `ExtractTenant` middleware

#### 1.4 Event Publisher
**File:** `events/events.go`

**Topics (8 channels — all `operan.templates.*` format):**
| Event | Topic | Go Struct Name |
|-------|-------|----------------|
| template.created | `operan.templates.template.created` | `TemplateCreatedEvent` |
| template.updated | `operan.templates.template.updated` | `TemplateUpdatedEvent` |
| template.deployed | `operan.templates.template.deployed` | `TemplateDeployedEvent` |
| template.deployment_failed | `operan.templates.template.deployment_failed` | `TemplateDeploymentFailedEvent` |
| template.undeployed | `operan.templates.template.undeployed` | `TemplateUndeployedEvent` |
| template.deleted | `operan.templates.template.deleted` | `TemplateDeletedEvent` |
| template.versioned | `operan.templates.template.versioned` | `TemplateVersionedEvent` |
| template.cloned | `operan.templates.template.cloned` | `TemplateClonedEvent` |

Each struct must include: `event`, `template_id`, `tenant_id`, timestamps, plus event-specific fields.

### Phase 2: Wiring (main.go)
- Wire config → handlers
- Wire middleware chain: JWTAuth → TraceID → RequestID → Logger → ExtractTenant
- Register all routes under `/api/v1/templates` (not `/templates`)
- Register event publisher

### Phase 3: Tests
- Table-driven tests for all handlers (success, 400, 401, 403, 404, 409, 422, 500)
- Store tests with tenant isolation verification
- Event struct validation tests
- Integration tests: deploy flow from template creation to operational status

---

## 📋 DEFINITION OF DONE

**Before submitting for REVIEW:**

- [ ] Config parsed from env vars via `ParseConfig()`
- [ ] All 6 middleware implemented and wired in `main.go`
- [ ] PostgreSQL tables created (5 tables + indexes)
- [ ] Redis cache layer with invalidate-on-write
- [ ] All 15 OpenAPI operations implemented
- [ ] All 8 AsyncAPI channels with typed Go structs
- [ ] Event publisher with real AMQP/Kafka (not stubs)
- [ ] RFC 7807 errors on ALL error responses
- [ ] `has_more` pagination on ALL list responses
- [ ] Tenant isolation on ALL store operations
- [ ] All routes under `/api/v1/templates`
- [ ] `tenant_id` NEVER read from request body
- [ ] ≥80% test coverage on handlers + stores
- [ ] `manifest.json` with coverage metric and `contract_compliant: true`

---

## 🚨 COMMON PITFALLS

1. **Reading tenant_id from request body** — Always use `tenant.TenantID(ctx)`. This is the #1 mistake.
2. **Using string context keys** — Use typed `contextKey` structs.
3. **Forgetting tenant isolation in stores** — All 4 stores need `byTenant` indexes.
4. **Wrong topic naming** — Must be `operan.templates.{entity}.{event}`, NOT `module05/...`.
5. **Embedding full objects where references expected** — Blueprint uses references to Modules 03/04/07/10; store the references, resolve at runtime.
6. **Status enum `reviewed`** — Must be `deprecated` per platform standard.
7. **Pagination `total_pages`** — Must be `has_more` per platform standard.
8. **Unwrapped routes** — Every route must pass through `ExtractTenant`.

---

## 📊 DEPENDENCY MAP

Module 05 depends on (read-only calls from handlers):

| Module | Purpose | Protocol |
|--------|---------|----------|
| 01 Tenant Control Plane | Validate tenant quota before deploy | REST |
| 02 IAM | RBAC/ABAC before state changes | REST |
| 03 Agent Orchestration | Resolve workflow references | REST |
| 04 Agent Registry | Validate agent references | REST |
| 07 Memory Fabric | Provision collections per topology | REST |
| 10 Policy/Governance | Compile policies → OPA/Rego | REST |
| 11 Observability | Generate dashboard configs | REST |
| 18 Enterprise Connector | Validate integration configs | REST |
| 20 Sovereign Deployment | Enforce data residency | REST |

**No module calls back to Module 05.** Strict downstream-only dependency direction.

---

## 📁 HANDOVER SUMMARY

| Item | Status |
|------|--------|
| OpenAPI contract (15 ops) | ✅ Fixed and locked |
| JSON Schema (18 definitions) | ✅ Fixed and locked |
| AsyncAPI (8 channels) | ✅ Fixed and locked |
| Architectural blueprint | ✅ Complete |
| Integration graph | ✅ Module 05 registered |
| Implementation pattern refs | ✅ Module 03/04 available |
| PostgreSQL schema design | ✅ In blueprint |
| P0-P1-P2-P3 scope defined | ✅ In blueprint |
| CODER handover | ✅ THIS DOCUMENT |

**Ready to begin implementation.**

---

**Handover Complete. Begin scaffolding.**
