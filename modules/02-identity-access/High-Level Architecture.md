# Architecture Design Document: Module 02 — Identity & Access Management

**Project:** Operan — Agentic Department Operating System (ADOS)  
**Module:** 02-identity-access  
**Core Engine:** Authentik (Self-hosted / Sovereign-deployed)  
**Last Updated:** 2026-05-28  
**Status:** Proposed Architecture for PRD Compliance & Agentic Scale  

---

## 1. Executive Summary

Based on the **PRD**, the **Master Contract Index** (which highlights Module 02 is currently at **~60% PRD compliance** with critical gaps in LDAP, AD, Authn flows, and Machine Identities), and the **Operan Manual Runbook** constraints, this document outlines the best-in-class architecture for **Module 02** using **Authentik**.

Authentik is the ideal choice for Operan because it is open-source, sovereign-friendly (self-hostable/air-gapped), supports unified Identity Federation, and has a powerful Policy Engine (Rego/Python) that aligns perfectly with Operan’s **Module 10 (Policy & Governance)**.

**Design Philosophy:** Human-Governed Autonomy, Sovereign by Design, Zero-Trust Agentic Execution.

---

## 2. High-Level Component Architecture

```text
┌─────────────────────────────────────────────────────────────────────┐
│                    EXPERIENCE LAYER (Web / CLI / SDK)               │
└──────────────┬──────────────────────────────────────┬───────────────┘
               │ OIDC/SAML Redirects                  │ REST (Bearer + X-Tenant-ID)
┌──────────────▼──────────────────────────────────────▼───────────────┐
│                  OPERAN API GATEWAY / EDGE (Module 20)              │
│  • Validates JWT (Human & Agent) • Injects X-Tenant-ID & X-Trace-Id │
└──────┬────────────────────────────────┬─────────────────────┬───────┘
       │                                │                     │
┌──────▼──────────────┐  ┌──────────────▼──────────┐  ┌───────▼───────────┐
│  02-IAM CONTROL     │  │  AUTHENTIK CORE CLUSTER │  │  AUTHENTIK        │
│  PLANE (OpenAPI)    │  │  (Flows, Policies, SCIM)│  │  OUTPOSTS (Edge)  │
│  • Tenant IAM Sync  │  │  • OAuth2/OIDC/SAML     │  │  • LDAP Provider  │
│  • Agent Minter     │  │  • MFA / WebAuthn       │  │  • Proxy/Forward  │
│  • SCIM Orchestrator│  │  • Rego Policy Engine   │  │  • AD Sync Agent  │
└──────┬──────────────┘  └──────────────┬──────────┘  └───────┬───────────┘
       │                                │                     │
┌──────▼────────────────────────────────▼─────────────────────▼───────┐
│                     ASYNCAPI EVENT BUS (RabbitMQ/Kafka)             │
│  Events: identity.user.created, identity.agent.authenticated, etc.  │
└──────┬────────────────────────────────┬─────────────────────┬───────┘
       │                                │                     │
┌──────▼──────────┐  ┌──────────────────▼──────┐  ┌───────────▼───────┐
│ 01-Tenant Plane │  │ 11-Observability (Audit)│  │ 10-Policy Engine  │
│ (Namespace/Keys)│  │ (Session Replay Link)   │  │ (ABAC/RBAC Eval)  │
└─────────────────┘  └─────────────────────────┘  └───────────────────┘
```



## **3. Bridging the PRD Compliance Gaps**

The Master Index notes Module 02 is at **~60% compliance**. Here is how Authentik natively solves the missing 40% without violating Runbook constraints (no cross-module imports):


