# Module 05 — Department Template Engine: REVIEW Drift Report

**Review Date:** 2026-05-30
**Reviewer:** REVIEW (automated validation gate)
**Module:** `05-department-template-engine`
**Contract Version:** v1.0.0
**Implementation Version:** v1.0.0 (in-memory MVP)

---

## Executive Summary

| Metric | Value | Gate | Status |
|--------|-------|------|--------|
| Build | PASS (go build ./...) | Required | ✅ PASS |
| go vet | PASS (no issues) | Required | ✅ PASS |
| Contract Drift | 8 findings | 0 allowed | ❌ FAIL |
| Test Coverage | 57.9% | ≥80% | ❌ FAIL |
| Security (secrets) | 1 warning | 0 allowed | ⚠️ WARN |
| Security (syscalls) | 0 findings | 0 allowed | ✅ PASS |
| Smoke Test | N/A (no docker-compose) | Required | ❌ FAIL |

**MERGE DECISION: REJECT**

---

## 1. Contract Drift — 8 Findings

### DRIFT-01 [HIGH] Status enum mismatch: `reviewed` vs `deprecated`

Three contracts disagree on the Template status enum:

| Contract | Status Enum |
|----------|------------|
| OpenAPI (`openapi-05-department-template-engine.yaml`) | `[draft, reviewed, published, archived]` |
| JSON Schema (`schema-05-department-template-engine.json`) | `[draft, deprecated, published, archived]` |
| AsyncAPI (`asyncapi-05-department-template-engine.yaml`) | `reviewed` in `changed_fields` enum |

**Expected:** `deprecated` (per handover remediation plan dated 2026-05-29)
**Actual:** OpenAPI and AsyncAPI still use `reviewed`
**Fix:** Update OpenAPI and AsyncAPI to use `deprecated` consistently across all 3 contracts.

---

### DRIFT-02 [HIGH] Route path mismatch: Custom templates

| Contract Path | Handler Route | Match |
|---------------|--------------|-------|
| `GET /templates/custom` | `GET /custom-templates` | ❌ |
| `GET /templates/custom/{id}` | `GET /custom-templates/{id}` | ❌ |
| `PATCH /templates/custom/{id}` | `PATCH /custom-templates/{id}` | ❌ |
| `DELETE /templates/custom/{id}` | `DELETE /custom-templates/{id}` | ❌ |
| `POST /templates/custom` | `POST /custom-templates` | ❌ |

**Expected:** All custom-template routes under `/templates/custom` (as defined in OpenAPI)
**Actual:** Routes registered under `/custom-templates`
**Fix:** Update `router.go` to register custom-template routes under `/templates/custom` prefix.

---

### DRIFT-03 [HIGH] Route path mismatch: Template versions

| Contract Path | Handler Route | Match |
|---------------|--------------|-------|
| `GET /templates/{id}/versions` | `GET /templates/versions` | ❌ |
| `GET /templates/{id}/versions/{version}` | `GET /templates/versions/{id}` | ❌ (UUID vs semver) |
| `POST /templates/{id}/versions` | `POST /templates/version` | ❌ (singular) |

**Contract specifies:**
- `listTemplateVersions`: `GET /templates/{id}/versions` (with `{id}` as path param)
- `getTemplateVersion`: `GET /templates/{id}/versions/{version}` (with `{version}` as semver path param)
- No `POST` endpoint defined — but handler implements `POST /templates/version`

**Actual:**
- `ListTemplateVersions`: `GET /templates/versions` (missing `{id}` in path)
- `GetTemplateVersion`: `GET /templates/versions/{id}` (expects UUID, not semver)
- `CreateTemplateVersion`: `POST /templates/version` (extra `version` endpoint, not in contract)

**Fix:** Align router paths to `GET /templates/{id}/versions`, `GET /templates/{id}/versions/{version}`, and remove the extra `POST /templates/version`.

---

### DRIFT-04 [HIGH] Missing route: Deploy lifecycle sub-paths

