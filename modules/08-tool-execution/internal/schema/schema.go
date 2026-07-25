// Package schema validates capability inputs against their JSON Schemas.
//
// This is real validation, not a presence check: the old executor validated
// nothing and echoed its input back, which is how 40% of catalogue steps came
// to "execute" as a note string. Writes must be typed — an invocation whose
// input does not conform never reaches a provider.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator compiles capability schemas once and caches them by capability id.
type Validator struct {
	mu       sync.Mutex
	compiled map[string]*jsonschema.Schema
}

func NewValidator() *Validator {
	return &Validator{compiled: map[string]*jsonschema.Schema{}}
}

// Validate checks input against the schema registered for key. The schema is
// a plain map (as capabilities carry it); it is compiled on first use.
func (v *Validator) Validate(key string, schemaDoc map[string]interface{}, input map[string]interface{}) error {
	sch, err := v.schemaFor(key, schemaDoc)
	if err != nil {
		return fmt.Errorf("schema for %s does not compile: %w", key, err)
	}
	// Round-trip through JSON so numbers and nested values take the exact
	// types the validator expects, regardless of how the caller built the map.
	raw, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("input not encodable: %w", err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("input not decodable: %w", err)
	}
	return sch.Validate(inst)
}

func (v *Validator) schemaFor(key string, doc map[string]interface{}) (*jsonschema.Schema, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if s, ok := v.compiled[key]; ok {
		return s, nil
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	url := "operan:///" + key + ".schema.json"
	if err := c.AddResource(url, parsed); err != nil {
		return nil, err
	}
	s, err := c.Compile(url)
	if err != nil {
		return nil, err
	}
	v.compiled[key] = s
	return s, nil
}
