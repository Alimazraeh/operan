package fixture

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads a fixture from path, auto-detecting YAML vs JSON by extension
// (.json → JSON; anything else → YAML, since JSON is valid YAML this is a
// display preference, not a parser fork), and validates it. It returns the
// raw bytes alongside the decoded Fixture because ScanForSecrets in
// Validate needs to see the untyped document.
func Load(path string) (*Fixture, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	f, err := Parse(raw)
	if err != nil {
		return nil, raw, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := Validate(f, raw); err != nil {
		return f, raw, err
	}
	return f, raw, nil
}

// Parse decodes a fixture document (YAML or JSON — JSON is valid YAML) into
// a Fixture without validating it. Exported separately from Load so tests
// and tools can inspect a deliberately-invalid document.
func Parse(raw []byte) (*Fixture, error) {
	var f Fixture
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// SaveYAML serializes f as YAML to path with 0644 permissions — the format
// fixtures are committed in: human-readable, diffable, comment-friendly.
func SaveYAML(path string, f *Fixture) error {
	b, err := MarshalYAML(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// MarshalYAML serializes f as YAML with a 2-space indent, matching the
// style already used by fixture-ish YAML elsewhere in this repo.
func MarshalYAML(f *Fixture) ([]byte, error) {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// MarshalJSON serializes f as indented JSON — offered as an export format
// alongside YAML because some downstream tooling (e.g. `jq`-based CI
// assertions) prefers it; the schema is identical either way.
func MarshalJSON(f *Fixture) ([]byte, error) {
	return json.MarshalIndent(f, "", "  ")
}
