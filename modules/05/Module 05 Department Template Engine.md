# Module 05: Department Template Engine — Architectural Blueprint

## ✅ Verdict: **Architecturally Aligned with Platform Standards**

Module 05 serves as the **blueprint factory** for Operan's agentic departments. It transforms abstract organizational designs into deployable, governed, multi-agent runtime configurations. The architecture below directly leverages lessons from Modules 02/03/04 audits and aligns with the PRD's "Select Template → Configure → Deploy → Operate" deployment flow.

---

## 🏗️ High-Level Architecture (Target State)

```
┌─────────────────────────────────────────────────────┐
│                 API Gateway Layer                    │
│  Base: /api/v1/templates                             │
│  Security: BearerAuth (JWT) + X-Tenant-ID header    │
│  ~38 OpenAPI operations across 10 path groups       │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Middleware Chain                     │
│  • JWTAuth (RSA via JWKS + HMAC fallback)           │
│  • TenantContext + Typed Context Keys               │
│  • RBAC/ABAC Evaluation Proxy (→ Module 02)         │
│  • Policy Validation Hook (→ Module 10)             │
│  • Request Validation (JSON Schema strict mode)     │
│  • TraceID / RequestID / Structured Logger          │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Service / Handler Layer              │
│  • Template CRUD + Version Lifecycle                │
│  • Template Instantiation Engine (→ Module 03)      │
│  • Agent-to-Workflow Binding Resolver               │
│  • Memory Topology Mapper (→ Module 07)             │
│  • Integration Config Validator (→ Module 18)       │
│  • KPI/Observability Profile Compiler               │
│  • Escalation Rule Template Processor               │
│  • Governance Policy Alignment (OPA/Rego hooks)     │
│  • Marketplace Publish/Unpublish Workflow           │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Persistence Layer                    │
│  • PostgreSQL (Primary): Templates, Versions,        │
│    Bindings, Policies, KPIs, Escalation Rules       │
│  • Redis (Cache): Hot template lookups,              │
│    instantiation cache, semantic search index       │
│  • Repository Pattern with tenant-scoped queries    │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Event Layer (AsyncAPI)               │
│  • ~22 channels                                      │
│  • Topic format: operan.templates.{entity}.{event}   │
│  • Events: created, versioned, instantiated,         │
│    deployed, policy_aligned, marketplace_published   │
│  • Real AMQP/Kafka publisher (no stubs)              │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│            Observability & Configuration             │
│  • Config: 12+ env vars (DB, Cache, Broker, JWT,     │
│    OTLP, Policy Engine, Module 03/07/18 endpoints)  │
│  • OpenTelemetry traces/metrics → OTLP collector     │
│  • Structured JSON logging with trace/tenant/request │
└─────────────────────────────────────────────────────┘
```

---

## 📦 Core Capabilities → Architectural Mapping (Per PRD)

| PRD Capability | Architectural Implementation | Key Components |
|----------------|------------------------------|----------------|
| **Deploy reusable organizational structures** | Template instantiation engine that resolves agents, workflows, memory, policies into executable department config | `InstantiationEngine`, `BindingResolver`, `DeploymentOrchestrator` |
| **Agents binding** | References Module 04 registry; resolves agent capabilities → workflow node requirements | `AgentBindingService`, `CapabilityMatcher` |
| **Workflows binding** | References Module 03 orchestration; validates DAG compatibility, execution stack selection | `WorkflowBindingService`, `StackRouter` |
| **Memory topology** | Maps semantic/episodic/graph memory requirements to Module 07 collections; enforces tenant isolation | `MemoryTopologyMapper`, `CollectionProvisioner` |
| **Governance rules** | Compiles template-level policies into Module 10 OPA/Rego rules; validates at instantiate/deploy | `PolicyCompiler`, `ComplianceValidator` |
| **KPIs** | Defines observability profiles consumed by Module 11; auto-generates dashboards/alerts | `KPICompiler`, `ObservabilityProfileBuilder` |
| **Integrations** | Validates connector configs against Module 18; generates secure credential bindings | `IntegrationValidator`, `ConnectorConfigBuilder` |
| **Operational policies** | Embeds escalation rules, approval chains, human-in-the-loop gates into workflow definitions | `EscalationRuleEngine`, `ApprovalChainBuilder` |

---

## 🗃️ Department Template Metadata Model (Expanded from PRD)

