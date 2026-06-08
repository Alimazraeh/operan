# Module 14 — Workflow Automation Engine

**Workflow orchestration.**

---

## Purpose

The Workflow Automation Engine provides the orchestration layer for complex, multi-step business processes in the Operan ecosystem. It supports pipeline design, stateful execution, scheduling, human-in-the-loop workflows, and comprehensive error handling with retry logic.

## Core Functions

### Pipeline Design
Visual and programmatic workflow creation:

| Step Type      | Description                            | Use Case                        |
|---------------|----------------------------------------|---------------------------------|
| api_call      | HTTP API invocation                    | External service integration    |
| agent_task    | Agent processing step                  | AI-powered decision making      |
| data_transform| Data transformation                    | Schema mapping, enrichment      |
| condition     | Conditional branching                  | Business logic routing          |
| delay         | Time-based pause                       | SLA waits, scheduled processing |
| human_approval| Human-in-the-loop                    | Approval workflows              |
| parallel      | Concurrent execution                   | Multi-threaded processing       |
| foreach       | Iteration                              | Batch processing                |
| webhook       | Event-driven trigger                   | External event integration      |
| code          | Custom code execution                  | Complex business logic          |
| notification  | Alert sending                          | Email, Slack, SMS notifications |

- **Step orchestration** — Define execution order with dependencies
- **Conditional routing** — Dynamic branching based on data
- **Parallel execution** — Fan-out to concurrent branches
- **Input/output passing** — Data flow between steps

### Execution Engine
Stateful workflow processing:

- **Real-time execution** — Stream-based step processing
- **Checkpointing** — Persist execution state for recovery
- **State machine** — Track execution lifecycle
- **Step-level visibility** — Monitor progress per step
- **Execution history** — Full audit trail

### Error Handling & Recovery
Robust failure management:

| Strategy   | Behavior                                   | Use Case                     |
|-----------|--------------------------------------------|------------------------------|
| fail      | Stop execution on error                    | Critical failures            |
| retry     | Retry with backoff                         | Transient failures           |
| skip      | Skip failed step and continue              | Non-critical operations      |
| abort     | Terminate entire pipeline                  | Unrecoverable errors         |
| branch    | Redirect to alternative path               | Fallback workflows           |

- **Exponential backoff** — Configurable retry strategy
- **Dead letter queue** — Persist failed executions
- **Alert integration** — Notify on critical failures
- **Partial completion** — Continue where possible

### Scheduling
Automated workflow execution:

- **Cron expressions** — Standard scheduling syntax
- **Recurring executions** — Daily, weekly, monthly patterns
- **Timezone support** — Localized scheduling
- **Concurrent run control** — Limit parallel executions
- **Trigger management** — Schedule management dashboard

### Human-in-the-Loop
Interactive workflow approval:

| Task Type     | Description                          | Outcome                        |
|--------------|--------------------------------------|--------------------------------|
| approval     | Approve or reject                    | Continue or terminate          |
| rejection    | Explicit decline                     | Terminate with reason          |
| input        | Collect additional information       | Enrich execution context       |
| review       | Assess output quality                | Approve or return for revision |
| confirmation | Verify action before proceeding      | Confirm or cancel              |

- **Timeout handling** — Auto-expire pending tasks
- **Role-based assignment** — Assign to users, roles, groups
- **Context preservation** — Show workflow state for decision
- **Response tracking** — Audit trail of human decisions

## Key Capabilities

- **Multi-trigger support** — Manual, event, schedule, webhook, API
- **Variable injection** — Parametric pipelines with dynamic inputs
- **Versioning** — Track pipeline changes across versions
- **Metrics collection** — Success rate, duration, step analytics
- **Tenant isolation** — Complete separation of workflow resources
- **Execution pause** — Pause and resume workflows mid-execution

## API Endpoints

