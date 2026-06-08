# Module 18 — Enterprise Connector Fabric

> Connect Operan to enterprise systems (SAP, Oracle, Salesforce, Microsoft 365, Google Workspace, ServiceNow, Slack, Teams, Jira, GitHub) with universal connectors, data mapping, and orchestration.

---

## Description

The Enterprise Connector Fabric provides a unified interface for connecting Operan to enterprise systems. It offers pre-built connectors for major platforms, flexible data mapping with transformations, scheduled and on-demand data synchronization, and multi-connector orchestration workflows. This module is essential for integrating Operan into existing enterprise technology stacks.

---

## Key Features

- **Universal Connectors**: Pre-built connectors for SAP, Oracle, Salesforce, Microsoft 365, Google Workspace, ServiceNow, Slack, Teams, Jira, and GitHub.
- **Multiple Authentication**: OAuth2, API key, basic auth, SSO, and certificate-based authentication for different enterprise systems.
- **Data Mapping**: Flexible field-level mapping between enterprise data models and Operan's internal schema with configurable transformations.
- **Data Synchronization**: Full and incremental sync with configurable schedules and monitoring.
- **Orchestration**: Multi-step workflows that chain connectors together with conditional logic and scheduling.
- **Connection Management**: Centralized management of connection endpoints, credentials, and status monitoring.

---

## API Documentation

| Resource | Method | Endpoint | Description |
|----------|--------|----------|-------------|
| **Connectors** | `POST` | `/api/v1/enterprise-connector/connectors` | Register enterprise connector |
| | `GET` | `/api/v1/enterprise-connector/connectors` | List enterprise connectors |
| | `GET` | `/api/v1/enterprise-connector/connectors/{id}` | Get connector details |
| | `PATCH` | `/api/v1/enterprise-connector/connectors/{id}` | Update connector |
| | `DELETE` | `/api/v1/enterprise-connector/connectors/{id}` | Delete connector |
| | `POST` | `/api/v1/enterprise-connector/connectors/{id}/test` | Test connector |
| | `POST` | `/api/v1/enterprise-connector/connectors/{id}/sync` | Trigger connector sync |
| | `GET` | `/api/v1/enterprise-connector/connectors/{id}/status` | Get connector status |
| **Connections** | `POST` | `/api/v1/enterprise-connector/connections` | Create data connection |
| | `GET` | `/api/v1/enterprise-connector/connections` | List data connections |
| | `GET` | `/api/v1/enterprise-connector/connections/{id}` | Get connection details |
| | `PATCH` | `/api/v1/enterprise-connector/connections/{id}` | Update connection |
| | `DELETE` | `/api/v1/enterprise-connector/connections/{id}` | Delete connection |
| **Data Mapping** | `POST` | `/api/v1/enterprise-connector/data-mappings` | Create data mapping |
| | `GET` | `/api/v1/enterprise-connector/data-mappings` | List data mappings |
| | `GET` | `/api/v1/enterprise-connector/data-mappings/{id}` | Get data mapping details |
| | `PATCH` | `/api/v1/enterprise-connector/data-mappings/{id}` | Update data mapping |
| | `DELETE` | `/api/v1/enterprise-connector/data-mappings/{id}` | Delete data mapping |
| **Data Sync** | `POST` | `/api/v1/enterprise-connector/data-syncs` | Create data sync job |
| | `GET` | `/api/v1/enterprise-connector/data-syncs` | List data sync jobs |
| | `GET` | `/api/v1/enterprise-connector/data-syncs/{id}` | Get data sync job details |
| | `DELETE` | `/api/v1/enterprise-connector/data-syncs/{id}` | Cancel data sync job |
| | `GET` | `/api/v1/enterprise-connector/data-syncs/{id}/logs` | Get data sync logs |
| **Orchestration** | `POST` | `/api/v1/enterprise-connector/orchestrations` | Create orchestration flow |
| | `GET` | `/api/v1/enterprise-connector/orchestrations` | List orchestrations |
| | `GET` | `/api/v1/enterprise-connector/orchestrations/{id}` | Get orchestration details |
| | `PATCH` | `/api/v1/enterprise-connector/orchestrations/{id}` | Update orchestration |
| | `DELETE` | `/api/v1/enterprise-connector/orchestrations/{id}` | Delete orchestration |
| | `POST` | `/api/v1/enterprise-connector/orchestrations/{id}/run` | Run orchestration flow |
| **Health** | `GET` | `/api/v1/enterprise-connector/health` | Service health check |