```yaml
department_template:
  id: uuid-v4
  tenant_id: uuid-v4
  name: string
  purpose: string
  version: semver (e.g., "2.1.0")
  status: draft | published | deprecated | archived
  category: hr | finance | procurement | engineering | legal | compliance | research | custom
  
  # Core composition (PRD: "A department is...")
  agents:
    - agent_ref: string (Module 04 agent_id)
      role: string
      min_instances: int
      max_instances: int
      capabilities_required: [string]
      memory_access: [semantic | episodic | graph]
      
  workflows:
    - workflow_ref: string (Module 03 workflow_id)
      trigger: event | schedule | manual
      priority: low | medium | high
      execution_stack: langgraph | temporal | ray | celery
      
  memory_topology:
    semantic_collections: [string]  # Module 07 collection names
    episodic_retention_days: int
    graph_schema_ref: string
    vector_model_hint: string
    
  governance_rules:
    - policy_ref: string (Module 10 policy_id)
      enforcement: strict | advisory
      scope: agent | workflow | department
    - escalation_chains:
        - condition: string (Rego/JSON logic)
          target: agent_id | human_role | external_webhook
          timeout_ms: int
          severity: low | medium | high | critical
          
  kpis:
    - name: string
      metric: string (Module 11 metric path)
      target: float
      threshold_warning: float
      threshold_critical: float
      aggregation_window: 1h | 24h | 7d
      
  integrations:
    - connector_ref: string (Module 18 connector_id)
      config: object (validated against connector schema)
      auth_method: oauth2 | api_key | mtls
      data_flow: inbound | outbound | bidirectional
      
  observability_profile:
    trace_sampling_rate: float (0.0-1.0)
    log_level: debug | info | warn | error
    metrics_enabled: [list of metric families]
    alert_rules_ref: [string]  # Module 11 alert rule IDs
    
  # Deployment metadata
  deployment_flow:
    - step: select_template
    - step: configure_policies
    - step: connect_data_sources
    - step: provision_memory
    - step: deploy_swarm
    - step: begin_operations
  estimated_resources:
    cpu_millicores: int
    memory_mb: int
    vector_storage_gb: float
    estimated_monthly_cost_usd: float
    
  created_at: ISO8601
  updated_at: ISO8601
  published_by: string (user/service ID)
  marketplace_visible: boolean  # For Module 15 integration
```

---

## 🔗 Integration Points & Dependency Graph

```
Module 05 (Department Template Engine)
    │
    ├──→ Module 01 (Tenant Control Plane)
    │     • Validates tenant quota/resources before instantiation
    │     • Reports deployment status back to tenant lifecycle
    │
    ├──→ Module 02 (IAM)
    │     • RBAC/ABAC evaluation before template publish/instantiate
    │     • Agent identity resolution for template bindings
    │
    ├──→ Module 03 (Agent Orchestration)
    │     • Resolves workflow references → executable DAGs
    │     • Triggers department deployment via orchestration API
    │     • Consumes workflow execution events for KPI tracking
    │
    ├──→ Module 04 (Agent Registry)
    │     • Validates agent references exist & are compatible
    │     • Fetches capability metadata for binding resolution
    │
    ├──→ Module 07 (Memory Fabric)
    │     • Provisions memory collections per topology spec
    │     • Validates vector model compatibility
    │
    ├──→ Module 10 (Policy/Governance)
    │     • Compiles template policies → OPA/Rego rules
    │     • Validates compliance before deployment
    │
    ├──→ Module 11 (Observability)
    │     • Generates dashboard/alert configs from KPI specs
    │     • Registers metric families for department-level monitoring
    │
    ├──→ Module 13 (Multi-Model Routing / Workflow)
    │     • Routes model selection hints per workflow node
    │     • Validates execution stack compatibility
    │
    ├──→ Module 15 (Agent Marketplace)
    │     • Publishes approved templates to marketplace
    │     • Handles template licensing/revenue share metadata
    │
    ├──→ Module 18 (Enterprise Connector Fabric)
    │     • Validates integration configs against connector schemas
    │     • Generates secure credential bindings
    │
    └──→ Module 20 (Sovereign Deployment)
          • Enforces data residency constraints per template
          • Validates on-prem/air-gapped compatibility flags
```

---

## 🛡️ Critical Implementation Guidelines (Learned from Mod 02/03/04 Audits)

| Anti-Pattern (Avoid) | Module 05 Standard (Enforce) |
|----------------------|------------------------------|
| In-memory-only storage | **PostgreSQL + Redis** from Day 1. Repository pattern with tenant isolation at query layer. |
| Unwired config/middleware | **`config.ParseConfig()`** wired at startup. JWT middleware **must** be first in chain before handlers. |
| Stub event publisher | **Real Kafka/AMQP broker** integration. Typed `Publish*` methods with retry, dead-letter queue, correlationId. |
| Inconsistent errors/pagination | **RFC 7807 errors**, `page`/`page_size`/`has_more` pagination, `additionalProperties: false` in all schemas. |
| String context keys | **Typed `contextKey`** (`TenantIDKey`, `UserIDKey`, `UserTypeKey`, `TraceIDKey`). |
| No test coverage | **≥80% unit + integration tests**. Table-driven tests for CRUD, instantiation, binding resolution, policy compilation. |
| Hardcoded secrets | **K8s Secrets / Vault** injection. `JWT_SECRET`, `DB_PASSWORD`, `BROKER_URI`, `POLICY_ENGINE_URL` via env. |
| Circular dependency risk | **Strict dependency direction**: Module 05 calls downstream modules (03,04,07,10); never the reverse. |

