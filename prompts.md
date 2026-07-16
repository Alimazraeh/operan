# Operan — Missing Module Implementation Prompts

This file contains self-contained implementation prompts for the 10 missing modules. Each prompt includes: database schema, OpenAPI endpoints, Kafka events, integration points, and test requirements. **All modules must use PostgreSQL from day one.** Kafka is preserved as the event bus (complementary to DB, not replaced).

---

## Implementation Order (based on dependency graph)

```
Phase 1 (foundation):  M12 (Model Abstraction)
Phase 2 (depends on M12):  M19 (Arabic Language Core), M17 (Cost Governance)
Phase 3 (depends on M19, M06):  M06 (Knowledge Ingestion), M13 (Multi-Model Routing)
Phase 4 (depends on M12, M16):  M16 (Execution Sandbox)
Phase 5 (depends on M07, M12):  M03-extensions (Agent Collaboration via M14), M05 (Agent Marketplace)
Phase 6 (depends on M12, M19):  M18 (Enterprise Connector Fabric)
Phase 7 (depends on all):  M10 (Policy Governance)
```

## Platform Standards (apply to all modules)

| Standard | Requirement |
|---|---|
| **Auth** | Every request carries Bearer JWT + `X-Tenant-ID` header |
| **DB** | PostgreSQL with `jackc/pgx/v5` driver |
| **Events** | Kafka via `segmentio/kafka-go`, topic prefix `operan.{module}.` |
| **Pagination** | `page` / `page_size` query params → `{items: [...], page, page_size, total}` |
| **Errors** | `{error: {code, message}}` JSON response |
| **Serialization** | `additionalProperties: false` on all JSON structs |
| **Fallback** | Log-only mode when Kafka is down — API never breaks |
| **Fail-fast** | Refuse to start if JWT secret is unset or the known default |
| **Tenant isolation** | Every query scoped by `tenant_id` |

---

