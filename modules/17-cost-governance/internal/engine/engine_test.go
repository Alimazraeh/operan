package engine

import (
	"testing"
)

func TestNewThrottleManager(t *testing.T) {
	m := NewThrottleManager()
	if m == nil {
		t.Fatal("expected non-nil ThrottleManager")
	}
	if m.GetState("tenant-1") != "" {
		t.Error("expected empty state for unknown tenant")
	}
}

func TestThrottleManager_SetAndGetState(t *testing.T) {
	m := NewThrottleManager()

	m.SetState("tenant-1", "hard")
	if s := m.GetState("tenant-1"); s != "hard" {
		t.Errorf("expected hard, got %s", s)
	}

	m.SetState("tenant-1", "soft")
	if s := m.GetState("tenant-1"); s != "soft" {
		t.Errorf("expected soft, got %s", s)
	}

	m.SetState("tenant-1", "none")
	if s := m.GetState("tenant-1"); s != "none" {
		t.Errorf("expected none, got %s", s)
	}
}

func TestThrottleManager_MultipleTenants(t *testing.T) {
	m := NewThrottleManager()

	m.SetState("tenant-a", "hard")
	m.SetState("tenant-b", "soft")

	if s := m.GetState("tenant-a"); s != "hard" {
		t.Errorf("tenant-a: expected hard, got %s", s)
	}
	if s := m.GetState("tenant-b"); s != "soft" {
		t.Errorf("tenant-b: expected soft, got %s", s)
	}
}

func TestThrottleManager_GetThrottleInfo(t *testing.T) {
	m := NewThrottleManager()

	m.SetState("tenant-1", "hard")
	state, updatedAt := m.GetThrottleInfo("tenant-1")

	if state != "hard" {
		t.Errorf("expected state hard, got %s", state)
	}
	if updatedAt.IsZero() {
		t.Error("expected non-zero updatedAt")
	}

	// Unknown tenant
	unknownState, unknownTime := m.GetThrottleInfo("unknown")
	if unknownState != "" {
		t.Errorf("expected empty state for unknown tenant, got %s", unknownState)
	}
	if !unknownTime.IsZero() {
		t.Error("expected zero time for unknown tenant")
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	if e == nil {
		t.Fatal("expected non-nil Engine")
	}
}