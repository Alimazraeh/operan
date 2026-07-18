# Module 21 — Experience Portal

The Web UI of the PRD's **Experience Layer** (`Web UI │ API │ SDK │ CLI │ Mobile`).
A single Go binary serves the embedded Operan portal SPA and reverse-proxies
`/svc/<name>/` to every platform service, so the browser stays same-origin
(no CORS changes anywhere) and JWTs are minted client-side from the tenant's
signing secret — the secret never leaves the page.

## What the portal covers

| View | Backing modules | What you do there |
|------|----------------|-------------------|
| Overview | 04 · 05 · 09 · 11 | KPI dashboard, decisions waiting, live Kafka activity stream |
| Departments | 05 · 04 · 07 | Catalog of department templates; one-click deploy runs the **real** Module 05 pipeline, registering agents in 04 and provisioning memory in 07 |
| Agents | 04 · 07 | Workforce registry; per-agent profile, teach memories, semantic ask |
| Workflows | 03 | Pipelines (with human-gate steps), executions, human tasks |
| Supervision | 09 (→ 03 via Kafka) | Manager inbox: approve/reject gates, escalations, interventions, risk score |
| Tools | 08 | Tool registry, execution log, register/execute |
| Observability | 11 | Traces, component health, alerts, live activity stream |
| Tenants | 01 | Tenant lifecycle, quota management (max agents/tokens/storage/workflows), resource usage tracking |
| IAM | 02 | 3-principal system (user/service/agent), RBAC roles, delegation, ABAC policies, audit log, permission evaluation |
| Knowledge | 06 | Data source configuration (SharePoint, Confluence, Jira, files, DB, API, Git), ingestion job monitoring, knowledge coverage metrics |
| Policies | 10 | Policy CRUD (allow/deny rules), policy groups, real-time policy evaluation, audit log |
| Cost | 17 | Budget management with utilization progress bars, cost event tracking, threshold alerts, emergency spending throttle |
| Connectors | 18 | Enterprise connector management (M365, Salesforce, SAP, HubSpot, REST, SMTP), sync triggers, health checks, exposed tools |
| The Story | all | Guided 8-step end-to-end scenario, every step a real API call |

## Authentication

The portal uses **password-file authentication** backed by Module 02:

1. Enter the admin password (set via `IAM_ADMIN_PASSWORD_FILE`)
2. Portal POSTs to `POST /svc/iam/admin/login` → M02 validates password against file
3. M02 returns a signed JWT (HMAC-SHA256, `IAM_TOKEN_SECRET`)
4. JWT is stored in the session for all subsequent requests

### Setup

```bash
# 1. Set the JWT signing secret
export IAM_TOKEN_SECRET="a-strong-secret-at-least-32-characters"

# 2. Generate a password file
# Option A: Use the bootstrap endpoint to generate one
curl -X POST http://localhost:8002/api/v1/iam/admin/login

# Option B: Set it manually
echo "your-secure-password" > /etc/operan/admin.pass
chmod 600 /etc/operan/admin.pass
```

On first login, the portal also calls `POST /svc/tenants/bootstrap` which creates a default-tenant if none exist.

## UI Features

- **Glassmorphism dark theme** — CSS design system with custom properties, backdrop-blur cards, gradient accents
- **Responsive layout** — sidebar collapses to overlay on mobile, hamburger menu toggle
- **Multi-tenant switcher** — dropdown of previously used tenants, stored in localStorage
- **Loading skeletons** — shimmer animation while views load data
- **Error states** — error boxes with retry buttons on every view
- **Deploy pipeline visualization** — step progress indicators for department deployment
- **Real-time activity stream** — auto-refreshes every 12s in Overview, Observability, Supervision
- **Arabic RTL support** — CSS hooks for `dir="rtl"` (full Arabic layout mirroring)
- **Toast notifications** — success/error/warning toasts for all actions
- **Health dots** — real-time up/down status of all 14 backend services in the topbar

## Architecture

```
Browser → / (SPA shell + views) ← Go binary → /svc/<name>/ → platform service
```

- Static assets (CSS, JS, HTML) embedded via Go `embed.FS`
- Reverse proxy strips `/svc/<name>/` prefix before forwarding
- JWT minted client-side using Web Crypto API (or pure-JS fallback for HTTP)
- Every request carries `Authorization: Bearer <JWT>` + `X-Tenant-ID: <tenant>`
- Services enforce JWT validation + tenant isolation independently

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MODULE21_PORT` | `8021` | HTTP listen port |
| `MODULE21_SVC_<NAME>` | in-cluster DNS | Override proxy target (TENANT, ORCHESTRATION, REGISTRY, TEMPLATES, KNOWLEDGE, MEMORY, TOOLS, SUPERVISION, POLICIES, OBSERVABILITY, COST, CONNECTORS, IAM) |

## Build & run

```bash
go test ./...          # server tests incl. proxy + SPA fallback
go run .               # http://localhost:8021 (proxies need the cluster or env overrides)
```

k8s: `deploy/k8s/portal.yaml` — NodePort **30088** (`http://<node-ip>:30088`).

## Files

```
modules/21-experience-portal/
├── main.go              # Go binary: embed + proxy + SPA fallback
├── main_test.go         # Server tests: static shell, proxy stripping, upstream error
├── web/
│   ├── index.html       # SPA shell (login + sidebar nav + view container)
│   ├── css/app.css      # Design system: tokens, components, responsive, RTL
│   ├── js/app.js        # Router, login, health checks, tenant switcher
│   ├── js/api.js        # API client + in-browser JWT minting (WebCrypto + JS fallback)
│   ├── js/ui.js         # DOM helpers, toast, timestamps, badges, event rows
│   └── js/views/
│       ├── overview.js          # KPI dashboard, activity feed
│       ├── departments.js       # Catalog, deploy pipeline, department detail
│       ├── agents.js            # Workforce registry, agent profiles, memory teach/ask
│       ├── workflows.js         # Pipelines, executions, human tasks
│       ├── supervision.js       # Manager inbox, approvals, escalations, interventions
│       ├── tools.js             # Tool registry, execution log
│       ├── observability.js     # Traces, health, alerts, activity stream
│       ├── scenario.js          # 8-step guided end-to-end demo
│       ├── tenants.js           # Tenant CRUD, quota management, resource usage
│       ├── iam.js               # 3-principal IAM, RBAC, delegation, ABAC, audit
│       ├── ingestion.js         # Knowledge ingestion: sources, jobs, coverage metrics
│       ├── policies.js          # Policy CRUD, groups, evaluation, audit log
│       ├── cost.js              # Budgets, cost events, alerts, spending throttle
│       └── connectors.js        # Enterprise connectors, sync, tools, health
└── README.md
```

## Known Limitations

| # | Limitation | Severity |
|---|-----------|----------|
| 1 | Login is secret-based (no Module 02 SSO yet — Authentik integration pending) | P1 |
| 2 | Served over plain HTTP on the LAN; put behind ingress + TLS for customer-facing use | Medium |
| 3 | Department→agent linkage relies on `department_id` set at deploy time | Low |