# Module 21 — Experience Portal

The Web UI of the PRD's **Experience Layer** (`Web UI │ API │ SDK │ CLI │ Mobile`).
A single Go binary serves the embedded Operan portal SPA and reverse-proxies
`/svc/<name>/` to every platform service, so the browser stays same-origin
(no CORS changes anywhere) and JWTs are minted client-side from the tenant's
signing secret — the secret never leaves the page.

## What the portal covers

| View | Backing modules | What you do there |
|------|----------------|-------------------|
| Overview | 04 · 05 · 09 · 11 | Departments deployed, agents employed, decisions waiting, live activity |
| Departments | 05 · 04 · 07 | Catalog of department templates; one-click deploy runs the **real** Module 05 pipeline (`select → … → operational`), registering agents in 04 and provisioning memory in 07; department detail with staff, governance, KPIs |
| Agents | 04 · 07 | Workforce registry; per-agent profile, teach memories, semantic ask |
| Workflows | 03 | Pipelines (with human-gate steps), executions, human tasks |
| Supervision | 09 (→ 03 via Kafka) | Manager inbox: approve/reject gates (enforced by the orchestrator), escalations, interventions, risk score |
| Tools | 08 | Tool registry, execution log, register/execute |
| Observability | 11 | Traces, component health, alerts, live activity stream |
| The Story | all | Guided 8-step end-to-end scenario, every step a real API call |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MODULE21_PORT` | `8021` | HTTP listen port |
| `MODULE21_SVC_<NAME>` | in-cluster DNS | Override a proxy target (TENANT, ORCHESTRATION, REGISTRY, TEMPLATES, MEMORY, TOOLS, SUPERVISION, OBSERVABILITY) |

No JWT secret here: the portal is an unauthenticated static host + proxy.
Authentication happens in the browser (HS256 JWT minted from the pasted
secret; pure-JS HMAC fallback for non-secure contexts) and every proxied
request carries `Authorization` + `X-Tenant-ID`, enforced by each service.

## Build & run

```bash
go test ./...          # server tests incl. proxy + SPA fallback
go run .               # http://localhost:8021 (proxies need the cluster or env overrides)
```

k8s: `deploy/k8s/portal.yaml` — NodePort **30088** (`http://<node-ip>:30088`).

## Known Limitations

| # | Limitation | Severity |
|---|-----------|----------|
| 1 | Login is secret-based (no Module 02 SSO yet — Authentik integration pending) | P1 |
| 2 | Served over plain HTTP on the LAN; put behind ingress + TLS for customer-facing use | Medium |
| 3 | Department→agent linkage relies on `department_id` set at deploy time | Low |
| 4 | English-only UI; Arabic localization (Module 19) not yet wired | Medium |
