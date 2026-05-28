# Module 03 — Agent Orchestration Engine

**Event-driven DAG execution with multi-stack orchestration: LangGraph, Temporal, Ray, and Celery/Kafka.**

---

## Purpose

The Agent Orchestration Engine is Operan's workflow execution core. It orchestrates multi-agent workflows as directed acyclic graphs (DAGs), manages agent assignment and scheduling, handles state checkpointing and recovery, and provides retry/timeout/escalation logic. This module is the central nervous system that coordinates all agent activity across the platform.

## Multi-Stack Architecture

Operan's orchestration engine leverages four specialized orchestration stacks, each handling a distinct concern:

### LangGraph — DAG-Based Workflow Definition

LangGraph provides the primary workflow graph definition and execution engine:

- **Graph-as-code**: Define workflows as Python/Go nodes and edges with explicit state management
- **Stateful graphs**: Maintain and evolve graph state across node executions
- **Conditional routing**: Dynamic edge traversal based on state predicates
- **Human-in-the-loop**: Pause at nodes for human review/approval
- **Streaming outputs**: Real-time state updates from running graphs

**Key Schemas**: `LangGraphDefinition`, `LangGraphNode`, `LangGraphEdge`, `LangGraphCheckpoint`

**Endpoints**: `/stack/langgraph/graphs` (CRUD), `/workflows` (DAG execution)

### Temporal — Durable Workflow Execution

Temporal provides guaranteed-once execution semantics with automatic checkpointing:

- **Durable execution**: Workflows survive process crashes, restarts, deployments
- **Automatic checkpointing**: State persisted at every workflow step
- **Workflow replay**: Re-execute from any checkpoint for debugging/recovery
- **Sagas/compensations**: Distributed transaction patterns with rollback
- **Timeouts and schedules**: Cron-like scheduling, workflow-level timeouts

**Key Schemas**: `TemporalWorkflowDefinition`, `TemporalCheckpointConfig`, `TemporalReplayConfig`

**Endpoints**: `/stack/temporal/workflows` (CRUD), `/stack/temporal/checkpoints`, `/workflows/{id}/replay`

### Ray — Distributed Execution Scaling

Ray provides horizontal scaling for compute-intensive orchestration tasks:

- **Distributed task execution**: Split graph nodes across worker pools
- **Auto-scaling**: Dynamically adjust worker count based on queue depth
- **Resource-aware scheduling**: CPU, GPU, memory constraints per task
- **Fault-tolerant workers**: Automatic worker replacement on failure
- **Cross-cluster orchestration**: Multi-cluster Ray deployment support

**Key Schemas**: `RayExecutionConfig`, `RayDistributedTask`, `RayWorkerPool`

**Endpoints**: `/stack/ray/pools` (CRUD + scale), `/workflows/{id}/execute/distributed`

### Celery/Kafka — Async Task Queuing and Event Streaming

Celery with Kafka provides the async event bus for decoupled orchestration:

- **Async task queues**: Decouple workflow triggers from execution
- **Event sourcing**: Every state change published as immutable event
- **Priority queues**: Critical workflows get priority processing
- **Dead letter queues**: Failed tasks captured for retry/review
- **Event replay**: Re-process events for audit/recovery

**Key Schemas**: `CeleryTaskQueue`, `CeleryConsumerConfig`

**Endpoints**: `/stack/celery/queues` (CRUD + publish), `/workflows/{id}/execute/async`

## Runtime Model

