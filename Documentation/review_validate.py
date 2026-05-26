#!/usr/bin/env python3
import json, subprocess, sys, os
from jsonschema import validate, ValidationError
from pathlib import Path

CONTRACTS_DIR = Path("contracts/v1")
MODULE_DIR = Path(f"modules/{os.getenv('MODULE_ID', '09-supervision')}")
REPORT_DIR = Path("reports")
REPORT_DIR.mkdir(exist_ok=True)

def run_cmd(cmd):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True)

def validate_openapi():
    openapi_files = list(CONTRACTS_DIR.glob("openapi-*.yaml"))
    for f in openapi_files:
        res = run_cmd(f"openapi-spec-validator {f}")
        if res.returncode != 0:
            return False, f"OpenAPI invalid: {f}"
    return True, "OpenAPI valid"

def validate_schemas():
    schema_files = list(CONTRACTS_DIR.glob("schema-*.json"))
    for s in schema_files:
        schema = json.loads(s.read_text())
        # Mock validation against sample payloads in module/tests/fixtures/
        fixtures = MODULE_DIR / "tests" / "fixtures"
        if fixtures.exists():
            for fx in fixtures.glob("*.json"):
                data = json.loads(fx.read_text())
                try:
                    validate(instance=data, schema=schema)
                except ValidationError as e:
                    return False, f"Schema drift in {fx.name}: {e.message}"
    return True, "JSON Schema compliant"

def smoke_test():
    compose_file = MODULE_DIR / "docker-compose.smoke.yml"
    if not compose_file.exists():
        return False, "Missing smoke compose file"
    run_cmd(f"docker-compose -f {compose_file} up -d")
    health = run_cmd(f"docker-compose -f {compose_file} ps --format json | jq -r '.[] | select(.Health==\"healthy\") | .Name'")
    run_cmd(f"docker-compose -f {compose_file} down")
    return "healthy" in health.stdout, "Smoke test " + ("PASSED" if "healthy" in health.stdout else "FAILED")

def generate_report(status, drift, smoke, coverage):
    decision = "APPROVE" if all(status, not drift, smoke) else "REJECT"
    report = REPORT_DIR / f"{os.getenv('MODULE_ID')}-review.md"
    report.write_text(f"""# Review Report: {os.getenv('MODULE_ID')}
| Check | Result |
|-------|--------|
| OpenAPI | {'✅ PASS' if status else '❌ FAIL'} |
| Schema Drift | {'✅ NONE' if not drift else f'❌ {drift}'} |
| Smoke Test | {'✅ PASS' if smoke else '❌ FAIL'} |
| Coverage | {coverage}% |
| Decision | {decision} |
""")
    return decision

if __name__ == "__main__":
    ok_api, msg_api = validate_openapi()
    ok_schema, msg_schema = validate_schemas()
    ok_smoke, _ = smoke_test()
    cov = 85.0  # Parse from manifest.json or coverage.xml in production
    decision = generate_report(ok_api and ok_schema, msg_schema, ok_smoke, cov)
    print(decision)
    sys.exit(0 if decision == "APPROVE" else 1)