‫##‬Prompts



    MODULE 10 — Policy Governance Engine

    Dependency: None (depends on M01/M02/M04 for context, not implementation)

      1 You are implementing Module 10 — Policy Governance Engine for the Operan platform. This is a critical foundational module. All other modules (orchestration, templates, tool execution, cost 
        governance, etc.) will depend on this module to validate policies before executing actions.
      2 
      3 ## Purpose
      4 Module 10 is a rule engine that answers "should this action be allowed?" before any module takes a meaningful action. It enforces approval thresholds, data usage policies, rate limits, audit 
        requirements, compliance rules, and custom conditions.
      5 
      6 ## Key Concepts
      7 - **Policies**: Named rules with conditions, enforcement levels, and actions
      8 - **Policy Evaluations**: Requests to check if a policy permits an action
      9 - **Policy Groups**: Collections of policies applied together (e.g., "finance-dept", "legal-review")
     10 - **Enforcement**: `enforce` (block), `warn` (log + continue), `log` (silent audit)
     11 - **Compliance Frameworks**: Pre-built policy sets (GDPR, SOX, HIPAA, NESA, UAE IA)
     12 
     13 ## Files to Create
     14 
    Directory Structure
      1 modules/10-policy-governance/
      2 ├── go.mod
      3 ├── main.go                    # Entry point, wire up server, listen port 8010
      4 ├── Dockerfile                 # Multi-stage, Go 1.22+, non-root user
      5 ├── chart/                     # Helm chart (deployment, service, configmap, ingress)
      6 └── internal/
      7     ├── config/
      8     │   └── config.go          # Env vars: JWT_SECRET, EVENT_BROKER_URL, DB_DSN, LISTEN_ADDR
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go         # TenantID, UserID, TraceID, RequestID
     11     ├── middleware/
     12     │   └── middleware.go      # JWTAuth (HMAC-S256), ExtractTenant, ChainJWTAuth, Logger, RequestID, TraceID
     13     ├── handler/
     14     │   ├── policies.go        # CRUD for policies
     15     │   ├── evaluations.go     # POST /evaluate, GET /evaluations
     16     │   ├── groups.go          # CRUD for policy groups
     17     │   └── router.go          # Register all routes on mux
     18     ├── store/
     19     │   ├── policies.go        # PostgreSQL: CREATE TABLE policies
     20     │   ├── evaluations.go     # PostgreSQL: CREATE TABLE evaluations
     21     │   ├── groups.go          # PostgreSQL: CREATE TABLE policy_groups
     22     │   └── models.go          # Go structs matching OpenAPI schemas
     23     └── events/
     24         └── events.go          # Kafka publisher: policy_evaluated, policy_violation

    Database Schema (PostgreSQL)

    policies table
      1 CREATE TABLE policies (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     description     TEXT,
      6     category        VARCHAR(50) NOT NULL CHECK (category IN ('access_control', 'data_usage', 'rate_limit', 'audit', 'compliance', 'custom')),
      7     type            VARCHAR(50) NOT NULL CHECK (type IN ('allow', 'deny', 'require_approval', 'log_only', 'warn')),
      8     enforcement     VARCHAR(20) NOT NULL DEFAULT 'log' CHECK (enforcement IN ('enforce', 'warn', 'log')),
      9     conditions      JSONB NOT NULL DEFAULT '{}',    -- { "operator": "gt", "field": "amount", "value": 10000 }
     10     actions         JSONB NOT NULL DEFAULT '[]',    -- [{ "action": "block", "comment": "..." }]
     11     priority        INT NOT NULL DEFAULT 50,        -- 1-100, higher = evaluated first
     12     is_active       BOOLEAN NOT NULL DEFAULT true,
     13     tags            TEXT[],
     14     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     15     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     16 );
     17 CREATE INDEX idx_policies_tenant ON policies(tenant_id);
     18 CREATE INDEX idx_policies_category ON policies(category, is_active);

    evaluations table
      1 CREATE TABLE evaluations (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     policy_ids      UUID[] NOT NULL,              -- which policies were evaluated
      5     context         JSONB NOT NULL DEFAULT '{}',   -- { "action": "send_contract", "agent_id": "...", "amount": 250000 }
      6     decision        VARCHAR(10) NOT NULL,           -- allow | deny | require_approval | warn
      7     rationale       TEXT,                           -- why this decision
      8     risk_score      FLOAT,                          -- 0.0-1.0
      9     latency_ms      INT,
     10     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE INDEX idx_evaluations_tenant ON evaluations(tenant_id);
     13 CREATE INDEX idx_evaluations_decision ON evaluations(decision);

    policy_groups table
      1 CREATE TABLE policy_groups (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     description     TEXT,
      6     policy_ids      UUID[] NOT NULL,
      7     framework       VARCHAR(50),                   -- gdpr | sox | hipaa | nesa | ia | custom
      8     is_active       BOOLEAN NOT NULL DEFAULT true,
      9     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     10     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE INDEX idx_policy_groups_tenant ON policy_groups(tenant_id);

    OpenAPI Endpoints (implement ALL)

    Based on contracts/v1/openapi-10-policy-governance.yaml (9 endpoints):


    ┌────────┬───────────────────┬───────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────────────────┐
    │ Method │ Path              │ Handler           │ Description                                                                                              │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/policies      │ CreatePolicy      │ Create a policy with conditions, actions, enforcement                                                    │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/policies      │ ListPolicies      │ Paginated list, filter by category/type/enforcement/framework                                            │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/policies/{id} │ GetPolicy         │ Get a policy by ID                                                                                       │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/policies/{id} │ UpdatePolicy      │ Update fields, partial update                                                                            │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/policies/{id} │ DeletePolicy      │ Soft delete (set is_active=false)                                                                        │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/evaluate      │ EvaluatePolicy    │ Evaluate one or more policies against a context. Returns { decision, rationale, risk_score, latency_ms } │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/evaluations   │ ListEvaluations   │ Paginated evaluation history, filter by decision/policy/tenant                                           │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/policy-groups │ ListPolicyGroups  │ List groups with framework filtering                                                                     │
    ├────────┼───────────────────┼───────────────────┼──────────────────────────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/policy-groups │ CreatePolicyGroup │ Create group with policy IDs and optional framework                                                      │
    └────────┴───────────────────┴───────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────────────────┘

    Additional:
     - GET /health — unauthenticated, returns { "status": "ok" }
     - All routes except /health require Bearer JWT + X-Tenant-ID

    Kafka Events

    Publishing (M10 → others)
     - operan.policy.policy_evaluated — published after every evaluation
     1   {
     2     "event_id": "uuid",
     3     "tenant_id": "uuid",
     4     "policy_ids": ["uuid", "..."],
     5     "decision": "allow|deny|require_approval",
     6     "risk_score": 0.75,
     7     "timestamp": "ISO8601"
     8   }
     - operan.policy.policy_violation — published when enforcement=enqueue and decision=deny
     1   {
     2     "event_id": "uuid",
     3     "tenant_id": "uuid",
     4     "policy_id": "uuid",
     5     "action": "send_contract",
     6     "denied_by": "compliance_threshold",
     7     "timestamp": "ISO8601"
     8   }

    Subscribing (M10 ← others)
    M10 should NOT subscribe to anything — it's a pure evaluation service. Other modules call it via REST (POST /v1/evaluate) and publish events to Kafka.

    Integration Points
     - M01 Tenant: Tenant context from JWT. Policies are tenant-scoped.
     - M02 IAM: JWT validated by middleware. RBAC roles determine who can CREATE/UPDATE policies. Only admin or policy_editor roles can mutate.
     - M04 Agent Registry: Evaluations reference agent_id in context. Use M04's agent registry to resolve agent capabilities when evaluating agent-specific policies.
     - M03 Orchestration: M03 calls POST /v1/evaluate before executing workflow nodes with risk_score > 0.5.
     - M08 Tool Execution: M08 calls POST /v1/evaluate before executing high-risk tools.
     - M17 Cost Governance: M17 calls POST /v1/evaluate before allowing budget exceeding thresholds.

    Implementation Notes
     - DB Driver: Use github.com/jackc/pgx/v5 for PostgreSQL
     - Kafka: Use github.com/segmentio/kafka-go v0.4.51 — same as other modules
     - Fail-closed: Refuse to start if JWT_SECRET is unset or default. If Kafka broker unreachable, degrade to log-only (never blocks API).
     - Evaluation Logic: The /v1/evaluate endpoint must:
       1. Resolve all applicable policies (by group, framework, or explicit IDs)
       2. Evaluate each policy's conditions against the request context using simple expression engine (supports operators: eq, ne, gt, lt, gte, lte, in, nin, contains, regex)
       3. Return the most restrictive decision (deny > require_approval > warn > allow)
       4. Record the evaluation in the DB
       5. Publish event to Kafka
     - Pagination: Use page + page_size with has_more (platform standard)
     - Error Schema: All errors follow { code: int, message: string, request_id: string }
     - additionalProperties: false on all request/response DTOs

    Tests (minimum 40 tests)
     - Policy CRUD: create valid, create with missing fields (400), create with invalid category (400), get by id, update partial, delete (soft), list with filters, pagination
     - Evaluation: allow (conditions met), deny (conditions met), require_approval, warn, multiple policies with conflicting decisions, empty context, invalid operator
     - Policy Groups: create with framework, create without framework, list by framework, update policies in group
     - Middleware: JWT validation (valid, expired, wrong issuer), tenant extraction, missing X-Tenant-ID (401), non-admin creating policy (403)
     - DB: connection failure handling, transaction rollback on error, concurrent evaluations
     - Kafka: publish events, handle broker unreachable (log-only fallback)

    Deliverables
     1. All files in modules/10-policy-governance/
     2. go build ./... passes
     3. go test ./... passes — 40+ tests
     4. Valid OpenAPI contract (parseable YAML)
     5. Helm chart with deployment, service (8010), configmap
     6. README.md with setup instructions

     1 
     2 ---
     3 
     4 # MODULE 16 — Execution Sandbox
     5 
     6 **Dependency:** M04 (Agent Registry — to know which agents need sandboxes)
    You are implementing Module 16 — Execution Sandbox for the Operan platform. This module provides isolated runtime environments for agent actions, ensuring agents cannot access resources outside
    their permission scope.

    Purpose
    When an agent executes a tool (send email, query DB, call API), it does so inside a sandbox — an isolated execution context with enforced resource limits, network access controls, file system
    boundaries, and environment variable scoping. M08 (Tool Execution) calls M16 before running tools.

    Key Concepts
     - Sandbox Instance: A single execution unit with isolated state
     - Sandbox Profiles: Templates defining resource limits, network rules, filesystem mounts, env vars
     - Isolation Levels: process (fork), container (cgroup namespace), full (minimal)
     - Resource Quotas: CPU, memory, network, filesystem, env vars per sandbox

    Files to Create

    Directory Structure
      1 modules/16-execution-sandbox/
      2 ├── go.mod
      3 ├── main.go                    # Entry point, listen port 8016
      4 ├── Dockerfile                 # Multi-stage, Go 1.22+, non-root user
      5 ├── chart/                     # Helm chart
      6 └── internal/
      7     ├── config/
      8     │   └── config.go
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── sandboxes.go       # CRUD sandboxes, create/destroy
     15     │   ├── profiles.go        # CRUD sandbox profiles
     16     │   └── router.go
     17     ├── store/
     18     │   ├── sandboxes.go       # PostgreSQL
     19     │   ├── profiles.go        # PostgreSQL
     20     │   └── models.go
     21     └── events/
     22         └── events.go          # Kafka publisher: sandbox_created, sandbox_destroyed, sandbox_resource_exceeded

    Database Schema

    sandbox_profiles table
      1 CREATE TABLE sandbox_profiles (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     description     TEXT,
      6     isolation_level VARCHAR(20) NOT NULL DEFAULT 'process' CHECK (isolation_level IN ('process', 'container', 'full')),
      7     resource_limits JSONB NOT NULL DEFAULT '{
      8         "cpu_millicores": 500,
      9         "memory_mb": 256,
     10         "network_access": ["outbound_http", "outbound_https"],
     11         "filesystem_ro": ["/"],
     12         "env_whitelist": ["PATH", "HOME"],
     13         "timeout_seconds": 30,
     14         "max_file_size_mb": 10
     15     }',
     16     is_default      BOOLEAN NOT NULL DEFAULT false,
     17     is_active       BOOLEAN NOT NULL DEFAULT true,
     18     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     19     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     20 );
     21 CREATE INDEX idx_profiles_tenant ON sandbox_profiles(tenant_id);
     22 CREATE INDEX idx_profiles_active ON sandbox_profiles(tenant_id, is_active);

    sandbox_instances table
      1 CREATE TABLE sandbox_instances (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     agent_id        VARCHAR(255),                    -- nullable
      5     workflow_id     VARCHAR(255),                    -- nullable
      6     profile_id      UUID NOT NULL,
      7     status          VARCHAR(20) NOT NULL DEFAULT 'starting'
      8                   CHECK (status IN ('starting', 'running', 'completed', 'failed', 'killed', 'timeout')),
      9     resource_usage  JSONB NOT NULL DEFAULT '{}',     -- { cpu: 234, memory_mb: 45, duration_ms: 1200 }
     10     exit_code       INT,
     11     error_message   TEXT,
     12     started_at      TIMESTAMPTZ,
     13     completed_at    TIMESTAMPTZ,
     14     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     15 );
     16 CREATE INDEX idx_instances_tenant ON sandbox_instances(tenant_id);
     17 CREATE INDEX idx_instances_status ON sandbox_instances(tenant_id, status);

    OpenAPI Endpoints


    ┌────────┬───────────────────────────┬───────────────┬───────────────────────────────────┐
    │ Method │ Path                      │ Handler       │ Description                       │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ POST   │ /v1/sandboxes             │ CreateSandbox │ Create execution sandbox instance │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ GET    │ /v1/sandboxes             │ ListSandboxes │ Paginated list of instances       │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ GET    │ /v1/sandboxes/{id}        │ GetSandbox    │ Get instance details              │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ POST   │ /v1/sandboxes/{id}/kill   │ KillSandbox   │ Force-kill running sandbox        │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ GET    │ /v1/sandbox-profiles      │ ListProfiles  │ List profiles                     │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ POST   │ /v1/sandbox-profiles      │ CreateProfile │ Create sandbox profile            │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ PATCH  │ /v1/sandbox-profiles/{id} │ UpdateProfile │ Update profile                    │
    ├────────┼───────────────────────────┼───────────────┼───────────────────────────────────┤
    │ DELETE │ /v1/sandbox-profiles/{id} │ DeleteProfile │ Soft delete                       │
    └────────┴───────────────────────────┴───────────────┴───────────────────────────────────┘

    Kafka Events

     - operan.sandbox.sandbox_created — { tenant_id, agent_id, profile_id, created_at }
     - operan.sandbox.sandbox_destroyed — { tenant_id, sandbox_id, exit_code, duration_ms }
     - operan.sandbox.sandbox_resource_exceeded — { tenant_id, sandbox_id, metric, limit, value } (emitted when usage hits 80%+ of profile limit)

    Integration Points
     - M04 Agent Registry: Resolves agent_id from the registry to determine which sandbox profile to apply (agents have runtime_constraints in their definition). If no agent_id, use tenant default
       profile.
     - M08 Tool Execution: M08 calls POST /v1/sandboxes BEFORE executing any tool. The sandbox response includes execution context (env vars, limits, network rules). After tool finishes, M08 calls POST 
       /v1/sandboxes/{id}/complete (implicit on response) to close the sandbox.
     - M17 Cost Governance: Sandbox resource usage (CPU, memory, duration) feeds into cost calculations.

    Implementation Notes
     - Sandbox Runtime: Since this is a Go-only module (no container runtime available), implement sandboxing using Go's syscall.Credential for user isolation, runtime.LockOSThread() for CPU throttling
       signals, and os.Chroot for filesystem boundaries. In production, these would map to Docker/LXC/cgroup v2 — but the API is the contract. Document this clearly.
     - Resource Monitoring: Use Go's runtime.ReadMemStats for memory/CPU tracking. Periodically update the sandbox instance record.
     - Timeout Enforcement: Use context.WithTimeout on all sandbox operations. If timeout hits, record status=timeout and publish event.
     - Network Access Control: The profile's network_access field is stored as metadata. Since we can't actually enforce networking in-process, this is a policy record that M08 passes to the tool
       executor. M08 should refuse to call tools with disallowed network access.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 30 tests)
     - Sandbox CRUD: create, list, get, kill, timeout
     - Profile CRUD: create, list, get, update, delete, set-as-default
     - Evaluation: profile resolution (agent-specific, tenant-default), resource limit enforcement simulation, timeout handling
     - Middleware: JWT validation, tenant isolation (tenant A cannot see tenant B's sandboxes), RBAC (non-admin cannot create profiles)
     - DB: connection handling, concurrent sandbox creation, index usage
     - Kafka: event publishing, broker unreachable fallback

    Deliverables
    Same as M10: build passes, 30+ tests, OpenAPI-compliant, Helm chart, README.
     1 
     2 ---
     3 
     4 # MODULE 12 — Model Abstraction Layer
     5 
     6 **Dependency:** M02 (IAM — for JWT validation, agent identity resolution)
    You are implementing Module 12 — Model Abstraction Layer for the Operan platform. This module decouples all AI inference from specific model providers by providing a unified interface that routes
    requests to any configured LLM backend.

    Purpose
    Agents, orchestrations, and workflows don't call OpenAI, Anthropic, or local models directly. They call M12's /v1/models/completions endpoint, which resolves which backend to use, transforms the
    request/response, tracks costs, and handles failover. M03 (Orchestration), M13 (Routing), and any agent calling an LLM goes through this module.

    Key Concepts
     - Model Providers: Registered LLM backends (OpenAI, Anthropic, LiteLLM, local Ollama, Azure OpenAI)
     - Model Registry: Mapping of model names to provider endpoints
     - Abstraction: Unified /v1/models/completions and /v1/models/embeddings endpoints regardless of backend
     - Cost Tracking: Every call records token usage and estimated cost (forwarded to M17)
     - Failover: If primary provider fails, try backup (configurable per model)

    Files to Create

    Directory Structure
      1 modules/12-model-abstraction/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8012
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go          # JWT_SECRET, EVENT_BROKER_URL, DB_DSN, PROVIDER_API_KEYS (JSON)
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── completions.go     # POST /v1/models/completions (the core endpoint)
     15     │   ├── embeddings.go      # POST /v1/models/embeddings
     16     │   ├── providers.go       # CRUD model providers
     17     │   ├── models.go          # CRUD model registry entries
     18     │   └── router.go
     19     ├── store/
     20     │   ├── providers.go       # PostgreSQL
     21     │   ├── models.go          # PostgreSQL
     22     │   ├── calls.go           # PostgreSQL
     23     │   └── models.go          # Go structs
     24     └── events/
     25         └── events.go          # Kafka: model_call_completed, model_failover, model_cost_recorded

    Database Schema

    model_providers table
      1 CREATE TABLE model_providers (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(100) NOT NULL,
      5     description     TEXT,
      6     type            VARCHAR(30) NOT NULL CHECK (type IN ('openai', 'anthropic', 'litellm', 'ollama', 'azure', 'custom')),
      7     base_url        VARCHAR(500) NOT NULL,
      8     api_key_secret_name VARCHAR(255),          -- stored in K8s secret reference, not plaintext
      9     is_active       BOOLEAN NOT NULL DEFAULT true,
     10     priority        INT NOT NULL DEFAULT 50,    -- for failover ordering
     11     max_retries     INT NOT NULL DEFAULT 2,
     12     timeout_ms      INT NOT NULL DEFAULT 30000,
     13     metadata        JSONB NOT NULL DEFAULT '{}',
     14     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     15     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     16 );
     17 CREATE INDEX idx_providers_tenant ON model_providers(tenant_id);

    model_registry table
      1 CREATE TABLE model_registry (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     model_name      VARCHAR(200) NOT NULL,       -- e.g., "qwen3.6-35b-a3b", "gpt-4o-mini"
      5     provider_id     UUID NOT NULL,
      6     provider_model_name VARCHAR(200),             -- the actual model name at the provider
      7     supports_chat   BOOLEAN NOT NULL DEFAULT true,
      8     supports_embed  BOOLEAN NOT NULL DEFAULT true,
      9     max_tokens      INT NOT NULL DEFAULT 8192,
     10     cost_per_token  JSONB NOT NULL DEFAULT '{
     11         "prompt": 0.0,
     12         "completion": 0.0
     13     }',
     14     is_default      BOOLEAN NOT NULL DEFAULT false,
     15     is_active       BOOLEAN NOT NULL DEFAULT true,
     16     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     17     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     18     UNIQUE(tenant_id, model_name)
     19 );
     20 CREATE INDEX idx_registry_tenant ON model_registry(tenant_id);

    model_calls table
      1 CREATE TABLE model_calls (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     agent_id        VARCHAR(255),
      5     workflow_id     VARCHAR(255),
      6     model_name      VARCHAR(200) NOT NULL,
      7     provider_id     UUID NOT NULL,
      8     prompt_tokens   INT NOT NULL DEFAULT 0,
      9     completion_tokens INT NOT NULL DEFAULT 0,
     10     total_tokens    INT NOT NULL DEFAULT 0,
     11     cost_usd        FLOAT NOT NULL DEFAULT 0.0,
     12     status          VARCHAR(20) NOT NULL DEFAULT 'success'
     13                   CHECK (status IN ('success', 'error', 'timeout', 'failover')),
     14     error_message   TEXT,
     15     latency_ms      INT,
     16     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     17 );
     18 CREATE INDEX idx_calls_tenant ON model_calls(tenant_id);
     19 CREATE INDEX idx_calls_workflow ON model_calls(tenant_id, workflow_id);

    OpenAPI Endpoints


    ┌────────┬──────────────────────────┬────────────────┬─────────────────────────────────────────────────────────────────┐
    │ Method │ Path                     │ Handler        │ Description                                                     │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/models/completions   │ Completion     │ Unified completion endpoint — forwards to provider, tracks cost │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/models/embeddings    │ Embeddings     │ Unified embeddings endpoint — forwards to provider              │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/model-providers      │ ListProviders  │ Paginated list                                                  │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/model-providers      │ CreateProvider │ Register a new LLM backend                                      │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/model-providers/{id} │ UpdateProvider │ Update provider config                                          │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/model-providers/{id} │ DeleteProvider │ Soft delete                                                     │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/model-registry       │ ListModels     │ Paginated model list, filter by provider/type                   │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/model-registry       │ CreateModel    │ Register a model name → provider mapping                        │
    ├────────┼──────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/model-registry/{id}  │ UpdateModel    │ Update model config (cost, tokens, default flag)                │
    └────────┴──────────────────────────┴────────────────┴─────────────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.model.model_call_completed — { tenant_id, agent_id, workflow_id, model_name, provider, prompt_tokens, completion_tokens, cost_usd, latency_ms, status }
     - operan.model.model_failover — { tenant_id, model_name, from_provider, to_provider, reason }
     - operan.model.model_cost_recorded — { tenant_id, agent_id, model_name, cost_usd, billing_tag } (sent to M17 for cost governance)

    Integration Points
     - M02 IAM: JWT validation, tenant context from JWT. Only users with model_admin role can register providers/models.
     - M03 Orchestration: M03 calls POST /v1/models/completions for all LLM inference (reasoning, classification, summarization).
     - M13 Multi-Model Routing: M13 calls /v1/model-registry to discover available models, then calls /v1/models/completions with a specific model name. M13 does NOT talk to providers directly.
     - M17 Cost Governance: M17 subscribes to operan.model.model_cost_recorded events for budget tracking.

    Implementation Notes
     - Provider Adapters: Implement adapters for:
       - openai: Standard OpenAI API format (chat completions, embeddings)
       - anthropic: Anthropic Messages API
       - litellm: LiteLLM proxy format
       - ollama: Ollama /api/chat and /api/embeddings
       - azure: Azure OpenAI format
     - Request Transformation: Each adapter transforms the unified request format to the provider's native format and back. The unified format matches OpenAI's chat completions API (widely adopted).
     - Cost Calculation: cost_usd = (prompt_tokens * cost_per_prompt_token) + (completion_tokens * cost_per_completion_token). Use the model's cost_per_token from the registry.
     - Failover: If a provider call fails, try the next provider with the same model (if registered), in priority order. Publish model_failover event.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 35 tests)
     - Completion: valid request to OpenAI adapter, valid to Anthropic adapter, valid to LiteLLM adapter, timeout handling, provider failure → failover, error passthrough
     - Embeddings: valid request, unsupported model (400)
     - Provider CRUD: create, list, get, update, soft delete
     - Model Registry: create, list by provider, set-as-default, update cost
     - Cost tracking: accurate calculation for various token counts
     - Middleware: JWT validation, tenant isolation, RBAC for provider management
     - DB: connection handling, concurrent calls
     - Kafka: event publishing, broker unreachable

    Deliverables
    Build passes, 35+ tests, OpenAPI-compliant, Helm chart, README.
     1 
     2 ---
     3 
     4 # MODULE 13 — Multi-Model Routing Engine
     5 
     6 **Dependency:** M02 (IAM), M12 (Model Abstraction Layer)
    You are implementing Module 13 — Multi-Model Routing Engine for the Operan platform. This module intelligently assigns the best LLM model to each task based on task type, cost, latency, accuracy
    requirements, and agent capabilities.

    Purpose
    Instead of hardcoding which model to use, M13 routes each inference request to the optimal model. It maintains routing rules, monitors model performance, and automatically switches models when one
     degrades. Think of it as a DNS server for AI models.

    Key Concepts
     - Routing Rules: Conditional rules mapping task types/contexts to model preferences
     - Task Classifiers: Determines what type of task is being performed (reasoning, classification, extraction, summarization, code, chat)
     - Model Performance: Tracks accuracy, latency, cost per model over time
     - Dynamic Routing: Routes change based on real-time model health and cost

    Files to Create

    Directory Structure
      1 modules/13-multi-model-routing-engine/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8013
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── routing.go         # POST /v1/routing/resolve (core endpoint)
     15     │   ├── rules.go           # CRUD routing rules
     16     │   ├── performance.go     # GET /v1/routing/performance
     17     │   └── router.go
     18     ├── store/
     19     │   ├── rules.go           # PostgreSQL
     20     │   ├── performance.go     # PostgreSQL
     21     │   └── models.go
     22     └── events/
     23         └── events.go          # Kafka: model_routed, model_switched

    Database Schema

    routing_rules table
      1 CREATE TABLE routing_rules (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     description     TEXT,
      6     task_type       VARCHAR(50) NOT NULL CHECK (task_type IN ('reasoning', 'classification', 'extraction', 'summarization', 'code', 'chat', 'embedding', 'custom')),
      7     priority        INT NOT NULL DEFAULT 50,
      8     model_preferences JSONB NOT NULL DEFAULT '[
      9         {"model_name": "qwen3.6-35b-a3b", "weight": 0.7},
     10         {"model_name": "gpt-4o-mini", "weight": 0.3}
     11     ]',
     12     conditions      JSONB NOT NULL DEFAULT '{}',  -- { "min_cost_tolerance": 0.01, "max_latency_ms": 5000, "require_arabic": true }
     13     is_active       BOOLEAN NOT NULL DEFAULT true,
     14     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     15     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     16 );
     17 CREATE INDEX idx_rules_tenant ON routing_rules(tenant_id);
     18 CREATE INDEX idx_rules_task ON routing_rules(tenant_id, task_type, is_active);

    routing_performance table
      1 CREATE TABLE routing_performance (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     model_name      VARCHAR(200) NOT NULL,
      5     task_type       VARCHAR(50) NOT NULL,
      6     avg_latency_ms  FLOAT NOT NULL DEFAULT 0,
      7     error_rate      FLOAT NOT NULL DEFAULT 0,
      8     avg_cost_per_call FLOAT NOT NULL DEFAULT 0,
      9     samples         INT NOT NULL DEFAULT 1,
     10     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE UNIQUE INDEX idx_perf_tenant_model_task ON routing_performance(tenant_id, model_name, task_type);

    OpenAPI Endpoints


    ┌────────┬─────────────────────────┬────────────────┬────────────────────────────────────────────────────────────────────────────────┐
    │ Method │ Path                    │ Handler        │ Description                                                                    │
    ├────────┼─────────────────────────┼────────────────┼────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/routing/resolve     │ ResolveRoute   │ Core endpoint — given a task description + context, return the best model name │
    ├────────┼─────────────────────────┼────────────────┼────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/routing/rules       │ ListRules      │ Paginated list, filter by task_type                                            │
    ├────────┼─────────────────────────┼────────────────┼────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/routing/rules       │ CreateRule     │ Create routing rule                                                            │
    ├────────┼─────────────────────────┼────────────────┼────────────────────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/routing/rules/{id}  │ UpdateRule     │ Update rule                                                                    │
    ├────────┼─────────────────────────┼────────────────┼────────────────────────────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/routing/rules/{id}  │ DeleteRule     │ Soft delete                                                                    │
    ├────────┼─────────────────────────┼────────────────┼────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/routing/performance │ GetPerformance │ Performance metrics by model/task_type                                         │
    └────────┴─────────────────────────┴────────────────┴────────────────────────────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.routing.model_routed — { tenant_id, task_type, resolved_model, rule_id, latency_ms }
     - operan.routing.model_switched — { tenant_id, model_name, from_score, to_score, reason } (emitted when dynamic routing changes model due to performance degradation)

    Integration Points
     - M02 IAM: JWT validation, tenant context.
     - M12 Model Abstraction: M13 resolves a model name, then the caller (M03 or any agent) calls M12's /v1/models/completions with that model name. M13 does NOT make LLM calls itself — it only
       resolves.
     - M03 Orchestration: Before executing a reasoning step, M03 calls POST /v1/routing/resolve with the task description to pick the best model.
     - M19 Arabic Language: M13's routing rules have a require_arabic condition (added later when M19 is implemented).

    Implementation Notes
     - Routing Algorithm: The /v1/routing/resolve endpoint:
       1. Classifies the task type from the input context (simple keyword-based for now: "reason" → reasoning, "classify" → classification, etc.)
       2. Finds all matching routing rules for the task type (sorted by priority)
       3. Filters rules by conditions (cost tolerance, latency, language requirements)
       4. Selects the model with highest weight × performance_score
       5. Records the routing decision in routing_performance
       6. Publishes model_routed event
     - Performance Tracking: After each call, M13 updates routing_performance with latency, error rate, cost. This is used for future routing decisions.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.
     - Simplicity: This is the routing layer, not the execution layer. Keep it stateless and fast (< 10ms resolve time).

    Tests (minimum 25 tests)
     - Routing resolution: task classification (reasoning, code, chat, embedding), rule priority, condition filtering, no matching rule (404)
     - Rule CRUD: create, list by task_type, update, delete
     - Performance: update performance metrics, query performance, stale data handling
     - Integration: resolve → return model that exists in registry
     - Middleware: JWT validation, tenant isolation
     - Kafka: event publishing, broker unreachable

    Deliverables
    Build passes, 25+ tests, OpenAPI-compliant, Helm chart, README.

     1 
     2 ---
     3 
     4 # MODULE 19 — Arabic Language Core
     5 
     6 **Dependency:** M02 (IAM), M07 (Memory Fabric — to store Arabic embeddings)
    You are implementing Module 19 — Arabic Language Core for the Operan platform. This module provides Arabic-native NLP capabilities: text normalization, dialect detection, terminology governance,
    and Arabic-specific embeddings.

    Purpose
    Arabic is the primary language for the target market (Saudi Arabia, Gulf region). Standard NLP tools fail on Arabic — it has diglossia (Modern Standard Arabic vs. dialects), complex morphology,
    and right-to-left script. M19 handles all Arabic-specific language processing.

    Key Concepts
     - Text Normalization: Converts different Arabic orthographies to a canonical form (removes tashkeel, normalizes alef variants, handles hamza)
     - Dialect Detection: Identifies whether text is MSA, Gulf, Levantine, Egyptian, Maghrebi
     - Terminology Governance: Ensures consistent Arabic terminology across departments (e.g., "contract" = "عقد" not "اتفاقية")
     - Arabic Embeddings: Arabic-aware vector representations for semantic search
     - RTL Support: All text processing respects bidirectional text

    Files to Create

    Directory Structure
      1 modules/19-arabic-language-core/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8019
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── normalize.go       # POST /v1/arabic/normalize
     15     │   ├── dialect.go         # POST /v1/arabic/detect-dialect
     16     │   ├── terminology.go     # CRUD terminology glossary
     17     │   ├── embeddings.go      # POST /v1/arabic/embeddings
     18     │   └── router.go
     19     ├── store/
     20     │   ├── terminology.go     # PostgreSQL: terminology glossary
     21     │   └── models.go
     22     ├── nlp/
     23     │   ├── normalize.go       # Arabic normalization logic
     24     │   ├── dialect.go         # Dialect detection logic
     25     │   └── embeddings.go      # Arabic embedding client (calls M07 or external model)
     26     └── events/
     27         └── events.go          # Kafka: terminology_added, embedding_generated

    Database Schema

    terminology_glossary table
      1 CREATE TABLE terminology_glossary (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     arabic_term     VARCHAR(200) NOT NULL,
      5     english_term    VARCHAR(200) NOT NULL,
      6     category        VARCHAR(50) NOT NULL CHECK (category IN ('legal', 'financial', 'hr', 'technical', 'general')),
      7     preferred_form  VARCHAR(200) NOT NULL,           -- canonical Arabic form
      8     alternate_forms TEXT[],                          -- accepted variants
      9     dialect_hint    VARCHAR(50),                     -- msa | gulf | levantine | egyptian | maghrebi
     10     is_mandatory    BOOLEAN NOT NULL DEFAULT false,   -- if true, violations are logged
     11     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     12     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     13     UNIQUE(tenant_id, arabic_term)
     14 );
     15 CREATE INDEX idx_term_tenant ON terminology_glossary(tenant_id);
     16 CREATE INDEX idx_term_category ON terminology_glossary(tenant_id, category);

    OpenAPI Endpoints


    ┌────────┬──────────────────────────────┬────────────────────┬───────────────────────────────────────────────────────────────────────────────────────┐
    │ Method │ Path                         │ Handler            │ Description                                                                           │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/arabic/normalize         │ NormalizeText      │ Normalize Arabic text (remove tashkeel, normalize alef/alef-madda, standardize hamza) │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/arabic/detect-dialect    │ DetectDialect      │ Detect Arabic dialect of input text                                                   │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/arabic/check-terminology │ CheckTerminology   │ Check if text uses approved terminology, returns suggestions                          │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/arabic/terminology       │ ListTerminology    │ Paginated glossary, filter by category/dialect                                        │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/arabic/terminology       │ CreateTerminology  │ Add term to glossary                                                                  │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/arabic/terminology/{id}  │ UpdateTerminology  │ Update term                                                                           │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/arabic/terminology/{id}  │ DeleteTerminology  │ Remove term                                                                           │
    ├────────┼──────────────────────────────┼────────────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/arabic/embeddings        │ GenerateEmbeddings │ Generate Arabic-aware embeddings for text batch                                       │
    └────────┴──────────────────────────────┴────────────────────┴───────────────────────────────────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.arabic.terminology_added — { tenant_id, arabic_term, category, preferred_form }
     - operan.arabic.embedding_generated — { tenant_id, text_length, embedding_model, dims }

    Integration Points
     - M02 IAM: JWT validation, tenant isolation for terminology glossary.
     - M07 Memory Fabric: Arabic embeddings are stored in M07's vector store. M19 calls M07's /vectors endpoint after generating embeddings. The embedding request includes embedding_type: "arabic" in
       metadata.
     - M13 Multi-Model Routing: M13 has a require_arabic: true condition in routing rules that selects Arabic-capable models.
     - M06 Knowledge Ingestion: M06 calls M19's /v1/arabic/normalize and /v1/arabic/embeddings when processing Arabic documents.

    Implementation Notes
     - Arabic Normalization: Implement these transformations:
       - Remove tashkeel (diacritical marks): َ ِ ُ ّ ً ٍ ُ ّ ٰ
       - Normalize alef variants: ا أ إ آ ئ → ا
       - Normalize yeh: ي ى → ي
       - Normalize ta marbuta: ة → ه (in normalization context)
       - Standardize hamza: ؤ ئ → و ي
     - Dialect Detection: Use keyword frequency analysis and simple n-gram matching against dialect profiles. For a Go-only implementation, use a lightweight rule-based approach (not a neural network).
       Document that production systems should integrate with an Arabic NLP service.
     - Terminology Checking: Compare input text against the glossary. If a non-preferred form is used, return suggestions. If a mandatory term is violated, flag it.
     - Arabic Embeddings: Since we can't run a full Arabic embedding model in Go, implement M19 as a client that:
       1. Takes Arabic text
       2. Normalizes it
       3. Calls an external embedding model (LiteLLM with an Arabic model like jina/jina-embeddings-v3 or a local Arabic model)
       4. Returns the embeddings
       5. Stores them in M07 (if requested)
     - RTL Handling: All text processing uses Go's unicode package for proper Arabic character handling.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 30 tests)
     - Normalization: remove tashkeel, normalize alef variants, normalize yeh, normalize hamza, mixed Arabic/English, empty input
     - Dialect detection: MSA, Gulf, Levantine, Egyptian, Maghrebi, mixed
     - Terminology: add term, list by category, check text (approved form, non-preferred form, mandatory violation), update term, delete term
     - Embeddings: valid request, invalid model, zero-length input
     - Integration: terminology stored in DB with tenant isolation
     - Middleware: JWT validation, tenant isolation
     - Kafka: event publishing

    Deliverables
    Build passes, 30+ tests, OpenAPI-compliant, Helm chart, README.

     1 
     2 ---
     3 
     4 # MODULE 06 — Knowledge Ingestion Pipeline
     5 
     6 **Dependency:** M02 (IAM), M07 (Memory Fabric), M19 (Arabic Language Core)
    You are implementing Module 06 — Knowledge Ingestion Pipeline for the Operan platform. This module ingests enterprise documents (PDF, Word, Excel, SharePoint, CRM exports) and transforms them into
     structured knowledge stored in M07's Memory Fabric.

    Purpose
    Agents need institutional knowledge, but enterprises don't store knowledge as clean text — they store it in PDFs, Word docs, Excel sheets, SharePoint pages, and CRM exports. M06 handles the full
    ingestion pipeline: extract → normalize → segment → embed → store.

    Key Concepts
     - Sources: PDF, DOCX, XLSX, TXT, HTML, CSV, SharePoint list, CRM export (JSON/CSV)
     - Extraction: Text extraction from binary formats
     - Segmentation: Split documents into semantically coherent chunks (paragraphs, pages, or table sections)
     - Metadata Enrichment: Extract document type, language, author, date, department
     - Arabic Support: Arabic text normalization via M19 before embedding

    Files to Create

    Directory Structure
      1 modules/06-knowledge-ingestion/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8006
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go          # JWT_SECRET, EVENT_BROKER_URL, DB_DSN, M07_BASE_URL, M19_BASE_URL
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── sources.go         # CRUD ingestion sources (SharePoint, CRM, file upload)
     15     │   ├── jobs.go            # Create/list/get/delete ingestion jobs
     16     │   ├── results.go         # List/get ingestion results
     17     │   └── router.go
     18     ├── store/
     19     │   ├── sources.go         # PostgreSQL
     20     │   ├── jobs.go            # PostgreSQL
     21     │   ├── results.go         # PostgreSQL
     22     │   └── models.go
     23     ├── ingest/
     24     │   ├── extractor.go       # Text extraction from PDF/DOCX/XLSX/TXT/HTML
     25     │   ├── segmenter.go       # Chunk text into segments
     26     │   ├── enricher.go        # Extract metadata
     27     │   ├── normalizer.go      # Call M19 for Arabic normalization
     28     │   ├── embedder.go        # Call M07 for embeddings
     29     │   └── pipeline.go        # Orchestrates the full pipeline
     30     └── events/
     31         └── events.go          # Kafka: job_created, job_completed, job_failed, segments_ingested

    Database Schema

    ingestion_sources table
      1 CREATE TABLE ingestion_sources (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     type            VARCHAR(30) NOT NULL CHECK (type IN ('file_upload', 'sharepoint', 'crm_export', 'database_query', 'web_scrape')),
      6     config          JSONB NOT NULL DEFAULT '{}',      -- type-specific config (url, credentials, query)
      7     schedule        VARCHAR(50),                      -- cron expression or null (one-time)
      8     is_active       BOOLEAN NOT NULL DEFAULT true,
      9     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     10     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE INDEX idx_sources_tenant ON ingestion_sources(tenant_id);

    ingestion_jobs table
      1 CREATE TABLE ingestion_jobs (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     source_id       UUID,                             -- nullable (one-time uploads)
      5     name            VARCHAR(255) NOT NULL,
      6     status          VARCHAR(20) NOT NULL DEFAULT 'pending'
      7                   CHECK (status IN ('pending', 'extracting', 'segmenting', 'normalizing', 'embedding', 'storing', 'completed', 'failed')),
      8     total_segments  INT NOT NULL DEFAULT 0,
      9     processed_segments INT NOT NULL DEFAULT 0,
     10     error_message   TEXT,
     11     started_at      TIMESTAMPTZ,
     12     completed_at    TIMESTAMPTZ,
     13     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     14 );
     15 CREATE INDEX idx_jobs_tenant ON ingestion_jobs(tenant_id);
     16 CREATE INDEX idx_jobs_status ON ingestion_jobs(tenant_id, status);

    ingestion_results table
      1 CREATE TABLE ingestion_results (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     job_id          UUID NOT NULL,
      5     source_id       UUID,
      6     document_id     VARCHAR(255) NOT NULL,
      7     language        VARCHAR(10) NOT NULL DEFAULT 'en',
      8     segment_text    TEXT NOT NULL,
      9     segment_type    VARCHAR(30) NOT NULL DEFAULT 'paragraph'
     10                   CHECK (segment_type IN ('paragraph', 'table', 'section', 'bullet_list', 'header')),
     11     metadata        JSONB NOT NULL DEFAULT '{}',
     12     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     13 );
     14 CREATE INDEX idx_results_tenant ON ingestion_results(tenant_id);
     15 CREATE INDEX idx_results_job ON ingestion_results(tenant_id, job_id);

    OpenAPI Endpoints


    ┌────────┬────────────────────────────┬──────────────┬────────────────────────────────────────────────────────┐
    │ Method │ Path                       │ Handler      │ Description                                            │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ POST   │ /v1/ingestion/sources      │ CreateSource │ Register an ingestion source                           │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/ingestion/sources      │ ListSources  │ Paginated list                                         │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/ingestion/sources/{id} │ GetSource    │ Get source details                                     │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/ingestion/sources/{id} │ UpdateSource │ Update source config                                   │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/ingestion/sources/{id} │ DeleteSource │ Soft delete                                            │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ POST   │ /v1/ingestion/jobs         │ CreateJob    │ Start an ingestion job (file upload or source trigger) │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/ingestion/jobs         │ ListJobs     │ Paginated job list                                     │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/ingestion/jobs/{id}    │ GetJob       │ Job status and progress                                │
    ├────────┼────────────────────────────┼──────────────┼────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/ingestion/jobs/{id}    │ CancelJob    │ Cancel a running job                                   │
    └────────┴────────────────────────────┴──────────────┴────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.ingestion.job_created — { tenant_id, source_id, job_id, document_count }
     - operan.ingestion.job_completed — { tenant_id, job_id, segments_ingested, duration_ms }
     - operan.ingestion.job_failed — { tenant_id, job_id, error }
     - operan.ingestion.segments_ingested — { tenant_id, job_id, document_id, segment_count }

    Integration Points
     - M02 IAM: JWT validation, tenant isolation for sources and jobs.
     - M07 Memory Fabric: After segmentation, M06 calls M07's /vectors endpoint to store each segment. The request includes tenant_id, agent_id (from context), and embedding_type: "knowledge_ingestion".
     - M19 Arabic Language Core: Before embedding, M06 calls M19's /v1/arabic/normalize if the detected language is Arabic (ar). M19 also provides dialect detection which is stored in metadata.

    Implementation Notes
     - Text Extraction: Use Go libraries:
       - PDF: github.com/unidoc/unipdf/v3/reader or github.com/ledongthuc/pdf
       - DOCX: github.com/yuin/goldmark (for HTML extraction via pandoc-like approach) or parse the XML directly
       - XLSX: github.com/xuri/excelize/v2
       - TXT/HTML/CSV: standard library
     - Segmentation Strategy:
       - PDF: Extract by page, then by paragraph (blank line boundaries)
       - DOCX: Extract by paragraph
       - XLSX: Each row is a segment, each column is a field
       - HTML: Extract by <p>, <li>, <table> elements
       - Target chunk size: 500-1000 tokens (approximate via character count for Arabic/English)
     - Pipeline Flow: Extract → Detect Language → (If Arabic) Normalize via M19 → Segment → Extract Metadata → (If embeddings enabled) Call M07 for embeddings → Store result in DB → Publish events
     - Async Processing: Jobs run asynchronously. The API returns job_id immediately. Clients poll /v1/ingestion/jobs/{id} for status.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 30 tests)
     - Source CRUD: create, list, get, update, delete
     - Job lifecycle: create job, poll status, cancel running job, job completes, job fails
     - Extraction: valid PDF, valid DOCX, valid XLSX, valid TXT, valid HTML, invalid file (400)
     - Segmentation: paragraph split, table extraction, bullet list, empty document
     - Arabic support: detect Arabic, normalize via M19 (mock), store with Arabic metadata
     - Integration: segments stored in M07 via HTTP call (mock M07 in tests)
     - Middleware: JWT validation, tenant isolation
     - Kafka: event publishing

    Deliverables
    Build passes, 30+ tests, OpenAPI-compliant, Helm chart, README.

     1 
     2 ---
     3 
     4 # MODULE 14 — Agent Collaboration Bus
     5 
     6 **Dependency:** M02 (IAM), M03 (Orchestration — for workflow context), M07 (Memory Fabric — for shared memory)
    You are implementing Module 14 — Agent Collaboration Bus for the Operan platform. This module enables agents within a department to communicate, share context, hand off tasks, and coordinate
    workflows through a real-time messaging layer.

    Purpose
    In a real department, agents don't work in isolation. A sales agent might need to hand off a contract review to a legal agent, or ask a pricing agent for a quote before drafting an email. M14
    provides the messaging, context sharing, and handoff mechanism for inter-agent collaboration.

    Key Concepts
     - Channels: Department-level or topic-level channels where agents post messages
     - Direct Messages: Agent-to-agent private messages
     - Context Sharing: Attachments that agents can share (documents, analysis results, memory queries)
     - Task Handoffs: Transfer a workflow task from one agent to another
     - Presence: Agent availability status

    Files to Create

    Directory Structure
      1 modules/14-agent-collaboration/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8014
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── channels.go        # CRUD channels
     15     │   ├── messages.go        # Send/list/get messages
     16     │   ├── handoffs.go        # Create/track task handoffs
     17     │   ├── presence.go        # Agent presence/status
     18     │   └── router.go
     19     ├── store/
     20     │   ├── channels.go        # PostgreSQL
     21     │   ├── messages.go        # PostgreSQL
     22     │   ├── handoffs.go        # PostgreSQL
     23     │   ├── presence.go        # PostgreSQL
     24     │   └── models.go
     25     └── events/
     26         └── events.go          # Kafka: message_sent, handoff_created, handoff_accepted, presence_changed

    Database Schema

    channels table
      1 CREATE TABLE channels (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     description     TEXT,
      6     type            VARCHAR(20) NOT NULL CHECK (type IN ('department', 'topic', 'direct')),
      7     member_agent_ids TEXT[],                           -- agent IDs that can post
      8     created_by      VARCHAR(255) NOT NULL,              -- requesting user or agent
      9     is_active       BOOLEAN NOT NULL DEFAULT true,
     10     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     11     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     12 );
     13 CREATE INDEX idx_channels_tenant ON channels(tenant_id);
     14 CREATE INDEX idx_channels_type ON channels(tenant_id, type, is_active);

    messages table
      1 CREATE TABLE messages (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     channel_id      UUID NOT NULL,
      5     sender_agent_id VARCHAR(255) NOT NULL,
      6     reply_to_id     UUID,                             -- nullable (threading)
      7     content         TEXT NOT NULL,
      8     attachments     JSONB NOT NULL DEFAULT '[]',      -- [{ "type": "document", "ref": "doc-uuid", "title": "..." }]
      9     context_refs    JSONB NOT NULL DEFAULT '[]',      -- [{ "type": "memory_query", "result_count": 3 }]
     10     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE INDEX idx_messages_channel ON messages(channel_id);
     13 CREATE INDEX idx_messages_sender ON messages(tenant_id, sender_agent_id);

    handoffs table
      1 CREATE TABLE handoffs (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     workflow_id     VARCHAR(255),                     -- nullable (standalone or workflow-linked)
      5     task_id         VARCHAR(255),
      6     from_agent_id   VARCHAR(255) NOT NULL,
      7     to_agent_id     VARCHAR(255) NOT NULL,
      8     context         JSONB NOT NULL DEFAULT '{}',      -- { "summary": "...", "attachments": [...] }
      9     status          VARCHAR(20) NOT NULL DEFAULT 'pending'
     10                   CHECK (status IN ('pending', 'accepted', 'rejected', 'completed', 'expired')),
     11     accepted_at     TIMESTAMPTZ,
     12     completed_at    TIMESTAMPTZ,
     13     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     14     expires_at      TIMESTAMPTZ
     15 );
     16 CREATE INDEX idx_handoffs_tenant ON handoffs(tenant_id);
     17 CREATE INDEX idx_handoffs_status ON handoffs(tenant_id, status);
     18 CREATE INDEX idx_handoffs_to_agent ON handoffs(tenant_id, to_agent_id, status);

    presence table
      1 CREATE TABLE presence (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     agent_id        VARCHAR(255) NOT NULL UNIQUE,
      5     status          VARCHAR(20) NOT NULL DEFAULT 'available'
      6                   CHECK (status IN ('available', 'busy', 'away', 'offline')),
      7     last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      8     metadata        JSONB NOT NULL DEFAULT '{}',
      9     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     10 );
     11 CREATE INDEX idx_presence_tenant ON presence(tenant_id);

    OpenAPI Endpoints


    ┌────────┬────────────────────────────┬─────────────────┬──────────────────────────────────────────────┐
    │ Method │ Path                       │ Handler         │ Description                                  │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ POST   │ /v1/channels               │ CreateChannel   │ Create a department/topic/direct channel     │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ GET    │ /v1/channels               │ ListChannels    │ Paginated list                               │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ GET    │ /v1/channels/{id}          │ GetChannel      │ Channel details                              │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ POST   │ /v1/channels/{id}/messages │ SendMessage     │ Post a message to channel                    │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ GET    │ /v1/channels/{id}/messages │ ListMessages    │ Paginated message list, with reply threading │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ POST   │ /v1/handoffs               │ CreateHandoff   │ Create a task handoff request                │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ GET    │ /v1/handoffs               │ ListHandoffs    │ List pending/accepted handoffs for agent     │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ PATCH  │ /v1/handoffs/{id}/accept   │ AcceptHandoff   │ Accept a handoff                             │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ PATCH  │ /v1/handoffs/{id}/reject   │ RejectHandoff   │ Reject a handoff                             │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ PATCH  │ /v1/handoffs/{id}/complete │ CompleteHandoff │ Mark handoff as completed                    │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ POST   │ /v1/presence               │ UpdatePresence  │ Set agent availability status                │
    ├────────┼────────────────────────────┼─────────────────┼──────────────────────────────────────────────┤
    │ GET    │ /v1/presence               │ ListPresence    │ List agent availability                      │
    └────────┴────────────────────────────┴─────────────────┴──────────────────────────────────────────────┘

    Kafka Events

     - operan.collaboration.message_sent — { tenant_id, channel_id, sender_agent_id, message_id }
     - operan.collaboration.handoff_created — { tenant_id, handoff_id, from_agent, to_agent, workflow_id }
     - operan.collaboration.handoff_accepted — { tenant_id, handoff_id, accepted_by }
     - operan.collaboration.handoff_completed — { tenant_id, handoff_id, duration_ms }
     - operan.collaboration.presence_changed — { tenant_id, agent_id, status }

    Integration Points
     - M02 IAM: JWT validation, tenant isolation. Only agents from the same tenant can collaborate.
     - M03 Orchestration: M03 uses handoffs to delegate tasks between agents. When M03 creates a workflow with multiple agents, it uses M14 to manage the handoff between them.
     - M07 Memory Fabric: Messages can include context_refs pointing to memory queries. When a message is sent, M14 stores the full message content in M07's episodic memory for retrieval later.

    Implementation Notes
     - Message Threading: Support reply chains via reply_to_id. ListMessages returns threads (root + replies).
     - Handoff Expiry: Handoffs have an optional expires_at. If not accepted by then, status → expired.
     - Presence Heartbeat: The presence endpoint is a "heartbeat" — clients POST their status every 60s. If no heartbeat for 5 minutes, status → offline.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.
     - Simplicity: This is a message bus, not a real-time WebSocket solution. All communication is HTTP REST + Kafka events. Clients poll for new messages.

    Tests (minimum 35 tests)
     - Channel CRUD: create department/topic/direct, list, get
     - Messages: send with/without attachments, send with context refs, list with threading
     - Handoffs: create with/without workflow link, accept, reject, complete, expire
     - Presence: update, list, stale → offline
     - Integration: handoff creates workflow task in M03 (mock M03), message stores to M07 memory (mock M07)
     - Middleware: JWT validation, tenant isolation (tenant A cannot see tenant B's channels)
     - Kafka: all 5 event types published correctly
     - DB: concurrent messages, handoff race conditions (double accept)

    Deliverables
    Build passes, 35+ tests, OpenAPI-compliant, Helm chart, README.
     1 
     2 ---
     3 
     4 # MODULE 15 — Agent Marketplace
     5 
     6 **Dependency:** M02 (IAM), M04 (Agent Registry)
    You are implementing Module 15 — Agent Marketplace for the Operan platform. This module provides a procurement ecosystem where departments can browse, license, and deploy pre-built agents,
    workflows, and templates from a marketplace.

    Purpose
    Not every department wants to build agents from scratch. The marketplace lets them discover pre-validated agents ("Sales Drafting Agent v2.1"), licensed workflows ("Contract Review Pipeline"), and
     department templates ("Legal Department Pro"), then deploy them with one click.

    Key Concepts
     - Marketplace Listings: Publishable agents/workflows/templates with descriptions, ratings, pricing
     - Licenses: Licensing terms (open-source, internal license, paid, subscription)
     - Vetted vs User-Generated: Platform-vetted listings vs community-submitted listings
     - Deployment: One-click deploy from marketplace into the tenant's agent registry

    Files to Create

    Directory Structure
      1 modules/15-agent-marketplace/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8015
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── listings.go        # Browse/list/get marketplace listings
     15     │   ├── licenses.go        # CRUD licensing terms
     16     │   ├── subscriptions.go   # Tenant subscriptions to listings
     17     │   └── router.go
     18     ├── store/
     19     │   ├── listings.go        # PostgreSQL
     20     │   ├── licenses.go        # PostgreSQL
     21     │   ├── subscriptions.go   # PostgreSQL
     22     │   └── models.go
     23     └── events/
     24         └── events.go          # Kafka: listing_purchased, listing_deployed, subscription_created

    Database Schema

    marketplace_listings table
      1 CREATE TABLE marketplace_listings (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     listing_id      VARCHAR(255) NOT NULL,            -- external listing identifier
      4     title           VARCHAR(255) NOT NULL,
      5     description     TEXT,
      6     category        VARCHAR(50) NOT NULL CHECK (category IN ('agent', 'workflow', 'template', 'tool', 'integration')),
      7     vendor_name     VARCHAR(255),                     -- vendor/creator
      8     version         VARCHAR(50) NOT NULL,
      9     rating          FLOAT NOT NULL DEFAULT 0,         -- 0-5
     10     review_count    INT NOT NULL DEFAULT 0,
     11     is_vetted       BOOLEAN NOT NULL DEFAULT false,
     12     license_id      UUID NOT NULL,
     13     compatibility   JSONB NOT NULL DEFAULT '[]',      -- [{ "module": "03", "min_version": "1.0.0" }]
     14     readme          TEXT,                             -- full documentation text
     15     screenshot_url  VARCHAR(500),
     16     is_published    BOOLEAN NOT NULL DEFAULT false,   -- false = draft
     17     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     18     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     19 );
     20 CREATE INDEX idx_listings_published ON marketplace_listings(is_published, category);

    licenses table
      1 CREATE TABLE licenses (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     name            VARCHAR(255) NOT NULL,
      4     description     TEXT,
      5     type            VARCHAR(30) NOT NULL CHECK (type IN ('open_source', 'internal', 'paid', 'subscription', 'trial')),
      6     terms           TEXT NOT NULL,                    -- license terms text
      7     cost_per_month  FLOAT NOT NULL DEFAULT 0,
      8     max_agents      INT NOT NULL DEFAULT -1,          -- -1 = unlimited
      9     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     10 );

    tenant_subscriptions table
      1 CREATE TABLE tenant_subscriptions (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     listing_id      VARCHAR(255) NOT NULL,
      5     license_id      UUID NOT NULL,
      6     status          VARCHAR(20) NOT NULL DEFAULT 'active'
      7                   CHECK (status IN ('active', 'expired', 'revoked')),
      8     started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      9     expires_at      TIMESTAMPTZ,
     10     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE UNIQUE INDEX idx_sub_tenant_listing ON tenant_subscriptions(tenant_id, listing_id);

    OpenAPI Endpoints


    ┌────────┬───────────────────────────────┬───────────────────┬─────────────────────────────────────────────────────────────────┐
    │ Method │ Path                          │ Handler           │ Description                                                     │
    ├────────┼───────────────────────────────┼───────────────────┼─────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/marketplace/listings      │ ListListings      │ Browse all published listings, filter by category/vendor/rating │
    ├────────┼───────────────────────────────┼───────────────────┼─────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/marketplace/listings/{id} │ GetListing        │ Full listing details                                            │
    ├────────┼───────────────────────────────┼───────────────────┼─────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/marketplace/subscribe     │ Subscribe         │ Subscribe to a listing's license (creates tenant_subscriptions) │
    ├────────┼───────────────────────────────┼───────────────────┼─────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/marketplace/subscriptions │ ListSubscriptions │ My tenant's active subscriptions                                │
    ├────────┼───────────────────────────────┼───────────────────┼─────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/marketplace/licenses      │ ListLicenses      │ All available license types                                     │
    └────────┴───────────────────────────────┴───────────────────┴─────────────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.marketplace.listing_purchased — { tenant_id, listing_id, license_id, cost }
     - operan.marketplace.listing_deployed — { tenant_id, listing_id, deployed_by } (triggered when tenant deploys to their registry)
     - operan.marketplace.subscription_created — { tenant_id, listing_id, license_id, expires_at }

    Integration Points
     - M02 IAM: JWT validation, tenant isolation for subscriptions. Only authenticated users can subscribe.
     - M04 Agent Registry: When a tenant "deploys" a marketplace listing, M15 calls M04's /registry/agents endpoint to register the agents defined in the listing. The listing's compatibility field is
       checked against the tenant's registered modules before deployment.

    Implementation Notes
     - Simplified Marketplace: This is a B2B platform marketplace, not a consumer app. Listings are curated, not user-submitted (no ratings/reviews from end users — only platform vetting).
     - Listing to Agent Deployment: A listing of type agent contains an agent_definition JSON blob (same structure as M04's CreateAgentRequest). Deployment copies this into M04.
     - No Payment Processing: This module handles licensing terms and subscriptions but does NOT process payments. That's Module 01's billing module (not yet implemented).
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 25 tests)
     - Listings: list published, filter by category, get by ID, get non-existent (404)
     - Subscriptions: create, list, check tenant can subscribe to listing they already have
     - Licenses: list all
     - Deployment flow: subscribe → deploy to M04 (mock M04), verify agent registered
     - Middleware: JWT validation, tenant isolation
     - Kafka: all 3 event types

    Deliverables
    Build passes, 25+ tests, OpenAPI-compliant, Helm chart, README.
     1 
     2 ---
     3 
     4 # MODULE 18 — Enterprise Connector Fabric
     5 
     6 **Dependency:** M02 (IAM), M04 (Agent Registry — for tool definitions), M08 (Tool Execution — to execute connector tools)
    You are implementing Module 18 — Enterprise Connector Fabric for the Operan platform. This module provides pre-built connectors for enterprise systems: SMTP/Email, Salesforce, HubSpot, Microsoft
    365, SAP, and a generic REST connector framework.

    Purpose
    Agents need to interact with real enterprise systems. Instead of each agent building its own API integrations, M18 provides pre-built, authenticated connectors that agents call through M08's tool
    execution layer.

    Key Concepts
     - Connector Types: Pre-built connectors for specific systems (SMTP, Salesforce, HubSpot, M365, SAP, generic REST)
     - Connector Configurations: Tenant-specific connection settings (API keys, OAuth tokens, endpoints)
     - Connector Tools: Each connector exposes tools (send_email, create_contact, update_record, query_records, upload_file)
     - Connector Health: Monitor connector availability and authentication status

    Files to Create

    Directory Structure
      1 modules/18-enterprise-connector-fabric/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8018
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go          # JWT_SECRET, EVENT_BROKER_URL, DB_DSN, M08_BASE_URL
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── connectors.go      # CRUD connector definitions
     15     │   ├── configs.go         # Tenant-specific connector configurations
     16     │   ├── tools.go           # List available tools per connector
     17     │   ├── health.go          # Check connector health/authentication
     18     │   └── router.go
     19     ├── store/
     20     │   ├── connectors.go      # PostgreSQL
     21     │   ├── configs.go         # PostgreSQL
     22     │   ├── tools.go           # PostgreSQL
     23     │   └── models.go
     24     ├── connector/
     25     │   ├── smtp.go            # SMTP email connector
     26     │   ├── salesforce.go      # Salesforce REST API connector
     27     │   ├── hubspot.go         # HubSpot REST API connector
     28     │   ├── m365.go            # Microsoft Graph API connector
     29     │   ├── sap.go             # SAP OData/REST connector
     30     │   ├── generic_rest.go    # Generic REST connector (configurable)
     31     │   └── registry.go        # Connector registry pattern
     32     └── events/
     33         └── events.go          # Kafka: connector_configured, connector_healthy, connector_auth_failed

    Database Schema

    connector_definitions table
      1 CREATE TABLE connector_definitions (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(100) NOT NULL,
      5     type            VARCHAR(30) NOT NULL CHECK (type IN ('smtp', 'salesforce', 'hubspot', 'm365', 'sap', 'generic_rest')),
      6     description     TEXT,
      7     is_active       BOOLEAN NOT NULL DEFAULT true,
      8     metadata        JSONB NOT NULL DEFAULT '{}',      -- { "base_url": "...", "api_version": "v55.0" }
      9     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     10     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     11 );
     12 CREATE INDEX idx_connectors_tenant ON connector_definitions(tenant_id);
     13 CREATE INDEX idx_connectors_type ON connector_definitions(tenant_id, type, is_active);

    connector_configs table
      1 CREATE TABLE connector_configs (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     connector_id    UUID NOT NULL,
      5     config          JSONB NOT NULL,                   -- encrypted secrets go here (API keys, tokens)
      6     auth_status     VARCHAR(20) NOT NULL DEFAULT 'pending'
      7                   CHECK (auth_status IN ('pending', 'authenticated', 'failed', 'expired')),
      8     auth_error      TEXT,
      9     last_auth_check TIMESTAMPTZ,
     10     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     11     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     12 );
     13 CREATE INDEX idx_configs_tenant ON connector_configs(tenant_id);

    connector_tools table
      1 CREATE TABLE connector_tools (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     connector_id    UUID NOT NULL,
      5     tool_name       VARCHAR(100) NOT NULL,
      6     description     TEXT NOT NULL,
      7     parameters      JSONB NOT NULL DEFAULT '{}',      -- { "type": "object", "properties": {...}, "required": [...] }
      8     return_type     VARCHAR(50) NOT NULL DEFAULT 'json',
      9     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     10 );
     11 CREATE INDEX idx_tools_connector ON connector_tools(connector_id);

    OpenAPI Endpoints


    ┌────────┬───────────────────────────────┬────────────────────┬────────────────────────────────────────────────────────┐
    │ Method │ Path                          │ Handler            │ Description                                            │
    ├────────┼───────────────────────────────┼────────────────────┼────────────────────────────────────────────────────────┤
    │ POST   │ /v1/connectors                │ CreateConnector    │ Register a connector definition                        │
    ├────────┼───────────────────────────────┼────────────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/connectors                │ ListConnectors     │ List all connector types available                     │
    ├────────┼───────────────────────────────┼────────────────────┼────────────────────────────────────────────────────────┤
    │ POST   │ /v1/connectors/{id}/configure │ ConfigureConnector │ Configure a connector with tenant-specific credentials │
    ├────────┼───────────────────────────────┼────────────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/connectors/{id}/health    │ CheckHealth        │ Check connector connectivity and auth status           │
    ├────────┼───────────────────────────────┼────────────────────┼────────────────────────────────────────────────────────┤
    │ GET    │ /v1/connectors/{id}/tools     │ ListTools          │ List available tools for this connector                │
    └────────┴───────────────────────────────┴────────────────────┴────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.connector.connector_configured — { tenant_id, connector_id, type }
     - operan.connector.connector_healthy — { tenant_id, connector_id, check_duration_ms }
     - operan.connector.connector_auth_failed — { tenant_id, connector_id, error }

    Integration Points
     - M02 IAM: JWT validation, tenant isolation. Only users with connector_admin role can configure connectors.
     - M04 Agent Registry: Connector tools are auto-registered in M04 when a connector is configured. M18 calls M04's /registry/agents/{agent_id}/tools to add connector tools to the agent's tool list.
     - M08 Tool Execution: When an agent executes a connector tool (e.g., send_email via SMTP), M08 calls M18's connector to actually perform the action. M18's connector returns the result, which M08
       records.

    Implementation Notes
     - Connector Pattern: Each connector type implements a common interface:
     1   type Connector interface {
     2       Name() string
     3       HealthCheck(ctx context.Context) (status string, err error)
     4       Execute(toolName string, params map[string]interface{}) (result map[string]interface{}, err error)
     5   }
     - SMTP Connector: Implements send_email tool. Uses net/smtp. Parameters: to, from, subject, body, html, attachments.
     - Salesforce Connector: Implements create_record, update_record, query_records. Uses Salesforce REST API (/services/data/v55.0/...). Parameters configurable per object.
     - HubSpot Connector: Similar to Salesforce, uses HubSpot CRM API (/crm/v3/objects/...).
     - Microsoft 365 Connector: Uses Microsoft Graph API (/api/v1.0/...). Supports send_email, create_calendar_event, upload_file.
     - SAP Connector: Uses SAP OData/REST API. Supports create_material, create_sales_order, query_inventory.
     - Generic REST Connector: Configurable HTTP client. User defines method, URL template, headers, body template, auth method.
     - Security: Connector configs contain secrets. Store in connector_configs.config as encrypted JSON. In a real deployment, use K8s secrets or HashiCorp Vault.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 35 tests)
     - Connector CRUD: create, list, configure, health check, list tools
     - SMTP: send valid email, send with invalid credentials (401), send with missing fields (400)
     - Salesforce: health check with mock server, query records, create record
     - HubSpot: health check, create contact
     - M365: health check, send email
     - Generic REST: configure custom URL, test connection
     - Integration: connector tool registered in M04 (mock M04)
     - Middleware: JWT validation, tenant isolation, RBAC
     - Kafka: all 3 event types

    Deliverables
    Build passes, 35+ tests, OpenAPI-compliant, Helm chart, README.
     1 
     2 ---
     3 
     4 # MODULE 17 — Cost Governance Engine
     5 
     6 **Dependency:** M10 (Policy Governance — for policy validation), M08 (Tool Execution — for cost data)
    You are implementing Module 17 — Cost Governance Engine for the Operan platform. This module prevents runaway costs by enforcing budgets, throttling high-cost operations, and providing cost
    visibility.

    Purpose
    Agents can burn through API costs rapidly. M17 provides budget enforcement, real-time cost tracking, and automated throttling to ensure agents don't exceed department or tenant budgets.

    Key Concepts
     - Budgets: Department-level, agent-level, or project-level spending limits
     - Throttling: Automatically slow down or block expensive operations when approaching limits
     - Alerting: Notify managers when budgets hit thresholds (75%, 90%, 100%)
     - Cost Allocation: Charge costs back to departments/agents for visibility

    Files to Create

    Directory Structure
      1 modules/17-cost-governance/
      2 ├── go.mod
      3 ├── main.go                    # Listen port 8017
      4 ├── Dockerfile
      5 ├── chart/
      6 └── internal/
      7     ├── config/
      8     │   └── config.go
      9     ├── ctxkeys/
     10     │   └── ctxkeys.go
     11     ├── middleware/
     12     │   └── middleware.go
     13     ├── handler/
     14     │   ├── budgets.go         # CRUD budgets
     15     │   ├── tracking.go        # Record/get cost events
     16     │   ├── throttling.go      # Check/throttle cost operations
     17     │   ├── alerts.go          # List/resolve cost alerts
     18     │   └── router.go
     19     ├── store/
     20     │   ├── budgets.go         # PostgreSQL
     21     │   ├── tracking.go        # PostgreSQL
     22     │   ├── alerts.go          # PostgreSQL
     23     │   └── models.go
     24     └── events/
     25         └── events.go          # Kafka: budget_threshold_reached, cost_recorded, throttled_operation

    Database Schema

    cost_budgets table
      1 CREATE TABLE cost_budgets (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     name            VARCHAR(255) NOT NULL,
      5     scope_type      VARCHAR(20) NOT NULL CHECK (scope_type IN ('tenant', 'department', 'agent', 'project')),
      6     scope_id        VARCHAR(255) NOT NULL,
      7     amount          FLOAT NOT NULL,
      8     currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
      9     period          VARCHAR(20) NOT NULL CHECK (period IN ('daily', 'weekly', 'monthly', 'quarterly', 'yearly')),
     10     soft_limit      FLOAT NOT NULL DEFAULT 0.75,       -- 75% threshold for warning
     11     hard_limit      FLOAT NOT NULL DEFAULT 1.0,        -- 100% threshold for block
     12     is_active       BOOLEAN NOT NULL DEFAULT true,
     13     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     14     updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     15 );
     16 CREATE INDEX idx_budgets_tenant ON cost_budgets(tenant_id);
     17 CREATE INDEX idx_budgets_scope ON cost_budgets(tenant_id, scope_type, scope_id);

    cost_events table
      1 CREATE TABLE cost_events (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     agent_id        VARCHAR(255),
      5     workflow_id     VARCHAR(255),
      6     department_id   VARCHAR(255),
      7     source          VARCHAR(50) NOT NULL,              -- model_call | tool_execution | storage | compute
      8     source_id       VARCHAR(255),
      9     amount          FLOAT NOT NULL,
     10     currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
     11     period          VARCHAR(20) NOT NULL,              -- matches budget period
     12     recorded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
     13 );
     14 CREATE INDEX idx_events_tenant ON cost_events(tenant_id);
     15 CREATE INDEX idx_events_period ON cost_events(tenant_id, period, recorded_at);

    cost_alerts table
      1 CREATE TABLE cost_alerts (
      2     id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      3     tenant_id       VARCHAR(255) NOT NULL,
      4     budget_id       UUID NOT NULL,
      5     threshold       FLOAT NOT NULL,                  -- 0.75, 0.9, 1.0
      6     message         TEXT NOT NULL,
      7     status          VARCHAR(20) NOT NULL DEFAULT 'active'
      8                   CHECK (status IN ('active', 'acknowledged', 'resolved')),
      9     resolved_at     TIMESTAMPTZ,
     10     resolved_by     VARCHAR(255),
     11     created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
     12 );
     13 CREATE INDEX idx_alerts_tenant ON cost_alerts(tenant_id);

    OpenAPI Endpoints


    ┌────────┬──────────────────────────────┬────────────────┬─────────────────────────────────────────────────────────────────────────────────┐
    │ Method │ Path                         │ Handler        │ Description                                                                     │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/cost-budgets             │ CreateBudget   │ Create a budget with soft/hard limits                                           │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/cost-budgets             │ ListBudgets    │ Paginated list                                                                  │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/cost-budgets/{id}        │ GetBudget      │ Budget details with current usage                                               │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/cost-budgets/{id}        │ UpdateBudget   │ Update budget amounts                                                           │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ DELETE │ /v1/cost-budgets/{id}        │ DeleteBudget   │ Soft delete                                                                     │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/cost/tracking            │ RecordCost     │ Record a cost event                                                             │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/cost/tracking            │ ListCostEvents │ Paginated cost history, filter by agent/department/source                       │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/cost/budget-usage        │ GetBudgetUsage │ Current spending vs budget for all budgets                                      │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ POST   │ /v1/cost/throttle            │ CheckThrottle  │ Ask if an operation should be throttled (returns allow/deny + remaining budget) │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ GET    │ /v1/cost/alerts              │ ListAlerts     │ Active cost alerts                                                              │
    ├────────┼──────────────────────────────┼────────────────┼─────────────────────────────────────────────────────────────────────────────────┤
    │ PATCH  │ /v1/cost/alerts/{id}/resolve │ ResolveAlert   │ Acknowledge or resolve an alert                                                 │
    └────────┴──────────────────────────────┴────────────────┴─────────────────────────────────────────────────────────────────────────────────┘

    Kafka Events

     - operan.cost.budget_threshold_reached — { tenant_id, budget_id, threshold, current_spend }
     - operan.cost.cost_recorded — { tenant_id, agent_id, source, amount, currency, period }
     - operan.cost.throttled_operation — { tenant_id, agent_id, operation, reason, remaining_budget }

    Integration Points
     - M10 Policy Governance: Before recording a cost event that exceeds the budget, M17 calls M10's /v1/evaluate to check if the cost is within policy limits. If a policy says "no single tool execution
        over $50", M17 blocks it.
     - M08 Tool Execution: M08 calls POST /v1/cost/throttle BEFORE executing expensive tools. If throttle says deny, M08 returns 429 to the caller. After execution, M08 calls POST /v1/cost/tracking to
       record the actual cost.
     - M12 Model Abstraction: M12 publishes model_cost_recorded events to Kafka, which M17 consumes to automatically track model costs.

    Implementation Notes
     - Period Rolling: Budgets have a period (daily/weekly/monthly). Calculate current period spending by filtering cost_events where recorded_at is within the current period window.
     - Throttle Logic: CheckThrottle endpoint:
       1. Find active budget for the agent/tenant
       2. Calculate current period spend
       3. If spend >= hard_limit → deny (429)
       4. If spend >= soft_limit → warn but allow
       5. If spend < soft_limit → allow
       6. Return { allowed: bool, remaining: float, spend_to_date: float, threshold: float }
     - Cost Aggregation: M17 subscribes to operan.model.model_cost_recorded (from M12) and operan.tool-execution.cost_recorded (from M08) to automatically track costs without requiring callers to
       explicitly record.
     - Fail-closed: Refuse to start if JWT_SECRET is unset. Log-only Kafka fallback.

    Tests (minimum 30 tests)
     - Budget CRUD: create, list, get, update, delete
     - Cost tracking: record event, list events, filter by agent/source
     - Throttle: below soft limit (allow), between soft/hard (warn+allow), above hard (deny), no budget (allow)
     - Alerts: threshold reached (75%, 90%, 100%), resolve alert
     - Budget usage: calculate current period spend, rolling window
     - Integration: M10 policy check (mock M10), M08 throttle integration
     - Middleware: JWT validation, tenant isolation
     - Kafka: all 3 event types

    Deliverables
    Build passes, 30+ tests, OpenAPI-compliant, Helm chart, README.

      1 
      2 ---
      3 
      4 That's all 10. Summary:
      5 
      6 | # | Module | Port | Depends On | Est. Complexity |
      7 |---|--------|------|-----------|-----------------|
      8 | 1 | **M10** Policy Governance | 8010 | — | Medium |
      9 | 2 | **M16** Execution Sandbox | 8016 | M04 | Medium |
     10 | 3 | **M12** Model Abstraction | 8012 | M02 | Medium-High |
     11 | 4 | **M13** Multi-Model Routing | 8013 | M02, M12 | Medium |
     12 | 5 | **M19** Arabic Language | 8019 | M02, M07 | Medium |
     13 | 6 | **M06** Knowledge Ingestion | 8006 | M02, M07, M19 | High |
     14 | 7 | **M14** Agent Collaboration | 8014 | M02, M03, M07 | Medium |
     15 | 8 | **M15** Agent Marketplace | 8015 | M02, M04 | Medium |
     16 | 9 | **M18** Enterprise Connectors | 8018 | M02, M04, M08 | High |
     17 | 10 | **M17** Cost Governance | 8017 | M10, M08 | Medium |
     18 
     19 Each prompt is self-contained: full DB schema, OpenAPI endpoints, Kafka events, integration points, tests, and deliverables. The coding agent should be able to implement each from scratch.
     20 
     21 Ready when you are — start with M10.