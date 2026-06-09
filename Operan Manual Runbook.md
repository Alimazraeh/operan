# 🚀 OPERAN MANUAL AGENTIC RUNBOOK
Project: Operan — Agentic Department Operating System (ADOS)
Sessions: 🟦 ARCH | 🟩 CODER_A | 🟨 CODER_B | 🟥 REVIEW

## 🔄 HANDOFF WORKFLOW (Per Module)
1️⃣ ARCH → Generate/Verify contracts (OpenAPI, Schema, AsyncAPI, Edge)
2️⃣ YOU → Save to contracts/v1/ → Paste into CODER Handoff Wrapper
3️⃣ CODER → Implement module → Output src/, tests/, Dockerfile, manifest.json
4️⃣ YOU → Save to modules/{ID}/ → Paste into REVIEW Handoff Wrapper
5️⃣ REVIEW → Run validation → Output APPROVE or REJECT report
6️⃣ YOU → If APPROVE: merge to main, update integration-graph.yaml
   → If REJECT: copy drift details → send back to CODER → loop to 3️⃣

## 📐 GLOBAL CONSTANTS (Copy-Paste Into Every Prompt)
• Tenant ID: `uuid-v4`
• Auth: `Authorization: Bearer {jwt}` + `X-Tenant-ID` header
• Errors: RFC 7807 `{ code: int, message: string, request_id: uuid }`
• Trace: `X-Trace-Id: {uuid}`
• Time: ISO 8601 UTC
• Pagination: `page`, `page_size`, `has_more`
• Schema: `additionalProperties: false`

## ✅ CODER DO NOT VIOLATE RULES
❌ NO imports from other modules
❌ NO assumed interfaces beyond contracts/v1/
❌ NO hardcoded secrets, tokens, or tenant IDs
✅ ≥80% unit tests, OpenTelemetry traces, deterministic errors
✅ Output manifest.json: `{ "coverage": 0.0, "contract_compliant": true }`

## 🛡️ REVIEW VALIDATION GATES
1️⃣ CONTRACT: Do API payloads match OpenAPI/Schema exactly? (line refs)
2️⃣ TESTS: Is coverage ≥80%? (parse manifest.json)
3️⃣ SMOKE: Would `docker-compose up && curl /health` succeed?
4️⃣ SECURITY: Any hardcoded secrets, unsafe syscalls, missing RBAC?
5️⃣ DECISION: APPROVE | REJECT (strict format only)

## 🚨 DRIFT FIX PROTOCOL
If REVIEW = REJECT:
1. Copy exact drift lines from REVIEW report
2. Paste to CODER with: "Fix EXACT mismatches. Re-output full module."
3. Re-run REVIEW after fix.
4. If still failing → send to ARCH with: "Contract drift detected. Reconcile v1 spec."
5. Never merge until APPROVE.

## 📁 LOCAL STRUCTURE
orchestra-manual/
├── contracts/v1/        ← ARCH outputs (OpenAPI, Schema, AsyncAPI, Edge)
├── modules/{ID}-{name}/ ← CODER outputs (src, tests, Dockerfile, helm, manifest.json)
├── reviews/             ← REVIEW outputs ({ID}-review.md)
├── contracts/orphan/    ← Quarantined drafts
└── MASTER_INDEX.md      ← Single source of truth

## ⏱️ EXPECTED CADENCE (Manual)
• ARCH batch: ~15 mins
• CODER implementation: ~30-45 mins/module
• REVIEW validation: ~10 mins
• Total per module: ~45-70 mins
• Wave 1 (4 modules): ~3-5 hours

---

## 📋 MODULE 04 — DELEGATION GUIDANCE

**Module ID:** `04-agent-registry`  
**Contracts:** `openapi-04-agent-registry.yaml` (16 ops), `schema-04-agent-registry.json` (20 defs), `asyncapi-04-agent-registry.yaml` (8 channels)  
**PRD Reference:** Sections 5 (Module 04), 8 (Agent Object Model), 9 (Multi-Tenant Isolation)

### Architecture Blueprint
- Base path: `/api/v1/registry`
- Security: BearerAuth (RSA/JWKS + HMAC fallback) + `X-Tenant-ID`
- Storage: PostgreSQL + Redis (from Day 1 — no in-memory stubs)
- Events: 18 AsyncAPI channels, topic format `operan.registry.{entity}.{event}`
- Test coverage target: ≥80%

### Current State: REJECT (45% compliant)
**Architectural review: 2026-05-28** — 4 Critical, 8 High, 6 Medium, 3 Low issues

**Before delegating implementation fixes, ARCH must resolve:**
1. Align `SearchAgents` to use tenant context (not request body)
2. Wrap `AgentByID` and `VersionByID` routes with `ExtractTenant`
3. Create event structs for 4 missing AsyncAPI events and rename existing ones to match operationIds
4. Fix `DependencyType` enum — add `hard` to JSON Schema and AsyncAPI (✅ done)

**Delegation checklist for CODER:**
- [ ] Wire config struct into `main.go`
- [ ] Implement JWTAuth middleware (RSA/JWKS + HMAC fallback)
- [ ] Add TraceID, RequestID, and Logger middleware
- [ ] Rewrite `CostProfile` DTO to match 6-field OpenAPI schema
- [ ] Rewrite `MemoryAccess` from `[]string` to structured object
- [ ] Fix `DependencyRequest.Type` → `Description`
- [ ] Add tenant isolation to VersionStore, CapabilityStore, DependencyStore
- [ ] Align tests to use valid `DependencyType` enum values (`hard`, `soft`, `optional`)
- [ ] Add missing Agent fields: `objectives`, `supported_languages`, `current_version_id`, `access_control`
- [ ] Set up PostgreSQL + Redis connection in config
- [ ] Implement real AMQP/Kafka event publisher (not stub)

**Key anti-patterns to avoid:**
- ❌ Storing `tenant_id` in request body — always use `tenant.TenantID()` from context
- ❌ Unwrapped routes that bypass `ExtractTenant`
- ❌ String context keys — use typed `contextKey` constants
- ❌ Hardcoded config values — use `config.ParseConfig()`
- ❌ String enum values not in OpenAPI schema — validate against contract