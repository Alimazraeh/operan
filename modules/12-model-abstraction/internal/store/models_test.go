package store

import (
	"testing"
)

func TestModelProvider_ZeroValues(t *testing.T) {
	p := ModelProvider{}
	if p.ID != "" {
		t.Error("expected zero ID")
	}
	if p.IsActive != false {
		t.Error("expected IsActive to be false by default")
	}
	if p.Priority != 0 {
		t.Error("expected Priority to be 0 by default")
	}
}

func TestModelRegistry_ZeroValues(t *testing.T) {
	m := ModelRegistry{}
	if m.ID != "" {
		t.Error("expected zero ID")
	}
	if m.SupportsChat != false {
		t.Error("expected SupportsChat to be false by default")
	}
	if m.IsDefault != false {
		t.Error("expected IsDefault to be false by default")
	}
}

func TestModelCall_ZeroValues(t *testing.T) {
	c := ModelCall{}
	if c.ID != "" {
		t.Error("expected zero ID")
	}
	if c.Status != "" {
		t.Error("expected empty Status by default")
	}
	if c.CostUSD != 0 {
		t.Error("expected zero CostUSD")
	}
}

func TestErrNoRows(t *testing.T) {
	if ErrNoRows == nil {
		t.Fatal("ErrNoRows should not be nil")
	}
	if ErrNoRows.Error() != "no rows in result set" {
		t.Errorf("unexpected error message: %s", ErrNoRows.Error())
	}
}