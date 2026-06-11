# Operan Demo Environment

Nine implemented services + single-node Kafka, wired into one event mesh.
The scripted demo walks the full customer story: provision a tenant, register
an agent, deploy a department template, store and recall agent memories,
pause on a human approval gate, execute a tool — then show every step as
traces, metrics, and health in Observability.

## Run it

```bash
cd deploy/demo
cp .env.example .env
echo "DEMO_JWT_SECRET=$(openssl rand -base64 32)" > .env

docker compose up --build -d     # first build takes a few minutes
docker compose ps                # wait until services are healthy
./demo.sh                        # scripted end-to-end flow with PASS/FAIL
```

Tear down with `docker compose down` (`-v` to also drop Kafka data).

## Services

| Service | Host port | Module |
|---------|-----------|--------|
| tenant-control-plane | 8080 | 01 |
| identity-access | 8002 | 02 (boots with placeholder Authentik — see below) |
| agent-orchestration | 8003 | 03 |
| agent-registry | 8083 | 04 |
| department-templates | 8005 | 05 |
| memory-fabric | 8007 | 07 |
| tool-execution | 8008 | 08 |
| human-supervision | 8009 | 09 |
| observability | 8011 | 11 (liveness at `/healthz`; `/health` is the tenant dashboard) |
| kafka | (internal) | apache/kafka 3.7, KRaft, auto-create topics |

One `DEMO_JWT_SECRET` signs HMAC-S256 tokens accepted by every module.
`demo.sh` mints its own token (claims: `sub`, `roles: ["admin"]`, `exp`).

## What the demo proves

1. **Multi-tenancy** — every call is scoped by JWT + `X-Tenant-ID`.
2. **Agent memory** — semantic search returns the relevant memory, not noise.
3. **Human-in-the-loop** — an agent action waits on a gate; a supervisor
   approves it from the queue; the decision is an auditable event.
4. **One event mesh** — every action lands in Kafka and Module 11's consumer
   turns it into spans (grouped into traces by `correlationId`), counters,
   component health, and alerts. The `human_gate` span type shows the
   approval inside the same trace as the memory and tool activity.

## Known demo-scope limits

- **Module 02** boots against a placeholder Authentik URL: its API responds
  and it publishes events, but real identity operations need an Authentik
  deployment (its `/ready` shows `authentik: fail` by design here).
- **Stores are in-memory** — restarting a service clears its data; re-run
  `demo.sh` to repopulate.
- **Module 03 does not yet consume gate/intervention events**, so approval
  gates pause nothing automatically — the gate flow itself is what's shown.
