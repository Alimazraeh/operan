# Operan Identity & Access Management (IAM)

Operan module 02 — Identity & Access Management. Provides user management, RBAC, ABAC, SSO/SCIM provisioning, MFA, LDAP/AD integration, and delegated administration.

## Quick Start

### Run locally

```bash
# Set required environment variables
export AUTHENTIK_SERVER_URL=https://authentik.operan.internal
export AUTHENTIK_ADMIN_API_TOKEN=your-admin-token
export IAM_TOKEN_SECRET=a-strong-random-secret

# Run
go run ./cmd/identity-access
```

### Docker

```bash
docker build -t operan-identity-access .
docker run -p 8002:8002 \
  -e AUTHENTIK_SERVER_URL=https://authentik.operan.internal \
  -e AUTHENTIK_ADMIN_API_TOKEN=your-admin-token \
  -e IAM_TOKEN_SECRET=a-strong-random-secret \
  operan-identity-access
```

### Kubernetes (Helm)

```bash
helm install operan-iam ./helm \
  --set authentik.serverURL=https://authentik.operan.internal \
  --set authentik.adminAPIToken=your-admin-token \
  --set tokenSecret=a-strong-random-secret
```

## Configuration Reference

All configuration is via environment variables:

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `IAM_PORT` | `8002` | No | HTTP listen port |
| `IAM_TOKEN_SECRET` | *(none)* | **Yes** | HMAC signing secret for internal service/agent tokens. Must be a strong random string. |
| `IAM_TOKEN_EXPIRY_MIN` | `60` | No | JWT token expiry in minutes |
| `IAM_EVENT_BROKER_URL` | *(empty — log-only)* | No | Kafka broker address (`host:port`) for event publishing; empty = log-only |
| `IAM_OTEL_ENABLED` | `true` | No | Enable OpenTelemetry tracing |
| `IAM_OTEL_COLLECTOR_URL` | `http://otel-collector:4317` | No | OTLP collector endpoint |
| `AUTHENTIK_SERVER_URL` | *(none)* | **Yes** | Base URL of Authentik API |
| `AUTHENTIK_ADMIN_API_TOKEN` | *(none)* | **Yes** | API token with admin privileges |
| `AUTHENTIK_TOKEN_TTL_MIN` | `0` | No | Per-tenant API token TTL in minutes (0 = no expiry) |
| `AUTHENTIK_PROVISIONING_METHOD` | `none` | No | `"none"`, `"docker-compose"`, or `"helm"` |
| `AUTHENTIK_INGRESS_DOMAIN` | `auth.operan.internal` | No | Domain for Ingress resources |

> **Security:** `IAM_TOKEN_SECRET` must be set to a strong, randomly generated value. The service will refuse to start with an empty or default value.

## Authentik Integration

This module manages per-tenant Authentik resources (tenant, application, OIDC/SAML providers, RBAC roles, branding) through the TenantManager. The `SetupTenant` endpoint provisions all resources automatically.

## API Endpoints

All endpoints are under `/api/v1/iam`.

### Users
- `POST /api/v1/iam/users` — Create user
- `GET /api/v1/iam/users` — List users
- `GET /api/v1/iam/users/{id}` — Get user
- `PATCH /api/v1/iam/users/{id}` — Update user
- `DELETE /api/v1/iam/users/{id}` — Deactivate user
- `PUT /api/v1/iam/users/{id}/roles` — Set user roles

### Roles
- `POST /api/v1/iam/roles` — Create role
- `GET /api/v1/iam/roles` — List roles
- `GET /api/v1/iam/roles/{id}` — Get role
- `DELETE /api/v1/iam/roles/{id}` — Delete role

### Service Identities
- `POST /api/v1/iam/service-identities` — Create service identity
- `GET /api/v1/iam/service-identities` — List service identities
- `GET /api/v1/iam/service-identities/{id}` — Get service identity

### Agent Identities
- `POST /api/v1/iam/agent-identities` — Register agent identity
- `GET /api/v1/iam/agent-identities` — List agent identities
- `GET /api/v1/iam/agent-identities/agent/{agent_id}` — Get agent by ID