```
┌─────────────────────────────────────────────────────────────────┐
│                    Agent Orchestration Engine                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │  LangGraph   │    │   Temporal   │    │    Celery/Kafka  │  │
│  │  (DAG Def)   │───▶│  (Durability)│    │   (Event Bus)    │  │
│  └──────────────┘    └──────────────┘    └──────────────────┘  │
│         │                     │                     │           │
│         │         ┌──────────────────┐                │           │
│         └────────▶│      Ray         │◀───────────────┘           │
│                   │  (Distributed    │                           │
│                   │   Execution)     │                           │
│                   └──────────────────┘                           │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Workflow Execution Pipeline                  │   │
│  │  DAG Parsing → Node Scheduling → Agent Assignment →      │   │
│  │  Execution (Local/Distributed) → Checkpointing → Event   │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Key Capabilities

### Workflow Orchestration
- **DAG-based execution**: Define complex multi-agent workflows as directed acyclic graphs
- **Conditional branching**: Dynamic execution paths based on runtime state
- **Parallel execution**: Concurrent node execution where dependencies allow
- **Sequential pipelines**: Linear execution chains for step-by-step workflows
- **Sub-workflow composition**: Nest workflows within workflows for modularity

### Agent Management
- **Dynamic agent assignment**: Assign agents to workflow nodes at runtime
- **Agent availability**: Real-time agent health monitoring and auto-reassignment
- **Skill-based routing**: Route tasks to agents based on capability matching
- **Load balancing**: Distribute work across available agents evenly

### State Management
- **Automatic checkpointing**: Persist workflow state at defined intervals
- **State versioning**: Track and compare state changes over time
- **Checkpoint recovery**: Restore from any previous checkpoint
- **State streaming**: Real-time state updates via Kafka event stream

### Fault Tolerance
- **Automatic retry**: Configurable retry with exponential backoff
- **Timeout handling**: Node-level and workflow-level timeouts
- **Circuit breaking**: Prevent cascading failures across agents
- **Dead letter queues**: Capture and isolate failed tasks for review
- **Workflow replay**: Re-execute workflows from checkpoints for debugging

### Observability
- **Execution traces**: Full end-to-end visibility into workflow execution
- **Node-level metrics**: Per-node timing, success rate, error rates
- **Agent utilization**: Track agent workload and throughput
- **Event audit log**: Immutable event log for compliance and debugging

## API Endpoints

### Workflow Management

| Method   | Endpoint                                              | Description                          |
|----------|-------------------------------------------------------|--------------------------------------|
| POST     | `/api/v1/workflows`                                   | Create workflow (LangGraph DAG)      |
| GET      | `/api/v1/workflows`                                   | List workflows                       |
| GET      | `/api/v1/workflows/{id}`                              | Get workflow details                 |
| POST     | `/api/v1/workflows/{id}/start`                        | Start workflow execution             |
| POST     | `/api/v1/workflows/{id}/pause`                        | Pause workflow execution             |
| POST     | `/api/v1/workflows/{id}/resume`                       | Resume paused workflow               |
| POST     | `/api/v1/workflows/{id}/cancel`                       | Cancel running workflow              |
| GET      | `/api/v1/workflows/{id}/status`                       | Get execution status                 |
| GET      | `/api/v1/workflows/{id}/state`                        | Get current state                    |
| POST     | `/api/v1/workflows/{id}/replay`                       | Replay from checkpoint               |
| DELETE   | `/api/v1/workflows/{id}`                              | Delete workflow                      |

### Distributed Execution

| Method   | Endpoint                                              | Description                          |
|----------|-------------------------------------------------------|--------------------------------------|
| POST     | `/api/v1/workflows/{id}/execute/distributed`          | Execute on Ray cluster               |
| POST     | `/api/v1/workflows/{id}/execute/async`                | Queue via Celery/Kafka               |
| GET      | `/api/v1/workflows/{id}/execution/{exec_id}`          | Get execution result                 |
| POST     | `/api/v1/workflows/{id}/execution/{exec_id}/cancel`   | Cancel execution                     |

### Agent Management

| Method   | Endpoint                                              | Description                          |
|----------|-------------------------------------------------------|--------------------------------------|
| POST     | `/api/v1/workflows/{id}/assign`                       | Assign agent to node                 |
| GET      | `/api/v1/workflows/{id}/assignments`                  | List all assignments                 |
| POST     | `/api/v1/agents/{id}/status`                          | Update agent status                  |
| GET      | `/api/v1/agents/{id}`                                 | Get agent details                    |

### Scheduling

| Method   | Endpoint                                              | Description                          |
|----------|-------------------------------------------------------|--------------------------------------|
| POST     | `/api/v1/schedules`                                   | Create schedule                      |
| GET      | `/api/v1/schedules`                                   | List schedules                       |
| POST     | `/api/v1/schedules/{id}/trigger`                      | Trigger schedule immediately         |
| DELETE   | `/api/v1/schedules/{id}`                              | Delete schedule                      |

### Human Tasks

| Method   | Endpoint                                              | Description                          |
|----------|-------------------------------------------------------|--------------------------------------|
| POST     | `/api/v1/human-tasks`                                 | Create human task                    |
| GET      | `/api/v1/human-tasks`                                 | List human tasks                     |
| POST     | `/api/v1/human-tasks/{id}/approve`                    | Approve human task                   |
| POST     | `/api/v1/human-tasks/{id}/reject`                     | Reject human task                    |

### Stack Management

| Method   | Endpoint                                              | Description                          |
|----------|-------------------------------------------------------|--------------------------------------|
| GET      | `/api/v1/stack/health`                                | Get all stack health status          |
| GET      | `/api/v1/stack/langgraph/graphs`                      | List LangGraph graphs                |
| POST     | `/api/v1/stack/langgraph/graphs`                      | Register LangGraph definition        |
| GET      | `/api/v1/stack/langgraph/graphs/{id}`                 | Get graph definition                 |
| PUT      | `/api/v1/stack/langgraph/graphs/{id}`                 | Update graph definition              |
| DELETE   | `/api/v1/stack/langgraph/graphs/{id}`                 | Unregister graph                     |
| GET      | `/api/v1/stack/temporal/workflows`                    | List Temporal workflows              |
| POST     | `/api/v1/stack/temporal/workflows`                    | Register Temporal workflow           |
| GET      | `/api/v1/stack/temporal/workflows/{id}`               | Get workflow details                 |
| PUT      | `/api/v1/stack/temporal/workflows/{id}`               | Update workflow                      |
| DELETE   | `/api/v1/stack/temporal/workflows/{id}`               | Unregister workflow                  |
| POST     | `/api/v1/stack/temporal/checkpoints`                  | Create checkpoint                    |
| GET      | `/api/v1/stack/temporal/checkpoints/{id}`             | Get checkpoint                       |
| GET      | `/api/v1/stack/ray/pools`                             | List worker pools                    |
| POST     | `/api/v1/stack/ray/pools`                             | Create worker pool                   |
| GET      | `/api/v1/stack/ray/pools/{id}`                        | Get pool details                     |
| PUT      | `/api/v1/stack/ray/pools/{id}`                        | Update pool configuration            |
| DELETE   | `/api/v1/stack/ray/pools/{id}`                        | Delete pool                          |
| POST     | `/api/v1/stack/ray/pools/{id}/scale`                  | Scale pool workers                   |
| GET      | `/api/v1/stack/celery/queues`                         | List task queues                     |
| POST     | `/api/v1/stack/celery/queues`                         | Create task queue                    |
| GET      | `/api/v1/stack/celery/queues/{id}`                    | Get queue details                    |
| PUT      | `/api/v1/stack/celery/queues/{id}`                    | Update queue configuration           |
| DELETE   | `/api/v1/stack/celery/queues/{id}`                    | Delete queue                         |
| POST     | `/api/v1/stack/celery/queues/{id}/publish`            | Publish task to queue                |

## Event Architecture (AsyncAPI)

### Kafka Topic Schema

```
operan.orchestration.{stack}.{entity}.{event}