See also `../../contracts/v1/openapi-18-enterprise-connector-fabric.yaml` and `../../contracts/v1/schema-18-enterprise-connector-fabric.json` for OpenAPI contract and JSON schemas.

---

## API Schemas

| Category | Schema | Description |
|----------|--------|-------------|
| Connectors | RegisterConnectorRequest, Connector, UpdateConnectorRequest, ConnectorTestResult, ConnectorStatus | Connector registration, testing, and monitoring |
| Connections | CreateConnectionRequest, Connection, UpdateConnectionRequest | Endpoint and credential management |
| Data Mapping | CreateDataMappingRequest, DataMapping, UpdateDataMappingRequest | Field mapping and transformations |
| Data Sync | CreateDataSyncRequest, DataSync, DataSyncLog | Synchronization jobs and monitoring |
| Orchestration | CreateOrchestrationRequest, Orchestration, UpdateOrchestrationRequest, OrchestrationRun | Multi-connector workflow orchestration |
| Health | HealthStatus | Service health monitoring |

---

## Data Flow

```
[Enterprise Systems] ◄► [Connector Fabric]
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
        [SAP Conn] [SFDC Conn] [M365 Conn]
              │           │           │
              └───────────┼───────────┘
                          ▼
                [Data Mapping Engine]
                          │
                          ▼
            [Transformation Pipeline]
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
      [Full Sync]  [Incremental]  [On-Demand]
            │             │             │
            └─────────────┼─────────────┘
                          ▼
                [Orchestration Engine]
                          │
                          ▼
                  [Operan Data Store]
```

---

## Example

```bash
# Register a Salesforce connector
curl -X POST /api/v1/enterprise-connector/connectors \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Salesforce",
    "type": "salesforce",
    "auth_type": "oauth2",
    "config": {
      "instance_url": "https://myorg.salesforce.com",
      "client_id": "$SFDC_CLIENT_ID",
      "client_secret": "$SFDC_CLIENT_SECRET"
    },
    "tags": ["production", "crm"]
  }'

# Create a data mapping for contacts
curl -X POST /api/v1/enterprise-connector/data-mappings \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "SFDC Contact to Operan Contact",
    "connector_id": "conn-sfdc-123",
    "source": {
      "entity": "Contact",
      "fields": {"FirstName": "first_name", "LastName": "last_name", "Email": "email"}
    },
    "target": {
      "entity": "contact",
      "fields": {"first_name": "$.FirstName", "last_name": "$.LastName", "email": "$.Email"}
    },
    "transformations": [
      {"field": "email", "type": "lowercase"}
    ]
  }'

# Trigger a sync job
curl -X POST /api/v1/enterprise-connector/data-syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Nightly Contact Sync", "source_connector_id": "conn-sfdc-123", "sync_type": "incremental"}'
```

---

## Connector Fabric Rules

1. Each connector must pass a connectivity test before being set to active status.
2. Data mappings preserve source data integrity; transformations are applied as a non-destructive layer.
3. Incremental syncs use the enterprise system's change tracking (modified date, CDC, or similar).
4. Full syncs replace all mapped data in the target entity for the selected period.
5. Orchestration steps execute sequentially unless explicitly configured for parallel execution.
6. Connector credentials are encrypted at rest and never logged or exposed in API responses.
7. All sync and orchestration runs are logged for audit and troubleshooting purposes.

---

## Module 18 — Enterprise Connector Fabric (Summary)

- **Category**: Integration & Connectivity
- **Status**: Ready
- **API**: `/api/v1/enterprise-connector`
- **OpenAPI**: `openapi-18-enterprise-connector-fabric.yaml`
- **Schema**: `schema-18-enterprise-connector-fabric.json`
- **Key Resources**: connectors, connections, data-mappings, data-syncs, orchestrations
- **Supported Platforms**: SAP, Oracle, Salesforce, Microsoft 365, Google Workspace, ServiceNow, Slack, Teams, Jira, GitHub
- **Purpose**: Enterprise integration, data synchronization, and workflow orchestration

---

*Next: Module 19 — Arabic Language Intelligence Layer*
