# Module 05: Department Template Engine

> Manages department templates including creation, versioning, deployment, and lifecycle operations for multi-agent department configurations.

## Overview

The Department Template Engine provides a comprehensive system for defining, versioning, and deploying department-level AI agent configurations. It serves as the bridge between governance policies and agent orchestration, allowing organizations to create reusable template configurations that standardize department operations.

## Features

- **Template CRUD**: Create, read, update, and delete department templates
- **Version Management**: Immutable version snapshots with automatic incrementing
- **Template Cloning**: Create variants of existing templates
- **Custom Templates**: User-defined custom templates with flexible content
- **Deployment Pipeline**: Multi-stage deployment workflow (select → configure → connect_data → provision_memory → deploy_swarm → operational)
- **Template Versioning**: Track all changes with immutable version history
- **Tenant Isolation**: Full multi-tenancy support with per-tenant data isolation
- **Event-Driven**: AsyncAPI-compliant event publishing for all lifecycle operations
- **RESTful API**: OpenAPI 3.0.3-compliant REST API

## Architecture

### Core Components

```
┌─────────────────────────────────────────────────────────┐
│                   HTTP Handlers                         │
│  (templates.go, custom_templates.go,                    │
│   deployments.go, versions.go, router.go)               │
└───────────────────┬─────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────┐
│               Middleware Chain                          │
│  Logger → RequestID → TraceID → JWTAuth →              │
│  TenantContext                                          │
└───────────────────┬─────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────┐
│                In-Memory Stores                         │
│  (TemplateStore, CustomTemplateStore,                   │
│   DeploymentStore, VersionStore)                        │
└───────────────────┬─────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────┐
│              Event Publisher                            │
│  (8 AsyncAPI channels, logBroker for dev)              │
└─────────────────────────────────────────────────────────┘
```

### Directory Structure

```
modules/05-department-template-engine/
├── main.go                          # Entry point, server setup
├── go.mod                           # Go module definition
├── Dockerfile                       # Multi-stage Docker build
├── manifest.json                    # Platform manifest
├── README.md                        # This file
├── chart/                           # Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── _helpers.tpl
│       ├── deployment.yaml
│       ├── hpa.yaml
│       ├── ingress.yaml
│       ├── service.yaml
│       └── serviceaccount.yaml
└── internal/
    ├── config/
    │   ├── config.go               # Environment-based configuration
    │   └── config_test.go          # Configuration tests
    ├── ctxkeys/
    │   └── ctxkeys.go              # Typed context keys
    ├── events/
    │   ├── events.go               # Event publisher and payloads
    │   └── events_test.go          # Event tests
    ├── handlers/
    │   ├── helpers.go              # JSON/err response helpers
    │   ├── templates.go            # Template CRUD handlers
    │   ├── custom_templates.go     # Custom template handlers
    │   ├── deployments.go          # Deployment handlers
    │   ├── versions.go             # Version/clone handlers
    │   ├── router.go               # Route registration
    │   └── handlers_test.go        # Handler tests
    ├── middleware/
    │   ├── middleware.go           # JWT, tenant, request ID, logger
    │   └── middleware_test.go      # Middleware tests
    └── store/
        ├── models.go               # Domain models, DTOs, errors
        ├── templates.go            # Template store
        ├── custom_templates.go     # Custom template store
        ├── deployments.go          # Deployment store
        ├── versions.go             # Version store
        └── store_test.go           # Store tests
```

## API Endpoints

