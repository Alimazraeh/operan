# Module 16 — Governance & Compliance Engine

> Audit and compliance tracking. Audit logging, compliance monitoring, regulatory reporting, access control, policy enforcement, and data governance.

---

## Description

The Governance & Compliance Engine provides a centralized system for tracking, auditing, and enforcing organizational policies and compliance standards. It enables organizations to maintain regulatory compliance (GDPR, HIPAA, SOC 2, ISO 27001, PCI-DSS, FedRAMP), enforce security policies, manage access controls, and govern data lifecycle and retention.

---

## Key Features

- **Audit Logging**: Comprehensive, immutable audit trails for all system actions.
- **Policy Management**: Create, enforce, and manage policies across data access, processing, privacy, security, and operations.
- **Compliance Standards**: Define and track compliance against frameworks like GDPR, HIPAA, SOC 2, ISO 27001, PCI-DSS, FedRAMP, and custom frameworks.
- **Access Control**: Role-based and attribute-based access control with real-time checking.
- **Data Governance**: Data classification, PII tracking, encryption settings, and data residency controls.
- **Data Retention**: Configurable retention policies with automated data lifecycle management.
- **Compliance Dashboard**: Real-time visibility into compliance posture across all standards.

---

## API Documentation

| Resource | Method | Endpoint | Description |
|----------|--------|----------|-------------|
| **Audit Logs** | `GET` | `/api/v1/governance/audit-logs` | List audit logs |
| | `GET` | `/api/v1/governance/audit-logs/{id}` | Get audit log entry |
| | `POST` | `/api/v1/governance/audit-logs/export` | Export audit logs |
| **Policies** | `POST` | `/api/v1/governance/policies` | Create a policy |
| | `GET` | `/api/v1/governance/policies` | List policies |
| | `GET` | `/api/v1/governance/policies/{id}` | Get policy details |
| | `PATCH` | `/api/v1/governance/policies/{id}` | Update policy |
| | `DELETE` | `/api/v1/governance/policies/{id}` | Delete policy |
| | `POST` | `/api/v1/governance/policies/{id}/enforce` | Enforce policy check |
| **Compliance Standards** | `POST` | `/api/v1/governance/compliance-standards` | Create compliance standard |
| | `GET` | `/api/v1/governance/compliance-standards` | List compliance standards |
| | `GET` | `/api/v1/governance/compliance-standards/{id}` | Get compliance standard details |
| | `PATCH` | `/api/v1/governance/compliance-standards/{id}` | Update compliance standard |
| | `DELETE` | `/api/v1/governance/compliance-standards/{id}` | Delete compliance standard |
| | `POST` | `/api/v1/governance/compliance-standards/{id}/assess` | Assess compliance |
| | `GET` | `/api/v1/governance/compliance-standards/{id}/reports` | Get compliance reports |
| | `POST` | `/api/v1/governance/compliance-standards/{id}/reports/export` | Export compliance report |
| **Access Controls** | `POST` | `/api/v1/governance/access-controls` | Create access control rule |
| | `GET` | `/api/v1/governance/access-controls` | List access controls |
| | `GET` | `/api/v1/governance/access-controls/{id}` | Get access control rule |
| | `PATCH` | `/api/v1/governance/access-controls/{id}` | Update access control rule |
| | `DELETE` | `/api/v1/governance/access-controls/{id}` | Delete access control rule |
| | `POST` | `/api/v1/governance/access-controls/{id}/check` | Check access |
| **Data Governance** | `GET` | `/api/v1/governance/data-governance` | Get data governance settings |
| | `PATCH` | `/api/v1/governance/data-governance` | Update data governance settings |
| | `GET` | `/api/v1/governance/data-retention` | List data retention policies |
| | `POST` | `/api/v1/governance/data-retention` | Create data retention policy |
| | `GET` | `/api/v1/governance/data-retention/{id}` | Get data retention policy |
| | `PATCH` | `/api/v1/governance/data-retention/{id}` | Update data retention policy |
| | `DELETE` | `/api/v1/governance/data-retention/{id}` | Delete data retention policy |
| **Dashboard** | `GET` | `/api/v1/governance/compliance-dashboard` | Get compliance dashboard |
| **Health** | `GET` | `/api/v1/governance/health` | Service health check |