Examples:
  operan.orchestration.langgraph.graph.registered
  operan.orchestration.langgraph.state.updated
  operan.orchestration.temporal.checkpoint.created
  operan.orchestration.temporal.workflow.replayed
  operan.orchestration.ray.task.submitted
  operan.orchestration.ray.task.completed
  operan.orchestration.celery.task.published
  operan.orchestration.celery.task.completed
  operan.orchestration.stack.health
```

### Event Categories

| Stack      | Events                                                  |
|------------|---------------------------------------------------------|
| LangGraph  | `graph.registered`, `state.updated`, `graph.deployed`   |
| Temporal   | `workflow.registered`, `checkpoint.created`, `workflow.replayed` |
| Ray        | `worker.pooled`, `task.submitted`, `task.completed`, `worker.status` |
| Celery     | `queue.created`, `task.published`, `task.consumed`, `task.completed`, `worker.heartbeat` |
| Stack      | `health` (periodic health report across all stacks)     |

### Legacy Events (Backward Compatible)

```
operan.events.orchestration.workflow.{created, started, paused, resumed, completed, failed, cancelled, checkpointed, replayed}
operan.events.orchestration.schedule.triggered
operan.events.orchestration.agent.{assigned, unavailable, online, offline}
operan.events.orchestration.escalation.{created, acknowledged, resolved}
operan.events.orchestration.retry.{requested, completed}
operan.events.orchestration.priority.changed
operan.events.orchestration.delegation.{created, completed}
operan.events.orchestration.node.{started, completed, failed}
```

## Critical Risks

| Risk                     | Mitigation                                               |
|--------------------------|----------------------------------------------------------|
| Workflow deadlock        | Cycle detection in DAG validation, timeout enforcement   |
| State corruption         | Temporal durability, atomic checkpoint commits           |
| Agent starvation         | Priority queues, fair scheduling, starvation detection   |
| Cascading failures       | Circuit breakers, bulkhead isolation, rate limiting      |
| Data inconsistency       | Event sourcing, idempotent handlers, exactly-once semantics |
| Performance degradation  | Ray auto-scaling, connection pooling, query optimization |
| Event loss               | Kafka durability, consumer offsets, dead letter queues   |
| Orchestration vendor lock-in | Abstraction layer, multi-backend support, migration tools |

## Module Dependencies

- **Module 01** — Tenant Control Plane (tenant context, RBAC)
- **Module 02** — Identity & Access (agent authentication)
- **Module 04** — Agent Registry (agent capabilities, availability)
- **Module 05** — Department Template Engine (workflow templates)
- **Module 06** — Knowledge Ingestion (knowledge retrieval during execution)
- **Module 07** — Memory Fabric (state persistence, context memory)
- **Module 08** — Tool Execution Layer (tool calls from workflow nodes)
- **Module 09** — Human Supervision (human-in-the-loop approvals)
- **Module 10** — Policy & Governance Engine (policy validation)
- **Module 11** — Observability (metrics, tracing, logging)
- **Module 17** — Cost Governance Engine (cost tracking per workflow)
- **Module 18** — Enterprise Connector Fabric (external system integration)

## Related Artifacts

- `contracts/v1/openapi-03-agent-orchestration.yaml` — OpenAPI specification
- `contracts/v1/asyncapi-03-agent-orchestration.yaml` — AsyncAPI event specification
- `contracts/v1/schema-03-agent-orchestration.json` — JSON Schema definitions
