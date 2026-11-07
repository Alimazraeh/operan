#!/usr/bin/env bash
# Re-seed the live cluster so the department work loop has something to run.
#
# As of 2026-11-07 the platform boots clean but holds no business state:
# M01 has 0 tenants, the orchestration DB is empty, and M05's PVC restored
# departments.json/requests.json/briefings.json as empty structures. Scaling
# pods does not fix that — a tenant and a deployed department have to exist.
#
# Usage:
#   bash deploy/demo/reseed.sh                 # tenant slug "acme-corp"
#   TENANT=my-co bash deploy/demo/reseed.sh    # pick the slug
#   TEMPLATE_ID=<id> bash deploy/demo/reseed.sh  # pick the template explicitly
#
# Reads the signing secret from the cluster the same way presenter-env.sh does.
# Prints PASS/FAIL per step and stops on the first real failure.

set -uo pipefail

TENANT="${TENANT:-acme-corp}"
# Only 7 of the 31 catalogue templates carry a service portfolio (it-small/medium/
# large, ops-small/medium/large/all). The other 24 — HR, Finance, Legal, Eng,
# Marketing, Procurement, Compliance, SoftDev — are org-chart-only: services=0,
# so a request has no SOP to compile and the work loop has nothing to run.
# Default to the fullest IT one: 5 agents, 8 services each with an SLA, and
# wf-access-001 = agent_call -> approval -> tool_call -> notification.
# Match on the NAME. GET /templates runs through toTemplateListResponse, which
# omits metadata — so catalog_id is NOT in the list response, and matching on
# "it-medium-001" finds nothing and selects whatever came back first.
TEMPLATE_MATCH="${TEMPLATE_MATCH:-medium it department}"
SERVICE_MATCH="${SERVICE_MATCH:-svc-access-management}"
M01_PORT=18080
M05_PORT=18005
NS=operan

step() { printf '\n== %s\n' "$1"; }
pass() { printf 'PASS  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1"; exit 1; }

# --- auth -------------------------------------------------------------------
step "Minting an admin JWT from secret/operan-jwt"
SECRET="$(kubectl -n "$NS" get secret operan-jwt -o jsonpath='{.data.secret}' | base64 -d)" \
  || fail "could not read secret/operan-jwt"
[ -n "$SECRET" ] || fail "secret/operan-jwt is empty"

TOKEN="$(python3 - "$SECRET" <<'EOF'
import hmac, hashlib, base64, json, sys, time
def b64(d): return base64.urlsafe_b64encode(d).rstrip(b'=').decode()
secret = sys.argv[1]
h = b64(json.dumps({"alg": "HS256", "typ": "JWT"}).encode())
p = b64(json.dumps({"sub": "reseed-admin", "iss": "operan-tenant-control-plane",
                    "role": "admin", "roles": ["admin"],
                    "exp": int(time.time()) + 14400}).encode())
print(f"{h}.{p}." + b64(hmac.new(secret.encode(), f"{h}.{p}".encode(), hashlib.sha256).digest()))
EOF
)" || fail "could not mint a token"
unset SECRET
pass "token minted (valid 4h, role admin)"

# --- port-forwards ----------------------------------------------------------
step "Port-forwarding M01 and M05"
kubectl -n "$NS" port-forward svc/tenant-control-plane "$M01_PORT:8080" >/dev/null 2>&1 &
PF1=$!
kubectl -n "$NS" port-forward svc/department-templates "$M05_PORT:8005" >/dev/null 2>&1 &
PF2=$!
trap 'kill $PF1 $PF2 2>/dev/null' EXIT

for _ in $(seq 1 30); do
  nc -z 127.0.0.1 "$M01_PORT" 2>/dev/null && nc -z 127.0.0.1 "$M05_PORT" 2>/dev/null && break
  sleep 0.3
done
nc -z 127.0.0.1 "$M05_PORT" 2>/dev/null || fail "port-forward to M05 never came up"
pass "forwards up (M01 :$M01_PORT, M05 :$M05_PORT)"

op() { # op METHOD URL [body]
  local method="$1" url="$2" body="${3:-}" args
  args=(-s -X "$method" "$url"
    -H "Authorization: Bearer $TOKEN"
    -H "X-Tenant-ID: $TENANT"
    -H "Content-Type: application/json")
  [ -n "$body" ] && args+=(-d "$body")
  curl --max-time 30 "${args[@]}"
}

