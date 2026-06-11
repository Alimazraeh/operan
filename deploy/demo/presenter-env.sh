# Source this before a live demo:  source deploy/demo/presenter-env.sh
# Starts port-forwards to the operan namespace, mints a supervisor JWT from
# the cluster secret, and defines `op` — a curl wrapper with auth + pretty
# JSON — so every runbook command is short and copy-pasteable.

export OPERAN_TENANT="${OPERAN_TENANT:-$(python3 -c 'import uuid; print(uuid.uuid4())')}"

_secret="$(kubectl -n operan get secret operan-jwt -o jsonpath='{.data.secret}' | base64 -d)"
export OPERAN_JWT=$(python3 - "$_secret" <<'EOF'
import hmac, hashlib, base64, json, sys, time
def b64(d): return base64.urlsafe_b64encode(d).rstrip(b'=').decode()
secret = sys.argv[1]
h = b64(json.dumps({"alg":"HS256","typ":"JWT"}).encode())
p = b64(json.dumps({"sub":"demo-supervisor","iss":"operan-tenant-control-plane",
                    "role":"admin","roles":["admin"],"exp":int(time.time())+14400}).encode())
print(f"{h}.{p}." + b64(hmac.new(secret.encode(), f"{h}.{p}".encode(), hashlib.sha256).digest()))
EOF
)
unset _secret

pkill -f "kubectl -n operan port-forward" 2>/dev/null
kubectl -n operan port-forward svc/tenant-control-plane 8080:8080 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/agent-orchestration  8003:8080 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/agent-registry       8083:8083 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/department-templates 8005:8005 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/memory-fabric        8007:8007 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/tool-execution       8008:8008 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/human-supervision    8009:8009 >/dev/null 2>&1 &
kubectl -n operan port-forward svc/observability        8011:8011 >/dev/null 2>&1 &
sleep 3

# op METHOD URL [json-body] — authenticated request, pretty-printed response.
op() {
  local method="$1" url="$2" body="${3:-}"
  local args=(-s -X "$method" "$url"
    -H "Authorization: Bearer $OPERAN_JWT"
    -H "X-Tenant-ID: $OPERAN_TENANT"
    -H "Content-Type: application/json")
  [ -n "$body" ] && args+=(-d "$body")
  curl "${args[@]}" | python3 -m json.tool 2>/dev/null || curl "${args[@]}"
}

echo "Demo terminal ready."
echo "  tenant : $OPERAN_TENANT   (fresh on each source; re-source for a clean run)"
echo "  usage  : op GET  http://localhost:8007/vectors"
echo "           op POST http://localhost:8009/approvals '{...json...}'"
