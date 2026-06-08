# Module 08 — Tool Execution Layer

**Secure external action execution engine.**

---

## Purpose

The Tool Execution Layer is Operan's interface between agent cognition and the real world. Agents reason, plan, and decide — this module ensures their decisions translate into safe, policy-governed, observable external actions.

## Tool Categories

| Category              | Capability                              | Examples                                          |
|----------------------|-----------------------------------------|---------------------------------------------------|
| API Execution        | HTTP/REST/gRPC calls                    | Create record, fetch data, trigger webhook         |
| Browser Automation   | Headless browser control                | Form filling, data scraping, UI testing            |
| ERP Interaction      | SAP/Oracle integration                  | Create PO, post invoice, update inventory          |
| Email                | Compose, send, parse emails             | Draft responses, send notifications                |
| Document Generation  | PDF, Word, spreadsheet creation         | Generate contracts, reports, invoices              |
| Workflow Trigger     | Start external workflow processes       | Approval requests, procurement flows               |
| File System          | Read/write managed files                | Export data, save artifacts                        |
| Database Query       | Executable queries                      | Generate SELECT statements for review              |
| Webhook              | Register and fire callbacks             | Event notifications, status updates                |

## Security Model

### Sandbox Isolation
Three isolation levels based on tool risk:
- **Container** — Full container for destructive operations
- **Namespace** — Namespace-level isolation for risky operations
- **Process** — Process-level isolation for safe operations

### Execution Controls
- **Policy validation** before every execution
- **Dry-run mode** for predicted impact analysis
- **Rollback support** for state-changing operations
- **Resource quotas** prevent runaway consumption
- **Network restrictions** prevent unauthorized access

### Policy Gating
Every execution passes through Module 10's Policy Engine:
- Tool capability checks
- Agent permission validation
- Input content scanning
- Threshold-based approval requirements

## Key Capabilities

- **MCP-native** tool protocol support
- **Capability isolation** — tools cannot escape their sandbox
- **Execution signing** — every action cryptographically attributed
- **Dry-run predictions** — side-effect analysis without real impact
- **Automatic rollback** — state-changing actions can be reversed
- **Execution provenance** — full audit trail from agent decision to action
- **Cost attribution** — every execution tied to agent, department, tenant

## API Endpoints

| Method   | Endpoint                                    | Description                          |
|----------|---------------------------------------------|--------------------------------------|
| GET      | `/api/v1/tools/catalog`                     | List available tools                 |
| GET      | `/api/v1/tools/{tool_id}`                   | Get tool details                     |
| POST     | `/api/v1/tools/{tool_id}/test`              | Test tool connection                 |
| POST     | `/api/v1/tools/execute`                     | Execute a tool call                  |
| GET      | `/api/v1/tools/execute`                     | List executions                      |
| GET      | `/api/v1/tools/execute/{id}`                | Get execution result                 |
| POST     | `/api/v1/tools/execute/{id}/cancel`         | Cancel in-flight execution           |
| POST     | `/api/v1/tools/execute/{id}/rollback`       | Roll back completed execution        |
| POST     | `/api/v1/tools/sandbox`                     | Create isolated sandbox              |
| GET      | `/api/v1/tools/sandbox/{id}`                | Get sandbox status                   |
| DELETE   | `/api/v1/tools/sandbox/{id}`                | Destroy sandbox                      |
| POST     | `/api/v1/tools/sandbox/{id}/exec`           | Execute within sandbox               |
| POST     | `/api/v1/tools/policies/check`              | Validate execution against policies  |
| POST     | `/api/v1/tools/dry-run`                     | Dry-run (no side effects)            |
| GET      | `/api/v1/tools/logs/{execution_id}`         | Get execution logs                   |

## Critical Risks

| Risk                   | Mitigation                                              |
|------------------------|---------------------------------------------------------|
| Unauthorized actions   | Policy gating, RBAC/ABAC, execution signing             |
| Sandbox escape         | Container isolation, syscall restrictions, cgroups      |
| Data exfiltration      | Network restrictions, egress filtering, content scanning|
| Runaway costs          | Resource quotas, token budgets, agent throttling        |
| Uncontrolled side effects | Dry-run mode, rollback support, execution approval   |
| Tool supply chain      | Tool cataloging, dependency scanning, version pinning   |

## Module Dependencies

- **Module 03** — Agent Orchestration Engine (issues tool calls)
- **Module 10** — Policy & Governance Engine (execution policy checks)
- **Module 16** — Execution Sandbox (container isolation runtime)
- **Module 17** — Cost Governance Engine (cost attribution)
- **Module 18** — Enterprise Connector Fabric (ERP, email, etc.)

## Related Artifacts

- `contracts/v1/openapi-08-tool-execution.yaml` — OpenAPI specification
- `contracts/v1/schema-08-tool-execution.json` — JSON Schema definitions
