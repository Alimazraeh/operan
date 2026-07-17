# Enterprise Department Templates

## Template Design Framework

This template library is designed around **three enterprise sizes**, each with distinct operational characteristics, governance requirements, and agent staffing models.

### Enterprise Size Definitions

| Size | Department Headcount | Characteristics | Governance Level |
|------|---------------------|-----------------|------------------|
| **Small (SMB)** | 2–5 FTE | Flat structure, generalist roles, lightweight processes, founder-led | Minimal compliance, manual approvals, ad-hoc reporting |
| **Medium (Mid-Market)** | 5–20 FTE | Specialized roles, defined processes, department manager + team leads | Formalized SOPs, segregation of duties, monthly reviews |
| **Large (Enterprise)** | 20–50+ FTE | Sub-teams, matrix reporting, center-of-excellence model | Full compliance framework, audit trails, multi-level approval, SLAs |

### Design Principles

Each template follows the **GARDEN** framework:

- **G**overnance — rules, constraints, compliance guardrails
- **A**gents — role definitions with capabilities, models, prompts
- **R**outines — workflows (standard operating procedures)
- **D**ashboards — KPIs and measurement frameworks
- **E**xternal — integrations (ERP, CRM, email, calendar)
- **N**etworking — cross-department handoffs and dependencies

### Agent Role Taxonomy

| Role Type | Small | Medium | Large |
|-----------|-------|--------|-------|
| **Director / Manager** | 1 (human or AI) | 1 human + 1 AI deputy | 1 human + 2 AI deputies |
| **Specialist** | 0–1 (generalist) | 2–3 | 5–15 (role-specific) |
| **Coordinator** | 0 | 1–2 | 3–8 |
| **Analyst** | 0 | 1 | 2–6 |
| **Support / GPO** | Shared GPO | 1 department GPO | Dedicated GPO pod |

---

## Template Index

| # | Department | Category | Size | Version | ID |
|---|-----------|----------|------|---------|----|
| 1 | HR — Small Business | hr | small | 1.0.0 | hr-small-001 |
| 2 | HR — Medium Enterprise | hr | medium | 1.0.0 | hr-medium-001 |
| 3 | HR — Large Enterprise | hr | large | 1.0.0 | hr-large-001 |
| 4 | Finance — Small Business | finance | small | 1.0.0 | finance-small-001 |
| 5 | Finance — Medium Enterprise | finance | medium | 1.0.0 | finance-medium-001 |
| 6 | Legal — Small/Medium | legal | small+medium | 1.0.0 | legal-sm-001 |
| 7 | Legal — Large Enterprise | legal | large | 1.0.0 | legal-large-001 |
| 8 | Compliance — Medium | compliance | medium | 1.0.0 | compliance-medium-001 |
| 9 | Engineering — Small | engineering | small | 1.0.0 | eng-small-001 |
| 10 | Engineering — Medium | engineering | medium | 1.0.0 | eng-medium-001 |
| 11 | Marketing — Small/Medium | marketing | small+medium | 1.0.0 | mkt-sm-001 |
| 12 | Procurement — Medium | procurement | medium | 1.0.0 | procurement-medium-001 |
| 13 | Engineering — Large | engineering | large | 1.0.0 | eng-large-001 |
| 14 | Finance — Large Enterprise | finance | large | 1.0.0 | finance-large-001 |
| 15 | IT/Operations — All Sizes | ops | all | 1.0.0 | ops-all-001 |

**Total: 15 templates across 9 departments × 3 enterprise sizes**

---

## Usage

Each template is a standalone JSON file that can be POSTed to `POST /templates` on the Department Template Engine (port 8005).

```bash
curl -X POST http://localhost:8005/templates \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: acme-corp" \
  -H "Authorization: Bearer <jwt>" \
  -d @templates/hr-small-001.json
```

## Cross-Department Dependencies

| Department | Depends On | Provides To |
|-----------|-----------|-------------|
| HR | M02 (Identity), M03 (Orchestration) | All departments (onboarding/offboarding) |
| Finance | M02, M17 (Cost Governance) | Executive team, Procurement |
| Legal | M10 (Policy Governance), M02 | All departments (policy enforcement) |
| Compliance | M10, M11 (Observability), M02 | All departments (audit reporting) |
| Engineering | M07 (Memory), M12 (Models), M16 (Sandbox) | All departments (data processing) |
| Marketing | M06 (Knowledge), M12, M18 (Connectors) | Sales, Product, Executive |
| Procurement | M17 (Cost Governance), M10 (Policy Governance) | All departments (vendor management) |