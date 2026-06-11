# Operan — Customer Demo Runbook

A six-act, ~15-minute live demo on the k8s cluster. Two ways to drive it:

- **Web console (recommended for customers)** — see "Console demo" below.
- **Terminal** — every command below is copy-paste with expected output.

Acts 3 and 4 are the wow moments — give them room to breathe.

## Console demo (browser, no terminal on screen)

```bash
kubectl -n operan port-forward svc/operan-console 8088:8080
# open http://localhost:8088
```

1. **Connect** — paste the JWT secret
   (`kubectl -n operan get secret operan-jwt -o jsonpath='{.data.secret}' | base64 -d`),
   click **New tenant**, then **Connect**. Eight green service dots appear.
   The secret never leaves the page — the JWT is minted in-browser.
2. **Memory** — click *Store memory* and *Store noise memory*, then *Search*
   with the pre-filled zero-overlap query. One result, the right one, with
   its cosine score and the qwen3 model name.
3. **Supervision** — click *Start gated workflow*. It appears in the queue
   with an `orchestrator: pending` badge. Click **Approve** — within a few
   seconds the badge flips to `orchestrator: approved`. That flip traveled
   through Kafka; the console never told the orchestrator anything.
4. **Health & trace panels** — every click above is already there as spans,
   metrics, and component health, refreshed live.
5. (Optional, from a terminal) `kubectl -n operan delete pod -l app.kubernetes.io/name=memory-fabric`,
   wait for Ready, hit *Search* again — the memory survived.

Content changes to the console: edit `deploy/k8s/console/index.html`, then
re-create the configmap and `kubectl -n operan rollout restart deployment operan-console`
(commands in `deploy/k8s/console/console.yaml`).

---


## Before the customer arrives (5 min)

```bash
kubectl -n operan get pods                 # 10/10 Running, READY 1/1
source deploy/demo/presenter-env.sh        # port-forwards + JWT + `op` helper
DEMO_JWT_SECRET="$(kubectl -n operan get secret operan-jwt -o jsonpath='{.data.secret}' | base64 -d)" \
  bash deploy/demo/demo.sh                 # full dress rehearsal: expect 24/24
source deploy/demo/presenter-env.sh        # re-source → fresh tenant for the real run
```

If the rehearsal isn't 24/24, fix before the meeting — usually a dead
port-forward (`re-source`) or a pod restart still warming up.

**Reset between runs:** just `source deploy/demo/presenter-env.sh` again — it
mints a fresh tenant UUID, so every run starts clean without touching storage.

---

## Act 1 — "This is a real platform, not slides" (1 min)

```bash
kubectl -n operan get pods
```

> *Nine microservices and a Kafka event bus, deployed from contracts-first
> OpenAPI/AsyncAPI specs. Every action you'll see becomes an event on the
> bus — that matters in Act 6.*

## Act 2 — Provision a tenant, an agent, a department (2 min)

```bash
op POST http://localhost:8080/v1/tenants '{
  "name": "Acme Corp", "plan": "enterprise", "region": "me-central",
  "contact_email": "ops@acme.example", "isolation_level": "namespace"}'

op POST http://localhost:8083/registry/agents '{
  "id": "a9e11700-0000-4000-8000-000000000007",
  "tenant_id": "'$OPERAN_TENANT'",
  "name": "sales-assistant", "role": "sales", "version": "1.0.0",
  "capabilities": ["draft_contracts", "crm_lookup"], "tools": ["send_email"]}'

op POST http://localhost:8005/templates '{
  "name": "Sales Department", "category": "sales",
  "description": "Standard sales department blueprint"}'
```

> *Multi-tenant from the first request: everything is scoped by JWT +
> tenant header. The agent registry and department templates are how a
> customer stamps out whole AI departments.*

## Act 3 — Agent memory with real semantic search (3 min) ⭐

Store two memories for the agent — one signal, one noise:

```bash
op POST http://localhost:8007/vectors '{"items": [
  {"document_id": "22222222-2222-4222-8222-222222222222",
   "embedding_type": "agent_personal",
   "semantic_content": "Customer Acme prefers Arabic-first UI and quarterly billing",
   "metadata": {"agent_id": "a9e11700-0000-4000-8000-000000000007"}},
  {"document_id": "22222222-2222-4222-8222-222222222222",
   "embedding_type": "agent_personal",
   "semantic_content": "Unrelated note about office plants",
   "metadata": {"agent_id": "a9e11700-0000-4000-8000-000000000007"}}]}'
```