### Templates

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/templates` | Create a new template |
| `GET` | `/api/v1/templates` | List templates (paginated) |
| `GET` | `/api/v1/templates/{id}` | Get template by ID |
| `PATCH` | `/api/v1/templates/{id}` | Update template (partial) |
| `DELETE` | `/api/v1/templates/{id}` | Delete template |
| `POST` | `/api/v1/templates/{id}/deploy` | Deploy template to environment |
| `POST` | `/api/v1/templates/{id}/clone` | Clone template |
| `POST` | `/api/v1/templates/{id}/version` | Create new version |

### Custom Templates

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/custom-templates` | Create custom template |
| `GET` | `/api/v1/custom-templates` | List custom templates |
| `GET` | `/api/v1/custom-templates/{id}` | Get custom template |
| `PATCH` | `/api/v1/custom-templates/{id}` | Update custom template |
| `DELETE` | `/api/v1/custom-templates/{id}` | Delete custom template |

### Deployments

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/deployments` | List all deployments |
| `GET` | `/api/v1/deployments/{id}` | Get deployment by ID |
| `PATCH` | `/api/v1/deployments/{id}` | Update deployment status |

### Versions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/versions` | List all versions |
| `GET` | `/api/v1/versions/{id}` | Get version by ID |
| `GET` | `/api/v1/versions/template/{template_id}` | Get versions for template |

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check endpoint |

## Event Topics

| Event | Topic | Description |
|-------|-------|-------------|
| `template.created` | `operan.templates.template.created` | Template created |
| `template.updated` | `operan.templates.template.updated` | Template updated |
| `template.deleted` | `operan.templates.template.deleted` | Template deleted |
| `template.versioned` | `operan.templates.template.versioned` | New version created |
| `template.deployed` | `operan.templates.template.deployed` | Template deployed |
| `template.deployment_failed` | `operan.templates.template.deployment_failed` | Deployment failed |
| `template.undeployed` | `operan.templates.template.undeployed` | Template undeployed |
| `template.cloned` | `operan.templates.template.cloned` | Template cloned |

## Configuration

All configuration is through environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MODULE05_PORT` | `8005` | HTTP port |
| `MODULE05_JWT_SECRET` | *(required)* | HMAC-S256 shared secret |
| `MODULE05_DB_URL` | *(empty)* | PostgreSQL connection URL |
| `MODULE05_REDIS_URL` | *(empty)* | Redis connection URL |
| `MODULE05_EVENT_BROKER_URL` | *(empty)* | Event broker URL (AMQP) |
| `MODULE05_OTLP_ENDPOINT` | `http://localhost:4318` | OpenTelemetry endpoint |
| `MODULE05_TEMPLATE_CACHE_TTL` | `300` | Template cache TTL in seconds |
| `MODULE05_MAX_PAGE_SIZE` | `100` | Maximum page size |

## Authentication

Module 05 uses HMAC-S256 JWT authentication. All API requests (except `/health`) require:

```
Authorization: Bearer <jwt_token>
X-Tenant-ID: <tenant_id>
```

## Deployment

### Docker

```bash
docker build -t operan/module05-department-template-engine:latest .
docker run -p 8005:8005 \
  -e MODULE05_JWT_SECRET=my-secret \
  operan/module05-department-template-engine:latest
```

### Kubernetes (Helm)

```bash
helm install module05 ./chart \
  --set image.tag=v1.0.0 \
  --set env.MODULE05_JWT_SECRET=my-secret
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run with verbose output
go test ./... -v
```

## Module Dependencies

Module 05 integrates with:
- **Module 01** (Tenant Control Plane): Tenant context validation
- **Module 03** (Agent Orchestration): Agent provisioning from templates
- **Module 04** (Agent Registry): Agent discovery and management
- **Module 07** (Memory Fabric): Memory topology configuration
- **Module 10** (Policy Governance): Governance rule enforcement
- **Module 11** (Observability): Metrics and tracing

## Contract Compliance

- **OpenAPI 3.0.3**: 15 operations across 4 resource types
- **AsyncAPI 2.6.0**: 8 event channels
- **JSON Schema**: Complete request/response schemas
- **RFC 7807**: Standardized error responses
- **Platform Standards**: BearerAuth, X-Tenant-ID, has_more pagination, additionalProperties: false

## License

Proprietary - Operan Systems