# Pull {id,name}-shaped objects out of any envelope this API returns.
pick() { python3 -c '
import json, sys
want_key, want_match = sys.argv[1], sys.argv[2].lower()
found = []
def walk(n):
    if isinstance(n, dict):
        if "id" in n and isinstance(n["id"], str):
            found.append(n)
        for v in n.values(): walk(v)
    elif isinstance(n, list):
        for v in n: walk(v)
try: walk(json.load(sys.stdin))
except Exception: sys.exit(1)
if want_match:
    for f in found:
        meta = f.get("metadata") if isinstance(f.get("metadata"), dict) else {}
        blob = " ".join(str(x) for x in (
            f.get("name", ""), f.get("display_name", ""), f.get("category", ""),
            f.get("slug", ""), f.get("id", ""), meta.get("catalog_id", ""))).lower()
        if want_match in blob:
            print(f[want_key]); sys.exit(0)
    sys.exit(2)  # a named thing was asked for and is not here — never guess
if found: print(found[0][want_key])
' "$@"; }

# --- 1. tenant record in M01 ------------------------------------------------
step "Creating the M01 tenant record ($TENANT)"
TENANT_BODY=$(cat <<JSON
{"name": "$TENANT", "display_name": "$TENANT", "plan": "enterprise",
 "region": "me-east-1", "isolation_level": "namespace",
 "quota": {"max_agents": 100, "max_workflows_per_day": 1000, "max_storage_gb": 100,
           "max_monthly_tokens": 50000000, "max_concurrent_workflows": 50}}
JSON
)
R=$(op POST "http://127.0.0.1:$M01_PORT/v1/tenants" "$TENANT_BODY")
echo "$R" | head -c 400; echo
echo "$R" | grep -q '"id"' || fail "tenant create did not return an id (401 means the token was rejected)"
pass "tenant record created"

# --- 2. seed the template catalogue for this tenant -------------------------
step "Seeding the template catalogue"
R=$(op POST "http://127.0.0.1:$M05_PORT/templates/seed" '{}')
echo "$R" | head -c 300; echo
pass "seed call returned"

# --- 3. choose a template ---------------------------------------------------
step "Choosing a template (match: '$TEMPLATE_MATCH')"
if [ -n "${TEMPLATE_ID:-}" ]; then
  TPL="$TEMPLATE_ID"
else
  TPL=$(op GET "http://127.0.0.1:$M05_PORT/templates?page_size=100" | pick id "$TEMPLATE_MATCH")
fi
[ -n "$TPL" ] || fail "no template matched '$TEMPLATE_MATCH' — the picker will not guess. List names with: op GET /templates?page_size=100"
pass "template $TPL"

# --- 4. deploy it -----------------------------------------------------------
step "Deploying the department (M05 walks its own pipeline)"
R=$(op POST "http://127.0.0.1:$M05_PORT/templates/$TPL/deploy" \
  '{"environment": "production", "configuration": {"region": "me-central"}}')
echo "$R" | head -c 400; echo
DEP=$(echo "$R" | pick id "")
[ -n "$DEP" ] || fail "deploy did not return a deployment id"

for i in $(seq 1 40); do
  S=$(op GET "http://127.0.0.1:$M05_PORT/templates/$TPL/deployments/$DEP" \
      | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("status") or (d.get("data") or {}).get("status",""))' 2>/dev/null)
  printf '  [%02d] status=%s\n' "$i" "${S:-?}"
  case "$S" in
    operational) pass "deployment $DEP operational"; break ;;
    failed) fail "deployment $DEP failed — check: kubectl -n $NS logs deploy/department-templates" ;;
  esac
  sleep 3
done
[ "${S:-}" = "operational" ] || fail "deployment did not reach operational in ~2 min"

# --- 5. find the department and a service ----------------------------------
step "Locating the department and its service portfolio"
DEPT=$(op GET "http://127.0.0.1:$M05_PORT/departments" | pick id "")
[ -n "$DEPT" ] || fail "no department came back from GET /departments"
pass "department $DEPT"

SVC=$(op GET "http://127.0.0.1:$M05_PORT/departments/$DEPT/services" | pick id "$SERVICE_MATCH")
[ -n "$SVC" ] || fail "department has no services — only it-small/medium/large and ops-small/medium/large/all carry a portfolio; the other 24 templates are org-chart-only"
pass "service $SVC"

# --- 6. drive the work loop ------------------------------------------------
step "Submitting a service request"
R=$(op POST "http://127.0.0.1:$M05_PORT/departments/$DEPT/requests" \
  "{\"service_id\": \"$SVC\", \"title\": \"Contractor VPN access for M. Haddad\",
    \"body\": \"New contractor starts Monday and needs VPN plus read access to the finance share. Standard 90-day expiry.\",
    \"priority\": \"P2\"}")
echo "$R" | head -c 400; echo
REQ=$(echo "$R" | pick id "")
[ -n "$REQ" ] || fail "request create did not return an id"
pass "request $REQ"

step "Watching the work loop (M05 polls every 15s; agent drafts via litellm)"
for i in $(seq 1 30); do
  ST=$(op GET "http://127.0.0.1:$M05_PORT/requests/$REQ" \
      | python3 -c 'import json,sys; d=json.load(sys.stdin); d=d.get("data",d); print(d.get("status",""), "awaiting_approval=%s" % d.get("awaiting_approval"), "tokens=%s" % d.get("tokens_used"))' 2>/dev/null)
  printf '  [%02d] %s\n' "$i" "${ST:-?}"
  case "$ST" in
    completed*) pass "request completed"; break ;;
    failed*)    fail "request failed — M03 run lost or the draft call errored" ;;
  esac
  sleep 10
done

cat <<EOF

-----------------------------------------------------------------
Seeded. Open the portal and sign in with tenant "$TENANT":
  http://192.168.8.210:30088/
(password field = the operan-jwt signing secret)

  tenant      $TENANT
  template    $TPL
  department  $DEPT
  request     $REQ
-----------------------------------------------------------------
EOF
