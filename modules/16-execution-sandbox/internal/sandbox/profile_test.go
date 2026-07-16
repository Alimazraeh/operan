package sandbox

import (
	"testing"
)

func TestValidateProfile_Valid(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 256, TimeoutSeconds: 60, MaxFileSizeMB: 1, MaxOutputSizeKB: 1}
	err := ValidateProfile(p)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateProfile_EmptyName(t *testing.T) {
	p := SandboxProfile{Name: "", MemoryMB: 256, TimeoutSeconds: 60}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateProfile_MemoryTooLow(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 8, TimeoutSeconds: 60}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for memory below 16")
	}
}

func TestValidateProfile_MemoryTooHigh(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 70000, TimeoutSeconds: 60}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for memory above 65536")
	}
}

func TestValidateProfile_TimeoutTooLow(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 256, TimeoutSeconds: 0}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for timeout below 1")
	}
}

func TestValidateProfile_TimeoutTooHigh(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 256, TimeoutSeconds: 4000}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for timeout above 3600")
	}
}

func TestValidateProfile_MaxFileSizeTooLow(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 256, TimeoutSeconds: 60, MaxFileSizeMB: 0}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for max_file_size_mb below 1")
	}
}

func TestValidateProfile_MaxOutputSizeTooLow(t *testing.T) {
	p := SandboxProfile{Name: "test", MemoryMB: 256, TimeoutSeconds: 60, MaxOutputSizeKB: 0}
	err := ValidateProfile(p)
	if err == nil {
		t.Error("expected error for max_output_size_kb below 1")
	}
}

func TestValidateProfile_MinimumValid(t *testing.T) {
	p := SandboxProfile{Name: "min", MemoryMB: 16, TimeoutSeconds: 1, MaxFileSizeMB: 1, MaxOutputSizeKB: 1}
	err := ValidateProfile(p)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestIsToolAllowed_AllowAll(t *testing.T) {
	p := SandboxProfile{AllowedTools: []string{}}
	if !IsToolAllowed(p, "anything") {
		t.Error("expected true for empty allowed_tools (permissive)")
	}
}

func TestIsToolAllowed_ExplicitAllow(t *testing.T) {
	p := SandboxProfile{AllowedTools: []string{"echo", "bash"}}
	if !IsToolAllowed(p, "echo") {
		t.Error("expected echo to be allowed")
	}
	if !IsToolAllowed(p, "bash") {
		t.Error("expected bash to be allowed")
	}
}

func TestIsToolAllowed_NotAllowed(t *testing.T) {
	p := SandboxProfile{AllowedTools: []string{"echo", "bash"}}
	if IsToolAllowed(p, "rm") {
		t.Error("expected rm to not be allowed")
	}
	if IsToolAllowed(p, "curl") {
		t.Error("expected curl to not be allowed")
	}
}

func TestIsToolAllowed_SingleTool(t *testing.T) {
	p := SandboxProfile{AllowedTools: []string{"python3"}}
	if !IsToolAllowed(p, "python3") {
		t.Error("expected python3 to be allowed")
	}
	if IsToolAllowed(p, "bash") {
		t.Error("expected bash to not be allowed")
	}
}