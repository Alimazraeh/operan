# Operan — Tenant Control Plane (Module 01)

Tenant Control Plane is the foundational module of the Operan platform. It manages tenant lifecycle, billing, resource quotas, secrets, deployments, environments, namespaces, policies, and agents.

## Purpose & Scope

This module implements:

- **Tenant CRUD** — Provision, activate, suspend, and deprovision tenants
- **Subscription & Billing** — Plan management, quota tracking, invoicing, payment methods
- **Secrets Management** — Encrypted secret storage with versioning
- **Resource Orchestration** — Deployments, environments, namespaces, and policies
- **Agent Management** — Agent lifecycle and configuration per tenant

## Configuration

All configuration is read from environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `OTLP_ENDPOINT` | `http://localhost:4318` | OpenTelemetry collector endpoint |
| `LOG_ENV` | `production` | Log environment (`debug`/`production`) |
| `MODULE_VERSION` | `1.0.0` | Module version string |
| `EVENT_BUS_HOST` | `events.operan.internal` | Event bus host |
| `EVENT_BUS_PORT` | `9092` | Event bus port |
| `EVENT_BUS_PROTO` | `kafka` | Event bus protocol; `kafka` enables publishing, any other value = log-only |
| `JWT_SECRET` | _(required)_ | JWT signing secret (HMAC-S256). Must be ≥32 bytes; startup fails if unset or default |
| `JWT_ISSUER` | `operan-tenant-control-plane` | JWT issuer claim value |

## Docker

Build and run:

```bash
docker build -t operan-tenant-control-plane .
docker run -p 8080:8080 \
  -e JWT_SECRET="your-32-byte-minimum-secret-string-here!!" \
  operan-tenant-control-plane
```

## Running Locally

```bash
go run ./cmd/tenant-control-plane
```

## Health Endpoint

```bash
curl http://localhost:8080/v1/status
```

Returns module status including uptime and version.

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/handler/...
go test ./internal/store/...
go test ./internal/middleware/...
```

## Endpoints

All endpoints are under `/v1/tenants/{id}/`. Key endpoints include:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/status` | Module health/status |
| `GET` | `/v1/tenants` | List tenants |
| `POST` | `/v1/tenants` | Create tenant |
| `GET` | `/v1/tenants/{id}` | Get tenant |
| `PATCH` | `/v1/tenants/{id}` | Patch tenant |
| `DELETE` | `/v1/tenants/{id}` | Delete tenant |
| `GET` | `/v1/tenants/{id}/subscriptions` | List subscriptions |
| `PATCH` | `/v1/tenants/{id}/subscriptions` | Patch subscription |
| `POST` | `/v1/tenants/{id}/subscriptions/upgrade` | Upgrade subscription |
| `GET` | `/v1/tenants/{id}/billing/usage` | Billing usage |
| `POST` | `/v1/tenants/{id}/billing/invoices/{id}/download` | Download invoice |
| `POST` | `/v1/tenants/{id}/secrets` | Create secret |
| `GET` | `/v1/tenants/{id}/secrets` | List secrets |
| `POST` | `/v1/tenants/{id}/deployments` | Create deployment |
| `POST` | `/v1/tenants/{id}/deployments/{id}/rollback` | Rollback deployment |
| `POST` | `/v1/tenants/{id}/environments` | Create environment |
| `POST` | `/v1/tenants/{id}/policies` | Create policy |
| `POST` | `/v1/tenants/{id}/agents` | Create agent |

See the OpenAPI contract at [`contracts/v1/openapi-01-tenant-control-plane.yaml`](../../../contracts/v1/openapi-01-tenant-control-plane.yaml) for the complete API specification.

## Architecture

```
cmd/tenant-control-plane/   # Entry point
internal/
├── config/                  # Configuration parsing
├── events/                  # Event publishing (Kafka; log-only fallback)
├── handler/                 # HTTP handlers & response types
├── middleware/              # JWT auth, request ID, tenant context
└── store/                   # In-memory data stores (12 stores)
```

## Development

### Adding a New Handler

1. Create `internal/handler/handler_<name>.go`
2. Register the route in `internal/handler/response_types.go` (the `RegisterRoutes` function)
3. Add unit tests in `internal/handler/handler_<name>_test.go`

### Tenant Isolation

All read operations use tenant-scoped store methods (`GetByIDAndTenant`) to ensure cross-tenant data isolation. The `JWTValidator` middleware extracts `tenant_id` from the JWT token and injects it into the request context.

## License

Proprietary — Operan
