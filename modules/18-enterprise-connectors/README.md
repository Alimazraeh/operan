# Module 18 — Enterprise Connector Fabric

Standardized connectors to enterprise systems (CRM, ERP, email, calendar, document management) that feed enterprise data into the agent ecosystem.

## Overview

The Enterprise Connector Fabric provides:

- **Connector Registry** — Pluggable connector implementations (SMTP, Salesforce, HubSpot, M365, SAP, Generic REST)
- **Authentication** — OAuth2, API keys, Basic auth
- **Data Sync** — Full/incremental sync with enterprise systems
- **Tool Registration** — Automatic registration of connector capabilities as tools in M04
- **Status Monitoring** — Health checks, sync logs, error tracking
- **Event Publishing** — Kafka events for sync lifecycle

## Connector Types

| Type | Auth | Capabilities |
|------|------|-------------|
| `smtp` | Basic (username/password) | Send email, send HTML, send attachments |
| `salesforce` | OAuth2 | Accounts, Contacts, Opportunities |
| `hubspot` | API Key | Contacts, Companies, Deals |
| `m365` | OAuth2 (Azure AD) | Outlook email, calendar, SharePoint |
| `sap` | API Key / Basic / OAuth2 / SAML | Configurable REST endpoints |
| `generic_rest` | API Key / Bearer / Basic | Any REST API with user-defined endpoints |

## Directory Structure

```
modules/18-enterprise-connectors/
├── go.mod
├── main.go                    # Listen port 8018
├── Dockerfile
├── chart/
│   ├── Chart.yaml
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml
│       └── service.yaml
├── contracts/
│   └── openapi-18-enterprise-connectors.yaml
├── migrations/
│   └── 001_create_schema.sql
├── README.md
└── internal/
    ├── config/
    │   └── config.go
    ├── ctxkeys/
    │   └── ctxkeys.go
    ├── middleware/
    │   └── middleware.go
    ├── handler/
    │   ├── connectors.go
    │   ├── sync.go
    │   ├── tools.go
    │   └── router.go
    ├── store/
    │   ├── connectors.go
    │   ├── syncs.go
    │   └── models.go
    ├── connectors/
    │   ├── connector.go
    │   ├── smtp.go
    │   ├── salesforce.go
    │   ├── hubspot.go
    │   ├── m365.go
    │   ├── sap.go
    │   └── rest.go
    ├── sync/
    │   └── engine.go
    ├── events/
    │   └── events.go
    └── clients/
        └── m04.go
```

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Kafka (optional, for event publishing)

### Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8018` | HTTP listen port |
| `JWT_SECRET` | *(required)* | JWT signing secret |
| `JWT_ISSUER` | `operan-tenant-control-plane` | JWT issuer |
| `DB_DSN` | *(required)* | PostgreSQL connection string |
| `EVENT_BROKER_URL` | `""` | Kafka broker URL (empty = log-only) |
| `M04_BASE_URL` | `http://localhost:8004` | Module 04 Agent Registry URL |

### Build

```bash
go build ./...
```

### Run

```bash
export JWT_SECRET="your-secret"
export DB_DSN="postgres://user:pass@localhost:5432/operan?sslmode=disable"
go run main.go
```

### Docker

```bash
docker build -t operan/enterprise-connectors:latest .
docker run -p 8018:8018 \
  -e JWT_SECRET=your-secret \
  -e DB_DSN="postgres://user:pass@db:5432/operan" \
  operan/enterprise-connectors:latest
```

## API Endpoints

### Health

```
GET /health
```

### Connectors

```
GET    /v1/connectors           # List connectors (paginated, filterable)
POST   /v1/connectors           # Create connector
GET    /v1/connectors/{id}      # Get connector detail
DELETE /v1/connectors/{id}      # Delete connector
```

### Sync

```
POST   /v1/connectors/{id}/sync       # Trigger sync (type=full|incremental)
GET    /v1/connectors/{id}/health     # Health check for connector
GET    /v1/sync-history               # List sync history
```

### Tools

```
GET    /v1/tools                       # All connector tools
GET    /v1/connectors/{id}/tools       # Tools for specific connector
```

All routes (except `/health`) require `Authorization: Bearer <JWT>` and `X-Tenant-ID` headers.

## Database Schema

```sql
-- See migrations/001_create_schema.sql
CREATE TABLE connector_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(200) NOT NULL,
    connector_type VARCHAR(50) NOT NULL CHECK (connector_type IN (...)),
    auth_method VARCHAR(30) NOT NULL DEFAULT 'api_key',
    config JSONB NOT NULL DEFAULT '{}',
    credentials JSONB NOT NULL DEFAULT '{}',
    sync_frequency VARCHAR(30) NOT NULL DEFAULT 'manual',
    status VARCHAR(30) NOT NULL DEFAULT 'inactive',
    tools_registered BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE connector_sync_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    connector_id UUID NOT NULL REFERENCES connector_definitions(id),
    sync_type VARCHAR(30) NOT NULL DEFAULT 'full',
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    objects_fetched INT NOT NULL DEFAULT 0,
    objects_updated INT NOT NULL DEFAULT 0,
    objects_failed INT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INT
);
```

## Integration Points

| Module | Integration |
|--------|------------|
| M04 Agent Registry | Connector tools registered as agent capabilities |
| M01 Tenant Control Plane | Tenant-scoped connectors via JWT |
| M03 Agent Orchestration | M03 workflows reference connector tools |
| M07 Memory Fabric | Connector sync data indexed for semantic search |
| M19 Arabic Core | Connector data normalized via M19 before storage |
| M21 Experience Portal | Connector status, sync history, tool catalog |

## Connector Interface

All connectors implement the `Connector` interface:

```go
type Connector interface {
    Name() string
    Type() string
    ValidateConfig(config map[string]interface{}) error
    ValidateCredentials(ctx context.Context, credentials map[string]interface{}) (*HealthCheckResult, error)
    Sync(ctx context.Context, credentials map[string]interface{}, config map[string]interface{}) (*SyncResult, error)
    GetTools() []ToolDefinition
}
```

## Kafka Events

| Event | Description |
|-------|------------|
| `operan.connector.sync_started` | Sync initiated |
| `operan.connector.sync_completed` | Sync completed with counts |
| `operan.connector.sync_failed` | Sync failed with error |
| `operan.connector.tools_registered` | Tools registered with M04 |
| `operan.connector.health_changed` | Connector health status changed |

## Testing

```bash
go test ./... -v -count=1
```

Target: 35+ tests, 80%+ coverage.