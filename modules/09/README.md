# Module 09 — Human Supervision Layer

**Govern autonomous execution safely.**

---

## Purpose

The Human Supervision Layer is the critical safety valve between autonomous agent action and real-world impact. It provides structured mechanisms for human oversight, intervention, and course correction without stifling agent productivity.

## Core Functions

### Approval Workflows
Structured governance for high-risk agent actions through four approval models:

| Model             | Description                                      | Use Case                        |
|------------------|--------------------------------------------------|---------------------------------|
| Sequential       | One approver after another must agree            | Multi-department financial ops  |
| Parallel         | Multiple approvers simultaneously                | Cross-functional policy changes |
| Conditional      | Logic-gated approvers based on context           | Conditional payment thresholds  |
| Threshold        | Majority or percentage-based consensus           | Team-wide process changes       |

### Escalations
Automated and manual escalation paths when agents encounter conditions beyond their scope:

- **Severity levels**: Low → Medium → High → Critical → P0
- **Categories**: Hallucination, Security, Financial, Operational, Compliance, System
- **Auto-escalation**: Time-based escalation if unresolved
- **Impact assessment**: Structured description of consequences
- **Evidence collection**: Attached logs, traces, context snapshots

### Interventions
Direct human overrides of agent behavior:

- **Pause** — Temporarily halt agent operations
- **Stop** — Immediately terminate agent execution
- **Restrict** — Limit agent to specific tools/actions
- **Override** — Substitute human decision for agent
- **Redirect** — Reassign work to different agent/team
- **Suspend** — Full suspension with auto-revert window

### Human-in-the-Loop (HITL)
Interactive Q&A between agents and humans during execution:

- Agents can pause and ask humans questions
- Humans provide context, preferences, or corrections
- Agent continues with human guidance
- Full audit trail of HITL interactions

## Key Capabilities

- **Approval delegation** — Approvals can be temporarily delegated
- **Auto-revert interventions** — Time-limited interventions that expire automatically
- **Queue summarization** — Human reviewers see prioritized, aggregated queues
- **Risk dashboard** — Real-time view of all active supervision items
- **Escalation SLAs** — Time-based escalation triggers for critical items
- **Full provenance** — Every human decision tracked with rationale

## API Endpoints

| Method   | Endpoint                                    | Description                           |
|----------|---------------------------------------------|---------------------------------------|
| POST     | `/api/v1/supervision/approvals`             | Submit approval request               |
| GET      | `/api/v1/supervision/approvals`             | List approval requests                |
| GET      | `/api/v1/supervision/approvals/{id}`        | Get approval details                  |
| POST     | `/api/v1/supervision/approvals/{id}/approve`| Approve a request                     |
| POST     | `/api/v1/supervision/approvals/{id}/reject` | Reject a request                      |
| POST     | `/api/v1/supervision/approvals/{id}/delegate`| Delegate an approval                |
| POST     | `/api/v1/supervision/escalations`           | Submit escalation                     |
| GET      | `/api/v1/supervision/escalations`           | List escalations                      |
| GET      | `/api/v1/supervision/escalations/{id}`      | Get escalation details                |
| POST     | `/api/v1/supervision/escalations/{id}/resolve`| Resolve an escalation              |
| POST     | `/api/v1/supervision/interventions`         | Issue intervention                    |
| GET      | `/api/v1/supervision/interventions`         | List active interventions             |
| GET      | `/api/v1/supervision/interventions/{id}`    | Get intervention details              |
| POST     | `/api/v1/supervision/interventions/{id}/revoke`| Revoke an intervention             |
| GET      | `/api/v1/supervision/overviews/queue`       | Get human review queue summary        |
| GET      | `/api/v1/supervision/overviews/risk-dashboard`| Get risk dashboard                 |
| POST     | `/api/v1/supervision/hitl/{id}/answer`      | Provide HITL answer                   |

## Governance & Safety

- Every approval requires **documented rationale**
- Critical escalations (P0) require **immediate acknowledgment**
- Interventions have **auto-revert** windows to prevent human bottleneck
- All supervision events feed **Module 11** observability for trend analysis
- Intervention chains are **cryptographically signed** for audit

## Critical Risks

| Risk                        | Mitigation                                  |
|-----------------------------|---------------------------------------------|
| Approval fatigue            | Smart batching, AI summarization of requests|
| Escalation fatigue          | Priority filtering, automated triage        |
| Intervention abuse          | Approval chain for interventions, audit     |
| HITL bottleneck             | Async HITL, batched questions, timeouts     |
| Delegate chain gaming       | Delegate limits, audit trail, approval      |

## Module Dependencies

- **Module 03** — Agent Orchestration Engine (requests approvals, triggers escalations)
- **Module 10** — Policy & Governance Engine (defines approval thresholds, escalation rules)
- **Module 11** — Observability & Telemetry (supervision metrics, SLA tracking)
- **Module 17** — Cost Governance Engine (supervision cost attribution)

## Related Artifacts

- `contracts/v1/openapi-09-human-supervision.yaml` — OpenAPI specification
- `contracts/v1/schema-09-human-supervision.json` — JSON Schema definitions