| Method   | Endpoint                                             | Description                        |
|----------|------------------------------------------------------|------------------------------------|
| POST     | `/api/v1/workflows/pipelines`                        | Create pipeline                    |
| GET      | `/api/v1/workflows/pipelines`                        | List pipelines                     |
| GET      | `/api/v1/workflows/pipelines/{id}`                   | Get pipeline details               |
| PATCH    | `/api/v1/workflows/pipelines/{id}`                   | Update pipeline                    |
| DELETE   | `/api/v1/workflows/pipelines/{id}`                   | Delete pipeline                    |
| POST     | `/api/v1/workflows/pipelines/{id}/activate`          | Activate pipeline                  |
| POST     | `/api/v1/workflows/pipelines/{id}/deactivate`        | Deactivate pipeline                |
| POST     | `/api/v1/workflows/pipelines/{id}/execute`           | Execute pipeline                   |
| GET      | `/api/v1/workflows/pipelines/{id}/executions`        | List executions                    |
| GET      | `/api/v1/workflows/executions/{execution_id}`        | Get execution details              |
| POST     | `/api/v1/workflows/executions/{execution_id}/cancel` | Cancel execution                   |
| POST     | `/api/v1/workflows/executions/{execution_id}/retry`  | Retry execution                    |
| GET      | `/api/v1/workflows/executions/{execution_id}/steps`  | Get execution steps                |
| POST     | `/api/v1/workflows/schedules`                        | Create schedule                    |
| GET      | `/api/v1/workflows/schedules`                        | List schedules                     |
| PATCH    | `/api/v1/workflows/schedules/{id}`                   | Update schedule                    |
| DELETE   | `/api/v1/workflows/schedules/{id}`                   | Delete schedule                    |
| POST     | `/api/v1/workflows/human-tasks`                      | Create human task                  |
| GET      | `/api/v1/workflows/human-tasks`                      | List human tasks                   |
| POST     | `/api/v1/workflows/human-tasks/{task_id}/respond`    | Respond to human task              |
| GET      | `/api/v1/workflows/analytics`                        | Get analytics                      |
| GET      | `/api/v1/workflows/health`                           | Health check                       |

## Integration with Modules

All modules use workflow automation:

1. **Module 06** — Data Ingestion pipelines for batch processing
2. **Module 07** — Memory update workflows with human review
3. **Module 08** — Tool orchestration workflows
4. **Module 09** — Supervision analysis pipelines
5. **Module 10** — Policy enforcement workflows
6. **Module 11** — Anomaly investigation pipelines
7. **Module 13** — Messaging engine triggers workflows via events

## Pipeline Example

```yaml
name: "Customer Data Enrichment Pipeline"
trigger_type: "event"
steps:
  - id: "extract"
    type: "api_call"
    config:
      url: "/api/v1/data/customers/{id}"
      method: "GET"
  
  - id: "enrich"
    type: "agent_task"
    config:
      task: "data_analyze"
      model: "gpt-4o"
  
  - id: "review"
    type: "human_approval"
    config:
      assignee_type: "role"
      assignee_id: "data-reviewer"
      task_type: "approval"
  
  - id: "save"
    type: "api_call"
    config:
      url: "/api/v1/data/enriched"
      method: "POST"
    condition: "review.result == 'approved'"
```

## Critical Risks

| Risk                        | Mitigation                                  |
|-----------------------------|---------------------------------------------|
| Pipeline failure loss       | Dead letter queue, retry, checkpointing     |
| Orphaned human tasks        | Timeout handling, auto-escalation           |
| Infinite retry loops        | Max retry limits, exponential backoff       |
| Schedule collision          | Concurrent run limits, queue management     |
| State corruption            | Checkpoint validation, recovery procedures  |

## Module Dependencies

- **Module 04** — Agent Engine (agent_task steps)
- **Module 06** — Data Ingestion (data processing)
- **Module 10** — Policy & Governance (policy checks)
- **Module 13** — Messaging Engine (event triggers)
- **Module 15** — Knowledge Base (knowledge workflows)

## Related Artifacts

- `contracts/v1/openapi-14-agent-collaboration.yaml` — OpenAPI specification
- `contracts/v1/schema-14-agent-collaboration.json` — JSON Schema definitions