See also `../../contracts/v1/openapi-16-execution-sandbox.yaml` and `../../contracts/v1/schema-16-execution-sandbox.json` for OpenAPI contract and JSON schemas.

---

## API Schemas

| Category | Schema | Description |
|----------|--------|-------------|
| Audit Logs | CreateAuditLogEntry, AuditLogEntry, ExportAuditRequest | Audit log entry creation, retrieval, and export |
| Policies | CreatePolicyRequest, Policy, UpdatePolicyRequest, EnforcementCheckRequest, EnforcementResult | Policy management and enforcement |
| Compliance | CreateComplianceStandardRequest, ComplianceStandard, UpdateComplianceStandardRequest, ComplianceAssessmentRequest, ComplianceAssessment, ExportReportRequest | Compliance standard definition, assessment, and reporting |
| Access Control | CreateAccessControlRequest, AccessControl, UpdateAccessControlRequest, AccessCheckRequest, AccessCheckResult | Access control rule management and real-time checking |
| Data Governance | DataGovernanceSettings, UpdateDataGovernanceRequest, CreateRetentionPolicyRequest, RetentionPolicy, UpdateRetentionPolicyRequest | Data governance settings and retention policies |
| Dashboard | ComplianceDashboard, HealthStatus | Overall compliance overview and service health |

---

## Data Flow

```
[Operan Services] ──► [Governance Engine] ──► [Audit Log Store]
                                │
                                ▼
                    [Policy Enforcement Engine]
                                │
                    ┌───────────┼───────────┐
                    ▼           ▼           ▼
              [Access Control] [Compliance] [Data Retention]
                    │           │           │
                    ▼           ▼           ▼
              [Check Results] [Assessments] [Auto-Purge]
                                │
                                ▼
                    [Compliance Dashboard]
```

---

## Compliance Frameworks

| Framework | Coverage | Key Requirements |
|-----------|----------|------------------|
| **GDPR** | EU data protection | Consent management, data subject rights, breach notification, data portability |
| **HIPAA** | US healthcare | PHI protection, access controls, audit controls, integrity controls |
| **SOC 2** | Trust principles | Security, availability, processing integrity, confidentiality, privacy |
| **ISO 27001** | Information security | Risk assessment, access control, cryptography, incident management |
| **PCI-DSS** | Payment card | Network security, encryption, access control, monitoring, testing |
| **FedRAMP** | US government | Cloud security, continuous monitoring, authorization |

---

## Example

```bash
# Create a GDPR compliance standard
curl -X POST /api/v1/governance/compliance-standards \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GDPR Compliance 2025",
    "framework": "gdpr",
    "version": "1.0",
    "requirements": [
      {"id": "art5", "description": "Principles relating to processing of personal data"},
      {"id": "art7", "description": "Conditions for consent"},
      {"id": "art15", "description": "Right of access by the data subject"}
    ]
  }'

# Run a compliance assessment
curl -X POST /api/v1/governance/compliance-standards/{id}/assess \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Content-Type: application/json" \
  -d '{"scope": ["data-processing", "access-control", "storage"]}'

# View compliance dashboard
curl -X GET /api/v1/governance/compliance-dashboard?tenant_id=$TENANT_ID \
  -H "Authorization: Bearer $TOKEN"
```

---

## Governance Rules

1. All policy changes require audit logging with actor identification.
2. Compliance assessments must be re-run when frameworks or standards are updated.
3. Access control checks are logged in the audit trail for all sensitive operations.
4. Data retention policies are enforced automatically with configurable grace periods.
5. Compliance reports can be exported in PDF, CSV, or JSON formats.
6. PII data is classified and tracked according to data governance settings.
7. Encryption at rest and in transit must be enabled for compliance standards.

---

## Module 16 — Governance & Compliance Engine (Summary)

- **Category**: Data Management
- **Status**: Ready
- **API**: `/api/v1/governance`
- **OpenAPI**: `openapi-16-execution-sandbox.yaml`
- **Schema**: `schema-16-execution-sandbox.json`
- **Key Resources**: audit-logs, policies, compliance-standards, access-controls, data-governance
- **Frameworks**: GDPR, HIPAA, SOC 2, ISO 27001, PCI-DSS, FedRAMP
- **Purpose**: Audit, compliance tracking, policy enforcement, access control, and data governance

---

*Next: Module 17 — [TBD]*
