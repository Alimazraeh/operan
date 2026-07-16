package store

import (
	"testing"

	"github.com/google/uuid"
)

func ptrStr(s string) *string {
	return &s
}

func ptrInt(i int) *int {
	return &i
}

func ptrBool(b bool) *bool {
	return &b
}

// TestProfileStore_ModelDefaults validates SandboxProfile field defaults.
func TestProfileStore_ModelDefaults(t *testing.T) {
	p := &SandboxProfile{
		TenantID: "tenant-1",
		Name:     "test",
	}
	// UUID is zero by default
	if p.ID != uuid.Nil {
		t.Errorf("expected zero UUID, got %s", p.ID)
	}
	// Description is nil
	if p.Description != nil {
		t.Errorf("expected nil description, got %s", *p.Description)
	}
	// IsActive defaults to false
	if p.IsActive {
		t.Error("expected IsActive to be false by default")
	}
	// CPUCores defaults to zero
	if p.CPUCores != 0 {
		t.Errorf("expected CPUCores 0, got %f", p.CPUCores)
	}
	// NetworkAccess defaults to false
	if p.NetworkAccess {
		t.Error("expected NetworkAccess to be false by default")
	}
}

// TestProfileStore_ModelValid validates SandboxProfile can be set correctly.
func TestProfileStore_ModelValid(t *testing.T) {
	desc := "A test profile"
	p := SandboxProfile{
		TenantID:         "tenant-1",
		Name:             "test-profile",
		Description:      &desc,
		CPUCores:         2.0,
		MemoryMB:         512,
		TimeoutSeconds:   120,
		NetworkAccess:    true,
		AllowedTools:     []string{"echo", "bash", "python3"},
		FilesystemAccess: true,
		MaxFileSizeMB:    100,
		MaxOutputSizeKB:  2048,
		IsActive:         true,
	}
	if p.TenantID != "tenant-1" {
		t.Errorf("expected tenant 'tenant-1', got '%s'", p.TenantID)
	}
	if p.CPUCores != 2.0 {
		t.Errorf("expected CPU 2.0, got %f", p.CPUCores)
	}
	if len(p.AllowedTools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(p.AllowedTools))
	}
	if p.MemoryMB != 512 {
		t.Errorf("expected memory 512, got %d", p.MemoryMB)
	}
}

// TestInstanceStore_ModelDefaults validates SandboxInstance field defaults.
func TestInstanceStore_ModelDefaults(t *testing.T) {
	inst := SandboxInstance{
		TenantID: "tenant-1",
		ToolName: "echo",
		Status:   "pending",
	}
	if inst.ID != uuid.Nil {
		t.Errorf("expected zero UUID, got %s", inst.ID)
	}
	if inst.AgentID != nil {
		t.Errorf("expected nil agent_id, got %s", *inst.AgentID)
	}
	if inst.ExitCode != nil {
		t.Errorf("expected nil exit_code, got %d", *inst.ExitCode)
	}
	if inst.Stdout != nil {
		t.Errorf("expected nil stdout, got %s", *inst.Stdout)
	}
	if inst.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", inst.Status)
	}
}

// TestInstanceStore_ModelWithResult validates SandboxInstance with execution results.
func TestInstanceStore_ModelWithResult(t *testing.T) {
	exitCode := 0
	stdout := "output data"
	stderr := ""
	cpuMs := 50
	memMB := 32

	inst := SandboxInstance{
		TenantID:     "tenant-1",
		AgentID:      ptrStr("agent-1"),
		ProfileID:    uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		ToolName:     "bash",
		InputData:    ptrStr("ls -la"),
		ExitCode:     &exitCode,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Status:       "completed",
		CPUMs:        &cpuMs,
		MemoryPeakMB: &memMB,
	}

	if *inst.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", *inst.ExitCode)
	}
	if *inst.Stdout != "output data" {
		t.Errorf("expected stdout 'output data', got '%s'", *inst.Stdout)
	}
	if *inst.CPUMs != 50 {
		t.Errorf("expected cpu 50ms, got %d", *inst.CPUMs)
	}
	if *inst.MemoryPeakMB != 32 {
		t.Errorf("expected memory 32MB, got %d", *inst.MemoryPeakMB)
	}
	if inst.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", inst.Status)
	}
}

// TestInstanceStore_ModelTimeout validates SandboxInstance with timeout status.
func TestInstanceStore_ModelTimeout(t *testing.T) {
	errMsg := "execution timed out"
	status := "timeout"

	inst := SandboxInstance{
		TenantID:     "tenant-1",
		ToolName:     "python3",
		Status:       status,
		ErrorMessage: &errMsg,
	}

	if inst.Status != "timeout" {
		t.Errorf("expected status 'timeout', got '%s'", inst.Status)
	}
	if *inst.ErrorMessage != "execution timed out" {
		t.Errorf("expected error 'execution timed out', got '%s'", *inst.ErrorMessage)
	}
}

// TestInstanceStore_ModelPolicyDenied validates SandboxInstance with policy_denied status.
func TestInstanceStore_ModelPolicyDenied(t *testing.T) {
	errMsg := "tool not permitted by policy"
	status := "policy_denied"

	inst := SandboxInstance{
		TenantID:     "tenant-1",
		AgentID:      ptrStr("agent-42"),
		ToolName:     "rm",
		Status:       status,
		ErrorMessage: &errMsg,
	}

	if inst.Status != "policy_denied" {
		t.Errorf("expected status 'policy_denied', got '%s'", inst.Status)
	}
	if *inst.ErrorMessage != "tool not permitted by policy" {
		t.Errorf("expected error 'tool not permitted by policy', got '%s'", *inst.ErrorMessage)
	}
}

// TestStore_ErrNotFound verifies ErrNotFound sentinel exists.
func TestStore_ErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Error("ErrNotFound should not be nil")
	}
	if ErrNotFound.Error() != "record not found" {
		t.Errorf("expected 'record not found', got '%s'", ErrNotFound.Error())
	}
}