#!/usr/bin/env bash
# Operan end-to-end demo: tenant → agent → department → memory → approval
# gate → tool → everything visible in Observability.
#
# Prereqs: `docker compose up --build -d` finished, .env has DEMO_JWT_SECRET.
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
[ -f "$HERE/.env" ] && . "$HERE/.env"
: "${DEMO_JWT_SECRET:?DEMO_JWT_SECRET not set (create deploy/demo/.env from .env.example)}"

TENANT="d3a0de01-0000-4000-8000-000000000001"
AGENT_ID="a9e11700-0000-4000-8000-000000000007"
REQUEST_ID="0e0a9e57-0000-4000-8000-000000000009"

JWT=$(python3 - "$DEMO_JWT_SECRET" <<'EOF'
import hmac, hashlib, base64, json, sys, time
def b64(d): return base64.urlsafe_b64encode(d).rstrip(b'=').decode()
secret = sys.argv[1]
header = b64(json.dumps({"alg":"HS256","typ":"JWT"}).encode())
payload = b64(json.dumps({
    "sub": "demo-supervisor",
    "iss": "operan-tenant-control-plane",  # module 01 validates issuer
    "role": "admin",                        # module 04 reads singular claim
    "roles": ["admin"],
    "exp": int(time.time()) + 7200,
}).encode())
sig = b64(hmac.new(secret.encode(), f"{header}.{payload}".encode(), hashlib.sha256).digest())
print(f"{header}.{payload}.{sig}")
EOF
)

AUTH=(-H "Authorization: Bearer $JWT" -H "X-Tenant-ID: $TENANT" -H "Content-Type: application/json")
PASS=0; FAIL=0

step() { # step <name> <expected-status> <method> <url> [json-body]
  local name="$1" expect="$2" method="$3" url="$4" body="${5:-}"
  local args=(-s -o /tmp/demo-step.json -w '%{http_code}' -X "$method" "$url" "${AUTH[@]}")
  [ -n "$body" ] && args+=(-d "$body")
  local code
  code=$(curl "${args[@]}")
  if [ "$code" = "$expect" ]; then
    echo "  ✔ $name ($code)"; PASS=$((PASS+1))
  else
    echo "  ✘ $name — got $code, want $expect: $(head -c 200 /tmp/demo-step.json)"; FAIL=$((FAIL+1))
  fi
}

json() { python3 -c "import json,sys; print(json.load(open('/tmp/demo-step.json'))$1)" 2>/dev/null; }

echo "── 0. Service health ─────────────────────────────────────────"
for svc in "tenant-control-plane:8080/health" "identity-access:8002/health" \
           "agent-orchestration:8003/health" "agent-registry:8083/health" \
           "department-templates:8005/health" "memory-fabric:8007/health" \
           "tool-execution:8008/health" "human-supervision:8009/health" \
           "observability:8011/healthz"; do
  name="${svc%%:*}"; path="localhost:${svc#*:}"
  code=$(curl -s -o /dev/null -w '%{http_code}' "http://$path")
  if [ "$code" = "200" ]; then echo "  ✔ $name"; PASS=$((PASS+1)); else echo "  ✘ $name ($code)"; FAIL=$((FAIL+1)); fi
done

echo "── 1. Provision tenant (Module 01) ───────────────────────────"
step "create tenant" 201 POST "http://localhost:8080/v1/tenants" \
  '{"name":"Acme Demo Corp","plan":"enterprise","region":"me-central","contact_email":"demo@acme.example","isolation_level":"namespace"}'

echo "── 2. Register agent (Module 04) ─────────────────────────────"
step "register agent" 201 POST "http://localhost:8083/registry/agents" \
  '{"id":"'$AGENT_ID'","tenant_id":"'$TENANT'","name":"sales-assistant","role":"sales","capabilities":["draft_contracts","crm_lookup"],"tools":["send_email"],"version":"1.0.0"}'

echo "── 3. Create department template (Module 05) ─────────────────"
step "create template" 201 POST "http://localhost:8005/templates" \
  '{"name":"Sales Department","category":"sales","description":"Standard sales department blueprint","agents":[{"role":"sales-assistant"}]}'

echo "── 4. Agent memory (Module 07) ───────────────────────────────"
step "ingest memories" 201 POST "http://localhost:8007/vectors" \
  '{"items":[{"document_id":"22222222-2222-4222-8222-222222222222","embedding_type":"agent_personal","semantic_content":"Customer Acme prefers Arabic-first UI and quarterly billing","metadata":{"agent_id":"'$AGENT_ID'"}},{"document_id":"22222222-2222-4222-8222-222222222222","embedding_type":"agent_personal","semantic_content":"Unrelated note about office plants","metadata":{"agent_id":"'$AGENT_ID'"}}]}'
step "semantic search finds the right memory" 200 POST "http://localhost:8007/search" \
  '{"query":"Arabic billing preferences","embedding_type":"agent_personal","relevance_threshold":0.3}'
echo "    top hit: $(json "['items'][0]['content']")"
step "agent memory state" 200 GET "http://localhost:8007/agents/$AGENT_ID"

echo "── 5. Human approval gate (Module 09) ────────────────────────"
step "agent raises approval gate" 201 POST "http://localhost:8009/approvals" \
  '{"request_id":"'$REQUEST_ID'","requester_id":"'$AGENT_ID'","type":"parallel","title":"Send contract to Acme (value: $250k)","expires_at":"2027-01-01T00:00:00Z"}'
APPROVAL_ID=$(json "['id']")
step "gate appears in human queue" 200 GET "http://localhost:8009/queue"
echo "    queue: $(json "['total']") item(s)"
step "supervisor approves" 200 POST "http://localhost:8009/approvals/$APPROVAL_ID/approve" \
  '{"approver_id":"5e500000-0000-4000-8000-000000000001","comment":"Contract terms verified"}'
echo "    status: $(json "['status']")"

echo "── 6. Tool execution (Module 08) ─────────────────────────────"
step "register tool" 201 POST "http://localhost:8008/tools/register" \
  '{"name":"send_email","description":"Send an email via SMTP relay","category":"communication"}'
TOOL_ID=$(json "['id']")
step "execute tool" 201 POST "http://localhost:8008/execute" \
  '{"tool":"send_email","agent_id":"'$AGENT_ID'","parameters":{"to":"cfo@acme.example","subject":"Contract"}}'

echo "── 7. Observability: the payoff (Module 11) ──────────────────"
sleep 3  # let the consumer drain the topics
step "spans ingested from platform events" 200 GET "http://localhost:8011/spans?page_size=50"
echo "    spans: $(json "['total']")"
step "human gate visible in traces" 200 GET "http://localhost:8011/spans?span_type=human_gate"
echo "    human_gate spans: $(json "['total']")"
step "event metrics recorded" 200 GET "http://localhost:8011/metrics?metric_name=operan.events.consumed"
echo "    consumed-event metrics: $(json "['total']")"
step "tenant system health" 200 GET "http://localhost:8011/health"
echo "    overall: $(json "['overall_status']"), components: $(python3 -c "import json; print(len(json.load(open('/tmp/demo-step.json'))['components']))" 2>/dev/null)"

echo "──────────────────────────────────────────────────────────────"
echo "RESULT: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ] || exit 1
