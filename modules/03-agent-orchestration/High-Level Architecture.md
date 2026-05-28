┌─────────────────────────────────────────────────────┐
│                 API Gateway Layer                    │
│  Base: /api/v1/orchestration                         │
│  Security: BearerAuth (JWT) + X-Tenant-ID header    │
│  54 OpenAPI operations across 18 path groups        │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Middleware Chain                     │
│  • JWTAuth (RSA via JWKS + HMAC fallback)           │
│  • TenantContext (X-Tenant-ID extraction)           │
│  • TraceID / RequestID / Logger                     │
│  • Typed context keys (TenantIDKey, UserIDKey, etc.)│
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Handler Layer                        │
│  • Workflow CRUD + state/checkpoint/replay/vars     │
│  • Schedule CRUD + trigger/pause/resume             │
│  • Agent assign/availability/delegate/list          │
│  • Escalations, Retries, Nodes, Results             │
│  • Stack management (LangGraph/Temporal/Ray/Celery) │
│  • Pipelines, Executions, Human Tasks               │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Store Layer                          │
│  • Repository pattern interface (for persistence)   │
│  • In-memory cache layer (MVP)                      │
│  • Tenant-isolated maps with sync.RWMutex           │
│  • Target: PostgreSQL/Redis for production          │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│            Execution Stack Abstraction               │
│  ┌─────────────┬─────────────┬─────────────┐        │
│  │ LangGraph   │ Temporal    │ Ray         │        │
│  │ (DAG flows) │ (long-run)  │ (dist. compute)│     │
│  └─────────────┴─────────────┴─────────────┘        │
│  • Stack-aware routing via workflow.execution_stack │
│  • Client wrappers per stack (Create/Get/Update/Delete)│
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│                 Event Layer (AsyncAPI)               │
│  • 37 channels, topic format:                        │
│    operan.orchestration.{stack}.{entity}.{event}    │
│  • Events: workflow.*, agent.*, node.*, escalation.*,│
│    retry.*, pipeline.*, human_task.*                │
│  • Publisher with typed methods + correlationId     │
└─────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────┐
│            Observability & Configuration             │
│  • Config: 8 env vars (LISTEN_ADDR, OTLP_ENDPOINT,  │
│    EVENT_BUS_*, JWT_SECRET, etc.)                   │
│  • OpenTelemetry traces/metrics to OTLP collector   │
│  • Structured JSON logging with trace_id/request_id │
└─────────────────────────────────────────────────────┘