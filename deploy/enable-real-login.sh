#!/usr/bin/env bash
# Turn on real per-user sign-in and close the hardcoded admin backdoor.
#
# Order matters and this script enforces it. The new M02 image fails closed when
# no admin password is configured, so deploying it before the secret exists
# would shut break-glass login with no per-user account to fall back on — and
# lock you out of your own portal. Secret first, env second, image last.
#
# Usage:
#   bash deploy/enable-real-login.sh                       # ali@adri.nz / acme-3
#   ADMIN_EMAIL=you@x.com ADMIN_TENANT=acme-4 bash deploy/enable-real-login.sh
#
# The password is generated here, stored only in the Kubernetes secret, and
# never printed. Read it back yourself with the command shown at the end.

set -uo pipefail

NS=operan
SECRET=operan-admin
# The image built from this branch's tip. 4ffe462 carries the M02 change; this
# is the tag an image actually exists for.
IMAGE_TAG="${IMAGE_TAG:-sha-ce8de2a}"
ADMIN_EMAIL="${ADMIN_EMAIL:-ali@adri.nz}"
ADMIN_TENANT="${ADMIN_TENANT:-acme-3}"

step() { printf '\n== %s\n' "$1"; }
pass() { printf 'PASS  %s\n' "$1"; }
fail() { printf 'FAIL  %s\n' "$1"; exit 1; }

# --- 1. the credential -------------------------------------------------------
step "Admin password secret ($SECRET)"
if kubectl -n "$NS" get secret "$SECRET" >/dev/null 2>&1; then
  pass "already exists — left untouched (delete it first if you want a new one)"
else
  kubectl -n "$NS" create secret generic "$SECRET" \
    --from-literal=password="$(openssl rand -base64 24)" >/dev/null \
    || fail "could not create secret/$SECRET"
  pass "created with a freshly generated password"
fi

# --- 2. the wiring -----------------------------------------------------------
# ADMIN_PASSWORD is the break-glass credential; the bootstrap admin falls back
# to it, so one secret serves both.
step "Wiring identity-access"
kubectl -n "$NS" patch deploy identity-access --type=strategic -p "$(cat <<JSON
{"spec":{"template":{"spec":{"containers":[{
  "name":"identity-access",
  "env":[
    {"name":"ADMIN_PASSWORD","valueFrom":{"secretKeyRef":{"name":"$SECRET","key":"password"}}},
    {"name":"IAM_BOOTSTRAP_ADMIN_EMAIL","value":"$ADMIN_EMAIL"},
    {"name":"IAM_BOOTSTRAP_ADMIN_TENANT","value":"$ADMIN_TENANT"}
  ]}]}}}}
JSON
)" >/dev/null || fail "could not patch the deployment"
pass "ADMIN_PASSWORD from the secret; bootstrap admin $ADMIN_EMAIL in $ADMIN_TENANT"

# --- 3. the image ------------------------------------------------------------
step "Rolling out identity-access:$IMAGE_TAG"
kubectl -n "$NS" set image deploy/identity-access \
  "identity-access=alimazraeh/operan-02-identity-access:$IMAGE_TAG" >/dev/null \
  || fail "could not set the image"
kubectl -n "$NS" rollout status deploy/identity-access --timeout=180s || fail "rollout did not complete"

got=$(kubectl -n "$NS" get deploy identity-access \
  -o jsonpath='{.spec.template.spec.containers[0].image}')
[ "$got" = "alimazraeh/operan-02-identity-access:$IMAGE_TAG" ] \
  || fail "image read back as $got — a kubectl apply can silently revert a set image"
pass "image read back: $got"

# --- 4. did it provision the account? ---------------------------------------
step "Bootstrap result (from the module's own log)"
kubectl -n "$NS" logs deploy/identity-access --tail=40 | grep -i 'BOOTSTRAP' || true

step "The account, as stored"
kubectl -n "$NS" exec postgresql-0 -- psql -U operan -d operan -c \
  "select email, tenant_id, (password_hash is not null) as has_password, status
     from users where email = '$ADMIN_EMAIL';"

cat <<EOF

-----------------------------------------------------------------
Sign in at http://192.168.8.210:30088/app/ with:

  email   $ADMIN_EMAIL
  tenant  $ADMIN_TENANT
  password:
    kubectl -n $NS get secret $SECRET -o jsonpath='{.data.password}' | base64 -d

The hardcoded "operan-admin-2026" no longer authenticates anywhere.
Break-glass login (blank email) takes that same password.

Each NEW tenant still needs its own user — provisioning-time seeding is
not built yet. Re-run with ADMIN_TENANT=<new-tenant> to add one.
-----------------------------------------------------------------
EOF
