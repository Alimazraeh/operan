package fixture

import (
	"strings"
	"testing"
)

func TestValidateAcceptsCleanFixture(t *testing.T) {
	f := sampleFixture()
	raw, err := MarshalYAML(f)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	if err := Validate(f, raw); err != nil {
		t.Fatalf("Validate: expected nil error for a clean fixture, got: %v", err)
	}
}

func TestValidateCatchesMissingRequiredFields(t *testing.T) {
	f := &Fixture{} // everything zero-value
	err := Validate(f, []byte("{}"))
	if err == nil {
		t.Fatal("Validate: expected an error for an empty fixture, got nil")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("Validate: expected *ValidationError, got %T", err)
	}
	// Spot check a handful of the required-field messages rather than every
	// one, so this test does not become a change-detector for wording.
	wantSubstrings := []string{
		"schema_version",
		"tenant.name",
		"tenant.plan",
		"users: at least one user",
		"department.template_id",
		"department.environment",
	}
	for _, want := range wantSubstrings {
		if !anyContains(ve.Issues, want) {
			t.Errorf("Validate: expected an issue mentioning %q, got issues:\n%s", want, strings.Join(ve.Issues, "\n"))
		}
	}
}

func TestValidateCatchesBadEnums(t *testing.T) {
	f := sampleFixture()
	f.Tenant.Plan = "gold-tier"
	f.Tenant.Region = "mars"
	f.Tenant.IsolationLevel = "vibes"
	f.Department.SeatBindings[0].HolderType = "superuser"

	raw, _ := MarshalYAML(f)
	err := Validate(f, raw)
	if err == nil {
		t.Fatal("Validate: expected an error for invalid enum values, got nil")
	}
	ve := err.(*ValidationError)
	for _, want := range []string{"tenant.plan", "tenant.region", "tenant.isolation_level", "holder_type"} {
		if !anyContains(ve.Issues, want) {
			t.Errorf("Validate: expected an issue mentioning %q, got:\n%s", want, strings.Join(ve.Issues, "\n"))
		}
	}
}

func TestValidateCatchesDanglingRefs(t *testing.T) {
	f := sampleFixture()
	f.Department.SeatBindings = append(f.Department.SeatBindings, SeatBinding{
		PositionID: "pos-ghost-01",
		HolderType: "human",
		UserRef:    "nobody-defined-this-ref",
	})
	raw, _ := MarshalYAML(f)
	err := Validate(f, raw)
	if err == nil {
		t.Fatal("Validate: expected an error for a dangling user_ref, got nil")
	}
	if !strings.Contains(err.Error(), "nobody-defined-this-ref") {
		t.Errorf("Validate error should name the dangling ref, got: %v", err)
	}
}

func TestValidateCatchesDuplicateRefs(t *testing.T) {
	f := sampleFixture()
	f.Users = append(f.Users, User{
		Ref:   "dana", // duplicate of the existing user's ref
		Email: "dana2@adri.nz", DisplayName: "Dana Duplicate", RoleIDs: []string{"department_head"},
	})
	raw, _ := MarshalYAML(f)
	err := Validate(f, raw)
	if err == nil {
		t.Fatal("Validate: expected an error for a duplicate user ref, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate ref") {
		t.Errorf("Validate error should flag the duplicate ref, got: %v", err)
	}
}

func TestValidateRejectsNonUUIDAgentID(t *testing.T) {
	f := sampleFixture()
	f.Agents[0].ID = "not-a-uuid"
	raw, _ := MarshalYAML(f)
	err := Validate(f, raw)
	if err == nil {
		t.Fatal("Validate: expected an error for a non-UUID agent id, got nil")
	}
	if !strings.Contains(err.Error(), "must be a UUID") {
		t.Errorf("Validate error should flag the bad UUID, got: %v", err)
	}
}

// --- Secret scanning -------------------------------------------------------
//
// These tests assert the scanner FAILS on planted secrets and PASSES on the
// clean fixture — the "verify it fails against the unfixed input" discipline
// this repo requires of every "catches a bug" claim, applied here to "catches
// a secret" instead.

func TestScanForSecretsCatchesSuspiciousKeyNames(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"password field", "tenant:\n  name: t\nusers:\n  - ref: a\n    password: hunter2\n"},
		{"api_key field", "connector:\n  api_key: sk-live-abc123\n"},
		{"nested secret", "department:\n  seat_bindings:\n    - config:\n        client_secret: xyz\n"},
		{"token field", "users:\n  - auth_token: eyJhbGciOiJIUzI1NiJ9\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := ScanForSecrets([]byte(tc.yaml))
			if len(findings) == 0 {
				t.Fatalf("ScanForSecrets: expected findings for %q, got none", tc.yaml)
			}
		})
	}
}

func TestScanForSecretsCatchesSuspiciousValues(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"bearer token value", "notes: \"use Bearer eyJhbGci... to auth\"\n"},
		{"pem block", "notes: |\n  -----BEGIN PRIVATE KEY-----\n  abc\n"},
		{"aws key id", "notes: \"AKIA-example\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := ScanForSecrets([]byte(tc.yaml))
			if len(findings) == 0 {
				t.Fatalf("ScanForSecrets: expected findings for %q, got none", tc.yaml)
			}
		})
	}
}

func TestScanForSecretsPassesCleanFixture(t *testing.T) {
	f := sampleFixture()
	raw, err := MarshalYAML(f)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	findings := ScanForSecrets(raw)
	if len(findings) != 0 {
		t.Fatalf("ScanForSecrets: expected no findings on the clean sample fixture, got: %v", findings)
	}
}

func TestValidateEndToEndRejectsFixtureWithPlantedSecret(t *testing.T) {
	// Belt-and-braces: even though the typed User struct has no password
	// field, hand-edited YAML can add arbitrary keys. Validate must catch
	// this via the raw-document scan, not just the typed struct.
	raw := []byte(`
schema_version: 1
metadata:
  name: smoke-tenant-demo
  provenance: hand-assembled
tenant:
  name: smoke-tenant
  plan: saas
  region: me-east-1
  isolation_level: namespace
users:
  - ref: dana
    email: dana@adri.nz
    display_name: Dana Q
    role_ids: [department_head]
    password: hunter2
department:
  template_id: it-medium-001
  environment: production
`)
	f, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(f, raw); err == nil {
		t.Fatal("Validate: expected the planted password field to be rejected, got nil error")
	} else if !strings.Contains(err.Error(), "credential-shaped") {
		t.Errorf("Validate error should explain the secret finding, got: %v", err)
	}
}

func anyContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