| Contract Operation | Contract Path | Handler | Match |
|--------------------|--------------|---------|-------|
| `getDeployment` | `GET /templates/{id}/deployments/{deploymentId}` | ❌ Missing | ❌ |
| `updateDeployment` | `PATCH /templates/{id}/deployments/{deploymentId}` | ❌ Missing | ❌ |

**Actual:** Handler registers:
- `GET /templates/deployments` (without `{id}` in path)
- `POST /templates/deploy` (not under `/templates/{id}/deploy`)
- `GET /templates/deployments/{id}` (without template context)
- `PATCH /templates/deployments/{id}` (without template context)

**Fix:** Restructure deployment routes to match contract path hierarchy.

---

### DRIFT-05 [MEDIUM] Tenant isolation not enforced in stores

Store comment in `templates.go:62-63`:
```go
// Note: In production, tenant_id would be stored on the template
// and queried here. For this MVP, we return all templates.
```

**Findings:**
- `Template` model has no `TenantID` field
- `CustomTemplate` model has no `TenantID` field
- `TemplateStore.List()` ignores `tenantID` parameter — returns all records
- `CustomTemplateStore.List()` ignores `tenantID` parameter — returns all records
- `DeploymentStore` and `VersionStore` have no tenant isolation at all

**Expected:** All stores filter by `tenantID`; Template and CustomTemplate models include `TenantID` field.
**Actual:** Tenant parameter accepted but unused.
**Fix:** Add `TenantID` to models and implement `byTenant` filtering in all store List methods.

---

### DRIFT-06 [MEDIUM] Response schema: `getTemplateVersion`

| Contract | Response Schema |
|----------|----------------|
| OpenAPI | `$ref: '#/components/schemas/Template'` |
| Handler | Inline `map[string]interface{}{id, template_id, version, snapshot, created_at}` |

**Expected:** Full Template object per contract.
**Actual:** Version-specific response without Template schema fields (name, category, status, etc.).

---

### DRIFT-07 [MEDIUM] Response schema: `deployTemplate` request body

| Contract | Required Fields |
|----------|----------------|
| OpenAPI (`TemplateDeployRequest`) | `template_id`, `version`, `environment` |
| Handler (`DeployRequest`) | `environment` only (version is optional) |

**Note:** `template_id` is already extracted from path param `{id}` in the contract. The contract requires it in both path AND body, which is redundant. Handler ignores body `template_id`.