Now the money query — **zero words in common** with the stored memory:

```bash
op POST http://localhost:8007/search '{
  "query": "which interface language does the client like",
  "embedding_type": "agent_personal", "relevance_threshold": 0.3}'
```

**Expect:** exactly one hit — the Arabic-UI memory — and not the plants.

> *No keyword matches that. The platform embeds every memory and every
> query through the Qwen embeddings model running on this same cluster —
> the agent genuinely understands what it remembers. Sovereign deployment:
> no data left this machine.*

## Act 4 — The human approval gate (4 min) ⭐⭐

The agent starts a workflow that pauses for human sign-off:

```bash
PIPE=$(op POST http://localhost:8003/api/v1/orchestration/pipeline '{
  "name": "send-contract",
  "steps": [{"id":"s1","name":"draft-contract","type":"agent"},
            {"id":"s2","name":"human-signoff","type":"human_gate"}]}' | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
EXEC=$(op POST http://localhost:8003/api/v1/orchestration/executions \
  '{"pipeline_id":"'$PIPE'"}' | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
TASK=$(op POST http://localhost:8003/api/v1/orchestration/human-tasks '{
  "pipeline_execution_id":"'$EXEC'", "step_id":"s2",
  "assignee_id":"supervisor-1",
  "instructions":"Approve sending the $250k contract to Acme"}' | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

GATE=$(op POST http://localhost:8009/approvals '{
  "request_id":"'$TASK'",
  "requester_id":"a9e11700-0000-4000-8000-000000000007",
  "type":"parallel", "title":"Send contract to Acme ($250k)"}' | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
```

Show the work waiting on a human, and the orchestrator task **pending**:

```bash
op GET "http://localhost:8009/queue"
op GET "http://localhost:8003/api/v1/orchestration/human-tasks/$TASK"
```

The supervisor approves — pause a beat, then show the orchestrator:

```bash
op POST "http://localhost:8009/approvals/$GATE/approve" \
  '{"approver_id":"5e500000-0000-4000-8000-000000000001","comment":"Terms verified — send it"}'
sleep 5
op GET "http://localhost:8003/api/v1/orchestration/human-tasks/$TASK"
```

**Expect:** status flipped `pending → approved`, with the approver's ID and
gate reference recorded.

> *Two separate services. Nobody called the orchestrator — the approval
> traveled as an event over Kafka and the orchestrator enforced it. Reject
> works the same way and fails the task. This is auditable human control
> over agents, built into the platform's nervous system.*

## Act 5 — Kill a pod, the agent still remembers (2 min) ⭐

```bash
kubectl -n operan delete pod -l app.kubernetes.io/name=memory-fabric
kubectl -n operan get pods -l app.kubernetes.io/name=memory-fabric -w   # Ctrl-C once Ready
```

Re-run the Act 3 search (port-forward survives the pod swap; if the call
hangs once, just repeat it):

```bash
op POST http://localhost:8007/search '{
  "query": "which interface language does the client like",
  "embedding_type": "agent_personal", "relevance_threshold": 0.3}'
```

**Expect:** same single hit. The replacement pod restored its memory
snapshots from node storage before serving.

## Act 6 — The payoff: everything you just did, observed (3 min)

```bash
op GET "http://localhost:8011/spans?page_size=50"            # every action as a span
op GET "http://localhost:8011/spans?span_type=human_gate"    # the approval, in the trace
op GET "http://localhost:8011/metrics?metric_name=operan.events.consumed"
op GET "http://localhost:8011/health"                        # per-component health rollup
```

> *Module 11 consumed every event the demo produced — tenant, registry,
> memory, the gate decision — and turned them into traces grouped by
> correlation ID, per-tenant metrics, and live component health. Audit and
> cost visibility aren't bolted on; they're a consumer of the same bus.*

---

## If something goes sideways

| Symptom | Fix |
|---------|-----|
| Connection refused / empty reply | Port-forward died: `source deploy/demo/presenter-env.sh` (keeps tenant if `OPERAN_TENANT` already set in the shell) |
| 401 on everything | JWT expired (4 h) — re-source |
| Act 4 task stays `pending` | Give Kafka a few more seconds; check `kubectl -n operan logs deploy/agent-orchestration \| grep GATES` |
| A pod not Ready | `kubectl -n operan describe pod <name>`; demo can proceed — acts are independent except 4 needs 03+09 |
| Need a totally clean slate | `source` again for a fresh tenant; storage from old tenants is invisible to the new one |
