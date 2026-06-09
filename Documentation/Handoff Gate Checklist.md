## CODER Handoff Gate Checklist

[ ] Module status = RECONCILED in MASTER_INDEX.md
[ ] OpenAPI + Schema + AsyncAPI + Edge all ✅
[ ] Platform standards applied (BearerAuth, X-Tenant-ID, pagination, Error schema)
[ ] Cross-spec inconsistencies resolved for this module (check tracker)
[ ] Orphan files moved to contracts/orphan/ (no accidental imports)
[ ] Dependency graph edges verified (no circular refs)
[ ] Global Constants pasted into handoff wrapper
[ ] REVIEW prompt pre-loaded with module-specific inconsistency notes

✅ If all checked → Proceed to CODER handoff
❌ If any unchecked → Fix first or skip to next module