# Operan on Kubernetes (single-node demo)

Deploys all nine implemented services + single-node Kafka into an `operan`
namespace, pulling the images that [`docker-publish.yml`](../../.github/workflows/docker-publish.yml)
pushes to Docker Hub (`alimazraeh/operan-<module>:latest`).

## Prerequisites

- The GitHub Actions run for the latest commit has finished (all nine images
  pushed — check the Actions tab).
- `kubectl` pointed at your cluster.

## Deploy

```bash
cd deploy/k8s

kubectl apply -f namespace.yaml

# One shared HMAC-S256 secret, accepted by every module:
kubectl -n operan create secret generic operan-jwt \
  --from-literal=secret="$(openssl rand -base64 32)"

kubectl apply -f kafka.yaml
kubectl apply -f modules.yaml

kubectl -n operan get pods -w   # wait until 10/10 Running and Ready
```

## Run the demo flow against the cluster

The services are ClusterIP. From your machine, port-forward the ones the
demo script uses (each in its own terminal, or background them):

```bash
kubectl -n operan port-forward svc/tenant-control-plane 8080:8080 &
kubectl -n operan port-forward svc/identity-access      8002:8002 &
kubectl -n operan port-forward svc/agent-orchestration  8003:8080 &
kubectl -n operan port-forward svc/agent-registry       8083:8083 &
kubectl -n operan port-forward svc/department-templates 8005:8005 &
kubectl -n operan port-forward svc/memory-fabric        8007:8007 &
kubectl -n operan port-forward svc/tool-execution       8008:8008 &
kubectl -n operan port-forward svc/human-supervision    8009:8009 &
kubectl -n operan port-forward svc/observability        8011:8011 &

# demo.sh signs JWTs with the same secret the cluster uses:
DEMO_JWT_SECRET="$(kubectl -n operan get secret operan-jwt -o jsonpath='{.data.secret}' | base64 -d)" \
  ../demo/demo.sh
```

With Kafka live in the cluster, step 7 of the demo shows real numbers:
spans ingested from every module's events, `human_gate` spans from the
approval, per-tenant metrics, and component health.

## Notes

- **In-memory stores**: pod restarts clear data; re-run `demo.sh`.
- **identity-access** runs against a placeholder Authentik; its readiness
  probe is `/health` (liveness-style), so the pod reports Ready while
  identity operations stay inert — expected for the demo.
- Kafka data is an `emptyDir` (demo scope, not durable).
- Tear down with `kubectl delete namespace operan`.