**Expected:** Contract should remove `template_id` from `TemplateDeployRequest` body (it's already in path).

---

### DRIFT-08 [LOW] Missing `created_at` in Template response

| Field | Contract Required | Handler Response |
|-------|------------------|------------------|
| `created_at` | Yes (date-time) | Yes |
| `updated_at` | Yes (date-time) | Yes |
| `tags` | Optional | ✅ Included in list response only, not get response |

**Minor:** `toTemplateResponse()` does not include `tags` field; `toTemplateListResponse()` includes it. Inconsistent.

---

## 2. Test Coverage — 57.9% (Gate: ≥80%)

| Package | Tests | Coverage |
|---------|-------|----------|
| `config` | 8 | 100.0% ✅ |
| `ctxkeys` | 0 | n/a (type constants) |
| `events` | 13 | 77.3% ✅ |
| `handlers` | 15 | 42.3% ❌ |
| `middleware` | 11 | 94.1% ✅ |
| `store` | 22 | 72.0% ⚠️ |
| **Total** | **70** | **57.9%** |

### Coverage gaps (by package):

**handlers (42.3%)** — Major gaps:
- No test for `UpdateTemplate` with invalid/missing fields
- No test for `UpdateDeployment` with `failed` status path
- No test for `UpdateDeployment` with `operational` status path
- No test for `ListDeployments` with pagination
- No test for `GetDeployment` with invalid IDs
- No test for `ListTemplateVersions` with pagination
- No test for `CloneTemplate` with missing body
- No test for 401 (auth failure) paths
- No test for 403 (forbidden) paths
- No concurrent request tests

**store (72.0%)** — Gaps:
- No tenant isolation tests (stores ignore tenantID)
- No edge case tests (empty strings, nil payloads, duplicate UUIDs)

---

## 3. Smoke Test — N/A (No docker-compose)

No `docker-compose.yml` exists for Module 05. The module has a Dockerfile and Helm chart, but no smoke test infrastructure.

**Required:** At minimum, a `docker-compose.test.yml` with:
- Module 05 service on port 8005
- Health check verification
- At least one contract endpoint test

---

## 4. Security Findings

### SECURITY-001 [WARN] Default JWT secret

**File:** `internal/config/config.go:34`
```go
JWTSecret: env("MODULE05_JWT_SECRET", "change-me-in-production")
```

**Analysis:** The default value `change-me-in-production` is correctly guarded by validation in `config.go:68`:
```go
if c.JWTSecret == "change-me-in-production" {
    return errors.New("JWT_SECRET must be changed from default")
}
```

This means the service will log a warning but still start — it does NOT fail-closed. This is inconsistent with Module 02's fail-closed pattern.

**Severity:** LOW (guarded by validation warning)
**Recommendation:** Fail-closed: exit with error if JWT_SECRET uses default.

### No unsafe syscalls found ✅
### No hardcoded tokens/secrets (API keys, etc.) ✅
### No net.Dial / http.DefaultClient / exec.Command usage ✅

---

## 5. Cross-Spec Inconsistencies

### `changed_fields` enum in AsyncAPI

**AsyncAPI (`asyncapi-05-department-template-engine.yaml`) — TemplateUpdatedPayload:**
```yaml
changed_fields:
  items:
    type: string
    enum: [agents, workflows, memory_topology, governance_rules, kpis, integrations, operational_policies, metadata, tags]
```

**Go handler (`handlers.go` — UpdateTemplate):**
```go
changedFields := []string{}
for field := range patch {
    changedFields = append(changedFields, field)
}
```

The Go handler sends arbitrary field names from the PATCH body. These may not match the AsyncAPI enum. The handler should validate against the allowed set before publishing.

---

## 6. Known P1 Issues (From Handover)

These were acknowledged in the coder handover and are NOT new findings:

| # | Issue | Severity |
|---|-------|----------|
| 1 | No database backend — all stores in-memory | P1 |
| 2 | Event publishing uses LogBroker — no production AMQP broker integration | P1 |

---

## Merge Decision

| Gate | Requirement | Actual | Result |
|------|------------|--------|--------|
| Build | Compiles | PASS | ✅ |
| go vet | Clean | PASS | ✅ |
| Contract Drift | 0 findings | 8 findings | ❌ REJECT |
| Test Coverage | ≥80% | 57.9% | ❌ REJECT |
| Security | No secrets/syscalls | 1 WARN | ⚠️ |
| Smoke Test | docker-compose pass | N/A | ❌ FAIL |

**FINAL DECISION: REJECT**

---

## Remediation Checklist

1. **[P0]** Fix status enum: `reviewed` → `deprecated` in OpenAPI + AsyncAPI
2. **[P0]** Fix custom-template route paths: `/custom-templates` → `/templates/custom`
3. **[P0]** Fix version route paths: align to `/templates/{id}/versions`
4. **[P0]** Fix deployment route paths: add template context `{id}`
5. **[P0]** Implement store-level tenant isolation (add TenantID to models + filtering)
6. **[P1]** Fix `getTemplateVersion` response to match Template schema
7. **[P1]** Add 10+ handler test cases (auth failures, edge cases, concurrent)
8. **[P1]** Add store tenant isolation tests
9. **[L]** Create docker-compose smoke test
10. **[L]** Fix JWT default: fail-closed instead of warning

---

*Report generated by REVIEW — deterministic contract compliance validator*
*Files audited: 30 implementation files, 3 contract files*
*Review scope: OpenAPI + JSON Schema + AsyncAPI + Go implementation + tests*
