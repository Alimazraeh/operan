# Operan — Agent Registry (Module 04)

Agent Registry is the central catalog for all agents in the Operan platform. It manages agent lifecycle, version control, capability tracking, dependency graphs, and cross-environment promotions.

## Purpose & Scope

This module implements:

- **Agent CRUD** — Register, list, retrieve, update, and deprecate agents
- **Version Management** — Create versions, promote across environments (dev → staging → production)
- **Capability Indexing** — Track and index agent capabilities with scoring
- **Dependency Management** — Define hard/soft dependencies between agents
- **Event Publishing** — Emit domain events for all lifecycle changes

## Configuration

All configuration is read from environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `OTLP_ENDPOINT` | `http://localhost:4318` | OpenTelemetry collector endpoint |
| `LOG_ENV` | `production` | Log environment (`debug`/`production`) |
| `MODULE_VERSION` | `1.0.0` | Module version string |
| `EVENT_BUS_HOST` | `events.operan.internal` | Kafka broker host |
| `EVENT_BUS_PORT` | `9092` | Kafka broker port |
| `EVENT_BUS_PROTO` | `kafka` | Event bus protocol |
| `JWT_SECRET` | _(required)_ | JWT signing secret (HMAC-S256). Must be ≥32 bytes; startup fails on the default value |
| `JWT_ISSUER` | `operan-agent-registry` | JWT issuer claim value |
| `DB_HOST` | _(not set)_ | PostgreSQL host (optional) |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `agent_registry` | PostgreSQL database name |
| `DB_USER` | _(not set)_ | PostgreSQL user (optional) |
| `DB_PASSWORD` | _(not set)_ | PostgreSQL password (optional) |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/registry/agents` | List agents |
| POST | `/registry/agents` | Register agent |
| GET | `/registry/agents/{id}` | Get agent |
| PATCH | `/registry/agents/{id}` | Update agent |
| POST | `/registry/agents/{id}/deprecate` | Deprecate agent |
| GET | `/registry/agents/{id}/versions` | List versions |
| POST | `/registry/agents/{id}/versions` | Create version |
| GET | `/registry/agents/{id}/versions/{versionId}` | Get version |
| POST | `/registry/agents/{id}/versions/{versionId}/promote` | Promote version |
| POST | `/registry/agents/{id}/capabilities/index` | Index capabilities |
| GET | `/registry/agents/{id}/dependencies` | List dependencies |
| POST | `/registry/agents/{id}/dependencies` | Add dependency |
| DELETE | `/registry/agents/{id}/dependencies` | Remove dependency |

## Events

| Topic | Description |
|-------|-------------|
| `agent.registered` | New agent registered |
| `agent.capabilities_updated` | Capabilities modified |
| `agent.version_created` | New version created |
| `agent.promoted` | Version promoted across environments |
| `agent.deprecated` | Agent deprecated |
| `agent.archived` | Agent archived |
| `dependency.added` | Dependency added |
| `dependency.removed` | Dependency removed |

## Docker

Build and run:

```bash
docker build -t operan-agent-registry .
docker run -p 8080:8080 \
  -e JWT_SECRET="your-32-byte-minimum-secret-string-here!!" \
  operan-agent-registry
```

## Running Locally

```bash
go run .
```

## Health Endpoints

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## Helm

Deploy to Kubernetes:

```bash
helm install operan-agent-registry modules/04-agent-registry/helm/ \
  --set jwtSecret="your-32-byte-minimum-secret-string-here!!" \
  --set eventBusHost="events.operan.internal" \
  --set eventBusPort="9092"
```

## Testing

```bash
go test ./...
go test ./... -cover
```