---

## 📊 Contract & Deployment Targets

| Dimension | Target |
|-----------|--------|
| OpenAPI Operations | ~38 (CRUD, versioning, instantiate, bind agents/workflows, policy compile, marketplace publish) |
| AsyncAPI Channels | ~22 (lifecycle, instantiation, deployment, policy alignment, marketplace events) |
| Base Path | `/api/v1/templates` |
| Security | `BearerAuth` + `X-Tenant-ID` |
| Pagination | `page`, `page_size`, `has_more` |
| Storage | PostgreSQL (primary), Redis (cache/search) |
| Test Coverage | ≥80% |
| Helm/Docker | Multi-stage build, non-root, HPA-ready, config-driven |
| Observability | OpenTelemetry traces/metrics, structured JSON logs, Grafana dashboard |

---

## 🎯 Phased Rollout Plan (Wave 2/3 Alignment)

```
Phase 0: Foundation (Sprint 1-2)
├─ Wire config, JWT middleware, typed context keys
├─ Set up PostgreSQL schema + Redis cache layer
├─ Implement core Template CRUD + Versioning (immutable published artifacts)
├─ Enforce platform error/pagination standards

Phase 1: Binding & Resolution (Sprint 2-3)
├─ Agent binding resolver (→ Module 04) with capability matching
├─ Workflow binding resolver (→ Module 03) with stack validation
├─ Memory topology mapper (→ Module 07) with collection provisioning
├─ Integration config validator (→ Module 18) with schema enforcement

Phase 2: Governance & Instantiation (Sprint 3-4)
├─ Policy compiler (→ Module 10 OPA/Rego) with compliance validation
├─ Template instantiation engine: resolves all bindings → executable department config
├─ Real event publisher (Kafka/AMQP) with AsyncAPI-compliant topic format
├─ KPI/observability profile compiler (→ Module 11)

Phase 3: Marketplace & Production (Sprint 4-5)
├─ Marketplace publish workflow (→ Module 15) with licensing metadata
├─ ≥80% test coverage, Helm chart, Dockerfile
├─ Observability wiring (OTLP, metrics, alerts)
├─ DoD gate validation & Wave 3 deployment
```

---

## 📋 Definition of Done Checklist (Module 05)

**Code Gate**
- [ ] ≥80% unit + integration test coverage
- [ ] SAST/DAST scan passed (no secrets, SQLi, or unsafe deserialization)
- [ ] All OpenAPI/AsyncAPI contracts strictly validated against implementations
- [ ] No circular dependencies; strict downstream-only calls to Modules 03/04/07/10/18

**Deployment Gate**
- [ ] Helm chart complete (Deployment, Service, ConfigMap, Secrets, HPA, Ingress)
- [ ] Dockerfile: multi-stage, non-root, minimal attack surface
- [ ] Deployed to dev/staging with automated smoke tests (instantiate sample template)

**Observability Gate**
- [ ] OpenTelemetry traces/metrics exported to OTLP collector
- [ ] Structured JSON logging with `trace_id`, `request_id`, `tenant_id`
- [ ] Grafana dashboard + alerting rules for SLOs (P95 < 500ms, error rate < 0.5%)

**Governance & Integration Gate**
- [ ] Multi-tenant isolation enforced at query & cache layer
- [ ] RBAC/ABAC evaluation proxied to Module 02 before state changes
- [ ] Policy compilation validated against Module 10 before template publish
- [ ] Event envelope matches platform standard (`correlationId`, `tenantId`, `timestamp`, `payload`)
- [ ] Instantiation engine validates all downstream dependencies before deployment

---

## 🔑 Key Takeaway

> **Module 05 is the "department factory" — where abstract organizational designs become executable agentic infrastructure.**

Unlike Module 03 (which started with in-memory stubs), Module 05 should ship **production-ready from Day 1**: persistent storage, wired config/middleware, real event publishing, strict contract compliance, and ≥80% test coverage. Its binding resolver, policy compiler, and instantiation engine will directly enable reliable department deployment across the platform.

**Critical success factor**: Module 05 must never become a "dumb template store." It must actively validate, compile, and resolve dependencies across Modules 03/04/07/10/18 to ensure every instantiated department is governance-compliant, resource-feasible, and operationally sound before deployment begins.

Let me know if you want the OpenAPI skeleton, AsyncAPI event map, or repository interface templates generated next.