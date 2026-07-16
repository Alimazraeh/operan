---
name: stale-test-remediation
description: Systematically fix broken handler tests after store/handler contract changes
source: auto-skill
extracted_at: '2026-07-15T08:47:36.580Z'
---

# Stale Test Remediation

When handler/store contracts change (e.g., tenant-scoped access, UUID seeding, mock interfaces), existing handler tests break with a consistent pattern. This skill covers the systematic approach to fix them.

## When to use

- Handler tests fail after a store or handler change (tenant isolation, param extraction, mock updates)
- Multiple test files show the same failure category (hardcoded IDs, missing tenant context, wrong mock returns)
- User asks to "fix the tests" or "get them passing"

## Root-cause patterns

Before touching code, identify which pattern(s) are at play by running `go test ./...` and examining the first failure:

| Pattern | Symptom | Example |
|---------|---------|---------|
| **Hardcoded IDs** | 404 Not Found or nil pointer on a known-valid test request | Test hardcodes "wf-1" but store now requires a real Create()-seeded workflow |
| **Missing tenant context** | 401 Unauthorized where old tests didn't set the tenant header | Handler now checks tenant from JWT context or X-Tenant-ID header |
| **Mock returns nil, nil** | nil pointer panic on handler dereference | Mock GetByID returns nil, nil but handler treats it as a 404 error case |
| **Wrong HTTP status expectation** | Test expects 500 for missing resource, handler returns 404 | Error classification changed |

## Fix procedure

### Step 1: Identify the dominant pattern

Run `go test ./... 2>&1 | tail -30`. Read the first failure stack trace. Determine whether the failure is:
- 404 Not Found -> likely hardcoded IDs or missing tenant
- nil pointer panic -> likely mock returning nil, nil
- 401 Unauthorized -> missing tenant context

### Step 2: Examine the passing seed pattern

Find a test that was already updated (or the store's Create signature) to understand the correct seeding approach:

Run: grep -n 'Create(&store.Workflow{' internal/handler/*_test.go | head -5
Check Create signature for required fields in the store package.

The correct pattern typically:
- Calls store.Create(seed data) to seed a real entity
- Uses the returned entity's UUID in the request path
- Calls setTenant(req) on the httptest request

### Step 3: Fix each test file in order

Fix test files in the order they appear in the failure output. For each file:

1. Read the relevant test functions
2. Replace hardcoded IDs with seeded Create() calls
3. Add req = setTenant(req) where the handler checks tenant
4. Fix mock nil, nil returns to errors.New("not found") (or errors.New("tenant mismatch"))
5. Correct expected HTTP status codes if they diverged from the handler

### Step 4: Fix mocks next

After fixing direct test functions, fix mock implementations. Common issue: mock GetByIDAndTenant returns nil, nil for both "not found" and "tenant mismatch" - these should be distinct error values.

### Step 5: Run tests, catch cascading failures

Run `go test ./... 2>&1 | tail -20`. Some failures from Step 3 may reveal additional issues (e.g., a test that now passes but the next one still fails for a different reason). Iterate:

- If 404 persists on a seeded entity -> check whether setTenant(req) is missing
- If panic on JSON unmarshal -> check whether the response is now an error JSON instead of the expected payload
- If new failure in an unrelated test -> likely the same pattern, fix the same way

### Step 6: Confirm with full suite

Run `go test ./...` from the module root. All tests must pass. If any remaining failure is a new, unrelated bug, flag it rather than silently patching.

## Anti-patterns

- Do not change the handler to accommodate the test - the handler's tenant-scoped contract is correct; the test is stale.
- Do not patch one test and move on without re-running - cascading failures are common.
- Do not use AddVariable("wf-1", ...) - this bypasses the store's Create logic entirely. Always seed through Create().
- Do not assume nil, nil equals not found - the handler's error classification depends on distinct error values.