| PRD Gap (from Audit)                    | Authentik Architecture Solution                                                                                                                     | Operan OpenAPI Contract Implementation                                                                                |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| **Authn Flow** (Missing login/callback) | Use **Authentik Flows**. API Gateway handles OIDC redirects to Authentik's `/flows/-/default/` endpoints.                                           | `GET /iam/sso/authorize` (Returns Authentik redirect URL) `POST /iam/sso/token` (Exchanges OIDC code for Operan JWT). |
| **LDAP & Active Directory** (❌ Missing) | **Authentik LDAP Outpost**. Translates modern OIDC/SAML into LDAP for legacy ERPs (Module 18). Also acts as LDAP Client to sync from enterprise AD. | `POST /iam/ldap/providers` (Triggers Authentik API to spin up an LDAP Outpost in the tenant's namespace).             |
| **SCIM** (Read-only)                    | **Authentik SCIM Provider**. Full CRUD support for external IdPs (Okta, Azure AD) to push users into Operan.                                        | `POST /iam/scim/config` (Generates SCIM Bearer Token & Endpoint for external IdP).                                    |
| **MFA** (No enroll/verify)              | **Authentik MFA Stages** (WebAuthn, TOTP, Duo).                                                                                                     | `POST /iam/mfa/enroll` (Returns QR/Challenge) `POST /iam/mfa/verify` (Validates via Authentik Flow API).              |
| **Session Replay** (No response schema) | IAM doesn't record screens. Module 02 binds `iam_session_id` to **Module 11 (Observability)** tools (e.g., PostHog/Sentry).                         | `GET /iam/sessions/{id}/replay` returns `{ "replay_url": "...", "trace_id": "..." }`.                                 |
| **Delegated Admin**                     | **Authentik Groups & Roles** with scoped permissions via Rego policies.                                                                             | `POST /iam/delegations` (Creates time-bound Authentik Role mapping).                                                  |


---

## **4. The "Agentic" Differentiator: Machine & Agent Identities**

Operan is an **Agentic Department Operating System**. Traditional IdPs fail at managing non-human swarm agents. Module 02 must handle **Agent Identities** securely.

### **Agentic Identity Flow (OAuth2 Client Credentials + JWT Minting)**

1. **Agent Registration (Module 04 → Module 02 via AsyncAPI)**:
  - When Module 04 registers an Agent, it emits `agent.registered`.
  - Module 02 listens and creates an **Authentik Service Account** and an **OAuth2 Client** (Client Credentials Grant).
2. **Execution Token Minting**:
  - Before an Agent executes a tool (Module 08), it requests a short-lived JWT from Module 02.
  - **Authentik Property Mappings** inject Agentic Context into the JWT Claims:
    ```
    json
    ```
    1
    2
    3
    4
    5
    6
    7
    8
    {
      "sub": "agent-uuid-123",
      "tenant_id": "uuid-v4",
      "agent_role": "finance_approver",
      "memory_scope": ["finance_db", "procurement_graph"],
      "escalation_target": "human-uuid-456",
      "execution_budget_usd": 50.00
    }
3. **Zero-Trust Tool Execution**:
  - Module 08 (Tool Execution) validates the JWT signature, checks `execution_budget` against Module 17 (Cost Governance), and enforces `memory_scope` via Module 10 (Policy).

---

## **5. Multi-Tenant & Sovereign Deployment Strategy**

Aligning with **PRD Section 4.1 (Sovereign by Design)** and **Module 20 (Sovereign Deployment Fabric)**:

### **Tier 1: Standard SaaS Tenants (Logical Isolation)**

- **Architecture**: Shared Authentik Cluster.
- **Isolation**: Utilizes **Authentik Brands** and **Custom Property Mappings** keyed by `X-Tenant-ID`. Data is logically separated in the PostgreSQL backend using Row-Level Security (RLS) tied to the Tenant UUID.

### **Tier 2: Enterprise / Sovereign Tenants (Cryptographic & Execution Isolation)**

- **Architecture**: **Hub-and-Spoke Federation**.
- **Implementation**:
  - Module 20 deploys a dedicated, air-gapped **Authentik Instance** inside the customer's VPC / On-Prem Kubernetes cluster.
  - Module 02 (Global Control Plane) acts purely as a **Configuration Syncer** via mTLS tunnels. It pushes Operan Governance Policies (Rego) and Department Templates to the local Authentik instance.
  - **Data Residency**: No PII or identity data ever leaves the sovereign perimeter.

---

## **6. Contract & Runbook Adherence (Strict Compliance)**

To ensure the **REVIEW** gate passes with `APPROVE`, Module 02 will strictly adhere to the Runbook:

1. **Global Constants Applied**:
  - All endpoints require `Authorization: Bearer {jwt}` and `X-Tenant-ID: {uuid-v4}`.
  - Pagination uses `page`, `page_size`, `has_more` (replacing limit/offset).
  - Errors return RFC 7807: `{ "code": 401, "message": "Invalid agent signature", "request_id": "uuid" }`.
2. **Schema Integrity**:
  - `additionalProperties: false` on all IAM Request/Response schemas.
  - Fix the **Cross-Spec Inconsistencies** noted in the Master Index:
    - *SSOConfig*: Standardize to Nested `{ "type": "saml", "configuration": { ... } }`.
    - *User.status*: Align OpenAPI and JSON Schema to `active`, `inactive`, `suspended`, `pending`.
    - *AuditEntry*: Standardize to `{ "actor_id": "uuid", "actor_type": "human|agent", "metadata": {} }`.
3. **No Cross-Module Imports**:
  - Module 02 will **never** import Module 10 (Policy) or Module 04 (Registry) code.
  - It will use **AsyncAPI** (`operan/events/identity.`*) to notify the system of auth events, and make internal HTTP/gRPC calls to Module 10 for real-time ABAC evaluation during login flows.

---

## **7. AsyncAPI Event Topology (Module 02)**

To feed **Module 11 (Observability)** and trigger **Module 03 (Orchestration)** workflows:

```
yaml
```

1

2

3

4

5

6

7

8

9

channels:

  operan/events/identity.user.authenticated:

    subscribe: ...

  operan/events/identity.agent.credentials.rotated:

    subscribe: ...

  operan/events/identity.mfa.failed:

    subscribe: ... *# Triggers Module 09 (Human Supervision) lockdown*

  operan/events/identity.scim.sync.completed:

    subscribe: ... *# Triggers Module 05 (Department Template) provisioning*

---

## **8. Summary of Value**

By leveraging **Authentik**, Operan avoids vendor lock-in (PRD 4.5), gains native LDAP/SCIM/MFA capabilities to close the 40% compliance gap, and utilizes Authentik's Rego-based policy engine to seamlessly integrate with Operan's **Module 10 Governance** and **Agentic Delegation** models.