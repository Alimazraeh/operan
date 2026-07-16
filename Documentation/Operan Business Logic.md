# Operan — End-to-End Business Logic

> This document describes the business narrative, workflows, and data flow of the Operan platform. It is grounded in the PRD, integration graph, demo script, and live implementation.

---

## 1. What Problem Does Operan Solve?

Operan solves one core problem: **how organizations actually deploy and operate AI workforces**, not just experiment with AI tools.

The PRD frames it explicitly: organizations do not fundamentally need "more employees, more dashboards, or more copilots." They need:
- **Scalable operational cognition** — agents that remember and reason
- **Persistent institutional memory** — knowledge that survives restarts
- **Reliable process execution** — workflows that can pause, resume, and recover
- **Governed autonomous systems** — agents that act independently but under human control

Operan repositions AI from assistant tooling into what the PRD calls **"Enterprise Agentic Workforce Infrastructure."** It competes not with AI startups but with BPO firms, ERP vendors, and consulting firms — a significantly larger category.

**Primary customers:** Enterprises and governments in the Gulf region (notably Saudi Arabia's ADRI) that need sovereign-grade deployment, data residency, and Arabic-first intelligence.

---

## 2. Core Business Capabilities

| Capability | What it means in business terms |
|---|---|
| **Deploy departments on demand** | A company selects a pre-built template (Sales, HR, Finance, Legal, etc.) and the platform provisions an entire operational department — agents, workflows, memory, governance — automatically. |
| **Institutional memory** | Agents remember organizational knowledge (customer preferences, past decisions, compliance rules) across executions and restarts. The memory is semantic, not keyword-based. |
| **Governed autonomous execution** | Agents can draft, analyze, coordinate, and take limited actions on their own, but critical actions (like sending a $250k contract) require human approval gates enforced through the event bus. |
| **Multi-tenant isolation** | Each organization gets isolated memory, orchestration, models, governance, execution logs, and identity. Sovereign-grade: deployable on-prem, air-gapped, with private model hosting and enterprise encryption. |
| **End-to-end auditability** | Every action an agent takes becomes an event on Kafka, consumable by an observability layer that provides distributed tracing, per-tenant metrics, and live component health. |
| **Sovereign deployment** | Deploy anywhere: public cloud, private cloud, air-gapped, or national infrastructure. Strategic differentiator for government customers who must keep data within national borders. |
| **Arabic-first intelligence** | Arabic-native embeddings, terminology governance, dialect normalization, and Arabic OCR — described in the PRD as "a national strategic moat." |

---

## 3. End-to-End Flow: A Realistic Business Scenario

Using the demo (`deploy/demo/demo.sh`, `deploy/demo/DEMO-RUNBOOK.md`) as the canonical flow, here is how a real customer engagement works:

### Step 1 — Onboard the Organization (Module 01: Tenant Control Plane)

Acme Corp is onboarded as a tenant with:
- **Plan type**: Enterprise
- **Region**: `me-central` (data stays in-region)
- **Quotas**: CPU, memory, API calls, storage limits
- **Isolation**: namespace-level, private encryption keys with rotation

Every request from this customer carries a JWT (issued by Module 01) and an `X-Tenant-ID` header. This is how everything else in the platform scopes data.

**What happens:** The tenant object is created with a unique namespace. Billing, quota, and environment policies are attached. The tenant is ready for identity setup.

---

### Step 2 — Set Up Identity & Access (Module 02: Identity & Access)

Module 02 provisions the tenant's IAM entities:
- **Users** enrolled via SSO (Authentik, Azure AD, Okta) with MFA
- **Agents** registered as first-class identities with roles and capabilities
- **RBAC/ABAC policies** defined (e.g., "Sales agents can draft contracts but not send them")
- **Audit trails** initialized

Every module calls Module 02 for authentication and authorization. The JWT from Step 1 is validated here, and the tenant context is injected into every request.

**What happens:** Acme Corp has users, agents, and roles. When a sales agent tries to do something, Module 02 answers "yes, no, or maybe" based on role + attributes (IP, time, department).

---

### Step 3 — Deploy a Sales Department (Module 05: Department Template Engine)

The customer browses a catalog of department templates: HR, Sales, Finance, Legal, Engineering, Research. They pick **Sales Department** and kick off a 6-stage pipeline:

| Stage | Description |
|---|---|
| 1. Select template | The template defines the department's structure, agent roles, and initial policies |
| 2. Configure policies | Governance rules are applied (Module 10 — not yet implemented) |
| 3. Connect data sources | Enterprise systems like CRM, ERP are linked (Module 18 — not yet implemented) |
| 4. Provision memory | The department's institutional knowledge is set up (Module 07) |
| 5. Deploy swarm | The actual agent workflows are created (Module 03) |
| 6. Begin operations | The department is live |

**What happens:** A deployment object is created. The pipeline moves through stages. Each stage publishes events to Kafka. When complete, Acme Corp has a working "sales department" running in the platform.

---

### Step 4 — Register the Sales Agents (Module 04: Agent Registry)

Within the deployed department, individual agents are registered:

- **Name/role**: `sales-assistant`, `contract-reviewer`
- **Capabilities**: `draft_contracts`, `crm_lookup`, `quote_generation`
- **Tool permissions**: `send_email`, `update_crm`, `read_quotes`
- **Versioning**: semver-tracked, promotable (dev → staging → prod)
- **Dependencies**: agent A depends on agent B's output

**What happens:** Agents become first-class objects in the registry. They have identities (from Module 02), capability indexes for discovery, and version histories for rollback.

---

### Step 5 — Ingest Institutional Knowledge (Module 07: Memory Fabric)

Acme Corp uploads customer preferences, past decisions, compliance rules, pricing guidelines. The platform:

1. **Embeds** the knowledge using a local model (`qwen3-embedding-4b` on the cluster — data never leaves)
2. **Stores** vectors with metadata (tenant, department, expiration)
3. **Ranks** by cosine similarity + token overlap fallback

> **Demo scenario:** An agent asks "which interface language does the client prefer?" (zero word overlap with the stored text "customer prefers Arabic-first UI and quarterly billing"). The system returns the right match because it understands meaning, not keywords.

**What happens:** The department has persistent, semantic memory. Agents remember across sessions and restarts. Retention policies auto-expire stale knowledge.

---

### Step 6 — Execute a Workflow (Module 03: Agent Orchestration)

A sales opportunity is created. The orchestrator kicks off a workflow:

```
fetch_crm_data → draft_contract → pricing_check → human_gate → send_contract
```

1. **Create DAG:** Define the workflow as a directed acyclic graph
2. **Execute nodes:** Agents fetch data, draft contracts, check pricing
3. **Hit human gate:** The workflow pauses at `human_gate` because it's a $250k contract

The gate raises an event on Kafka. Module 09 picks it up.

**What happens:** Workflows are DAGs with conditional edges, retry policies, and checkpoint/replay. The orchestrator speaks multiple backends (LangGraph, Temporal, Ray, Celery). It can pause, resume, cancel, or replay from any checkpoint.

---

### Step 7 — Human Approval (Module 09: Human Supervision)

The human gate creates an approval request in Module 09:

- **Request:** "Sales agent wants to send $250k contract to Acme Corp"
- **Context:** workflow ID, agent ID, contract draft, tenant, risk score
- **Actions:** Approve, Reject, Delegate, Escalate

A supervisor reviews in their inbox (via Module 21, the Experience Portal). When they approve:
- The approval is published as a Kafka event
- Module 03's orchestrator consumes it
- The workflow resumes from the gate
- If rejected, the task is failed

**What happens:** "Auditable human control over agents, built into the platform's nervous system." No agent can take a critical action without clearance. Every decision is logged.

---

### Step 8 — Execute with Tools (Module 08: Tool Execution)

Once approved, the agent executes its tools:
- Send email via SMTP relay
- Update CRM via API
- Generate PDF

Tool execution is:
- **Policy-governed** (Module 10 checks permissions first)
- **Sandboxed** (Module 16 — not yet implemented)
- **Fully observable** (Module 11 records spans and metrics)
- **Cost-tracked** (Module 17 — not yet implemented)

**What happens:** The agent takes real action in the enterprise. The execution is recorded, costed, and observable. If something goes wrong, it's retried or failed cleanly.

---

### Step 9 — Everything is Observed (Module 11: Observability)

Every single step from Steps 1–8 is published as an event on Kafka. Module 11 consumes all of it:
- **Metrics:** per-tenant, per-department, per-agent counts and latencies
- **Spans:** distributed traces grouped by correlation ID
- **Traces:** full execution paths that can be replayed
- **Alerts:** health checks, anomaly detection
- **Health:** live component status dashboard

**What happens:** You can watch the entire department operation in real time. You can replay any execution. You can audit every decision. And when Kafka is down, everything degrades to log-only — the API never breaks.

---

## 4. The Platform's Nervous System — Kafka Event Bus

All modules communicate through **Kafka**. Every action produces an event. The event bus is the connective tissue:

```
Tenant (M01) → IAM (M02) → Policy (M10) → Observability (M11)
     ↓                                              ↑
Agent Registry (M04) ←→ Orchestration (M03) ←→ Human Gate (M09)
     ↓                    ↓                          ↓
   Memory (M07)          ↓                   Tool Execution (M08)
     ↘                   ↓                      ↓
      └──→ Experience Portal (M21) ←────────────┘
```

### Topics by Module

| Module | Topic Pattern | Events |
|---|---|---|
| **01** Tenant | `operan.tenant.*` | provisioned, suspended, deprovisioned, quota_exceeded |
| **02** IAM | `operan.iam.*` | auth, authorize, scim_sync, audit, sso_login |
| **03** Orchestration | `operan.orchestration.*` | workflow_created, node_started, node_completed, gate_raised, gate_responded, checkpoint_saved |
| **04** Agent Registry | `operan.agent-registry.*` | agent_registered, capability_updated, version_created, version_promoted, agent_deprecated, agent_archived |
| **05** Templates | `operan.templates.template.*` | template_created, template_deployed, deployment_stage_changed, deployment_completed |
| **07** Memory | `operan.memory.*` | vectors_ingested, search_completed, retention_fired, gc_completed |
| **08** Tools | `operan.tool-execution.*` | tool_registered, tool_executed, execution_result, cost_recorded |
| **09** Supervision | `operan.supervision.*` | gate_raised, gate_responded, escalation_created, intervention_recorded |
| **11** Observability | `operan.observability.*` | metric_recorded, span_created, trace_completed, alert_fired |

When Kafka is down, modules degrade to **log-only mode** — API responses never break. This ensures the system is fault-tolerant by design.

---

## 5. The Experience Portal (Module 21)

This is the Web UI. It has no contract of its own — it's a consumer that proxies to all services. It provides:

| View | Purpose |
|---|---|
| **Overview** | Dashboard with tenant stats, department health, KPIs |
| **Agent Catalog** | Browse and register agents from the registry |
| **Tools** | Manage tool definitions and execution history |
| **Workflows** | Create, monitor, pause, resume, replay workflows |
| **Departments** | Deploy templates, configure policies, provision memory |
| **Supervision** | Approve/reject agent requests, manage escalation queue |
| **Observability** | Live metrics, traces, health status |
| **Guided Scenarios** | Demo workflows for onboarding and training |

Accessible on the cluster via NodePort `30088` (the only externally exposed service).

---

## 6. What's NOT Running (Contract-Only Modules)

Modules **06, 10, 12, 13, 14, 15, 16, 17, 18, 19, 20** are all designed (full OpenAPI + AsyncAPI + Schema specs) but not implemented:

| Module | Capability |
|---|---|
| **06** Knowledge Ingestion | PDF/SharePoint/CRM document processing |
| **10** Policy Governance | Rule engine for pre-execution validation |
| **12** Model Abstraction | Decouple from LLM vendors |
| **13** Multi-Model Routing | Smart model assignment per task |
| **14** Agent Collaboration | Inter-agent messaging |
| **15** Agent Marketplace | Procurement ecosystem for agents/templates |
| **16** Execution Sandbox | Isolated agent runtime environments |
| **17** Cost Governance | Budget throttling, runaway cost prevention |
| **18** Enterprise Connectors | SAP/Salesforce/M365 integrations |
| **19** Arabic Language Core | Arabic embeddings, terminology, OCR |
| **20** Sovereign Deployment | On-prem/air-gapped/national infrastructure provisioning |

---

## 7. Platform Standards

All implemented modules share these standards:

| Standard | Description |
|---|---|
| **Auth** | Bearer JWT + `X-Tenant-ID` header on every request |
| **Events** | Kafka via `segmentio/kafka-go` v0.4.51, `operan.*` topic prefix |
| **Fallback** | Log-only mode when Kafka broker is down |
| **Fail-fast** | Modules refuse to start when JWT secret is unset or the known default |
| **Tenant isolation** | All stores filter queries by TenantID |
| **In-memory stores** | Pod restarts clear data; re-run demo.sh to repopulate |

---

*Documented: 2026-07-15*
*Sources: PRD.md, integration-graph.yaml, demo.sh, DEMO-RUNBOOK.md, Master Contract Index.md*