### SSO/SCIM
- `POST /api/v1/iam/auth/sso/configure` — Configure SSO
- `POST /api/v1/iam/auth/sso/test` — Test SSO connection
- `GET /api/v1/iam/auth/sso/config` — Get SSO config
- `POST /api/v1/iam/scim/users` — Provision user
- `PATCH /api/v1/iam/scim/users` — Update user
- `DELETE /api/v1/iam/scim/users` — Delete user
- `POST /api/v1/iam/scim/bulk` — Bulk provisioning

### MFA
- `POST /api/v1/iam/mfa/enroll` — Enroll MFA
- `POST /api/v1/iam/mfa/verify` — Verify MFA
- `POST /api/v1/iam/mfa/disable` — Disable MFA
- `GET /api/v1/iam/mfa/enrolled` — List enrolled devices
- `POST /api/v1/iam/mfa/recovery-codes` — Regenerate recovery codes

### LDAP / AD
- `POST /api/v1/iam/auth/ldap/configure` — Configure LDAP
- `GET /api/v1/iam/auth/ldap/config` — Get LDAP config
- `PATCH /api/v1/iam/auth/ldap/config` — Update LDAP config
- `DELETE /api/v1/iam/auth/ldap/config` — Delete LDAP config
- `POST /api/v1/iam/auth/ldap/test` — Test LDAP connection
- `POST /api/v1/iam/auth/ad/configure` — Configure Active Directory
- `GET /api/v1/iam/auth/ad/config` — Get AD config
- `PATCH /api/v1/iam/auth/ad/config` — Update AD config
- `DELETE /api/v1/iam/auth/ad/config` — Delete AD config
- `POST /api/v1/iam/auth/ad/test` — Test AD connection

### Delegated Administration
- `POST /api/v1/iam/admin/delegation` — Create delegation role
- `POST /api/v1/iam/admin/delegations` — List/create delegation roles
- `GET /api/v1/iam/admin/delegations/{id}` — Get delegation role
- `PATCH /api/v1/iam/admin/delegations/{id}` — Update delegation role
- `DELETE /api/v1/iam/admin/delegations/{id}` — Delete delegation role
- `POST /api/v1/iam/admin/delegations/{id}/grant` — Grant delegation
- `POST /api/v1/iam/admin/delegations/{id}/revoke` — Revoke delegation
- `GET /api/v1/iam/admin/delegations/{id}/delegations` — List delegation grants

### ABAC
- `POST /api/v1/iam/abac/evaluate` — Evaluate ABAC policy
- `POST /api/v1/iam/abac/policies` — Create ABAC policy
- `GET /api/v1/iam/abac/policies` — List ABAC policies
- `GET /api/v1/iam/abac/policies/{id}` — Get ABAC policy
- `DELETE /api/v1/iam/abac/policies/{id}` — Delete ABAC policy

### Audit & RBAC
- `GET /api/v1/iam/audit/trails` — List audit trails
- `GET /api/v1/iam/audit/trails/{id}` — Get audit trail
- `GET /api/v1/iam/audit/session-replay/{id}` — Get session replay
- `POST /api/v1/iam/rbac/evaluate` — Evaluate RBAC

### Health
- `GET /health` — Health check (always returns 200)
- `GET /ready` — Readiness check (checks Authentik, event broker, etc.)

## Testing

```bash
go test ./...
go test ./... -cover
```

## Module Health

```bash
# Module status
curl http://localhost:8002/health

# Readiness with dependency checks
curl http://localhost:8002/ready
```

## Architecture

```
┌──────────────────────────────────────────────┐
│              API Gateway / Load Balancer       │
└──────────────┬───────────────────────────────┘
               │
┌──────────────▼───────────────────────────────┐
│           Middleware Chain                     │
│  TraceInjector → AuthValidator → TenantInject  │
└──────────────┬───────────────────────────────┘
               │
┌──────────────▼───────────────────────────────┐
│              HTTP Handlers                     │
│  users │ roles │ service-ids │ agent-ids       │
│  sso │ scim │ mfa │ ldap │ ad │ delegations   │
│  abac │ audit │ rbac                              │
└──────────────┬───────────────────────────────┘
               │
  ┌────────────┼────────────┐
  ▼            ▼            ▼
┌──────┐  ┌────────┐  ┌─────────┐
│ Store│  │Authentik│  │Publisher│
│(In-Memory) │(API)   │  │(AMQP)   │
└──────┘  └────────┘  └─────────┘
```

## License

Operan internal use.
