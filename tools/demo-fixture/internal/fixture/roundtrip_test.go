package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleFixture() *Fixture {
	return &Fixture{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Name:        "smoke-tenant-demo",
			Description: "test fixture",
			Provenance:  ProvenanceLiveExport,
			ExportedAt:  "2026-07-27T00:00:00Z",
		},
		Tenant: Tenant{
			Name:           "smoke-tenant",
			DisplayName:    "Smoke Tenant",
			Plan:           "saas",
			Region:         "me-east-1",
			IsolationLevel: "namespace",
		},
		Users: []User{
			{
				Ref:         "dana",
				Email:       "dana@adri.nz",
				DisplayName: "Dana Q",
				RoleIDs:     []string{"department_head"},
			},
		},
		Agents: []Agent{
			{
				Ref:  "triage-agent",
				ID:   "3a0c0c3c-c849-4b74-883a-9ccf85b14b5c",
				Name: "Triage Agent",
				Role: "specialist",
			},
		},
		Department: Department{
			TemplateID:  "it-medium-001",
			Environment: "production",
			SeatBindings: []SeatBinding{
				{PositionID: "pos-it-manager-01", HolderType: "human", UserRef: "dana"},
				{PositionID: "pos-triage-01", HolderType: "ai_agent", AgentRef: "triage-agent"},
			},
			SyncWorkflows: true,
		},
		History: []HistoricalRequest{
			{
				ServiceID: "svc-access-request",
				Title:     "Grant read access",
				Status:    "completed",
				Timeline: []HistoricalEvent{
					{Kind: "created"},
					{Kind: "completed"},
				},
				Invocations: []HistoricalInvocation{
					{CapabilityID: "identity.access.grant", Status: "completed", Simulated: true},
				},
			},
		},
		Replay: &ReplaySpec{
			ServiceID:   "svc-access-request",
			Title:       "Grant replay-test read access",
			Priority:    "normal",
			ApproverRef: "dana",
		},
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	orig := sampleFixture()

	raw, err := MarshalYAML(orig)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}

	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v\nraw:\n%s", err, raw)
	}

	assertFixturesEqual(t, orig, got)
}

func TestJSONRoundTrip(t *testing.T) {
	orig := sampleFixture()

	raw, err := MarshalJSON(orig)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Parse uses yaml.Unmarshal, which must also accept JSON since JSON is
	// a syntactic subset of YAML — this is the property io.go's doc comment
	// relies on.
	got, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(json): %v\nraw:\n%s", err, raw)
	}

	assertFixturesEqual(t, orig, got)
}

func TestSaveAndLoadYAML(t *testing.T) {
	orig := sampleFixture()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.yaml")

	if err := SaveYAML(path, orig); err != nil {
		t.Fatalf("SaveYAML: %v", err)
	}

	got, raw, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v\nraw:\n%s", err, raw)
	}
	assertFixturesEqual(t, orig, got)

	// The file on disk must be human-readable YAML, not a binary blob or a
	// single unreadable line — spot check it looks like YAML.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := "schema_version: 1"; !strings.Contains(string(b), want) {
		t.Errorf("saved YAML missing expected line %q; got:\n%s", want, b)
	}
	if want := "smoke-tenant"; !strings.Contains(string(b), want) {
		t.Errorf("saved YAML missing tenant name; got:\n%s", b)
	}
}

func TestLoadRejectsUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.yaml")
	f := sampleFixture()
	f.SchemaVersion = 999
	if err := SaveYAML(path, f); err != nil {
		t.Fatalf("SaveYAML: %v", err)
	}

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("Load: expected an error for an unrecognized schema_version, got nil")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("Load error should mention schema_version, got: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("Load: expected an error for a missing file, got nil")
	}
}

func assertFixturesEqual(t *testing.T, want, got *Fixture) {
	t.Helper()
	wantB, err := MarshalJSON(want)
	if err != nil {
		t.Fatalf("MarshalJSON(want): %v", err)
	}
	gotB, err := MarshalJSON(got)
	if err != nil {
		t.Fatalf("MarshalJSON(got): %v", err)
	}
	if string(wantB) != string(gotB) {
		t.Errorf("round trip mismatch.\nwant:\n%s\ngot:\n%s", wantB, gotB)
	}
}
